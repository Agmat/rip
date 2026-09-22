# Pack Image from TCGplayer Implementation Plan

> **For agentic workers:** implement task by task, in order. Use the agent-skills workflow skills where they fit (`agent-skills:build` / `agent-skills:test` for the Go tasks, `agent-skills:review` before merge, `agent-skills:git-workflow-and-versioning` for commits). Do **not** invoke any `superpowers:*` skill. Tick each `- [ ]` as you go; run the listed checks before every commit.

**Goal:** Stop scraping mtg.wiki for the Play Booster product image. Resolve it deterministically from MTGJSON's `sealedProduct[].identifiers.tcgplayerProductId` via TCGplayer's public image CDN, with white-background removal so the frontend keeps getting a transparent PNG.

**Why:** mtg.wiki resolution is an HTML scrape keyed on the set's *name* against hand-curated filenames (`FND_Play_Booster.png` for FDN...). Any editor rename, page restructure, or bot block silently drops the image. MTGJSON already ships the id we need for every play booster set (verified 2026-09-22: 20 play-booster sets in `SetList.json`, 0 missing `tcgplayerProductId`; `https://product-images.tcgplayer.com/fit-in/600x600/562116.jpg` returns `200 image/jpeg` with our UA).

**Architecture:** This is mostly a targeted revert of the image half of `cf23fcf` (which replaced TCGplayer with mtg.wiki). Keep `mcmId` on `SealedProduct` (pricing uses it), add `tcgplayerProductId` back, restore `packart.go` (flood-fill background removal, already tuned against real TCGplayer photos) from git history, delete `packimage.go`. No schema, API, or frontend change: `sets.pack_image` stays a `bytea` PNG.

**Tech Stack:** Go stdlib only (`image`, `image/jpeg`, `image/png`, `net/http`).

## Decisions (locked)

1. **Source:** `https://product-images.tcgplayer.com/fit-in/600x600/<tcgplayerProductId>.jpg`, id from the MTGJSON `sealedProduct` entry with `category == "booster_pack"` and `subtype == boosterType` - the same entry `packMCMID` already selects.
2. **Background removal:** restore `packart.go` / `packart_test.go` verbatim from `git show cf23fcf^:backend/cmd/import/packart.go` (and `_test.go`). Do not retune thresholds.
3. **Failure policy unchanged:** missing id, fetch failure, or decode failure → `slog.Warn`, import continues with `pack_image = NULL`.
4. **No fallback chain:** no mtg.wiki fallback, no checked-in `art/` files. One source. Add a second only if TCGplayer actually fails for a real set.
5. **User-Agent:** keep sending the existing `userAgent` constant on the CDN request (harmless, and consistent with the rest of the importer).

## Global Constraints

- Commit messages: imperative, no prefix, end with `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- `go test -race -short ./...` from `backend/` must pass after every task.
- `go vet ./...` and any configured linter must pass.
- No changes under `backend/migrations`, `backend/queries`, `backend/openapi.yaml`, or `frontend/`.
- After the last task, run `graphify update .` from the repo root.

---

## File map

| File | Change |
|---|---|
| `backend/internal/mtgjson/mtgjson.go` | Add `TCGplayerProductID string \`json:"tcgplayerProductId"\`` to `SealedProduct.Identifiers`; fix doc comment |
| `backend/cmd/import/packart.go` | Restore from `cf23fcf^` (flood-fill background removal) |
| `backend/cmd/import/packart_test.go` | Restore from `cf23fcf^` |
| `backend/cmd/import/packimage.go` | Rewrite: `packImageURL` + `fetchPackImage` (TCGplayer); drop wiki constants and regex |
| `backend/cmd/import/packimage_test.go` | Rewrite: test `packImageURL` selection and `fetchPackImage` against httptest serving a JPEG |
| `backend/cmd/import/main.go` | Swap the `fetchPackImageFromWiki` block for `packImageURL` + `fetchPackImage`; update file header comment |

---

## Task 1: Expose `tcgplayerProductId` from MTGJSON

- [x] In `backend/internal/mtgjson/mtgjson.go`, add `TCGplayerProductID string \`json:"tcgplayerProductId"\`` next to `MCMID` in `SealedProduct.Identifiers`.
- [x] Update the `SealedProduct` doc comment: it now yields both the Cardmarket id (price) and the TCGplayer id (pack photo); remove the "(Pack art no longer comes from here...)" line.
- [x] Add one table case to the existing `ParseSetFile` test (or a new tiny test) with a `sealedProduct` entry carrying both ids; assert both decode.
- [x] `go test -race -short ./internal/mtgjson/`
- [x] Commit: `Decode tcgplayerProductId on MTGJSON sealed products`

## Task 2: Restore background removal

- [x] `git show cf23fcf^:backend/cmd/import/packart.go > backend/cmd/import/packart.go`
- [x] `git show cf23fcf^:backend/cmd/import/packart_test.go > backend/cmd/import/packart_test.go`
- [x] `go test -race -short ./cmd/import/` - the restored swatch tests must pass unchanged. If they don't compile against current code, fix the call site, not the algorithm.
- [x] Commit: `Restore TCGplayer photo background removal`

