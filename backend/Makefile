.PHONY: dev run test lint fmt

# Start Postgres (docker-compose.yml lives at the repo root).
dev:
	docker compose -f ../docker-compose.yml up -d

run:
	go run ./cmd/api

test:
	go test -race ./...

lint:
	go vet ./...
	golangci-lint run ./...

fmt:
	gofmt -l -w .
