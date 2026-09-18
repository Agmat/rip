# Cardmarket Pack Pricing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the Cardmarket price of a sealed booster before opening, the Cardmarket price of every card pulled, and a "pulled €X from a €Y pack" summary after opening.

**Architecture:** Cardmarket publishes a daily public price guide (`price_guide_1.json`, ~26 MB, no auth) keyed by `idProduct`. MTGJSON already gives us that id as `mcmId` for every card and for the sealed booster product. We store `mcm_id` at import, and a refresh function (`internal/prices.Refresh`) downloads the guide once and writes `trend` prices into columns on `cards` and `sets`. The API reads those columns, computes the pack summary server-side, and the frontend only renders.

**Tech Stack:** Go 1.22+ (net/http, pgx/v5, sqlc, goose), Postgres, Next.js/TypeScript/Tailwind.

**Spec:** Decisions were made in the brainstorming conversation on 2026-09-18 and are recorded in "Decisions" below; there is no separate spec file.

## Decisions (locked)

1. **Source:** Cardmarket public price guide `https://downloads.s3.cardmarket.com/productCatalog/priceGuide/price_guide_1.json`, joined on MTGJSON `identifiers.mcmId` (cards) and `sealedProduct[].identifiers.mcmId` (booster). No Scryfall prices, no scraping, no Cardmarket API.
2. **Refresh:** a `cmd/prices` command runs the refresh on demand. v2 (Task 11, specified but not built in this pass) adds an in-process `time.Ticker` in the API server.
3. **Figure:** `trend` for non-foil, `trend-foil` for foil picks, falling back to `trend` when `trend-foil` is null or 0. Only these two are stored.
4. **Storage:** columns on existing tables. `cards.mcm_id, price_eur, price_foil_eur, priced_at`; `sets.mcm_id, pack_price_eur, pack_priced_at`. No history table. Replays show today's value.
5. **Import** stores `mcm_id`s, then calls the same `prices.Refresh` at the end so a fresh set is priced immediately.
6. **Unpriced cards** (no `mcm_id` or not in the guide) are `null`, excluded from the total, and counted in `unpriced_cards`.
7. **API computes** the summary: `pricing: {pack_price_eur, total_value_eur, unpriced_cards, priced_at}` on the pack response; `price_eur` on each pick.
8. **Preview** on the pack picker shows only the price ("€4.36 on Cardmarket"). No age, no expected value.
9. **Summary UI:** "Pulled €12.40 from a €4.36 pack (+€8.04)", green when positive, red when negative, plus "N cards unpriced" when any.
10. **Money type:** `double precision` in Postgres (`pgtype.Float8` in Go, `*float64` in JSON), rounded to 2 decimals at the API boundary.

## Global Constraints

