FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/ ./cmd/api ./cmd/migrate ./cmd/prices

# Distroless static ships CA certs, needed for Neon, Scryfall and Cardmarket TLS.
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/ /
# Fly release_command runs /migrate; the nightly machine runs /prices.
CMD ["/api"]