## Task 3: Replace the wiki fetcher with the TCGplayer fetcher

- [x] Rewrite `backend/cmd/import/packimage.go`:
  - Keep `userAgent` and `fetchBytes` as they are.
  - Delete `DefaultMTGWikiPageBaseURL`, `DefaultMTGWikiFilesBaseURL`, `playBoosterImageRe`, `fetchPackImageFromWiki`.
  - Add `const DefaultTCGplayerImageBaseURL = "https://product-images.tcgplayer.com/fit-in/600x600"` (overridable for tests, same pattern as the wiki constants had).
  - Add `func packImageURL(sf *mtgjson.SetFile, boosterType, baseURL string) (string, bool)` - selection identical to `packMCMID` (`booster_pack` + matching subtype), returns `false` when the id is empty. Doc comment: why TCGplayer (MTGJSON/Scryfall host no pack art), why this entry.
  - Add `func (imp *importer) fetchPackImage(ctx context.Context, url string) ([]byte, error)` = `fetchBytes` then `removeWhiteBackground`.
- [x] Rewrite `backend/cmd/import/packimage_test.go`:
  - `TestPackImageURL_PicksBoosterPackOfMatchingSubtype` - mirror `TestPackMCMID_...` in `main_test.go` (box / collector / play entries; only play's id is used) plus a case with empty id → `false`.
  - `TestFetchPackImage_ReturnsTransparentPNG` - httptest server returns a JPEG-encoded `swatch()` (from `packart_test.go`); assert result decodes as PNG and a corner pixel has alpha 0.
  - `TestFetchPackImage_Non200IsError`.
- [x] `go test -race -short ./cmd/import/`
- [x] Commit: `Fetch pack image from TCGplayer CDN instead of mtg.wiki`

## Task 4: Wire it into the importer

- [x] In `backend/cmd/import/main.go` `importSet`, replace the `fetchPackImageFromWiki` block with:
  ```go
  var packImage []byte
  if packImgURL, ok := packImageURL(primary, boosterType, DefaultTCGplayerImageBaseURL); !ok {
      slog.Warn("no TCGplayer pack image id, importing without one", "set", setCode, "booster_type", boosterType)
  } else if img, err := imp.fetchPackImage(ctx, packImgURL); err != nil {
      // Decorative art must never block getting the cards in.
      slog.Warn("failed to fetch/process pack image, importing without one", "set", setCode, "error", err)
  } else {
      packImage = img
  }
  ```
- [x] Update the file header comment on `main.go` (line 2) if it names the image source.
- [x] `go build ./... && go vet ./... && go test -race -short ./...` from `backend/`.
- [x] Grep: `grep -rn "wiki" backend/` must return nothing.
- [x] Commit: `Use TCGplayer pack image in cmd/import`

## Task 5: Verify against real data

- [x] With local Postgres up (`docker-compose up -d`), run `go run ./cmd/import FDN` and confirm the log shows no pack-image warning.
- [x] `curl -s localhost:8080/v1/sets/FDN/pack.png -o /tmp/fdn.png && file /tmp/fdn.png` → `PNG image data ... 8-bit/color RGBA`; open it and check the background is transparent and the pack isn't chewed. (Endpoint is `/v1/sets/{code}/pack.png` per `openapi.yaml`, not `/pack-image` as this line originally said.)
- [x] Run `go run ./cmd/import --all` and grep the log for `importing without one`; every play booster set should get an image (0 missing ids as of 2026-09-22). Any set that warns → record it in this plan under "Known gaps" rather than adding a fallback.
- [x] `graphify update .` from the repo root.
- [ ] Commit any doc/graph updates: `Update knowledge graph for TCGplayer pack images` (skipped: `graphify update .` reported no tracked graph changes)

## Known gaps

- None. Verified end-to-end with local Postgres (OrbStack) 2026-09-22: `go run ./cmd/import FDN` fetched and stored the pack image with no warning; `GET /v1/sets/FDN/pack.png` returned a 316x600 8-bit RGBA PNG with a clean transparent background matching the source photo; `go run ./cmd/import --all` imported all 18 play-booster sets with zero `importing without one` warnings. The 11 rows still NULL in `sets.pack_image` (BIG, EOS, FCA, M3C, MAR, OTP, PLST, PZA, SOA, SPG, TLE) are non-primary source sets (e.g. SPG "Special Guests") that were never imported as a primary set, so a NULL pack image there is expected, not a gap.

## Explicitly not doing

- No mtg.wiki fallback, no `art/` checked-in fallback (rung 1: nothing has failed yet).
- No retuning of flood-fill thresholds; they were verified against FDN/BLB TCGplayer photos.
- No caching / conditional re-download: `--all` re-fetches ~20 × 50 KB, which is fine.
