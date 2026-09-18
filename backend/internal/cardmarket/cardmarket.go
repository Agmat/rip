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
