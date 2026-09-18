package prices

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Agmat/rip/backend/internal/cardmarket"
)

func TestLookup(t *testing.T) {
	guide := map[int]cardmarket.Price{
		796513: {Trend: 19.89, TrendFoil: 24.83},
		781936: {Trend: 4.36, TrendFoil: 0},
		42:     {Trend: 0, TrendFoil: 0},
	}

	tests := []struct {
		name          string
		mcmID         pgtype.Int4
		wantTrend     pgtype.Float8
		wantTrendFoil pgtype.Float8
		wantFound     bool
	}{
		{"both prices", pgtype.Int4{Int32: 796513, Valid: true}, pgtype.Float8{Float64: 19.89, Valid: true}, pgtype.Float8{Float64: 24.83, Valid: true}, true},
		{"foil 0 becomes null", pgtype.Int4{Int32: 781936, Valid: true}, pgtype.Float8{Float64: 4.36, Valid: true}, pgtype.Float8{}, true},
		{"trend 0 becomes null", pgtype.Int4{Int32: 42, Valid: true}, pgtype.Float8{}, pgtype.Float8{}, true},
		{"not in guide", pgtype.Int4{Int32: 1, Valid: true}, pgtype.Float8{}, pgtype.Float8{}, false},
		{"null mcm id", pgtype.Int4{}, pgtype.Float8{}, pgtype.Float8{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trend, trendFoil, found := Lookup(guide, tt.mcmID)
			if trend != tt.wantTrend || trendFoil != tt.wantTrendFoil || found != tt.wantFound {
				t.Errorf("Lookup = (%+v, %+v, %v), want (%+v, %+v, %v)",
					trend, trendFoil, found, tt.wantTrend, tt.wantTrendFoil, tt.wantFound)
			}
		})
	}
}
