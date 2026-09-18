// Command import loads one MTG set's play booster into the database:
// booster composition from MTGJSON, card images from Scryfall. Re-running
// it for a set that's already imported creates a new booster_configs
// version rather than mutating the old one, and upserts card rows in place.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Agmat/rip/backend/internal/config"
	"github.com/Agmat/rip/backend/internal/db"
	"github.com/Agmat/rip/backend/internal/mtgjson"
	"github.com/Agmat/rip/backend/internal/scryfall"
)

// boosterType is the only booster product rip currently opens. Collector,
// prerelease, etc. can be added by parameterizing this when they're needed.
const boosterType = "play"

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("import failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: import <SET_CODE>")
	}
	setCode := args[0]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}

	imp := &importer{httpClient: httpClient}
	return imp.run(ctx, pool, setCode)
}

type importer struct {
	httpClient *http.Client
}

func (imp *importer) run(ctx context.Context, pool *pgxpool.Pool, setCode string) error {
	primary, err := mtgjson.FetchSet(ctx, imp.httpClient, mtgjson.DefaultBaseURL, setCode)
	if err != nil {
		return err
	}

	boosterCfg, ok := primary.Data.Booster[boosterType]
	if !ok {
		return fmt.Errorf("set %s has no %q booster", setCode, boosterType)
	}

	var packImage []byte
	if packImgURL, ok := packImageURL(primary, boosterType); !ok {
		slog.Warn("no TCGplayer pack image found, importing without one", "set", setCode, "booster_type", boosterType)
	} else if img, err := imp.fetchPackImage(ctx, packImgURL); err != nil {
		// Decorative art must never block getting the cards in - any
		// failure here (fetch, decode, process) is a warning, not an error.
		slog.Warn("failed to fetch/process pack image, importing without one", "set", setCode, "error", err)
	} else {
		packImage = img
	}

	setFiles, err := imp.fetchSourceSets(ctx, primary, boosterCfg)
	if err != nil {
		return err
	}

	cardsByUUID := make(map[string]mtgjson.Card)
	for _, sf := range setFiles {
		for _, c := range sf.Data.Cards {
			cardsByUUID[c.UUID] = c
		}
	}

	neededUUIDs := sheetCardUUIDs(boosterCfg)
	slog.Info("booster config parsed", "set", setCode, "sheets", len(boosterCfg.Sheets), "cards_needed", len(neededUUIDs))

	scryfallIDs := make([]string, 0, len(neededUUIDs))
	for _, uuid := range neededUUIDs {
		card, ok := cardsByUUID[uuid]
		if !ok {
			return fmt.Errorf("card %s referenced by booster sheet but not found in any fetched set", uuid)
		}
		scryfallIDs = append(scryfallIDs, card.Identifiers.ScryfallID)
	}

	scryfallCards, err := scryfall.FetchCollection(ctx, imp.httpClient, scryfall.DefaultBaseURL, scryfallIDs)
	if err != nil {
		return fmt.Errorf("fetch card images: %w", err)
	}
	scryfallByID := make(map[string]scryfall.Card, len(scryfallCards))
	for _, c := range scryfallCards {
		scryfallByID[c.ID] = c
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	if err := upsertSets(ctx, q, setFiles, primary.Data.Code, packImage); err != nil {
		return err
	}
	if err := upsertCards(ctx, q, neededUUIDs, cardsByUUID, scryfallByID); err != nil {
		return err
	}
	version, err := activateNewBoosterConfig(ctx, q, setCode, boosterCfg)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	slog.Info("import complete", "set", setCode, "booster_type", boosterType, "version", version, "cards", len(neededUUIDs))
	return nil
}

// fetchSourceSets returns the primary set file plus every other set
// referenced by the booster's sourceSetCodes (e.g. FDN's play booster also
// draws from SPG). The primary is not re-fetched.
func (imp *importer) fetchSourceSets(ctx context.Context, primary *mtgjson.SetFile, boosterCfg mtgjson.BoosterConfig) ([]*mtgjson.SetFile, error) {
	setFiles := []*mtgjson.SetFile{primary}

	for _, code := range boosterCfg.SourceSetCodes {
		if code == primary.Data.Code {
			continue
		}
		sf, err := mtgjson.FetchSet(ctx, imp.httpClient, mtgjson.DefaultBaseURL, code)
		if err != nil {
			return nil, err
		}
		setFiles = append(setFiles, sf)
	}

	return setFiles, nil
}

// sheetCardUUIDs collects every distinct card uuid referenced across all of
// a booster config's sheets, regardless of which pack variant uses them.
func sheetCardUUIDs(cfg mtgjson.BoosterConfig) []string {
	seen := make(map[string]struct{})
	var uuids []string
	for _, sheet := range cfg.Sheets {
		for uuid := range sheet.Cards {
			if _, ok := seen[uuid]; !ok {
				seen[uuid] = struct{}{}
				uuids = append(uuids, uuid)
			}
		}
	}
	return uuids
}

// upsertSets stores every fetched set (the primary plus any booster source
// sets, e.g. FDN's play booster also drawing from SPG). Only the primary -
// the one whose pack is actually opened - gets a pack image; source sets
// aren't themselves an openable product.
func upsertSets(ctx context.Context, q *db.Queries, setFiles []*mtgjson.SetFile, primaryCode string, primaryPackImage []byte) error {
	for _, sf := range setFiles {
		params := db.UpsertSetParams{Code: sf.Data.Code, Name: sf.Data.Name}
		if sf.Data.Code == primaryCode {
			params.PackImage = primaryPackImage
		}
		if _, err := q.UpsertSet(ctx, params); err != nil {
			return fmt.Errorf("upsert set %s: %w", sf.Data.Code, err)
		}
	}
	return nil
}

// fetchPackImage downloads the product photo at url and removes its studio
// background (see packart.go).
func (imp *importer) fetchPackImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := imp.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return removeWhiteBackground(body)
}

