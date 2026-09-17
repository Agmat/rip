// Package scryfall fetches card image data from api.scryfall.com. It never
// caches or republishes Scryfall's own data beyond image URLs (the Scryfall
// API guidelines prohibit repackaging card data); the importer only asks it
// "given these ids, where are the images".
package scryfall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultBaseURL is Scryfall's API host. Overridable so tests can point
// FetchCollection at an httptest server instead of the real network.
const DefaultBaseURL = "https://api.scryfall.com"

// userAgent identifies this project per Scryfall's required-header policy:
// https://scryfall.com/docs/api - "must be accurate to your usage context".
const userAgent = "rip-import/0.1 (+https://github.com/Agmat/rip)"

// collectionBatchSize is Scryfall's documented limit for POST /cards/collection.
const collectionBatchSize = 75

// ImageURIs mirrors Scryfall's image_uris object.
type ImageURIs struct {
	Small      string `json:"small,omitempty"`
	Normal     string `json:"normal,omitempty"`
	Large      string `json:"large,omitempty"`
	PNG        string `json:"png,omitempty"`
	ArtCrop    string `json:"art_crop,omitempty"`
	BorderCrop string `json:"border_crop,omitempty"`
}

// CardFace is one face of a double-faced card; only relevant fields kept.
type CardFace struct {
	Name      string    `json:"name"`
	ImageURIs ImageURIs `json:"image_uris"`
}

// Card is the subset of a Scryfall card object the importer needs. A
// single-faced card has ImageURIs; a double-faced card has CardFaces
// instead, one entry per face, each with its own images.
type Card struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	ImageURIs *ImageURIs `json:"image_uris,omitempty"`
	CardFaces []CardFace `json:"card_faces,omitempty"`
}

// CollectionResponse is the shape of POST /cards/collection.
type CollectionResponse struct {
	Data     []Card   `json:"data"`
	NotFound []Card   `json:"not_found"`
	Warnings []string `json:"warnings,omitempty"`
}

// ParseCollection decodes raw JSON as returned by POST /cards/collection.
func ParseCollection(data []byte) (*CollectionResponse, error) {
	var cr CollectionResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return nil, fmt.Errorf("decode collection response: %w", err)
	}
	return &cr, nil
}

// FetchCollection resolves Scryfall card ids to full card objects (image
// data in particular), batching requests at Scryfall's documented limit of
// 75 ids per call. baseURL is normally DefaultBaseURL; tests override it.
func FetchCollection(ctx context.Context, client *http.Client, baseURL string, ids []string) ([]Card, error) {
	var cards []Card

	for start := 0; start < len(ids); start += collectionBatchSize {
		end := min(start+collectionBatchSize, len(ids))

		batch, err := fetchBatch(ctx, client, baseURL, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("collection batch %d-%d: %w", start, end, err)
		}
		cards = append(cards, batch...)
	}

	return cards, nil
}

func fetchBatch(ctx context.Context, client *http.Client, baseURL string, ids []string) ([]Card, error) {
	type identifier struct {
		ID string `json:"id"`
	}
	payload := struct {
		Identifiers []identifier `json:"identifiers"`
	}{}
	for _, id := range ids {
		payload.Identifiers = append(payload.Identifiers, identifier{ID: id})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/cards/collection", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	cr, err := ParseCollection(respBody)
	if err != nil {
		return nil, err
	}
	if len(cr.NotFound) > 0 {
		return nil, fmt.Errorf("%d card(s) not found on Scryfall", len(cr.NotFound))
	}

	return cr.Data, nil
}

// storedImageURIs is the shape written to cards.image_uris: either a single
// set of images, or one per face for a double-faced card.
type storedImageURIs struct {
	ImageURIs *ImageURIs `json:"image_uris,omitempty"`
	Faces     []CardFace `json:"faces,omitempty"`
}

// ImageURIsJSON returns the JSON to store in cards.image_uris: the card's
// own image_uris for a single-faced card, or {"faces": [...]} for a
// double-faced one (Scryfall puts per-face images under card_faces instead
// of a top-level image_uris).
func (c Card) ImageURIsJSON() ([]byte, error) {
	if len(c.CardFaces) > 0 {
		return json.Marshal(storedImageURIs{Faces: c.CardFaces})
	}
	if c.ImageURIs != nil {
		return json.Marshal(storedImageURIs{ImageURIs: c.ImageURIs})
	}
	return nil, fmt.Errorf("card %s (%s) has neither image_uris nor card_faces", c.ID, c.Name)
}
