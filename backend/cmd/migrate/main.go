// Command migrate applies pending goose migrations from the embedded SQL
// files. It exists so the deployed image can migrate itself (Fly release
// command) without shipping the goose CLI; locally `make migrate-up` does
// the same thing.
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"github.com/Agmat/rip/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Direct URL on purpose: DDL and goose's advisory lock need a real
	// session, which Neon's pooler doesn't guarantee.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	return goose.Up(db, ".")
}