// packImageURL finds the set's booster pack product (MTGJSON sealedProduct
// entry with category "booster_pack" and a matching subtype) and resolves
// it to the product's real photo via TCGplayer's public image CDN - neither
// MTGJSON nor Scryfall hosts pack art. false if the set has no such listing;
// callers should treat that as non-fatal (decorative, not draw-affecting).
func packImageURL(sf *mtgjson.SetFile, boosterType string) (string, bool) {
	for _, p := range sf.Data.SealedProduct {
		if p.Category == "booster_pack" && p.Subtype == boosterType && p.Identifiers.TCGplayerProductID != "" {
			return fmt.Sprintf("https://product-images.tcgplayer.com/fit-in/600x600/%s.jpg", p.Identifiers.TCGplayerProductID), true
		}
	}
	return "", false
}

func upsertCards(ctx context.Context, q *db.Queries, uuids []string, cardsByUUID map[string]mtgjson.Card, scryfallByID map[string]scryfall.Card) error {
	for _, uuid := range uuids {
		mCard := cardsByUUID[uuid]

		sCard, ok := scryfallByID[mCard.Identifiers.ScryfallID]
		if !ok {
			return fmt.Errorf("card %s (%s): no matching Scryfall card for id %s", mCard.Name, uuid, mCard.Identifiers.ScryfallID)
		}
		imageUris, err := sCard.ImageURIsJSON()
		if err != nil {
			return fmt.Errorf("card %s: %w", mCard.Name, err)
		}

		id, err := parseUUID(uuid)
		if err != nil {
			return fmt.Errorf("card %s: %w", mCard.Name, err)
		}
		scryfallID, err := parseUUID(mCard.Identifiers.ScryfallID)
		if err != nil {
			return fmt.Errorf("card %s: %w", mCard.Name, err)
		}

		_, err = q.UpsertCard(ctx, db.UpsertCardParams{
			ID:              id,
			SetCode:         mCard.SetCode,
			Name:            mCard.Name,
			Rarity:          mCard.Rarity,
			CollectorNumber: mCard.Number,
			ScryfallID:      scryfallID,
			ImageUris:       imageUris,
			Finishes:        mCard.Finishes,
		})
		if err != nil {
			return fmt.Errorf("upsert card %s: %w", mCard.Name, err)
		}
	}
	return nil
}

// activateNewBoosterConfig stores boosterCfg as the next version for
// (setCode, boosterType) and deactivates whichever version was active
// before, so exactly one version is ever active at a time.
func activateNewBoosterConfig(ctx context.Context, q *db.Queries, setCode string, boosterCfg mtgjson.BoosterConfig) (int32, error) {
	version, err := q.NextBoosterConfigVersion(ctx, db.NextBoosterConfigVersionParams{
		SetCode:     setCode,
		BoosterType: boosterType,
	})
	if err != nil {
		return 0, fmt.Errorf("compute next booster config version: %w", err)
	}

	if err := q.DeactivateBoosterConfigs(ctx, db.DeactivateBoosterConfigsParams{
		SetCode:     setCode,
		BoosterType: boosterType,
	}); err != nil {
		return 0, fmt.Errorf("deactivate old booster configs: %w", err)
	}

	configJSON, err := json.Marshal(boosterCfg)
	if err != nil {
		return 0, fmt.Errorf("marshal booster config: %w", err)
	}

	if _, err := q.InsertBoosterConfig(ctx, db.InsertBoosterConfigParams{
		SetCode:     setCode,
		BoosterType: boosterType,
		Version:     version,
		Config:      configJSON,
	}); err != nil {
		return 0, fmt.Errorf("insert booster config: %w", err)
	}

	return version, nil
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse uuid %q: %w", s, err)
	}
	return u, nil
}
