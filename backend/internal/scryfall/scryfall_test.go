package scryfall_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Agmat/rip/backend/internal/scryfall"
)

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestParseCollection(t *testing.T) {
	cr, err := scryfall.ParseCollection(mustReadFile(t, "testdata/collection.json"))
	if err != nil {
		t.Fatalf("ParseCollection: %v", err)
	}
	if len(cr.Data) != 8 {
		t.Errorf("len(Data) = %d, want 8", len(cr.Data))
	}
	if len(cr.NotFound) != 0 {
		t.Errorf("NotFound = %v, want none", cr.NotFound)
	}
	for _, c := range cr.Data {
		if c.ImageURIs == nil {
			t.Errorf("card %s has no image_uris", c.Name)
		}
	}
}

func TestFetchCollection(t *testing.T) {
	fixture := mustReadFile(t, "testdata/collection.json")

	var gotMethod, gotPath, gotUA, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	cards, err := scryfall.FetchCollection(context.Background(), srv.Client(), srv.URL, []string{"id-1", "id-2"})
	if err != nil {
		t.Fatalf("FetchCollection: %v", err)
	}
	if len(cards) != 8 {
		t.Errorf("len(cards) = %d, want 8", len(cards))
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/cards/collection" {
		t.Errorf("path = %q, want /cards/collection", gotPath)
	}
	if gotUA == "" {
		t.Error("User-Agent header was not set (required by Scryfall)")
	}
	if gotAccept == "" {
		t.Error("Accept header was not set (required by Scryfall)")
	}
}

func TestFetchCollection_Batches75(t *testing.T) {
	// Scryfall caps /cards/collection at 75 identifiers per request; 80 ids
	// must produce two requests, not one oversized one.
	var requestSizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Identifiers []struct{ ID string } `json:"identifiers"`
		}
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		requestSizes = append(requestSizes, len(body.Identifiers))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"not_found":[]}`))
	}))
	defer srv.Close()

	ids := make([]string, 80)
	for i := range ids {
		ids[i] = fmt.Sprintf("id-%d", i)
	}

	if _, err := scryfall.FetchCollection(context.Background(), srv.Client(), srv.URL, ids); err != nil {
		t.Fatalf("FetchCollection: %v", err)
	}

	if len(requestSizes) != 2 {
		t.Fatalf("made %d requests, want 2", len(requestSizes))
	}
	if requestSizes[0] != 75 || requestSizes[1] != 5 {
		t.Errorf("batch sizes = %v, want [75 5]", requestSizes)
	}
}

func TestFetchCollection_NotFoundIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"not_found":[{"id":"missing"}]}`))
	}))
	defer srv.Close()

	_, err := scryfall.FetchCollection(context.Background(), srv.Client(), srv.URL, []string{"missing"})
	if err == nil {
		t.Fatal("expected an error when Scryfall reports not_found cards, got nil")
	}
}

func TestCard_ImageURIsJSON_SingleFaced(t *testing.T) {
	c := scryfall.Card{
		ID:        "abc",
		Name:      "Mountain",
		ImageURIs: &scryfall.ImageURIs{Normal: "https://example.com/mountain.jpg"},
	}

	data, err := c.ImageURIsJSON()
	if err != nil {
		t.Fatalf("ImageURIsJSON: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := got["image_uris"]; !ok {
		t.Error("expected an \"image_uris\" key for a single-faced card")
	}
	if _, ok := got["faces"]; ok {
		t.Error("did not expect a \"faces\" key for a single-faced card")
	}
}

func TestCard_ImageURIsJSON_DoubleFaced(t *testing.T) {
	c := scryfall.Card{
		ID:   "abc",
		Name: "Delver // Insectile Aberration",
		CardFaces: []scryfall.CardFace{
			{Name: "Delver", ImageURIs: scryfall.ImageURIs{Normal: "https://example.com/front.jpg"}},
			{Name: "Insectile Aberration", ImageURIs: scryfall.ImageURIs{Normal: "https://example.com/back.jpg"}},
		},
	}

	data, err := c.ImageURIsJSON()
	if err != nil {
		t.Fatalf("ImageURIsJSON: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := got["faces"]; !ok {
		t.Error("expected a \"faces\" key for a double-faced card")
	}
	if _, ok := got["image_uris"]; ok {
		t.Error("did not expect an \"image_uris\" key for a double-faced card")
	}
}

func TestCard_ImageURIsJSON_Neither(t *testing.T) {
	c := scryfall.Card{ID: "abc", Name: "Broken"}
	if _, err := c.ImageURIsJSON(); err == nil {
		t.Fatal("expected an error for a card with neither image_uris nor card_faces")
	}
}