- Commit messages follow the repo style: imperative, no conventional-commit prefix (e.g. `Expose pack_image_url on GET /v1/sets`). End every commit with `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- After editing any `queries/*.sql` or `migrations/*.sql`, run `make sqlc-generate` from `backend/` and commit the regenerated `internal/db/*.go`.
- Run Go tests with `go test -race ./...` from `backend/`. Integration tests (`internal/api`) need Docker; `-short` skips them.
- Frontend: `pnpm lint` and `pnpm build` from `frontend/` must pass. `frontend/src/lib/types.ts` mirrors `backend/openapi.yaml` by hand — update both.
- Follow existing patterns: doc comments on every exported symbol explaining *why*; `httpx.WriteError` for errors; `slog` for logs.
- After the last task, run `graphify update .` from the repo root (project CLAUDE.md rule).

---

## File map

| File | Responsibility |
|---|---|
| `backend/migrations/00005_prices.sql` | new columns |
| `backend/queries/cards.sql`, `sets.sql`, `pack_opens.sql`, `booster_configs.sql` | store `mcm_id`, update/read prices |
| `backend/internal/mtgjson/mtgjson.go` | parse `mcmId` for cards and sealed products |
| `backend/internal/cardmarket/cardmarket.go` (+test, testdata) | fetch + parse the price guide |
| `backend/internal/prices/prices.go` (+test) | `Refresh`: guide → DB |
| `backend/cmd/prices/main.go` | CLI wrapper around `Refresh` |
| `backend/cmd/import/main.go` | store `mcm_id`, call `Refresh` |
| `backend/internal/api/packs.go`, `sets.go` (+tests) | expose prices and the summary |
| `backend/openapi.yaml`, `README.md`, `backend/.env.example` | docs |
| `frontend/src/lib/types.ts`, `money.ts`, `components/CardTile.tsx`, `PackOpener.tsx` | render |

---

### Task 1: Migration + queries for prices

**Files:**
- Create: `backend/migrations/00005_prices.sql`
- Modify: `backend/queries/sets.sql`, `backend/queries/cards.sql`, `backend/queries/pack_opens.sql`, `backend/queries/booster_configs.sql`
- Regenerate: `backend/internal/db/*.go`

**Interfaces:**
- Produces (sqlc-generated, in package `db`):
  - `UpsertSetParams` gains `McmID pgtype.Int4`
  - `UpsertCardParams` gains `McmID pgtype.Int4`
  - `ListSetsWithMcmID(ctx) ([]ListSetsWithMcmIDRow{Code string; McmID pgtype.Int4}, error)`
  - `ListCardsWithMcmID(ctx) ([]ListCardsWithMcmIDRow{ID pgtype.UUID; McmID pgtype.Int4}, error)`
  - `UpdateSetPackPrice(ctx, UpdateSetPackPriceParams{Code string; PackPriceEur pgtype.Float8; PackPricedAt pgtype.Timestamptz}) error`
  - `UpdateCardPrice(ctx, UpdateCardPriceParams{ID pgtype.UUID; PriceEur, PriceFoilEur pgtype.Float8; PricedAt pgtype.Timestamptz}) error`
  - `ListPackOpenCardsRow` gains `PriceEur, PriceFoilEur pgtype.Float8; PricedAt pgtype.Timestamptz`
  - `GetPackOpenRow` gains `PackPriceEur pgtype.Float8; PackPricedAt pgtype.Timestamptz`
  - `ListActiveBoosterConfigsWithSetNameRow` gains `PackPriceEur pgtype.Float8; PackPricedAt pgtype.Timestamptz`

- [ ] **Step 1: Write the migration**

`backend/migrations/00005_prices.sql`:

```sql
-- +goose Up
-- Cardmarket product ids (MTGJSON's mcmId) and the latest Cardmarket
-- "trend" prices, refreshed by cmd/prices from Cardmarket's public daily
-- price guide. Prices are nullable: a card with no mcm_id, or one the
-- guide doesn't list, simply has no price.
ALTER TABLE cards
    ADD COLUMN mcm_id         int,
    ADD COLUMN price_eur      double precision,
    ADD COLUMN price_foil_eur double precision,
    ADD COLUMN priced_at      timestamptz;

ALTER TABLE sets
    ADD COLUMN mcm_id         int,
    ADD COLUMN pack_price_eur double precision,
    ADD COLUMN pack_priced_at timestamptz;

-- +goose Down
ALTER TABLE sets DROP COLUMN pack_priced_at, DROP COLUMN pack_price_eur, DROP COLUMN mcm_id;
ALTER TABLE cards DROP COLUMN priced_at, DROP COLUMN price_foil_eur, DROP COLUMN price_eur, DROP COLUMN mcm_id;
```

- [ ] **Step 2: Update `backend/queries/sets.sql`**

Replace the whole file with:

```sql
-- name: UpsertSet :one
-- mcm_id is COALESCEd so re-importing a set as a *source* set (e.g. SPG
-- pulled in by FDN's booster, with no sealed product of its own) doesn't
-- wipe the id a previous primary import stored.
INSERT INTO sets (code, name, pack_image, mcm_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (code) DO UPDATE SET
    name       = EXCLUDED.name,
    pack_image = EXCLUDED.pack_image,
    mcm_id     = COALESCE(EXCLUDED.mcm_id, sets.mcm_id)
RETURNING *;

-- name: GetSetPackImage :one
SELECT pack_image FROM sets WHERE code = $1;

-- name: ListSetsWithMcmID :many
SELECT code, mcm_id FROM sets WHERE mcm_id IS NOT NULL;

-- name: UpdateSetPackPrice :exec
UPDATE sets SET pack_price_eur = $2, pack_priced_at = $3 WHERE code = $1;
```

- [ ] **Step 3: Update `backend/queries/cards.sql`**

Replace the whole file with:

```sql
-- name: UpsertCard :one
-- Price columns are deliberately not touched here: re-importing a set
-- refreshes card data, not prices (cmd/prices owns those).
INSERT INTO cards (id, set_code, name, rarity, collector_number, scryfall_id, image_uris, finishes, mcm_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET
    name             = EXCLUDED.name,
    rarity           = EXCLUDED.rarity,
    collector_number = EXCLUDED.collector_number,
    scryfall_id      = EXCLUDED.scryfall_id,
    image_uris       = EXCLUDED.image_uris,
    finishes         = EXCLUDED.finishes,
    mcm_id           = EXCLUDED.mcm_id
RETURNING *;

-- name: ListCardsWithMcmID :many
SELECT id, mcm_id FROM cards WHERE mcm_id IS NOT NULL;

-- name: UpdateCardPrice :exec
UPDATE cards SET price_eur = $2, price_foil_eur = $3, priced_at = $4 WHERE id = $1;
```

- [ ] **Step 4: Update `backend/queries/pack_opens.sql`**

Change `GetPackOpen` and `ListPackOpenCards` to:

```sql
-- name: GetPackOpen :one
SELECT po.id, po.created_at, bc.set_code, bc.booster_type, bc.version AS config_version,
       s.pack_price_eur, s.pack_priced_at
FROM pack_opens po
JOIN booster_configs bc ON bc.id = po.booster_config_id
JOIN sets s ON s.code = bc.set_code
WHERE po.id = $1;

-- name: ListPackOpenCards :many
SELECT poc.slot, poc.sheet_name, poc.foil,
       c.id AS card_id, c.name, c.rarity, c.collector_number, c.set_code, c.image_uris,
       c.price_eur, c.price_foil_eur, c.priced_at
FROM pack_open_cards poc
JOIN cards c ON c.id = poc.card_id
WHERE poc.pack_open_id = $1
ORDER BY poc.slot;
```

Leave `InsertPackOpen` and `InsertPackOpenCard` unchanged.

- [ ] **Step 5: Update `backend/queries/booster_configs.sql`**

Change only `ListActiveBoosterConfigsWithSetName`'s SELECT list:

```sql
SELECT bc.set_code, s.name AS set_name, COALESCE(md5(s.pack_image), '')::text AS pack_image_hash, bc.booster_type,
       s.pack_price_eur, s.pack_priced_at
```

- [ ] **Step 6: Regenerate and compile**

Run from `backend/`: `make sqlc-generate && go build ./...`

Expected: build fails in `cmd/import` and `internal/api/testhelper_test.go` only if positional params changed — they didn't (new params are named struct fields with zero-value = NULL), so expect a clean build. Then `go vet ./...` clean.

- [ ] **Step 7: Run existing tests**

Run: `go test -race ./...`
Expected: PASS (integration tests run the new migration against a real Postgres; `-short` if Docker is unavailable).

- [ ] **Step 8: Commit**

```bash
git add backend/migrations/00005_prices.sql backend/queries backend/internal/db
git commit -m "Schema: Cardmarket mcm_id and trend price columns on cards and sets

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 2: Parse `mcmId` from MTGJSON

**Files:**
- Modify: `backend/internal/mtgjson/mtgjson.go`
- Modify: `backend/internal/mtgjson/testdata/fdn.json`
- Test: `backend/internal/mtgjson/mtgjson_test.go`

**Interfaces:**
- Produces: `mtgjson.Identifiers.MCMID string` (json `mcmId`), `mtgjson.SealedProduct.Identifiers.MCMID string` (json `mcmId`). MTGJSON encodes these ids as JSON strings (`"781936"`); callers `strconv.Atoi`.

- [ ] **Step 1: Add fixture data**

In `backend/internal/mtgjson/testdata/fdn.json`, under `data`:
- Add `"mcmId": "796513"` to the `identifiers` object of the first card (`Ruby, Daring Tracker`, uuid `01a67c48-...`). Leave the other six cards without `mcmId` so "unpriced card" paths are exercised by the API fixtures later.
- Add a top-level `"sealedProduct"` array beside `"booster"`:

```json
"sealedProduct": [
  {
    "category": "booster_box",
    "subtype": "play",
    "identifiers": { "tcgplayerProductId": "562118", "mcmId": "781940" }
  },
  {
    "category": "booster_pack",
    "subtype": "play",
    "identifiers": { "tcgplayerProductId": "562116", "mcmId": "781936" }
  }
]
```

(These are the real FDN ids; the box entry is there so the lookup must filter on `booster_pack`.)

- [ ] **Step 2: Write the failing test**

Append to `backend/internal/mtgjson/mtgjson_test.go`:

```go
func TestParseSetFile_McmIDs(t *testing.T) {
	data, err := os.ReadFile("testdata/fdn.json")
	if err != nil {
		t.Fatal(err)
	}
	sf, err := ParseSetFile(data)
	if err != nil {
		t.Fatal(err)
	}

	if got := sf.Data.Cards[0].Identifiers.MCMID; got != "796513" {
		t.Errorf("cards[0].Identifiers.MCMID = %q, want 796513", got)
	}

	var pack *SealedProduct
	for i := range sf.Data.SealedProduct {
		if sf.Data.SealedProduct[i].Category == "booster_pack" {
			pack = &sf.Data.SealedProduct[i]
		}
	}
	if pack == nil {
		t.Fatal("no booster_pack sealed product in fixture")
	}
	if pack.Identifiers.MCMID != "781936" {
		t.Errorf("booster_pack MCMID = %q, want 781936", pack.Identifiers.MCMID)
	}
}
```

(Add `"os"` to the test file's imports if it isn't already there.)

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/mtgjson -run TestParseSetFile_McmIDs`
Expected: FAIL — `MCMID undefined`.

- [ ] **Step 4: Add the fields**

In `backend/internal/mtgjson/mtgjson.go`:

```go
// Identifiers holds cross-references to other card databases: the Scryfall
// id (joined against Scryfall's image data) and Cardmarket's product id
// (joined against Cardmarket's price guide). MTGJSON encodes mcmId as a
// string; it's absent for cards Cardmarket doesn't list.
type Identifiers struct {
	ScryfallID string `json:"scryfallId"`
	MCMID      string `json:"mcmId"`
}
```

and in `SealedProduct`:

```go
	Identifiers struct {
		TCGplayerProductID string `json:"tcgplayerProductId"`
		MCMID              string `json:"mcmId"`
	} `json:"identifiers"`
```

Update the `SealedProduct` doc comment's last sentence to: `Only enough to find "the play booster pack", its TCGplayer id (resolves to the product's real photo) and its Cardmarket id (resolves to its price).`

- [ ] **Step 5: Run tests**

Run: `go test ./internal/mtgjson`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/mtgjson
git commit -m "MTGJSON: parse Cardmarket mcmId for cards and sealed products

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 3: `internal/cardmarket` — fetch and parse the price guide

**Files:**
- Create: `backend/internal/cardmarket/cardmarket.go`
- Create: `backend/internal/cardmarket/testdata/price_guide.json`
- Test: `backend/internal/cardmarket/cardmarket_test.go`

**Interfaces:**
- Produces:
  - `const cardmarket.DefaultPriceGuideURL = "https://downloads.s3.cardmarket.com/productCatalog/priceGuide/price_guide_1.json"`
  - `type cardmarket.Price struct { Trend, TrendFoil float64 }` — `0` means "no price".
  - `func cardmarket.ParsePriceGuide(data []byte) (map[int]Price, error)` — keyed by `idProduct`.
  - `func cardmarket.FetchPriceGuide(ctx context.Context, client *http.Client, url string) (map[int]Price, error)`

- [ ] **Step 1: Write the fixture**

`backend/internal/cardmarket/testdata/price_guide.json` (trimmed real shape; `trend-foil` is `null` for the booster and a number for the card):

```json
{
  "version": 1,
  "createdAt": "2026-09-17T02:00:00+0200",
  "priceGuides": [
    {"idProduct": 781936, "idCategory": 2, "avg": 4.57, "low": 3.49, "trend": 4.36, "avg1": null, "avg7": null, "avg30": null, "avg-foil": null, "low-foil": null, "trend-foil": 0, "avg1-foil": null, "avg7-foil": null, "avg30-foil": null},
    {"idProduct": 796513, "idCategory": 1, "avg": 19.67, "low": 18, "trend": 19.89, "avg1": 21.3, "avg7": 19.9, "avg30": 19.61, "avg-foil": 25.21, "low-foil": 20, "trend-foil": 24.83, "avg1-foil": 27.5, "avg7-foil": 25.93, "avg30-foil": 25.89},
    {"idProduct": 1, "idCategory": 1, "avg": 0.09, "low": 0.02, "trend": null, "trend-foil": null}
  ]
}
```

- [ ] **Step 2: Write the failing tests**

`backend/internal/cardmarket/cardmarket_test.go`:

```go
package cardmarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParsePriceGuide(t *testing.T) {
	data, err := os.ReadFile("testdata/price_guide.json")
	if err != nil {
		t.Fatal(err)
	}
	guide, err := ParsePriceGuide(data)
	if err != nil {
		t.Fatal(err)
	}

	if p := guide[781936]; p.Trend != 4.36 || p.TrendFoil != 0 {
		t.Errorf("booster price = %+v, want Trend 4.36, TrendFoil 0", p)
	}
	if p := guide[796513]; p.Trend != 19.89 || p.TrendFoil != 24.83 {
		t.Errorf("card price = %+v, want Trend 19.89, TrendFoil 24.83", p)
	}
	// null trend decodes to 0, i.e. "no price", rather than failing the whole guide.
	if p := guide[1]; p.Trend != 0 {
		t.Errorf("null trend = %v, want 0", p.Trend)
	}
	if _, ok := guide[999999]; ok {
		t.Error("unknown product should be absent")
	}
}

func TestFetchPriceGuide(t *testing.T) {
	data, err := os.ReadFile("testdata/price_guide.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/guide.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	guide, err := FetchPriceGuide(context.Background(), srv.Client(), srv.URL+"/guide.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(guide) != 3 {
		t.Errorf("len(guide) = %d, want 3", len(guide))
	}

	if _, err := FetchPriceGuide(context.Background(), srv.Client(), srv.URL+"/missing.json"); err == nil {
		t.Error("expected error for 404, got nil")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/cardmarket`
Expected: FAIL — package has no non-test Go files / undefined symbols.

- [ ] **Step 4: Implement**

`backend/internal/cardmarket/cardmarket.go`:

```go
// Package cardmarket reads Cardmarket's public daily price guide. Cardmarket
// has no open API (the partner API is closed to new apps and the site is
// behind Cloudflare), but it publishes one JSON export per game per day
// with trend/avg/low prices for every product - singles and sealed alike -
// keyed by idProduct, which is the same id MTGJSON exposes as mcmId.
package cardmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultPriceGuideURL is the Magic (game id 1) price guide. Overridable so
// tests can point FetchPriceGuide at an httptest server.
const DefaultPriceGuideURL = "https://downloads.s3.cardmarket.com/productCatalog/priceGuide/price_guide_1.json"

// Price is the subset of a price guide entry rip uses: Cardmarket's
// headline "trend" figure for non-foil and foil. 0 means the guide has no
// figure (JSON null or 0), never a real price.
type Price struct {
	Trend     float64
	TrendFoil float64
}

// guideFile is the top-level shape of the export. Everything except trend
// and trend-foil is ignored at decode time.
type guideFile struct {
	PriceGuides []struct {
		IDProduct int      `json:"idProduct"`
		Trend     *float64 `json:"trend"`
		TrendFoil *float64 `json:"trend-foil"`
	} `json:"priceGuides"`
}

// ParsePriceGuide decodes a price guide export into a map keyed by
// Cardmarket product id.
func ParsePriceGuide(data []byte) (map[int]Price, error) {
	var gf guideFile
	if err := json.Unmarshal(data, &gf); err != nil {
		return nil, fmt.Errorf("decode price guide: %w", err)
	}
	guide := make(map[int]Price, len(gf.PriceGuides))
	for _, e := range gf.PriceGuides {
		guide[e.IDProduct] = Price{Trend: deref(e.Trend), TrendFoil: deref(e.TrendFoil)}
	}
	return guide, nil
}

func deref(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

// FetchPriceGuide downloads and decodes the price guide at url (normally
// DefaultPriceGuideURL). The file is ~26 MB; callers should fetch it once
// per refresh, not per product.
func FetchPriceGuide(ctx context.Context, client *http.Client, url string) (map[int]Price, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch price guide: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch price guide: unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read price guide: %w", err)
	}
	return ParsePriceGuide(body)
}
```

- [ ] **Step 5: Run tests**

Run: `go test -race ./internal/cardmarket`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/cardmarket
git commit -m "Add cardmarket package: fetch and parse the public price guide

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 4: `internal/prices.Refresh` — guide → DB

**Files:**
- Create: `backend/internal/prices/prices.go`
- Test: `backend/internal/prices/prices_test.go`

**Interfaces:**
- Consumes: `cardmarket.FetchPriceGuide`, `cardmarket.Price`; `db.ListCardsWithMcmID`, `db.UpdateCardPrice`, `db.ListSetsWithMcmID`, `db.UpdateSetPackPrice` (Task 1).
- Produces:
  - `type prices.Stats struct { Cards, CardsMissing, Sets, SetsMissing int }`
  - `func prices.Refresh(ctx context.Context, pool *pgxpool.Pool, client *http.Client, guideURL string) (Stats, error)`
  - `func prices.Lookup(guide map[int]cardmarket.Price, mcmID pgtype.Int4) (trend, trendFoil pgtype.Float8, found bool)` — pure, unit-tested.

- [ ] **Step 1: Write the failing test for `Lookup`**

`backend/internal/prices/prices_test.go`:

```go
package prices

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Agmat/rip/backend/internal/cardmarket"
)

func TestLookup(t *testing.T) {
	guide := map[int]cardmarket.Price{
		796513: {Trend: 19.89, TrendFoil: 24.83},
		781936: {Trend: 4.36, TrendFoil: 0},
		42:     {Trend: 0, TrendFoil: 0},
	}

	tests := []struct {
		name          string
		mcmID         pgtype.Int4
		wantTrend     pgtype.Float8
		wantTrendFoil pgtype.Float8
		wantFound     bool
	}{
		{"both prices", pgtype.Int4{Int32: 796513, Valid: true}, pgtype.Float8{Float64: 19.89, Valid: true}, pgtype.Float8{Float64: 24.83, Valid: true}, true},
		{"foil 0 becomes null", pgtype.Int4{Int32: 781936, Valid: true}, pgtype.Float8{Float64: 4.36, Valid: true}, pgtype.Float8{}, true},
		{"trend 0 becomes null", pgtype.Int4{Int32: 42, Valid: true}, pgtype.Float8{}, pgtype.Float8{}, true},
		{"not in guide", pgtype.Int4{Int32: 1, Valid: true}, pgtype.Float8{}, pgtype.Float8{}, false},
		{"null mcm id", pgtype.Int4{}, pgtype.Float8{}, pgtype.Float8{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trend, trendFoil, found := Lookup(guide, tt.mcmID)
			if trend != tt.wantTrend || trendFoil != tt.wantTrendFoil || found != tt.wantFound {
				t.Errorf("Lookup = (%+v, %+v, %v), want (%+v, %+v, %v)",
					trend, trendFoil, found, tt.wantTrend, tt.wantTrendFoil, tt.wantFound)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/prices`
Expected: FAIL — no non-test Go files.

- [ ] **Step 3: Implement**

`backend/internal/prices/prices.go`:

```go
// Package prices copies Cardmarket trend prices into the database. It is
// the one code path shared by cmd/prices (manual refresh), cmd/import
// (price a freshly imported set) and, later, a periodic refresh inside
// the API server - so there is exactly one definition of "refresh prices".
package prices

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Agmat/rip/backend/internal/cardmarket"
	"github.com/Agmat/rip/backend/internal/db"
)

// Stats reports what a Refresh touched. *Missing counts rows that have an
// mcm_id but no entry in the guide; they keep whatever price they had.
type Stats struct {
	Cards, CardsMissing int
	Sets, SetsMissing   int
}

// Refresh downloads the price guide once and updates every card and set
// that has an mcm_id, in a single transaction so readers never see a
// half-refreshed catalogue. Rows without an mcm_id are left alone.
func Refresh(ctx context.Context, pool *pgxpool.Pool, client *http.Client, guideURL string) (Stats, error) {
	guide, err := cardmarket.FetchPriceGuide(ctx, client, guideURL)
	if err != nil {
		return Stats{}, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := db.New(tx)
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	var st Stats

	cards, err := q.ListCardsWithMcmID(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("list cards: %w", err)
	}
	for _, c := range cards {
		trend, trendFoil, found := Lookup(guide, c.McmID)
		if !found {
			st.CardsMissing++
			continue
		}
		if err := q.UpdateCardPrice(ctx, db.UpdateCardPriceParams{
			ID: c.ID, PriceEur: trend, PriceFoilEur: trendFoil, PricedAt: now,
		}); err != nil {
			return Stats{}, fmt.Errorf("update card %s price: %w", c.ID, err)
		}
		st.Cards++
	}

	sets, err := q.ListSetsWithMcmID(ctx)
	if err != nil {
		return Stats{}, fmt.Errorf("list sets: %w", err)
	}
	for _, s := range sets {
		trend, _, found := Lookup(guide, s.McmID)
		if !found {
			st.SetsMissing++
			continue
		}
		if err := q.UpdateSetPackPrice(ctx, db.UpdateSetPackPriceParams{
			Code: s.Code, PackPriceEur: trend, PackPricedAt: now,
		}); err != nil {
			return Stats{}, fmt.Errorf("update set %s pack price: %w", s.Code, err)
		}
		st.Sets++
	}

	if err := tx.Commit(ctx); err != nil {
		return Stats{}, fmt.Errorf("commit transaction: %w", err)
	}
	return st, nil
}

// Lookup resolves an mcm_id to nullable trend prices. A 0 in the guide
// means "no figure" and is stored as NULL, so the API can tell "worth
// nothing" apart from "unknown". found is false when the id is NULL or
// absent from the guide.
func Lookup(guide map[int]cardmarket.Price, mcmID pgtype.Int4) (trend, trendFoil pgtype.Float8, found bool) {
	if !mcmID.Valid {
		return pgtype.Float8{}, pgtype.Float8{}, false
	}
	p, ok := guide[int(mcmID.Int32)]
	if !ok {
		return pgtype.Float8{}, pgtype.Float8{}, false
	}
	return nullableFloat(p.Trend), nullableFloat(p.TrendFoil), true
}

func nullableFloat(f float64) pgtype.Float8 {
	if f == 0 {
		return pgtype.Float8{}
	}
	return pgtype.Float8{Float64: f, Valid: true}
}
```

- [ ] **Step 4: Run tests**

Run: `go test -race ./internal/prices && go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/prices
git commit -m "Add prices.Refresh: write Cardmarket trend prices to cards and sets

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 5: `cmd/prices` command + docs

**Files:**
- Create: `backend/cmd/prices/main.go`
- Modify: `backend/Makefile`, `README.md`

**Interfaces:**
- Consumes: `prices.Refresh` (Task 4), `config.Load`.

- [ ] **Step 1: Write the command**

`backend/cmd/prices/main.go`:

```go
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
```

- [ ] **Step 2: Add a Makefile target**

In `backend/Makefile`, add `prices` to `.PHONY` and:

```make
prices:
	go run ./cmd/prices
```

- [ ] **Step 3: Document in README**

In `README.md`, after step **2. Import a set**, add:

```markdown
**2b. Refresh prices** (optional — `import` already prices a set once; re-run whenever you want
current Cardmarket prices)

```bash
make prices
```

Prices come from Cardmarket's public daily price guide, joined on the Cardmarket product ids
MTGJSON provides. Cards Cardmarket doesn't list stay unpriced.
```

And in the **Card data** section append: `Card and sealed-booster prices are Cardmarket "trend" prices from Cardmarket's public price guide, refreshed by `make prices`.`

- [ ] **Step 4: Build and smoke-run**

Run from `backend/`: `go build ./... && go vet ./...`
Expected: clean.

If a local Postgres with the migration applied is running: `set -a && source .env && set +a && make prices` — expect a log line `prices refreshed cards=0 ... sets=0` (no mcm_ids stored yet; that's Task 6).

- [ ] **Step 5: Commit**

```bash
git add backend/cmd/prices backend/Makefile README.md
git commit -m "Add cmd/prices: refresh Cardmarket prices on demand

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 6: Import stores `mcm_id`s and prices the set

**Files:**
- Modify: `backend/cmd/import/main.go`
- Test: `backend/cmd/import/main_test.go`

**Interfaces:**
- Consumes: `mtgjson.Identifiers.MCMID`, `mtgjson.SealedProduct.Identifiers.MCMID` (Task 2), `db.UpsertSetParams.McmID`, `db.UpsertCardParams.McmID` (Task 1), `prices.Refresh` (Task 4).
- Produces: `func packMCMID(sf *mtgjson.SetFile, boosterType string) pgtype.Int4`, `func mcmID(s string) pgtype.Int4`.

- [ ] **Step 1: Write the failing tests**

Append to `backend/cmd/import/main_test.go`:

```go
func TestMcmID(t *testing.T) {
	if got := mcmID("796513"); !got.Valid || got.Int32 != 796513 {
		t.Errorf("mcmID(\"796513\") = %+v, want valid 796513", got)
	}
	if got := mcmID(""); got.Valid {
		t.Errorf("mcmID(\"\") = %+v, want null", got)
	}
	if got := mcmID("abc"); got.Valid {
		t.Errorf("mcmID(\"abc\") = %+v, want null", got)
	}
}

func TestPackMCMID_PicksBoosterPackOfMatchingSubtype(t *testing.T) {
	var sf mtgjson.SetFile
	box := mtgjson.SealedProduct{Category: "booster_box", Subtype: "play"}
	box.Identifiers.MCMID = "781940"
	collector := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "collector"}
	collector.Identifiers.MCMID = "1"
	play := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "play"}
	play.Identifiers.MCMID = "781936"
	sf.Data.SealedProduct = []mtgjson.SealedProduct{box, collector, play}

	if got := packMCMID(&sf, "play"); !got.Valid || got.Int32 != 781936 {
		t.Errorf("packMCMID = %+v, want valid 781936", got)
	}
	if got := packMCMID(&sf, "draft"); got.Valid {
		t.Errorf("packMCMID(draft) = %+v, want null", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/import`
Expected: FAIL — `mcmID`, `packMCMID` undefined.

- [ ] **Step 3: Implement the helpers**

In `backend/cmd/import/main.go`, add `"strconv"` to imports and, next to `packImageURL`:

```go
// packMCMID finds the Cardmarket product id of the set's booster pack (same
// sealedProduct lookup as packImageURL), or NULL if the set has no such
// listing - the pack is then simply unpriced.
func packMCMID(sf *mtgjson.SetFile, boosterType string) pgtype.Int4 {
	for _, p := range sf.Data.SealedProduct {
		if p.Category == "booster_pack" && p.Subtype == boosterType {
			return mcmID(p.Identifiers.MCMID)
		}
	}
	return pgtype.Int4{}
}

// mcmID parses MTGJSON's string-encoded mcmId into a nullable int. Empty
// or malformed ids become NULL rather than failing the import: a missing
// price must never block getting the cards in.
func mcmID(s string) pgtype.Int4 {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}
}
```

- [ ] **Step 4: Wire `mcm_id` into the upserts**

In `upsertSets`, change the signature and body so the primary set gets its pack id:

```go
func upsertSets(ctx context.Context, q *db.Queries, setFiles []*mtgjson.SetFile, primaryCode string, primaryPackImage []byte, primaryMCMID pgtype.Int4) error {
	for _, sf := range setFiles {
		params := db.UpsertSetParams{Code: sf.Data.Code, Name: sf.Data.Name}
		if sf.Data.Code == primaryCode {
			params.PackImage = primaryPackImage
			params.McmID = primaryMCMID
		}
		...
```

Update its doc comment: `Only the primary ... gets a pack image and a Cardmarket product id; source sets aren't themselves an openable product.`

In `run`, change the call to `upsertSets(ctx, q, setFiles, primary.Data.Code, packImage, packMCMID(primary, boosterType))`.

In `upsertCards`, add `McmID: mcmID(mCard.Identifiers.MCMID),` to the `db.UpsertCardParams` literal.

- [ ] **Step 5: Price the set after the import commits**

In `run`, after the existing `slog.Info("import complete", ...)` line and before `return nil`:

```go
	// Price what was just imported. Non-fatal: the set is fully usable
	// unpriced, and `make prices` can be re-run at any time.
	st, err := prices.Refresh(ctx, pool, imp.httpClient, cardmarket.DefaultPriceGuideURL)
	if err != nil {
		slog.Warn("price refresh failed, set imported without prices", "set", setCode, "error", err)
		return nil
	}
	slog.Info("prices refreshed", "cards", st.Cards, "cards_missing", st.CardsMissing, "sets", st.Sets, "sets_missing", st.SetsMissing)
	return nil
```

Add imports `"github.com/Agmat/rip/backend/internal/cardmarket"` and `"github.com/Agmat/rip/backend/internal/prices"`. Bump the `context.WithTimeout` in `run()` from `2*time.Minute` to `5*time.Minute` (the guide download is ~26 MB). Update the package doc comment's first sentence to end with `..., card images from Scryfall, prices from Cardmarket.`

- [ ] **Step 6: Run tests and build**

Run: `go test -race ./cmd/import && go build ./... && go vet ./...`
Expected: PASS, clean.

- [ ] **Step 7: Manual verification (needs Postgres running)**

From `backend/`: `set -a && source .env && set +a && go run ./cmd/import FDN`
Expected log tail: `import complete ...` then `prices refreshed cards=<hundreds> cards_missing=<small> sets=1 sets_missing=0`.

Then: `psql "$DATABASE_URL" -c "select code, mcm_id, pack_price_eur, pack_priced_at from sets where code='FDN'"` — expect `781936`, a price around 4–5, and a recent timestamp. And `select count(*) from cards where price_eur is not null` — expect most of the set.

- [ ] **Step 8: Commit**

```bash
git add backend/cmd/import
git commit -m "Import: store Cardmarket ids and price the set on import

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 7: API — per-card price and pack pricing summary

**Files:**
- Modify: `backend/internal/api/packs.go`
- Create: `backend/internal/api/pricing.go`
- Test: `backend/internal/api/pricing_test.go` (pure, no Docker), `backend/internal/api/api_test.go` (integration)

**Interfaces:**
- Consumes: `db.ListPackOpenCardsRow.{PriceEur, PriceFoilEur, PricedAt}`, `db.GetPackOpenRow.{PackPriceEur, PackPricedAt}` (Task 1).
- Produces (JSON):
  - `cardPick.price_eur: number|null` — foil-aware effective price of that pick.
  - `packOpenResponse.pricing: { pack_price_eur: number|null, total_value_eur: number, unpriced_cards: integer, priced_at: date-time|null }`
- Go: `func pickPrice(foil bool, price, foilPrice pgtype.Float8) *float64`, `func summarize(packPrice pgtype.Float8, pricedAt pgtype.Timestamptz, cards []cardPick) pricing`, `func round2(f float64) float64`.

- [ ] **Step 1: Write the failing unit tests**

`backend/internal/api/pricing_test.go`:

```go
package api

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func f8(v float64) pgtype.Float8 { return pgtype.Float8{Float64: v, Valid: true} }

func TestPickPrice(t *testing.T) {
	tests := []struct {
		name       string
		foil       bool
		price      pgtype.Float8
		foilPrice  pgtype.Float8
		want       *float64
	}{
		{"non-foil uses trend", false, f8(1.5), f8(9), ptr(1.5)},
		{"foil uses trend-foil", true, f8(1.5), f8(9), ptr(9)},
		{"foil falls back to trend when no foil price", true, f8(1.5), pgtype.Float8{}, ptr(1.5)},
		{"unpriced", false, pgtype.Float8{}, pgtype.Float8{}, nil},
		{"non-foil ignores foil-only price", false, pgtype.Float8{}, f8(9), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickPrice(tt.foil, tt.price, tt.foilPrice)
			switch {
			case got == nil && tt.want == nil:
			case got == nil || tt.want == nil || *got != *tt.want:
				t.Errorf("pickPrice = %v, want %v", deref(got), deref(tt.want))
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	cards := []cardPick{
		{PriceEUR: ptr(0.1)},
		{PriceEUR: ptr(0.2)},
		{PriceEUR: nil},
		{PriceEUR: ptr(12.345)},
	}
	got := summarize(f8(4.36), pgtype.Timestamptz{}, cards)

	if got.PackPriceEUR == nil || *got.PackPriceEUR != 4.36 {
		t.Errorf("PackPriceEUR = %v, want 4.36", deref(got.PackPriceEUR))
	}
	// 0.1 + 0.2 + 12.345 = 12.645 -> rounded to cents, and no 0.30000000000000004 leaks.
	if got.TotalValueEUR != 12.65 && got.TotalValueEUR != 12.64 {
		t.Errorf("TotalValueEUR = %v, want 12.64 or 12.65 (2 decimals)", got.TotalValueEUR)
	}
	if got.UnpricedCards != 1 {
		t.Errorf("UnpricedCards = %d, want 1", got.UnpricedCards)
	}
	if got.PricedAt != nil {
		t.Errorf("PricedAt = %v, want nil for invalid timestamp", got.PricedAt)
	}

	empty := summarize(pgtype.Float8{}, pgtype.Timestamptz{}, nil)
	if empty.PackPriceEUR != nil || empty.TotalValueEUR != 0 || empty.UnpricedCards != 0 {
		t.Errorf("empty summary = %+v, want zero values", empty)
	}
}

func ptr(f float64) *float64 { return &f }

func deref(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -short ./internal/api -run 'TestPickPrice|TestSummarize'`
Expected: FAIL — `pickPrice`, `summarize`, `cardPick.PriceEUR` undefined.

- [ ] **Step 3: Implement `pricing.go`**

`backend/internal/api/pricing.go`:

```go
package api

import (
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pricing is the value summary attached to a pack open. Prices are
// Cardmarket "trend" figures in EUR as of PricedAt (the last refresh), not
// as of the open - see internal/prices.
type pricing struct {
	PackPriceEUR  *float64   `json:"pack_price_eur"`  // sealed booster; nil if unpriced
	TotalValueEUR float64    `json:"total_value_eur"` // sum of priced picks, 2 decimals
	UnpricedCards int        `json:"unpriced_cards"`  // picks excluded from the total
	PricedAt      *time.Time `json:"priced_at"`       // nil if never refreshed
}

// pickPrice is the effective price of one pick: the foil price for a foil
// pick when Cardmarket has one, otherwise the non-foil trend. nil when the
// card has no price at all - callers must not treat that as 0.
func pickPrice(foil bool, price, foilPrice pgtype.Float8) *float64 {
	if foil && foilPrice.Valid {
		return &foilPrice.Float64
	}
	if price.Valid {
		return &price.Float64
	}
	return nil
}

// summarize totals the priced picks and pairs them with the pack's price.
func summarize(packPrice pgtype.Float8, pricedAt pgtype.Timestamptz, cards []cardPick) pricing {
	var p pricing
	if packPrice.Valid {
		v := round2(packPrice.Float64)
		p.PackPriceEUR = &v
	}
	if pricedAt.Valid {
		t := pricedAt.Time
		p.PricedAt = &t
	}
	var total float64
	for _, c := range cards {
		if c.PriceEUR == nil {
			p.UnpricedCards++
			continue
		}
		total += *c.PriceEUR
	}
	p.TotalValueEUR = round2(total)
	return p
}

// round2 rounds to cents so float sums like 0.1+0.2 don't leak
// 0.30000000000000004 into the JSON.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
```

- [ ] **Step 4: Wire into `packs.go`**

In `backend/internal/api/packs.go`:

```go
type packOpenResponse struct {
	OpenID        pgtype.UUID        `json:"open_id"`
	SetCode       string             `json:"set_code"`
	BoosterType   string             `json:"booster_type"`
	ConfigVersion int32              `json:"config_version"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	Cards         []cardPick         `json:"cards"`
	Pricing       pricing            `json:"pricing"`
}

type cardPick struct {
	Slot      int32       `json:"slot"`
	SheetName string      `json:"sheet_name"`
	Foil      bool        `json:"foil"`
	PriceEUR  *float64    `json:"price_eur"` // effective price for this pick (foil-aware); nil if unpriced
	Card      cardSummary `json:"card"`
}
```

In `assemblePackResponse`, set `PriceEUR: pickPrice(row.Foil, row.PriceEur, row.PriceFoilEur),` in the `cardPick` literal, and add `Pricing: summarize(po.PackPriceEur, po.PackPricedAt, cards),` to the returned `packOpenResponse`.

`pricedAt` for the summary: cards and sets are refreshed in the same transaction, so the set's `pack_priced_at` is the refresh time for everything. (If the set has no mcm_id, `pack_priced_at` stays null even when cards are priced; acceptable — the preview shows no pack price in that case anyway.)

- [ ] **Step 5: Run unit tests**

Run: `go test -short -race ./internal/api`
Expected: PASS.

- [ ] **Step 6: Extend the integration test**

In `backend/internal/api/testhelper_test.go`, at the end of `seedFixtures` (after the booster config insert), price the fixture so the round-trip test can check the summary:

```go
	// Price the set and the one fixture card that has an mcm_id, the way
	// prices.Refresh would, so pricing assertions have something to see.
	pricedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	if err := q.UpdateSetPackPrice(ctx, db.UpdateSetPackPriceParams{
		Code: "FDN", PackPriceEur: pgtype.Float8{Float64: 4.36, Valid: true}, PackPricedAt: pricedAt,
	}); err != nil {
		t.Fatalf("seed pack price: %v", err)
	}
	if err := q.UpdateCardPrice(ctx, db.UpdateCardPriceParams{
		ID:           mustParseTestUUID(t, "01a67c48-2ba1-55ba-aee9-8f0ad4ccf5c9"), // Ruby, Daring Tracker
		PriceEur:     pgtype.Float8{Float64: 0.25, Valid: true},
		PriceFoilEur: pgtype.Float8{Float64: 1.5, Valid: true},
		PricedAt:     pricedAt,
	}); err != nil {
		t.Fatalf("seed card price: %v", err)
	}
```

(Add `"time"` to that file's imports.) Also pass `McmID: pgtype.Int4{Int32: 781936, Valid: true}` in the `UpsertSetParams` for `FDN` only:

```go
		params := db.UpsertSetParams{Code: sf.Data.Code, Name: sf.Data.Name}
		if sf.Data.Code == "FDN" {
			params.McmID = pgtype.Int4{Int32: 781936, Valid: true}
		}
		if _, err := q.UpsertSet(ctx, params); err != nil {
```

Then in `api_test.go`, inside `TestOpenPack_And_GetPack_RoundTrip` after the `len(opened.Cards) != 6` check, add:

```go
	if opened.Pricing.PackPriceEUR == nil || *opened.Pricing.PackPriceEUR != 4.36 {
		t.Errorf("pricing.pack_price_eur = %v, want 4.36", opened.Pricing.PackPriceEUR)
	}
	// The draw is random, so check the summary against the picks it came with
	// rather than a fixed total.
	var wantTotal float64
	var wantUnpriced int
	for _, c := range opened.Cards {
		if c.PriceEUR == nil {
			wantUnpriced++
			continue
		}
		wantTotal += *c.PriceEUR
	}
	if opened.Pricing.TotalValueEUR != round2(wantTotal) {
		t.Errorf("pricing.total_value_eur = %v, want %v", opened.Pricing.TotalValueEUR, round2(wantTotal))
	}
	if opened.Pricing.UnpricedCards != wantUnpriced {
		t.Errorf("pricing.unpriced_cards = %d, want %d", opened.Pricing.UnpricedCards, wantUnpriced)
	}
	if wantUnpriced == 0 {
		t.Error("fixture should leave at least one pick unpriced (only one card is priced)")
	}
	if opened.Pricing.PricedAt == nil {
		t.Error("pricing.priced_at is nil, want the seeded refresh time")
	}
```

- [ ] **Step 7: Run the full suite**

Run: `go test -race ./...` (needs Docker)
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/api
git commit -m "Expose per-pick Cardmarket price and a pack value summary on pack opens

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 8: API — pack price on `GET /v1/sets` + OpenAPI

**Files:**
- Modify: `backend/internal/api/sets.go`, `backend/internal/api/api_test.go`, `backend/openapi.yaml`

**Interfaces:**
- Consumes: `db.ListActiveBoosterConfigsWithSetNameRow.{PackPriceEur, PackPricedAt}` (Task 1), `round2` (Task 7).
- Produces (JSON on `SetSummary`): `pack_price_eur: number|null`, `pack_priced_at: date-time|null`.

- [ ] **Step 1: Extend the test**

In `TestListSets` (`api_test.go`), after the booster-types check:

```go
	if resp.Sets[0].PackPriceEUR == nil || *resp.Sets[0].PackPriceEUR != 4.36 {
		t.Errorf("pack_price_eur = %v, want 4.36", resp.Sets[0].PackPriceEUR)
	}
	if resp.Sets[0].PackPricedAt == nil {
		t.Error("pack_priced_at is nil, want the seeded refresh time")
	}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -race ./internal/api -run TestListSets`
Expected: FAIL — `PackPriceEUR` undefined.

- [ ] **Step 3: Implement**

In `sets.go`:

```go
type setSummary struct {
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	PackImageURL *string    `json:"pack_image_url"`
	PackPriceEUR *float64   `json:"pack_price_eur"` // Cardmarket trend price of one sealed booster; nil if unpriced
	PackPricedAt *time.Time `json:"pack_priced_at"`
	BoosterTypes []string   `json:"booster_types"`
}
```

and in `handleListSets` where the summary is created:

```go
			sets = append(sets, setSummary{
				Code:         row.SetCode,
				Name:         row.SetName,
				PackImageURL: packImageURLPath(row.SetCode, row.PackImageHash),
				PackPriceEUR: nullableFloat(row.PackPriceEur),
				PackPricedAt: nullableTime(row.PackPricedAt),
			})
```

Add to `pricing.go`:

```go
func nullableFloat(f pgtype.Float8) *float64 {
	if !f.Valid {
		return nil
	}
	v := round2(f.Float64)
	return &v
}

func nullableTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
```

and refactor `summarize` to use them:

```go
	p := pricing{PackPriceEUR: nullableFloat(packPrice), PricedAt: nullableTime(pricedAt)}
```

(Add `"time"` to `sets.go` imports.)

- [ ] **Step 4: Update `backend/openapi.yaml`**

Under `SetSummary.properties` add:

```yaml
        pack_price_eur:
          type: number
          nullable: true
          description: Cardmarket trend price (EUR) of one sealed booster, as of pack_priced_at. Null if unpriced.
          example: 4.36
        pack_priced_at:
          type: string
          format: date-time
          nullable: true
```

Under `CardPick.properties` add:

```yaml
        price_eur:
          type: number
          nullable: true
          description: >
            Cardmarket trend price (EUR) of this pick: the foil price for a
            foil pick when Cardmarket lists one, else the non-foil price.
            Null when the card is unpriced.
          example: 0.25
```

Under `PackOpen.properties` add:

```yaml
        pricing:
          $ref: "#/components/schemas/Pricing"
```

and a new schema:

```yaml
    Pricing:
      type: object
      description: Value summary of the pack. Prices are as of priced_at (last Cardmarket refresh), not as of the open.
      properties:
        pack_price_eur:
          type: number
          nullable: true
          example: 4.36
        total_value_eur:
          type: number
          description: Sum of price_eur over priced picks, rounded to cents.
          example: 12.4
        unpriced_cards:
          type: integer
          description: Picks with no price, excluded from total_value_eur.
          example: 1
        priced_at:
          type: string
          format: date-time
          nullable: true
```

- [ ] **Step 5: Run tests**

Run: `go test -race ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/api backend/openapi.yaml
git commit -m "Expose pack_price_eur on GET /v1/sets; document pricing in OpenAPI

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 9: Frontend — types, money formatting, price preview on the pack picker

**Files:**
- Modify: `frontend/src/lib/types.ts`, `frontend/src/components/PackOpener.tsx`
- Create: `frontend/src/lib/money.ts`

**Interfaces:**
- Consumes: API fields from Tasks 7–8.
- Produces: `formatEUR(n: number): string` → `"€4.36"`; TS types `SetSummary.pack_price_eur`, `SetSummary.pack_priced_at`, `CardPick.price_eur`, `PackOpen.pricing: Pricing`.

- [ ] **Step 1: Types**

In `frontend/src/lib/types.ts`:

```ts
export type SetSummary = {
  code: string;
  name: string;
  pack_image_url: string | null;
  /** Cardmarket trend price of one sealed booster, EUR. Null if unpriced. */
  pack_price_eur: number | null;
  pack_priced_at: string | null;
  booster_types: string[];
};

