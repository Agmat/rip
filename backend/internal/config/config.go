// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
)

// Config holds the API server's runtime settings, read once at startup.
type Config struct {
	Port          string
	DatabaseURL   string
	LogLevel      string
	AllowedOrigin string
}

// Load reads Config from the environment. DATABASE_URL is required; PORT and
// LOG_LEVEL fall back to sane defaults so a fresh checkout only needs Postgres
// to run.
func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return Config{
		Port:          getenvDefault("PORT", "8080"),
		DatabaseURL:   dbURL,
		LogLevel:      getenvDefault("LOG_LEVEL", "info"),
		AllowedOrigin: getenvDefault("ALLOWED_ORIGIN", "http://localhost:3000"),
	}, nil
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
