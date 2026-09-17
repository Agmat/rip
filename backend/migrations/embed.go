// Package migrations embeds the SQL migration files so goose can run them
// programmatically (used by integration tests against a fresh testcontainers
// database) without depending on the goose CLI being present.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
