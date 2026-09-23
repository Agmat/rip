package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver, used only to run migrations
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/Agmat/rip/backend/internal/db"
	"github.com/Agmat/rip/backend/internal/mtgjson"
	"github.com/Agmat/rip/backend/internal/scryfall"
	"github.com/Agmat/rip/backend/migrations"
)

// newTestServer starts a real Postgres in a container, runs the actual
// migrations against it, seeds the same trimmed FDN/SPG/Scryfall fixtures
// the mtgjson and scryfall packages use, and returns a Server wired to it.
// Requires Docker; skipped under -short.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test requires Docker, skipped under -short")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("rip_test"),
		postgres.WithUsername("rip"),
		postgres.WithPassword("rip"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	runMigrations(t, connStr)

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	seedFixtures(t, ctx, pool)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(pool, logger)
}

func runMigrations(t *testing.T, connStr string) {
	t.Helper()

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("open migration connection: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}

// seedFixtures loads the same trimmed real-data fixtures internal/mtgjson
// and internal/scryfall test against, and imports them through the same
// db.Queries the real cmd/import uses - so this test exercises the actual
// write path, not a hand-rolled shortcut.
func seedFixtures(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	q := db.New(pool)

	fdnData, err := os.ReadFile("../mtgjson/testdata/fdn.json")
	if err != nil {
		t.Fatalf("read fdn fixture: %v", err)
	}
	spgData, err := os.ReadFile("../mtgjson/testdata/spg.json")
	if err != nil {
		t.Fatalf("read spg fixture: %v", err)
	}
	collectionData, err := os.ReadFile("../scryfall/testdata/collection.json")
	if err != nil {
		t.Fatalf("read scryfall fixture: %v", err)
	}

	fdnSet, err := mtgjson.ParseSetFile(fdnData)
	if err != nil {
		t.Fatalf("parse fdn fixture: %v", err)
	}
	spgSet, err := mtgjson.ParseSetFile(spgData)
	if err != nil {
		t.Fatalf("parse spg fixture: %v", err)
	}
	collection, err := scryfall.ParseCollection(collectionData)
	if err != nil {
		t.Fatalf("parse scryfall fixture: %v", err)
	}

	scryfallByID := make(map[string]scryfall.Card, len(collection.Data))
	for _, c := range collection.Data {
		scryfallByID[c.ID] = c
	}

	for _, sf := range []*mtgjson.SetFile{fdnSet, spgSet} {
		params := db.UpsertSetParams{Code: sf.Data.Code, Name: sf.Data.Name}
		if sf.Data.Code == "FDN" {
			params.McmID = pgtype.Int4{Int32: 781936, Valid: true}
		}
		if _, err := q.UpsertSet(ctx, params); err != nil {
			t.Fatalf("seed set %s: %v", sf.Data.Code, err)
		}
		for _, c := range sf.Data.Cards {
			sCard, ok := scryfallByID[c.Identifiers.ScryfallID]
			if !ok {
				t.Fatalf("fixture card %s has no matching scryfall fixture entry", c.Name)
			}
			imageURIs, err := sCard.ImageURIsJSON()
			if err != nil {
				t.Fatalf("seed card %s: %v", c.Name, err)
			}
			cardID := mustParseTestUUID(t, c.UUID)
			scryfallID := mustParseTestUUID(t, c.Identifiers.ScryfallID)

			if _, err := q.UpsertCard(ctx, db.UpsertCardParams{
				ID:              cardID,
				SetCode:         c.SetCode,
				Name:            c.Name,
				Rarity:          c.Rarity,
				CollectorNumber: c.Number,
				ScryfallID:      scryfallID,
				ImageUris:       imageURIs,
				Finishes:        c.Finishes,
			}); err != nil {
				t.Fatalf("seed card %s: %v", c.Name, err)
			}
		}
	}

	boosterCfg := fdnSet.Data.Booster["play"]
	configJSON, err := json.Marshal(boosterCfg)
	if err != nil {
		t.Fatalf("marshal fixture booster config: %v", err)
	}
	if _, err := q.InsertBoosterConfig(ctx, db.InsertBoosterConfigParams{
		SetCode:     fdnSet.Data.Code,
		BoosterType: "play",
		Version:     1,
		Config:      configJSON,
	}); err != nil {
		t.Fatalf("seed booster config: %v", err)
	}

	// Price the set and the one fixture card that has an mcm_id, the way
	// prices.Refresh would, so pricing assertions have something to see.
	pricedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if err := q.UpdateSetPackPrices(ctx, db.UpdateSetPackPricesParams{
		Codes: []string{"FDN"}, Prices: []float64{4.36}, PricedAt: pricedAt,
	}); err != nil {
		t.Fatalf("seed pack price: %v", err)
	}
	if err := q.UpdateCardPrices(ctx, db.UpdateCardPricesParams{
		Ids:        []pgtype.UUID{mustParseTestUUID(t, "01a67c48-2ba1-55ba-aee9-8f0ad4ccf5c9")}, // Ruby, Daring Tracker
		Prices:     []float64{0.25},
		FoilPrices: []float64{1.5},
		PricedAt:   pricedAt,
	}); err != nil {
		t.Fatalf("seed card price: %v", err)
	}
}

func mustParseTestUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	id, err := parseUUID(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}