export type CardPick = {
  slot: number;
  sheet_name: string;
  foil: boolean;
  /** Foil-aware Cardmarket trend price for this pick, EUR. Null if unpriced. */
  price_eur: number | null;
  card: CardSummary;
};

export type Pricing = {
  pack_price_eur: number | null;
  total_value_eur: number;
  unpriced_cards: number;
  priced_at: string | null;
};

export type PackOpen = {
  open_id: string;
  set_code: string;
  booster_type: string;
  config_version: number;
  created_at: string;
  cards: CardPick[];
  pricing: Pricing;
};
```

- [ ] **Step 2: Money helper**

`frontend/src/lib/money.ts`:

```ts
const eur = new Intl.NumberFormat("en", { style: "currency", currency: "EUR" });

/** "€4.36". Prices are Cardmarket EUR trend figures; the API already rounds to cents. */
export function formatEUR(n: number): string {
  return eur.format(n);
}
```

- [ ] **Step 3: Preview on the picker**

In `PackOpener.tsx`, in the pre-open return, between the `<div className="flex items-center gap-4">…</div>` (pack + arrows) and the rip button, add:

```tsx
      {selectedSet.pack_price_eur != null && (
        <p className="text-sm text-muted">{formatEUR(selectedSet.pack_price_eur)} on Cardmarket</p>
      )}
