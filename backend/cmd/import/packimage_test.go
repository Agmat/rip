package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Agmat/rip/backend/internal/mtgjson"
)

// jpegSwatch draws a white canvas with a colored "pack" rectangle and a
// generous margin, like a real TCGplayer product photo - big enough that
// JPEG's block compression doesn't distort the corner background badly
// enough to trip the seed threshold, unlike packart_test.go's tiny swatch()
// (built for exact-pixel NRGBA assertions, not a JPEG round trip).
func jpegSwatch() image.Image {
	const size = 200
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	packColor := color.NRGBA{R: 200, G: 40, B: 40, A: 255}
	for y := range size {
		for x := range size {
			img.Set(x, y, white)
		}
	}
	for y := 20; y < size-20; y++ {
		for x := 20; x < size-20; x++ {
			img.Set(x, y, packColor)
		}
	}
	return img
}

func TestPackImageURL_PicksBoosterPackOfMatchingSubtype(t *testing.T) {
	var sf mtgjson.SetFile
	box := mtgjson.SealedProduct{Category: "booster_box", Subtype: "play"}
	box.Identifiers.TCGplayerProductID = "1"
	collector := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "collector"}
	collector.Identifiers.TCGplayerProductID = "2"
	play := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "play"}
	play.Identifiers.TCGplayerProductID = "562116"
	sf.Data.SealedProduct = []mtgjson.SealedProduct{box, collector, play}

	got, ok := packImageURL(&sf, "play", "https://cdn.example.com")
	if !ok || got != "https://cdn.example.com/562116.jpg" {
		t.Errorf("packImageURL = (%q, %v), want (https://cdn.example.com/562116.jpg, true)", got, ok)
	}
	if _, ok := packImageURL(&sf, "draft", "https://cdn.example.com"); ok {
		t.Error("packImageURL(draft) = ok true, want false (no matching subtype)")
	}
}

func TestPackImageURL_EmptyIDIsFalse(t *testing.T) {
	var sf mtgjson.SetFile
	play := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "play"}
	sf.Data.SealedProduct = []mtgjson.SealedProduct{play}

	if _, ok := packImageURL(&sf, "play", "https://cdn.example.com"); ok {
		t.Error("packImageURL with empty id = ok true, want false")
	}
}

func TestFetchPackImage_ReturnsTransparentPNG(t *testing.T) {
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, jpegSwatch(), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpegData.Bytes())
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	got, err := imp.fetchPackImage(context.Background(), srv.URL+"/562116.jpg")
	if err != nil {
		t.Fatalf("fetchPackImage: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode result as PNG: %v", err)
	}
	// cropToPack trims the background away rather than leaving it
	// transparent in place, so the result is smaller than the source photo.
	if b := decoded.Bounds(); b.Dx() >= 200 || b.Dy() >= 200 {
		t.Errorf("decoded bounds = %v, want smaller than the 200x200 source (background cropped away)", b)
	}
}

func TestFetchPackImage_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	imp := &importer{httpClient: srv.Client()}
	if _, err := imp.fetchPackImage(context.Background(), srv.URL+"/missing.jpg"); err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}
