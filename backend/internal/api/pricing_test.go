package api

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func f8(v float64) pgtype.Float8 { return pgtype.Float8{Float64: v, Valid: true} }

func TestPickPrice(t *testing.T) {
	tests := []struct {
		name      string
		foil      bool
		price     pgtype.Float8
		foilPrice pgtype.Float8
		want      *float64
	}{
		{"non-foil uses trend", false, f8(1.5), f8(9), ptr(1.5)},
		{"foil uses trend-foil", true, f8(1.5), f8(9), ptr(9)},
		{"foil falls back to trend when no foil price", true, f8(1.5), pgtype.Float8{}, ptr(1.5)},
		{"unpriced", false, pgtype.Float8{}, pgtype.Float8{}, nil},
		{"non-foil ignores foil-only price", false, pgtype.Float8{}, f8(9), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickPrice(tt.foil, tt.price, tt.foilPrice)
			switch {
			case got == nil && tt.want == nil:
			case got == nil || tt.want == nil || *got != *tt.want:
				t.Errorf("pickPrice = %v, want %v", deref(got), deref(tt.want))
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	cards := []cardPick{
		{PriceEUR: ptr(0.1)},
		{PriceEUR: ptr(0.2)},
		{PriceEUR: nil},
		{PriceEUR: ptr(12.345)},
	}
	got := summarize(f8(4.36), pgtype.Timestamptz{}, cards)

	if got.PackPriceEUR == nil || *got.PackPriceEUR != 4.36 {
		t.Errorf("PackPriceEUR = %v, want 4.36", deref(got.PackPriceEUR))
	}
	// 0.1 + 0.2 + 12.345 = 12.645 -> rounded to cents, and no 0.30000000000000004 leaks.
	if got.TotalValueEUR != 12.65 && got.TotalValueEUR != 12.64 {
		t.Errorf("TotalValueEUR = %v, want 12.64 or 12.65 (2 decimals)", got.TotalValueEUR)
	}
	if got.UnpricedCards != 1 {
		t.Errorf("UnpricedCards = %d, want 1", got.UnpricedCards)
	}
	if got.PricedAt != nil {
		t.Errorf("PricedAt = %v, want nil for invalid timestamp", got.PricedAt)
	}

	empty := summarize(pgtype.Float8{}, pgtype.Timestamptz{}, nil)
	if empty.PackPriceEUR != nil || empty.TotalValueEUR != 0 || empty.UnpricedCards != 0 {
		t.Errorf("empty summary = %+v, want zero values", empty)
	}
}

func ptr(f float64) *float64 { return &f }

func deref(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}
