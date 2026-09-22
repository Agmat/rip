package main

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/Agmat/rip/backend/internal/mtgjson"
)

const (
	// DefaultTCGplayerImageBaseURL is TCGplayer's public image CDN.
	// Overridable so tests can point packImageURL/fetchPackImage at an
	// httptest server instead of the real network.
	DefaultTCGplayerImageBaseURL = "https://product-images.tcgplayer.com/fit-in/600x600"

	// userAgent identifies this project, matching internal/scryfall's own
	// policy of always sending a descriptive one.
	userAgent = "rip-import/0.1 (+https://github.com/Agmat/rip)"
)

// packImageURL builds a set's Play Booster product photo URL from the
// MTGJSON sealedProduct entry with category "booster_pack" and a matching
// subtype - the same entry packMCMID uses for the price id. Neither MTGJSON
// nor Scryfall host pack art directly, but MTGJSON does carry the
// TCGplayer product id needed to construct the CDN URL. false means no such
// id exists for this set; the caller imports without a pack image.
func packImageURL(sf *mtgjson.SetFile, boosterType, baseURL string) (string, bool) {
	for _, p := range sf.Data.SealedProduct {
		if p.Category == "booster_pack" && p.Subtype == boosterType {
			id := p.Identifiers.TCGplayerProductID
			if id == "" {
				return "", false
			}
			return fmt.Sprintf("%s/%s.jpg", baseURL, id), true
		}
	}
	return "", false
}

// fetchPackImage downloads a pack photo from TCGplayer and strips its
// studio white background to transparency via removeWhiteBackground.
func (imp *importer) fetchPackImage(ctx context.Context, url string) ([]byte, error) {
	jpegData, err := imp.fetchBytes(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch pack image: %w", err)
	}
	return removeWhiteBackground(jpegData)
}

func (imp *importer) fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := imp.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}
