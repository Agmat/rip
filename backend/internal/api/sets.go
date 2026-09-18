package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Agmat/rip/backend/internal/httpx"
)

type setSummary struct {
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	PackImageURL *string    `json:"pack_image_url"`
	PackPriceEUR *float64   `json:"pack_price_eur"` // Cardmarket trend price of one sealed booster; nil if unpriced
	PackPricedAt *time.Time `json:"pack_priced_at"`
	BoosterTypes []string   `json:"booster_types"`
}

func (s *Server) handleListSets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.queries.ListActiveBoosterConfigsWithSetName(r.Context())
	if err != nil {
		s.logger.Error("list sets", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to list sets")
		return
	}

	// Rows are one per (set, booster_type); group into one entry per set. A
	// slice preserves the query's ORDER BY set_code so the response is
	// stable rather than following Go's randomized map iteration.
	var sets []setSummary
	index := make(map[string]int) // set_code -> index into sets
	for _, row := range rows {
		i, ok := index[row.SetCode]
		if !ok {
			i = len(sets)
			index[row.SetCode] = i
			sets = append(sets, setSummary{
				Code:         row.SetCode,
				Name:         row.SetName,
				PackImageURL: packImageURLPath(row.SetCode, row.PackImageHash),
				PackPriceEUR: nullableFloat(row.PackPriceEur),
				PackPricedAt: nullableTime(row.PackPricedAt),
			})
		}
		sets[i].BoosterTypes = append(sets[i].BoosterTypes, row.BoosterType)
	}
	if sets == nil {
		sets = []setSummary{} // {"sets":[]}, not {"sets":null}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"sets": sets})
}

// packImageURLPath builds the path clients fetch the set's pack photo from,
// or nil if the set has none. The hash is a cache-buster baked into the URL
// itself: re-importing a set with a different processed image changes the
// hash, so it's a new URL rather than a stale cached one - which is what
// lets the response be marked immutable.
func packImageURLPath(setCode, packImageHash string) *string {
	if packImageHash == "" {
		return nil
	}
	path := fmt.Sprintf("/v1/sets/%s/pack.png?v=%s", setCode, packImageHash)
	return &path
}

func (s *Server) handleGetSetPackImage(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	image, err := s.queries.GetSetPackImage(r.Context(), code)
	if errors.Is(err, pgx.ErrNoRows) || len(image) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "no pack image for this set")
		return
	}
	if err != nil {
		s.logger.Error("get set pack image", "error", err, "set", code)
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load pack image")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	// The URL's ?v= query param changes whenever the image bytes do, so
	// this is never stale in a way a browser can't already tell apart.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image)
}
