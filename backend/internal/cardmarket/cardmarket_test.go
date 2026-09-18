package cardmarket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestParsePriceGuide(t *testing.T) {
	data, err := os.ReadFile("testdata/price_guide.json")
	if err != nil {
		t.Fatal(err)
	}
	guide, err := ParsePriceGuide(data)
	if err != nil {
		t.Fatal(err)
	}

	if p := guide[781936]; p.Trend != 4.36 || p.TrendFoil != 0 {
		t.Errorf("booster price = %+v, want Trend 4.36, TrendFoil 0", p)
	}
	if p := guide[796513]; p.Trend != 19.89 || p.TrendFoil != 24.83 {
		t.Errorf("card price = %+v, want Trend 19.89, TrendFoil 24.83", p)
	}
	// null trend decodes to 0, i.e. "no price", rather than failing the whole guide.
	if p := guide[1]; p.Trend != 0 {
		t.Errorf("null trend = %v, want 0", p.Trend)
	}
	if _, ok := guide[999999]; ok {
		t.Error("unknown product should be absent")
	}
}

func TestFetchPriceGuide(t *testing.T) {
	data, err := os.ReadFile("testdata/price_guide.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/guide.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	guide, err := FetchPriceGuide(context.Background(), srv.Client(), srv.URL+"/guide.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(guide) != 3 {
		t.Errorf("len(guide) = %d, want 3", len(guide))
	}

	if _, err := FetchPriceGuide(context.Background(), srv.Client(), srv.URL+"/missing.json"); err == nil {
		t.Error("expected error for 404, got nil")
	}
}
