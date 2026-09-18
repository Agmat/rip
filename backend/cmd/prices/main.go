// Command prices refreshes Cardmarket prices for every imported card and
// set from Cardmarket's public daily price guide. Run it whenever prices
// should be brought up to date; cmd/import runs the same refresh once at
// the end of an import.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Agmat/rip/backend/internal/cardmarket"
	"github.com/Agmat/rip/backend/internal/config"
	"github.com/Agmat/rip/backend/internal/prices"
)

func main() {
	if err := run(); err != nil {
		slog.Error("price refresh failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// The guide is ~26 MB; give the download room on a slow link.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create db pool: %w", err)
	}
	defer pool.Close()

	st, err := prices.Refresh(ctx, pool, &http.Client{Timeout: 2 * time.Minute}, cardmarket.DefaultPriceGuideURL)
	if err != nil {
		return err
	}
	slog.Info("prices refreshed",
		"cards", st.Cards, "cards_missing", st.CardsMissing,
		"sets", st.Sets, "sets_missing", st.SetsMissing)
	return nil
}
