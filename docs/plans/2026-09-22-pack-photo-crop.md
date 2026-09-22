# Pack Photo Crop + Edge-Band Clamp Implementation Plan

> **For agentic workers:** implement task by task, in order. Use the agent-skills workflow skills where they fit (`agent-skills:build` / `agent-skills:test` for the Go tasks, `agent-skills:review` before merge, `agent-skills:git-workflow-and-versioning` for commits). Do **not** invoke any `superpowers:*` skill. Tick each `- [x]` as you go; run the listed checks before every commit.

**Goal:** Make every Play Booster render at the same on-screen size regardless of how TCGplayer framed the product photo, and stop the white-background removal from eating white wrappers (Final Fantasy).

**Scope:** Only `backend/cmd/import/packart.go` and its test. This is the image-processing half of `2026-09-22-tcgplayer-pack-image.md`; that plan's Task 2 delegates here. It can be executed on its own (Task 1 restores the file it edits), but the result is only wired into the importer by the parent plan's Tasks 3–4.

**Tech Stack:** Go stdlib only (`image`, `image/draw`, `image/jpeg`, `image/png`).

## Problem (measured 2026-09-22 on all 20 play-booster TCGplayer photos)

| Photo shape | Sets | Old `floodFillTransparent` result |
|---|---|---|
| ~316–340 × 600, pack fills the frame, thin white margin | 15 sets (FDN, BLB, DSK, …) | Correct: 1–12 % cut away |
| **600 × 600, pack ~320 × 590 centred in wide white padding** | **MKM, OTJ, TLA** | Fill reaches 48–51 % of pixels → trips `maxBackgroundFraction = 0.40` → returns the **opaque padded photo** → pack renders small and square |
| ~336 × 600, **the wrapper itself is white** | **FIN** | Guard doesn't trip (14 %) but the fill leaks through the white wrapper and clears the pack interior |

The frontend (`Pack.tsx`) renders the PNG with `fill` into a fixed-aspect box, so any padding left in the PNG directly shrinks the pack on screen.

## Decisions (locked)

