package main

import (
	"image"
	"image/color"
	"testing"
)

// swatch draws a synthetic photo: a white canvas, a colored rectangle (the
// "pack") with a thin margin of white (the "studio background") around it -
// proportioned like the real product photos this was tuned against (a few
// percent background, not a boundary case for the >40% guard) - and a white
// square *inside* the rectangle (a stand-in for the pack's own white
// text/logo), isolated from the border by colored pixels on every side.
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

func TestFloodFillTransparent_AllWhiteTripsGuardAndStaysOpaque(t *testing.T) {
	const size = 20
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, white)
		}
	}

	out := floodFillTransparent(img)

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if a := alphaAt(out, x, y); a != 255 {
				t.Fatalf("all-white image should trip the >%.0f%% guard and stay fully opaque; pixel (%d,%d) has alpha %d", maxBackgroundFraction*100, x, y, a)
			}
		}
	}
}
