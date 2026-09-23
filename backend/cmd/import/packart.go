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
	// fringeDepth: how many px in from the cut the photo's pixels are still
	// a blend of pack and white background (JPEG + anti-aliasing), on a
	// 600px-tall photo. defringe re-derives their color and alpha from the
	// first pixel past this depth.
	fringeDepth = 2
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
	return encodePNG(defringe(cropToPack(floodFillTransparent(src))))
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

// cropToPack trims the flood-filled image to the bounding box of its mostly
// opaque (alpha >= 128) pixels, so padded 600x600 product photos end up the
// same shape as the tightly framed ones, faint JPEG specks left in the
// background can't push the box out, and a silver crimp (HOB, FIN: close to
// white, so only partly opaque) still counts as pack. It then re-opaques
// anything the fill reached deeper than edgeBand inside that box. Returns img unchanged if nothing is
// opaque - the caller's photo was all background, which is nonsense
// but not worth erroring over for decorative art. Pixels outside the box are
// dropped; defringe handles the partly transparent ones inside it.
func cropToPack(img *image.NRGBA) *image.NRGBA {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.Pix[img.PixOffset(x, y)+3] < 128 {
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

// defringe removes the light halo the cut leaves around the pack. The flood
// fill only lowers alpha, so an edge pixel keeps its near-white color, and
// the first opaque pixels are still half white: on a dark page that reads
// as a white outline. Every pixel within fringeDepth of the cut (or not
// fully opaque) instead takes the color of the nearest solid pack pixel
// past that depth, with alpha set to how far it sits from white towards
// that color - un-blending it from the background. Pixels more than
// 2*fringeDepth from any solid pixel are leftover background noise and go
// fully transparent.
func defringe(img *image.NRGBA) *image.NRGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	idx := func(x, y int) int { return y*w + x }
	dirs := [4]image.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}

	// depth: 4-connected distance from the nearest non-opaque pixel or the
	// image edge (which is where the crop cut the background away).
	depth := make([]int, w*h)
	queue := make([]image.Point, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case img.Pix[img.PixOffset(x, y)+3] != 255:
				depth[idx(x, y)] = 0
			case x == 0 || y == 0 || x == w-1 || y == h-1:
				depth[idx(x, y)] = 1
			default:
				depth[idx(x, y)] = -1
				continue
			}
			queue = append(queue, image.Point{X: x, Y: y})
		}
	}
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		for _, d := range dirs {
			nx, ny := p.X+d.X, p.Y+d.Y
			if nx < 0 || nx >= w || ny < 0 || ny >= h || depth[idx(nx, ny)] != -1 {
				continue
			}
			depth[idx(nx, ny)] = depth[idx(p.X, p.Y)] + 1
			queue = append(queue, image.Point{X: nx, Y: ny})
		}
	}

	// src: the nearest core pixel (deeper than fringeDepth) for every other
	// pixel, by BFS outward from the core; dist is how far away it is.
	src := make([]int, w*h)
	dist := make([]int, w*h)
	queue = queue[:0]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := idx(x, y)
			if depth[i] > fringeDepth || depth[i] == -1 {
				src[i], dist[i] = i, 0
				queue = append(queue, image.Point{X: x, Y: y})
			} else {
				src[i], dist[i] = -1, 0
			}
		}
	}
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		pi := idx(p.X, p.Y)
		for _, d := range dirs {
			nx, ny := p.X+d.X, p.Y+d.Y
			if nx < 0 || nx >= w || ny < 0 || ny >= h || src[idx(nx, ny)] != -1 {
				continue
			}
			src[idx(nx, ny)], dist[idx(nx, ny)] = src[pi], dist[pi]+1
			queue = append(queue, image.Point{X: nx, Y: ny})
		}
	}

	out := image.NewNRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for i := range src {
		if dist[i] == 0 {
			continue // core, or no core anywhere: leave as is
		}
		o := 4 * i
		if src[i] == -1 || dist[i] > 2*fringeDepth {
			out.Pix[o+3] = 0
			continue
		}
		s := 4 * src[i]
		// Project the pixel onto the white -> core-color line: 0 is pure
		// background, 1 is pure pack.
		var num, den float64
		for c := 0; c < 3; c++ {
			bg := 255 - float64(img.Pix[s+c])
			num += (255 - float64(img.Pix[o+c])) * bg
			den += bg * bg
		}
		if den < 30*30 {
			continue // pack is itself near-white here: nothing to un-blend
		}
		a := math.Min(1, math.Max(0, num/den))
		copy(out.Pix[o:o+3], img.Pix[s:s+3])
		out.Pix[o+3] = uint8(math.Round(255 * a))
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
