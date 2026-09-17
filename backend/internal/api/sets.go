package api

import (
	"net/http"

	"github.com/Agmat/rip/backend/internal/httpx"
)

type setSummary struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	BoosterTypes []string `json:"booster_types"`
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
			sets = append(sets, setSummary{Code: row.SetCode, Name: row.SetName})
		}
		sets[i].BoosterTypes = append(sets[i].BoosterTypes, row.BoosterType)
	}
	if sets == nil {
		sets = []setSummary{} // {"sets":[]}, not {"sets":null}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"sets": sets})
}