```

Import: `import { formatEUR } from "@/lib/money";`

- [ ] **Step 4: Lint, build, look**

Run from `frontend/`: `pnpm lint && pnpm build`
Expected: clean.

With the API and frontend running, open `localhost:3000`: the price line appears under the FDN pack. Flip through sets: an unpriced set shows no line (no empty gap-filler).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/money.ts frontend/src/components/PackOpener.tsx
git commit -m "Frontend: show the sealed booster's Cardmarket price on the pack picker

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 10: Frontend — per-card price and pack value summary after the reveal

**Files:**
- Modify: `frontend/src/components/CardTile.tsx`, `frontend/src/components/PackOpener.tsx`

**Interfaces:**
- Consumes: `CardPick.price_eur`, `PackOpen.pricing`, `formatEUR` (Task 9). Existing reveal animation `animate-[reveal_0.4s_ease-out_forwards]` with an 80 ms stagger per card.

- [ ] **Step 1: Price on the card tile**

In `CardTile.tsx`, after the rarity `<p>`, add:

```tsx
      <p className="text-xs text-muted">
        {pick.price_eur != null ? formatEUR(pick.price_eur) : "—"}
      </p>
```

Import `formatEUR` from `@/lib/money`.

- [ ] **Step 2: Summary block**

In `PackOpener.tsx`, in the post-open return, between the card grid and the "rip another" button, add:

```tsx
        <PackValue pricing={pack.pricing} revealDelayMs={pack.cards.length * 80 + 400} />