1. **Keep** `seedThreshold = 12` and `growThreshold = 60` unchanged.
2. **Remove** `maxBackgroundFraction` and the "undo the cut" branch. It was a proxy for "the cut went wrong" that misfires on padded photos; the geometric clamp below is the replacement guard.
3. **Add** `cropToPack`, run after the flood fill and before PNG encoding:
   1. bounding box of pixels with `alpha > 0` (FIN's grey outline/shadow pixels survive the fill, so the box still hugs the pack);
   2. crop to that box;
   3. inside the crop, force `alpha = 255` on any pixel deeper than `edgeBand = 14` px from the nearest crop edge - background can only exist in the margin and the rounded corners, anything deeper that the fill reached is pack interior.
4. `edgeBand` is a constant, not configurable. 14 px was picked visually on 600 px-tall photos (corner radius + anti-aliasing) and verified on all four problem sets.
5. No per-set overrides, no second image source.

Verified in a scratch harness: TLA → 344×600, OTJ → 320×592, MKM → 320×584, FIN → 336×600 (wrapper intact), FDN → 316×600 (unchanged). All clean over a magenta test background.

## Global Constraints

- Commit messages: imperative, no prefix, end with `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`.
- `go test -race -short ./...` and `go vet ./...` from `backend/` must pass after every task.
- Nothing outside `backend/cmd/import/packart.go` and `backend/cmd/import/packart_test.go` changes.
- After the last task, run `graphify update .` from the repo root.

---

## Task 1: Restore the flood-fill code (skip if `packart.go` already exists)

- [x] `git show cf23fcf^:backend/cmd/import/packart.go > backend/cmd/import/packart.go`
- [x] `git show cf23fcf^:backend/cmd/import/packart_test.go > backend/cmd/import/packart_test.go`
- [x] `go build ./cmd/import/ && go test -race -short ./cmd/import/` - baseline green. `removeWhiteBackground` is unreferenced until the parent plan wires it in; that is fine for `go build` (unused functions are allowed) but check `go vet` / the linter don't flag it. If they do, add a `//nolint:unused` with a comment pointing at the parent plan rather than deleting it.
- [x] Commit: `Restore TCGplayer photo background removal`

## Task 2: Replace the fraction guard with `cropToPack`

- [x] In `packart.go`:
  - Delete `maxBackgroundFraction` and, in `floodFillTransparent`, the `if float64(touched)/... > maxBackgroundFraction { ... }` block and the `touched` counter. The function now always returns the filled image.
  - Add to the const block:
    ```go
    // edgeBand: how deep (px) the background can legitimately reach inside
    // the pack's bounding box - the rounded corners plus the anti-aliased
    // margin, on a 600px-tall photo. Anything the flood fill cleared deeper
    // than this is pack interior that happens to be near-white (Final
    // Fantasy's wrapper is white) and gets its alpha restored. Replaces the
    // old "more than 40% cleared means a bad cut" guard, which misfired on
    // photos that are 600x600 with the pack centred in wide white padding
    // (MKM, OTJ, TLA) - there, ~50% cleared is the correct answer.
    edgeBand = 14
    ```
  - Add:
    ```go
    // cropToPack trims the flood-filled image to the pack's bounding box
    // (so padded 600x600 product photos end up the same shape as the
    // tightly framed ones) and re-opaques anything the fill reached deeper
    // than edgeBand inside that box. Returns img unchanged if nothing is
    // opaque - the caller's photo was all background, which is nonsense
    // but not worth erroring over for decorative art.
    func cropToPack(img *image.NRGBA) *image.NRGBA
    ```
    Implementation: one pass over pixels to compute the bbox via `image.Rectangle.Union` on 1×1 rects (or track min/max ints); `out := image.NewNRGBA(image.Rect(0, 0, bb.Dx(), bb.Dy()))`; second pass copies 4 bytes per pixel and sets `out.Pix[di+3] = 255` when `min(x-bb.Min.X, bb.Max.X-1-x, y-bb.Min.Y, bb.Max.Y-1-y) > edgeBand`. Use Go 1.21 builtin `min`.
  - Change `removeWhiteBackground` to `return encodePNG(cropToPack(floodFillTransparent(src)))`.
  - Update the file's top doc comment block so it describes the three-step pipeline (fill → crop/clamp → encode) and no longer mentions the fraction guard.
- [x] In `packart_test.go`:
  - Delete the test that exercises `maxBackgroundFraction` (the one that expects a fully opaque result on a mostly-white image).
  - Add `TestCropToPack_TrimsPadding`: 100×100 white canvas, coloured rect at (30,5)–(70,95), run `floodFillTransparent` then `cropToPack`; assert `Bounds().Size() == (40, 90)` and pixel (0,0) of the result has alpha 0... note: with the swatch's hard edge and `seedThreshold`, the fill stops exactly at the coloured rect so the bbox is the rect itself and the corner is opaque; instead assert size only, plus that the *pre-crop* image had transparent padding at (0,0). Keep it simple: size is the contract.
  - Add `TestCropToPack_RestoresInteriorAlpha`: take `swatch()`, run `floodFillTransparent`, then manually set alpha 0 on a pixel at (20,20) (deeper than `edgeBand` from every edge) and on a pixel at (2,20) (inside the band); call `cropToPack`; assert (20,20) alpha is 255 and the band pixel keeps whatever alpha the fill gave it (0).
  - Add `TestCropToPack_AllTransparentIsNoop`: fully transparent 10×10 → same bounds back.
- [x] `go test -race -short ./cmd/import/ && go vet ./cmd/import/`
- [x] Commit: `Crop pack photos to the pack and clamp background removal to the edge band`

## Task 3: Verify on real photos

- [x] Write a throwaway `go run` harness in the scratchpad (not committed) that calls `removeWhiteBackground` on the TCGplayer JPEGs for FDN, FIN, TLA, OTJ, MKM (`https://product-images.tcgplayer.com/fit-in/600x600/<id>.jpg`, ids: 562116, 618889, 648640, 541083, 529962), prints the output PNG size, and composites it over magenta.
- [x] Assert by eye: the four problem sets are ~320–344 px wide × ~584–600 px tall like FDN; no white padding remains; FIN's white wrapper is still there with only a thin outer strip softened.
- [x] Record the measured sizes in this plan under "Verification log".
- [x] `graphify update .` from the repo root.

## Verification log

- Scratch harness, 2026-09-22 (pre-implementation): TLA 344×600, OTJ 320×592, MKM 320×584, FIN 336×600, FDN 316×600.
- Post-implementation, 2026-09-22: TLA 344×600, OTJ 320×592, MKM 320×584, FIN 336×600, FDN 316×600 — identical to the pre-implementation measurements. Visually verified over magenta: no white padding remains on TLA/OTJ/MKM, FIN's white wrapper is intact with only a thin outer strip softened, all four rounded corners look clean.

## Explicitly not doing

- Not retuning `seedThreshold` / `growThreshold`.
- Not normalising all outputs to one exact pixel size - the frontend box already scales, and ±5 % in aspect is the packs' real shape variance.
- Not adding per-set overrides or a second source for FIN.
