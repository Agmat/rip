package api

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pricing is the value summary attached to a pack open. Prices are
// Cardmarket "trend" figures in EUR as of PricedAt (the last refresh), not
// as of the open - see internal/prices.
type pricing struct {
	PackPriceEUR  *float64   `json:"pack_price_eur"`  // sealed booster; nil if unpriced
	TotalValueEUR float64    `json:"total_value_eur"` // sum of priced picks, 2 decimals
	UnpricedCards int        `json:"unpriced_cards"`  // picks excluded from the total
	PricedAt      *time.Time `json:"priced_at"`       // nil if never refreshed
}

// pickPrice is the effective price of one pick: the foil price for a foil
// pick when Cardmarket has one, otherwise the non-foil trend. nil when the
// card has no price at all - callers must not treat that as 0.
func pickPrice(foil bool, price, foilPrice pgtype.Float8) *float64 {
	if foil && foilPrice.Valid {
		return &foilPrice.Float64
	}
	if price.Valid {
		return &price.Float64
	}
	return nil
}

// summarize totals the priced picks and pairs them with the pack's price.
func summarize(packPrice pgtype.Float8, pricedAt pgtype.Timestamptz, cards []cardPick) pricing {
	var p pricing
	if packPrice.Valid {
		v := round2(packPrice.Float64)
		p.PackPriceEUR = &v
	}
	if pricedAt.Valid {
		t := pricedAt.Time
		p.PricedAt = &t
	}
	var total float64
	for _, c := range cards {
		if c.PriceEUR == nil {
			p.UnpricedCards++
			continue
		}
		total += *c.PriceEUR
	}
	p.TotalValueEUR = round2(total)
	return p
}

// round2 rounds to cents so float sums like 0.1+0.2 don't leak
// 0.30000000000000004 into the JSON.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
