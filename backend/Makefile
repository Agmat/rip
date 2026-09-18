.PHONY: dev run prices test lint fmt migrate-up migrate-down sqlc-generate

DATABASE_URL ?= postgres://rip:rip@localhost:5432/rip?sslmode=disable

# Start Postgres (docker-compose.yml lives at the repo root).
dev:
	docker compose -f ../docker-compose.yml up -d

run:
	go run ./cmd/api

prices:
	go run ./cmd/prices

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run ./...

fmt:
	gofmt -l -w .

migrate-up:
	goose -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DATABASE_URL)" down

sqlc-generate:
	sqlc generate
