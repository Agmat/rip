package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Agmat/rip/backend/internal/booster"
	"github.com/Agmat/rip/backend/internal/db"
	"github.com/Agmat/rip/backend/internal/httpx"
	"github.com/Agmat/rip/backend/internal/mtgjson"
)

// defaultBoosterType is used when a request omits booster_type. "play" is
// the only booster type rip opens in v1.
const defaultBoosterType = "play"

type packOpenResponse struct {
	OpenID        pgtype.UUID        `json:"open_id"`
	SetCode       string             `json:"set_code"`
	BoosterType   string             `json:"booster_type"`
	ConfigVersion int32              `json:"config_version"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	Cards         []cardPick         `json:"cards"`
}

type cardPick struct {
	Slot      int32       `json:"slot"`
	SheetName string      `json:"sheet_name"`
	Foil      bool        `json:"foil"`
	Card      cardSummary `json:"card"`
}

type cardSummary struct {
	ID              pgtype.UUID     `json:"id"`
	Name            string          `json:"name"`
	Rarity          string          `json:"rarity"`
	CollectorNumber string          `json:"collector_number"`
	SetCode         string          `json:"set_code"`
	ImageURIs       json.RawMessage `json:"image_uris"`
}

func assemblePackResponse(po db.GetPackOpenRow, cardRows []db.ListPackOpenCardsRow) packOpenResponse {
	cards := make([]cardPick, len(cardRows))
	for i, row := range cardRows {
		cards[i] = cardPick{
			Slot:      row.Slot,
			SheetName: row.SheetName,
			Foil:      row.Foil,
			Card: cardSummary{
				ID:              row.CardID,
				Name:            row.Name,
				Rarity:          row.Rarity,
				CollectorNumber: row.CollectorNumber,
				SetCode:         row.SetCode,
				ImageURIs:       row.ImageUris,
			},
		}
	}
	return packOpenResponse{
		OpenID:        po.ID,
		SetCode:       po.SetCode,
		BoosterType:   po.BoosterType,
		ConfigVersion: po.ConfigVersion,
		CreatedAt:     po.CreatedAt,
		Cards:         cards,
	}
}

type openPackRequest struct {
	SetCode     string `json:"set_code"`
	BoosterType string `json:"booster_type"`
}

func (s *Server) handleOpenPack(w http.ResponseWriter, r *http.Request) {
	var req openPackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	if req.SetCode == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "set_code is required")
		return
	}
	if req.BoosterType == "" {
		req.BoosterType = defaultBoosterType
	}

	ctx := r.Context()

	activeConfig, err := s.queries.GetActiveBoosterConfig(ctx, db.GetActiveBoosterConfigParams{
		SetCode:     req.SetCode,
		BoosterType: req.BoosterType,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("no active %s booster for set %s", req.BoosterType, req.SetCode))
		return
	}
	if err != nil {
		s.logger.Error("get active booster config", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load booster config")
		return
	}

	var cfg mtgjson.BoosterConfig
	if err := json.Unmarshal(activeConfig.Config, &cfg); err != nil {
		s.logger.Error("decode booster config", "error", err, "booster_config_id", activeConfig.ID)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "stored booster config is corrupt")
		return
	}

	seed, err := booster.NewSeed()
	if err != nil {
		s.logger.Error("generate seed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to generate a seed")
		return
	}

	variantIndex, picks, err := booster.Open(cfg, seed)
	if err != nil {
		s.logger.Error("draw pack", "error", err, "booster_config_id", activeConfig.ID)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to draw a pack from the active config")
		return
	}

	packOpenID, err := s.insertPackOpen(ctx, activeConfig.ID, variantIndex, seed, picks)
	if err != nil {
		s.logger.Error("insert pack open", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to record the pack open")
		return
	}

	po, cardRows, err := s.fetchPackOpen(ctx, packOpenID)
	if err != nil {
		s.logger.Error("fetch pack open after insert", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "pack was opened but could not be loaded back")
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, assemblePackResponse(po, cardRows))
}

// insertPackOpen records a pack_opens row and its pack_open_cards in one
// transaction, so a failure partway through never leaves a pack_open with
// missing or partial cards.
func (s *Server) insertPackOpen(ctx context.Context, boosterConfigID int64, variantIndex int, seed [32]byte, picks []booster.Pick) (pgtype.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)

	po, err := q.InsertPackOpen(ctx, db.InsertPackOpenParams{
		BoosterConfigID:  boosterConfigID,
		PackVariantIndex: int32(variantIndex),
		Seed:             seed[:],
	})
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("insert pack_opens: %w", err)
	}

	for _, p := range picks {
		cardID, err := parseUUID(p.CardUUID)
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("pick card id: %w", err)
		}
		if err := q.InsertPackOpenCard(ctx, db.InsertPackOpenCardParams{
			PackOpenID: po.ID,
			Slot:       int32(p.Slot),
			SheetName:  p.SheetName,
			Foil:       p.Foil,
			CardID:     cardID,
		}); err != nil {
			return pgtype.UUID{}, fmt.Errorf("insert pack_open_cards (slot %d): %w", p.Slot, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return pgtype.UUID{}, fmt.Errorf("commit transaction: %w", err)
	}
	return po.ID, nil
}

func (s *Server) fetchPackOpen(ctx context.Context, id pgtype.UUID) (db.GetPackOpenRow, []db.ListPackOpenCardsRow, error) {
	po, err := s.queries.GetPackOpen(ctx, id)
	if err != nil {
		return db.GetPackOpenRow{}, nil, fmt.Errorf("get pack_opens: %w", err)
	}
	cardRows, err := s.queries.ListPackOpenCards(ctx, id)
	if err != nil {
		return db.GetPackOpenRow{}, nil, fmt.Errorf("list pack_open_cards: %w", err)
	}
	return po, cardRows, nil
}

func (s *Server) handleGetPack(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "id is not a valid UUID")
		return
	}

	ctx := r.Context()
	po, err := s.queries.GetPackOpen(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "pack open not found")
		return
	}
	if err != nil {
		s.logger.Error("get pack open", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load pack open")
		return
	}

	cardRows, err := s.queries.ListPackOpenCards(ctx, id)
	if err != nil {
		s.logger.Error("list pack open cards", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load pack open cards")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, assemblePackResponse(po, cardRows))
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parse uuid %q: %w", s, err)
	}
	return u, nil
}