```

and define, in the same file below `PackOpener`:

```tsx
// Shown once the last card has revealed (same reveal animation, delayed past
// the stagger) so the total lands as the punchline rather than a spoiler.
function PackValue({ pricing, revealDelayMs }: { pricing: Pricing; revealDelayMs: number }) {
  const { pack_price_eur: packPrice, total_value_eur: total, unpriced_cards: unpriced } = pricing;
  const delta = packPrice != null ? total - packPrice : null;

  return (
    <div
      className="text-center opacity-0 animate-[reveal_0.4s_ease-out_forwards]"
      style={{ animationDelay: `${revealDelayMs}ms` }}
    >
      <p className="text-ink">
        {packPrice != null ? (
          <>
            Pulled {formatEUR(total)} from a {formatEUR(packPrice)} pack{" "}
            <span className={delta! >= 0 ? "text-green-400" : "text-red-400"}>
              ({delta! >= 0 ? "+" : "−"}{formatEUR(Math.abs(delta!))})
            </span>
          </>
        ) : (
          <>Pack value: {formatEUR(total)}</>
        )}
      </p>
      {unpriced > 0 && (
        <p className="text-xs text-muted">
          {unpriced} card{unpriced === 1 ? "" : "s"} unpriced on Cardmarket
        </p>
      )}
    </div>
  );
}
```

Add `Pricing` to the type import: `import type { PackOpen, Pricing, SetSummary } from "@/lib/types";`

Note: the Cardmarket rule that a value is "only price" applies to the pre-open preview; the post-open summary is Decision 9.

- [ ] **Step 3: Lint, build, look**

Run from `frontend/`: `pnpm lint && pnpm build`
Expected: clean.

Rip a pack in the browser: each card shows a price or "—"; the summary fades in after the last card; a negative delta is red, positive green; "N cards unpriced" appears when any card is unpriced.

- [ ] **Step 4: Commit and update the graph**

```bash
git add frontend/src/components/CardTile.tsx frontend/src/components/PackOpener.tsx
git commit -m "Frontend: per-card Cardmarket price and pack value summary after the reveal

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
graphify update .
```

---

### Task 11 (v2 — specified, not built in this pass): periodic refresh inside the API

Build this when manual `make prices` becomes a chore. Same `prices.Refresh`; no new infra.

**Files:**
- Modify: `backend/internal/config/config.go`, `backend/cmd/api/main.go`, `backend/.env.example`, `README.md`

**Interfaces:**
- Consumes: `prices.Refresh`, `cardmarket.DefaultPriceGuideURL`.
- Produces: `Config.PriceRefreshInterval time.Duration` from env `PRICE_REFRESH_INTERVAL` (Go duration string, e.g. `24h`; `0` or unset disables).

- [ ] **Step 1: Config**

In `config.go` add `PriceRefreshInterval time.Duration` to `Config`, and in `Load`:

```go
	interval, err := time.ParseDuration(getenvDefault("PRICE_REFRESH_INTERVAL", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("PRICE_REFRESH_INTERVAL: %w", err)
	}
	...
		PriceRefreshInterval: interval,
```

Add `PRICE_REFRESH_INTERVAL=24h` to `.env.example` with a comment `# 0 disables the in-process Cardmarket price refresh`.

- [ ] **Step 2: Ticker in `cmd/api/main.go`**

After `apiServer := api.NewServer(pool, logger)`:

```go
	if cfg.PriceRefreshInterval > 0 {
		go refreshPricesPeriodically(ctx, logger, pool, cfg.PriceRefreshInterval)
	}
```

and:

```go
// refreshPricesPeriodically re-runs the Cardmarket price refresh every
// interval until ctx is cancelled. It also runs once at startup so a
// restart never pushes the next refresh a full interval out. Failures are
// logged and retried on the next tick; a stale price is not an outage.
// ponytail: every replica refreshes independently - harmless (idempotent
// UPDATEs), move to a leader lock if replicas multiply.
func refreshPricesPeriodically(ctx context.Context, logger *slog.Logger, pool *pgxpool.Pool, interval time.Duration) {
	client := &http.Client{Timeout: 2 * time.Minute}
	run := func() {
		st, err := prices.Refresh(ctx, pool, client, cardmarket.DefaultPriceGuideURL)
		if err != nil {
			logger.Error("price refresh", "error", err)
			return
		}
		logger.Info("prices refreshed", "cards", st.Cards, "cards_missing", st.CardsMissing, "sets", st.Sets, "sets_missing", st.SetsMissing)
	}
	run()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
```

- [ ] **Step 3: Test**

Unit-test the config parsing (`PRICE_REFRESH_INTERVAL=bogus` → error; unset → 0) in `internal/config/config_test.go`. Manually: `PRICE_REFRESH_INTERVAL=1m make run` and watch for `prices refreshed` at boot and after a minute.

- [ ] **Step 4: Docs + commit**

README "Refresh prices" paragraph: add `Set PRICE_REFRESH_INTERVAL=24h to have the API refresh on its own.`

```bash
git commit -am "API: refresh Cardmarket prices on a PRICE_REFRESH_INTERVAL ticker

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## End-to-end verification (after Task 10)

1. `cd backend && make dev && make migrate-up`
2. `set -a && source .env && set +a && go run ./cmd/import FDN` — log ends with `prices refreshed ... sets=1`.
3. `make run`; in another terminal `curl -s localhost:8080/v1/sets | jq '.sets[0] | {code, pack_price_eur, pack_priced_at}'` → a price near 4–5 EUR.
4. `curl -s -X POST localhost:8080/v1/packs/open -d '{"set_code":"FDN"}' | jq '{pricing, prices: [.cards[] | {name: .card.name, foil, price_eur}]}'` — every pick has `price_eur` or `null`; `pricing.total_value_eur` equals the sum of the non-null ones (to the cent); `unpriced_cards` equals the null count.
5. `cd frontend && pnpm dev`, open localhost:3000: price under the pack; rip; prices under each card; summary line with a colored delta after the last card reveals.
6. `make prices` again — log shows the same counts; `pack_priced_at` advances.
