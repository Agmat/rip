package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func doRequest(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestListSets(t *testing.T) {
	srv := newTestServer(t)

	rec := doRequest(t, srv, http.MethodGet, "/v1/sets", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	resp := decodeJSON[struct {
		Sets []setSummary `json:"sets"`
	}](t, rec)
	if len(resp.Sets) != 1 {
		t.Fatalf("len(Sets) = %d, want 1", len(resp.Sets))
	}
	if resp.Sets[0].Code != "FDN" || resp.Sets[0].Name != "Foundations" {
		t.Errorf("set = %+v, want FDN/Foundations", resp.Sets[0])
	}
	if len(resp.Sets[0].BoosterTypes) != 1 || resp.Sets[0].BoosterTypes[0] != "play" {
		t.Errorf("booster types = %v, want [play]", resp.Sets[0].BoosterTypes)
	}
	if resp.Sets[0].PackPriceEUR == nil || *resp.Sets[0].PackPriceEUR != 4.36 {
		t.Errorf("pack_price_eur = %v, want 4.36", resp.Sets[0].PackPriceEUR)
	}
	if resp.Sets[0].PackPricedAt == nil {
		t.Error("pack_priced_at is nil, want the seeded refresh time")
	}
}

func TestOpenPack_And_GetPack_RoundTrip(t *testing.T) {
	srv := newTestServer(t)

	openRec := doRequest(t, srv, http.MethodPost, "/v1/packs/open", map[string]string{"set_code": "FDN"})
	if openRec.Code != http.StatusCreated {
		t.Fatalf("open status = %d, want 201; body: %s", openRec.Code, openRec.Body)
	}
	opened := decodeJSON[packOpenResponse](t, openRec)

	if opened.SetCode != "FDN" || opened.BoosterType != "play" {
		t.Errorf("open response = %+v, want set_code=FDN booster_type=play", opened)
	}
	// The fixture's one variant draws exactly 1 card from each of 6 sheets.
	if len(opened.Cards) != 6 {
		t.Fatalf("len(Cards) = %d, want 6", len(opened.Cards))
	}
	if opened.Pricing.PackPriceEUR == nil || *opened.Pricing.PackPriceEUR != 4.36 {
		t.Errorf("pricing.pack_price_eur = %v, want 4.36", opened.Pricing.PackPriceEUR)
	}
	// The draw is random, so check the summary against the picks it came with
	// rather than a fixed total.
	var wantTotal float64
	var wantUnpriced int
	for _, c := range opened.Cards {
		if c.PriceEUR == nil {
			wantUnpriced++
			continue
		}
		wantTotal += *c.PriceEUR
	}
	if opened.Pricing.TotalValueEUR != round2(wantTotal) {
		t.Errorf("pricing.total_value_eur = %v, want %v", opened.Pricing.TotalValueEUR, round2(wantTotal))
	}
	if opened.Pricing.UnpricedCards != wantUnpriced {
		t.Errorf("pricing.unpriced_cards = %d, want %d", opened.Pricing.UnpricedCards, wantUnpriced)
	}
	if wantUnpriced == 0 {
		t.Error("fixture should leave at least one pick unpriced (only one card is priced)")
	}
	if opened.Pricing.PricedAt == nil {
		t.Error("pricing.priced_at is nil, want the seeded refresh time")
	}
	for _, c := range opened.Cards {
		if c.Card.Name == "" {
			t.Errorf("card in slot %d has an empty name", c.Slot)
		}
		if len(c.Card.ImageURIs) == 0 {
			t.Errorf("card in slot %d has no image_uris", c.Slot)
		}
	}

	getRec := doRequest(t, srv, http.MethodGet, "/v1/packs/"+opened.OpenID.String(), nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want 200; body: %s", getRec.Code, getRec.Body)
	}
	fetched := decodeJSON[packOpenResponse](t, getRec)

	if fetched.OpenID != opened.OpenID {
		t.Errorf("fetched open_id = %v, want %v", fetched.OpenID, opened.OpenID)
	}
	if len(fetched.Cards) != len(opened.Cards) {
		t.Fatalf("fetched %d cards, want %d", len(fetched.Cards), len(opened.Cards))
	}
	for i := range opened.Cards {
		if fetched.Cards[i].Card.ID != opened.Cards[i].Card.ID {
			t.Errorf("slot %d: fetched card %v, want %v", i, fetched.Cards[i].Card.ID, opened.Cards[i].Card.ID)
		}
	}
}

func TestOpenPack_MissingSetCode(t *testing.T) {
	srv := newTestServer(t)
	rec := doRequest(t, srv, http.MethodPost, "/v1/packs/open", map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestOpenPack_UnknownSet(t *testing.T) {
	srv := newTestServer(t)
	rec := doRequest(t, srv, http.MethodPost, "/v1/packs/open", map[string]string{"set_code": "ZZZ"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
}

func TestGetPack_NotFound(t *testing.T) {
	srv := newTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/v1/packs/00000000-0000-0000-0000-000000000000", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body)
	}
}

func TestGetPack_InvalidID(t *testing.T) {
	srv := newTestServer(t)
	rec := doRequest(t, srv, http.MethodGet, "/v1/packs/not-a-uuid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

// TestOpenPack_Concurrent fires many simultaneous opens at the same server
// and checks each one completes successfully with its own distinct open,
// exercising the real transactional insert path under concurrency.
func TestOpenPack_Concurrent(t *testing.T) {
	srv := newTestServer(t)

	const n = 20
	var wg sync.WaitGroup
	// Each goroutine only writes its own slot in these slices and never
	// touches *testing.T - t.Fatalf/t.Helper are only valid from the
	// goroutine running the test, so all assertions happen after wg.Wait().
	bodies := make([][]byte, n)
	statuses := make([]int, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, err := json.Marshal(map[string]string{"set_code": "FDN"})
			if err != nil {
				panic(err) // marshaling a static map literal cannot fail
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/packs/open", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			srv.Routes().ServeHTTP(rec, req)
			statuses[i] = rec.Code
			bodies[i] = rec.Body.Bytes()
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, status := range statuses {
		if status != http.StatusCreated {
			t.Errorf("request %d: status = %d, want 201; body: %s", i, status, bodies[i])
			continue
		}
		var resp packOpenResponse
		if err := json.Unmarshal(bodies[i], &resp); err != nil {
			t.Errorf("request %d: decode response: %v", i, err)
			continue
		}
		id := resp.OpenID.String()
		if seen[id] {
			t.Errorf("request %d: open_id %s was returned by another request too", i, id)
		}
		seen[id] = true
	}
}
