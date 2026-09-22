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
	// maxBackgroundFraction: if the fill touches more of the image than
	// this, the thresholds above are almost certainly wrong for this photo
	// (e.g. a wrapper that's itself near-white at the edges) rather than
	// correctly finding a thin margin. Better to keep a whole pack with its
	// original white edges than a pack chewed into by a bad cut.
	maxBackgroundFraction = 0.40
)

// removeWhiteBackground turns a product photo shot on a plain white studio
// background (as TCGplayer's are) into a PNG with that background made
// transparent. See the threshold constants above for how.
func removeWhiteBackground(jpegData []byte) ([]byte, error) {
	src, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("decode jpeg: %w", err)
	}
	return encodePNG(floodFillTransparent(src))
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
	touched := 0
	for len(queue) > 0 {
		p := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		touched++
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

	if float64(touched)/float64(w*h) > maxBackgroundFraction {
		// The cut is almost certainly wrong for this photo - undo it and
		// hand back a fully opaque image (the original white edges) rather
		// than one that's had a chunk of its own artwork removed.
		opaque := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(opaque, opaque.Bounds(), src, bounds.Min, draw.Src)
		for i := 3; i < len(opaque.Pix); i += 4 {
			opaque.Pix[i] = 255
		}
		return opaque
	}

	return img
}

func encodePNG(img *image.NRGBA) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
