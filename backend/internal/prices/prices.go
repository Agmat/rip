// Package prices copies Cardmarket trend prices into the database. It is
// the one code path shared by cmd/prices (manual refresh), cmd/import
// (price a freshly imported set) and, later, a periodic refresh inside
// the API server - so there is exactly one definition of "refresh prices".
package prices

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Agmat/rip/backend/internal/cardmarket"
	"github.com/Agmat/rip/backend/internal/db"
)

// Stats reports what a Refresh touched. *Missing counts rows that have an
// mcm_id but no entry in the guide; they keep whatever price they had.
type Stats struct {
	Cards, CardsMissing int
	Sets, SetsMissing   int
}

// Refresh downloads the price guide once and updates every card and set
// that has an mcm_id, in a single transaction so readers never see a
// half-refreshed catalogue. Rows without an mcm_id are left alone.
func Refresh(ctx context.Context, pool *pgxpool.Pool, client *http.Client, guideURL string) (Stats, error) {
	guide, err := cardmarket.FetchPriceGuide(ctx, client, guideURL)
	if err != nil {
		return Stats{}, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	var st Stats

	cards, err := q.ListCardsWithMcmID(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("list cards: %w", err)
	}
	for _, c := range cards {
		trend, trendFoil, found := Lookup(guide, c.McmID)
		if !found {
			st.CardsMissing++
			continue
		}
		if err := q.UpdateCardPrice(ctx, db.UpdateCardPriceParams{
			ID: c.ID, PriceEur: trend, PriceFoilEur: trendFoil, PricedAt: now,
		}); err != nil {
			return Stats{}, fmt.Errorf("update card %s price: %w", c.ID, err)
		}
		st.Cards++
	}

	sets, err := q.ListSetsWithMcmID(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("list sets: %w", err)
	}
	for _, s := range sets {
		trend, _, found := Lookup(guide, s.McmID)
		if !found {
			st.SetsMissing++
			continue
		}
		if err := q.UpdateSetPackPrice(ctx, db.UpdateSetPackPriceParams{
			Code: s.Code, PackPriceEur: trend, PackPricedAt: now,
		}); err != nil {
			return Stats{}, fmt.Errorf("update set %s pack price: %w", s.Code, err)
		}
		st.Sets++
	}

	if err := tx.Commit(ctx); err != nil {
		return Stats{}, fmt.Errorf("commit transaction: %w", err)
	}
	return st, nil
}

// Lookup resolves an mcm_id to nullable trend prices. A 0 in the guide
// means "no figure" and is stored as NULL, so the API can tell "worth
// nothing" apart from "unknown". found is false when the id is NULL or
// absent from the guide.
func Lookup(guide map[int]cardmarket.Price, mcmID pgtype.Int4) (trend, trendFoil pgtype.Float8, found bool) {
	if !mcmID.Valid {
		return pgtype.Float8{}, pgtype.Float8{}, false
	}
	p, ok := guide[int(mcmID.Int32)]
	if !ok {
		return pgtype.Float8{}, pgtype.Float8{}, false
	}
	return nullableFloat(p.Trend), nullableFloat(p.TrendFoil), true
}

func nullableFloat(f float64) pgtype.Float8 {
	if f == 0 {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: f, Valid: true}
}
