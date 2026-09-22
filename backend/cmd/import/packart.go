package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
)

// Background removal thresholds, tuned and verified against real TCGplayer
// product photos (FDN, BLB): a flood fill from the image's four edges turns
// the plain white studio background transparent, without touching white
// elements inside the artwork (logo text, card-count badge) since those
// aren't connected to the border through any path of near-white pixels.
// The fill is then cropped to the pack's bounding box and clamped by
// edgeBand (see cropToPack) rather than guarded by a fraction threshold -
// see that function's comment for why.
const (
	// seedThreshold: how close to pure white a border pixel must be to
	// start the fill. Strict, so we only ever seed from genuine background.
	seedThreshold = 12.0
	// growThreshold: how close a neighboring pixel must be to keep growing
	// into it. Looser, to eat through the anti-aliased/JPEG-compressed
	// transition band between background and subject. Alpha ramps linearly
	// across this band (0 at pure white, 255 at the threshold) so the cut
	// edge softens instead of aliasing.
	growThreshold = 60.0
	// edgeBand: how deep (px) the background can legitimately reach inside
	// the pack's bounding box - the rounded corners plus the anti-aliased
	// margin, on a 600px-tall photo. Anything the flood fill cleared deeper
	// than this is pack interior that happens to be near-white (Final
	// Fantasy's wrapper is white) and gets its alpha restored. Replaces the
	// old "more than 40% cleared means a bad cut" guard, which misfired on
	// photos that are 600x600 with the pack centred in wide white padding
	// (MKM, OTJ, TLA) - there, ~50% cleared is the correct answer.
	edgeBand = 14
)

// removeWhiteBackground turns a product photo shot on a plain white studio
// background (as TCGplayer's are) into a PNG with that background made
// transparent, cropped to the pack itself so every photo - tightly framed
// or padded - ends up the same shape. See the threshold constants above.
func removeWhiteBackground(jpegData []byte) ([]byte, error) {
	src, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	return encodePNG(cropToPack(floodFillTransparent(src)))
}

// floodFillTransparent runs the background-removal flood fill on any
// image.Image (jpeg.Decode's output for the real pipeline, or a synthetic
// image.NRGBA in tests).
func floodFillTransparent(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), src, bounds.Min, draw.Src)

	pixelAt := func(x, y int) (r, g, b uint8) {
		i := img.PixOffset(x, y)
		return img.Pix[i], img.Pix[i+1], img.Pix[i+2]
	}
	distFromWhite := func(x, y int) float64 {
		r, g, b := pixelAt(x, y)
		dr, dg, db := 255-float64(r), 255-float64(g), 255-float64(b)
		return math.Sqrt(dr*dr + dg*dg + db*db)
	}
	setAlpha := func(x, y int, a uint8) {
		img.Pix[img.PixOffset(x, y)+3] = a
	}
	alphaForDistance := func(d float64) uint8 {
		if d >= growThreshold {
			return 255
		}
		return uint8(255 * d / growThreshold)
	}

	visited := make([]bool, w*h)
	idx := func(x, y int) int { return y*w + x }

	queue := make([]image.Point, 0, w+h)
	seed := func(x, y int) {
		if visited[idx(x, y)] {
			return
		}
		if distFromWhite(x, y) <= seedThreshold {
			visited[idx(x, y)] = true
			queue = append(queue, image.Point{X: x, Y: y})
		}
	}
	for x := 0; x < w; x++ {
		seed(x, 0)
		seed(x, h-1)
	}
	for y := 0; y < h; y++ {
		seed(0, y)
		seed(w-1, y)
	}

	dirs := [4]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		d := distFromWhite(p.X, p.Y)
		setAlpha(p.X, p.Y, alphaForDistance(d))
		for _, dir := range dirs {
			nx, ny := p.X+dir.X, p.Y+dir.Y
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			ni := idx(nx, ny)
			if visited[ni] {
				continue
			}
			if distFromWhite(nx, ny) <= growThreshold {
				visited[ni] = true
				queue = append(queue, image.Point{X: nx, Y: ny})
			}
		}
	}

	return img
}

// cropToPack trims the flood-filled image to the pack's bounding box
// (so padded 600x600 product photos end up the same shape as the
// tightly framed ones) and re-opaques anything the fill reached deeper
// than edgeBand inside that box. Returns img unchanged if nothing is
// opaque - the caller's photo was all background, which is nonsense
// but not worth erroring over for decorative art.
func cropToPack(img *image.NRGBA) *image.NRGBA {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.Pix[img.PixOffset(x, y)+3] == 0 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if !found {
		return img
	}

	bb := image.Rect(minX, minY, maxX+1, maxY+1)
	out := image.NewNRGBA(image.Rect(0, 0, bb.Dx(), bb.Dy()))
	for y := bb.Min.Y; y < bb.Max.Y; y++ {
		for x := bb.Min.X; x < bb.Max.X; x++ {
			si := img.PixOffset(x, y)
			dx, dy := x-bb.Min.X, y-bb.Min.Y
			di := out.PixOffset(dx, dy)
			copy(out.Pix[di:di+4], img.Pix[si:si+4])
			if min(dx, bb.Dx()-1-dx, dy, bb.Dy()-1-dy) > edgeBand {
				out.Pix[di+3] = 255
			}
		}
	}
	return out
}

func encodePNG(img *image.NRGBA) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
