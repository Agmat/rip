package main

import (
	"image"
	"image/color"
	"testing"
)

// swatch draws a synthetic photo: a white canvas, a colored rectangle (the
// "pack") with a thin margin of white (the "studio background") around it,
// and a white square *inside* the rectangle (a stand-in for the pack's own
// white text/logo), isolated from the border by colored pixels on every
// side.
func swatch() *image.NRGBA {
	const size = 40
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	packColor := color.NRGBA{R: 200, G: 40, B: 40, A: 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, white)
		}
	}
	for y := 1; y < 39; y++ {
		for x := 1; x < 39; x++ {
			img.Set(x, y, packColor)
		}
	}
	for y := 15; y < 25; y++ {
		for x := 15; x < 25; x++ {
			img.Set(x, y, white)
		}
	}
	return img
}

func alphaAt(img *image.NRGBA, x, y int) uint8 {
	return img.Pix[img.PixOffset(x, y)+3]
}

func TestFloodFillTransparent_RemovesOnlyBorderConnectedWhite(t *testing.T) {
	out := floodFillTransparent(swatch())

	if a := alphaAt(out, 0, 0); a != 0 {
		t.Errorf("border pixel (0,0): want alpha 0 (background), got %d", a)
	}
	if a := alphaAt(out, 0, 20); a != 0 {
		t.Errorf("margin pixel (0,20): want alpha 0 (background), got %d", a)
	}
	if a := alphaAt(out, 20, 20); a != 255 {
		t.Errorf("interior white square (20,20): want alpha 255 (untouched, not border-connected), got %d", a)
	}
	if a := alphaAt(out, 10, 10); a != 255 {
		t.Errorf("pack color (10,10): want alpha 255 (opaque), got %d", a)
	}
}

func TestCropToPack_TrimsPadding(t *testing.T) {
	const size = 100
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	packColor := color.NRGBA{R: 200, G: 40, B: 40, A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, white)
		}
	}
	for y := 5; y < 95; y++ {
		for x := 30; x < 70; x++ {
			img.Set(x, y, packColor)
		}
	}

	filled := floodFillTransparent(img)
	if a := alphaAt(filled, 0, 0); a != 0 {
		t.Fatalf("pre-crop image should have transparent padding at (0,0), got alpha %d", a)
	}

	out := cropToPack(filled)
	if got := out.Bounds().Size(); got != (image.Point{X: 40, Y: 90}) {
		t.Errorf("cropped size: want (40, 90), got %v", got)
	}
}

func TestCropToPack_RestoresInteriorAlpha(t *testing.T) {
	filled := floodFillTransparent(swatch())
	// Pack's opaque bbox is (1,1)-(38,38): (20,20) is > edgeBand from every
	// crop edge, (2,20) is within edgeBand of the left edge.
	filled.Pix[filled.PixOffset(20, 20)+3] = 0
	filled.Pix[filled.PixOffset(2, 20)+3] = 0

	out := cropToPack(filled)

	if a := alphaAt(out, 19, 19); a != 255 {
		t.Errorf("interior pixel beyond edgeBand: want alpha restored to 255, got %d", a)
	}
	if a := alphaAt(out, 1, 19); a != 0 {
		t.Errorf("pixel within edgeBand: want alpha left at 0, got %d", a)
	}
}

func TestCropToPack_AllTransparentIsNoop(t *testing.T) {
	const size = 10
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	out := cropToPack(img)

	if got := out.Bounds().Size(); got != (image.Point{X: size, Y: size}) {
		t.Errorf("all-transparent image: want unchanged bounds %dx%d, got %v", size, size, got)
	}
}

// A dark pack on white whose outermost column is a 50/50 blend with the
// background (as JPEG/anti-aliasing leave it), plus a faint near-white
// speck out in the padding.
func blendedPack() *image.NRGBA {
	const size = 60
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	for y := 10; y < 50; y++ {
		for x := 10; x < 50; x++ {
			img.Set(x, y, color.NRGBA{R: 20, G: 20, B: 40, A: 255})
		}
		img.Set(9, y, color.NRGBA{R: 138, G: 138, B: 148, A: 255})
	}
	img.Set(2, 2, color.NRGBA{R: 240, G: 240, B: 240, A: 255})
	return img
}

func TestDefringe_UnblendsEdgeAndIgnoresSpecks(t *testing.T) {
	out := defringe(cropToPack(floodFillTransparent(blendedPack())))

	// Speck at (2,2) must not stretch the crop: box is the pack plus its
	// blended column, 41x40.
	if got := out.Bounds().Size(); got != (image.Point{X: 41, Y: 40}) {
		t.Fatalf("cropped size: want (41, 40), got %v", got)
	}
	i := out.PixOffset(0, 20)
	if r, a := out.Pix[i], out.Pix[i+3]; r != 20 || a < 110 || a > 145 {
		t.Errorf("blended edge pixel: want pack color (r=20) at ~half alpha, got r=%d a=%d", r, a)
	}
	if a := alphaAt(out, 20, 20); a != 255 {
		t.Errorf("pack interior: want alpha 255, got %d", a)
	}
}
