// Package mtgjson fetches and decodes set data from mtgjson.com, in
// particular the booster block: the only structured source of real MTG
// booster-pack composition (which weighted variants exist, which named
// print sheets they draw from, and each card's weight within a sheet).
// Scryfall, by contrast, has no notion of pack composition at all.
package mtgjson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultBaseURL is mtgjson's per-set API host. Overridable so tests can
// point FetchSet at an httptest server instead of the real network.
const DefaultBaseURL = "https://mtgjson.com/api/v5"

// Card is the subset of an MTGJSON card object the importer needs: enough
// to upsert a row in `cards` and to resolve it to a Scryfall id for images.
type Card struct {
	UUID        string      `json:"uuid"`
	Name        string      `json:"name"`
	Rarity      string      `json:"rarity"`
	Number      string      `json:"number"`
	SetCode     string      `json:"setCode"`
	Finishes    []string    `json:"finishes"`
	Identifiers Identifiers `json:"identifiers"`
}

// Identifiers holds cross-references to other card databases: the Scryfall
// id (joined against Scryfall's image data) and Cardmarket's product id
// (joined against Cardmarket's price guide). MTGJSON encodes mcmId as a
// string; it's absent for cards Cardmarket doesn't list.
type Identifiers struct {
	ScryfallID string `json:"scryfallId"`
	MCMID      string `json:"mcmId"`
}

// BoosterSheet is one named print sheet: a weighted pool of cards a slot is
// drawn from. Weight is proportional, not a probability - e.g. a mythic in
// a combined rare/mythic sheet has a smaller weight than a rare so that
// mythics come up roughly 1-in-8 rather than equally often.
type BoosterSheet struct {
	Cards           map[string]int `json:"cards"` // card uuid -> weight
	TotalWeight     int            `json:"totalWeight"`
	Foil            bool           `json:"foil"`
	AllowDuplicates bool           `json:"allowDuplicates,omitempty"`
	Fixed           bool           `json:"fixed,omitempty"`
}

// BoosterPack is one weighted variant of a booster's contents, e.g. "788 of
// every 1000 packs are 7 common + 3 uncommon + ...; 12 of 1000 substitute a
// Special Guest slot". Contents maps a sheet name to how many cards that
// variant draws from it.
type BoosterPack struct {
	Contents map[string]int `json:"contents"` // sheet name -> slot count
	Weight   int            `json:"weight"`
}

// BoosterConfig is everything needed to open a pack of one product (e.g.
// Foundations' "play" booster): the weighted pack variants and the named
// sheets they draw from. Stored close to verbatim in booster_configs.config
// so an open can be replayed later against the exact config used.
type BoosterConfig struct {
	Boosters            []BoosterPack           `json:"boosters"`
	BoostersTotalWeight int                     `json:"boostersTotalWeight"`
	Name                string                  `json:"name,omitempty"`
	Sheets              map[string]BoosterSheet `json:"sheets"`
	SourceSetCodes      []string                `json:"sourceSetCodes"`
}

// SealedProduct is one purchasable product for a set (a booster pack, box,
// bundle, etc.) as MTGJSON models it. Only enough to find "the play booster
// pack" and its Cardmarket id, which resolves to the sealed booster's price.
// (Pack art no longer comes from here - see cmd/import/packimage.go.)
type SealedProduct struct {
	Category    string `json:"category"`
	Subtype     string `json:"subtype"`
	Identifiers struct {
		MCMID string `json:"mcmId"`
	} `json:"identifiers"`
}

// SetFile is the top-level shape of https://mtgjson.com/api/v5/<CODE>.json.
type SetFile struct {
	Data struct {
		Name          string                   `json:"name"`
		Code          string                   `json:"code"`
		Cards         []Card                   `json:"cards"`
		Booster       map[string]BoosterConfig `json:"booster"`
		SealedProduct []SealedProduct          `json:"sealedProduct"`
	} `json:"data"`
}

// ParseSetFile decodes raw JSON as returned by mtgjson.com/api/v5/<CODE>.json.
func ParseSetFile(data []byte) (*SetFile, error) {
	var sf SetFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("decode set file: %w", err)
	}
	return &sf, nil
}

// FetchSet downloads and decodes a set file for the given set code (e.g.
// "FDN"). baseURL is normally DefaultBaseURL; tests override it.
func FetchSet(ctx context.Context, client *http.Client, baseURL, code string) (*SetFile, error) {
	url := fmt.Sprintf("%s/%s.json", baseURL, code)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", code, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch set %s: %w", code, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch set %s: unexpected status %s", code, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read set %s response: %w", code, err)
	}

	sf, err := ParseSetFile(body)
	if err != nil {
		return nil, fmt.Errorf("set %s: %w", code, err)
	}
	return sf, nil
}
