// Package api holds the v1 HTTP routes: parse the request, call the draw
// engine and/or db queries, write the response. No business logic beyond
// that orchestration lives here - the draw itself is internal/booster, and
// the data access is the generated internal/db queries.
package api

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Agmat/rip/backend/internal/db"
	"github.com/Agmat/rip/backend/internal/httpx"
)

// Server holds the dependencies every handler needs.
type Server struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *slog.Logger
}

// NewServer builds a Server backed by pool. pool is also used directly (not
// just through queries) so write handlers can start their own transaction.
func NewServer(pool *pgxpool.Pool, logger *slog.Logger) *Server {
	return &Server{pool: pool, queries: db.New(pool), logger: logger}
}

// Routes returns the full v1 API plus /healthz, ready to wrap in middleware.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /v1/sets", s.handleListSets)
	mux.HandleFunc("POST /v1/packs/open", s.handleOpenPack)
	mux.HandleFunc("GET /v1/packs/{id}", s.handleGetPack)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
