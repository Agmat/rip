package mtgjson_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Agmat/rip/backend/internal/mtgjson"
)

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestParseSetFile(t *testing.T) {
	sf, err := mtgjson.ParseSetFile(mustReadFile(t, "testdata/fdn.json"))
	if err != nil {
		t.Fatalf("ParseSetFile: %v", err)
	}

	if sf.Data.Code != "FDN" {
		t.Errorf("Code = %q, want FDN", sf.Data.Code)
	}
	if len(sf.Data.Cards) != 7 {
		t.Errorf("len(Cards) = %d, want 7", len(sf.Data.Cards))
	}

	play, ok := sf.Data.Booster["play"]
	if !ok {
		t.Fatal("booster[\"play\"] missing")
	}
	if got, want := play.SourceSetCodes, []string{"FDN", "SPG"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("SourceSetCodes = %v, want %v", got, want)
	}

	common, ok := play.Sheets["common"]
	if !ok {
		t.Fatal("sheets[\"common\"] missing")
	}
	if len(common.Cards) != 2 {
		t.Errorf("common sheet has %d cards, want 2", len(common.Cards))
	}

	special, ok := play.Sheets["specialGuest"]
	if !ok {
		t.Fatal("sheets[\"specialGuest\"] missing")
	}
	if len(special.Cards) != 1 {
		t.Errorf("specialGuest sheet has %d cards, want 1 (from SPG)", len(special.Cards))
	}
}

func TestFetchSet(t *testing.T) {
	fixture := mustReadFile(t, "testdata/fdn.json")

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	sf, err := mtgjson.FetchSet(context.Background(), srv.Client(), srv.URL, "FDN")
	if err != nil {
		t.Fatalf("FetchSet: %v", err)
	}
	if sf.Data.Code != "FDN" {
		t.Errorf("Code = %q, want FDN", sf.Data.Code)
	}
	if gotPath != "/FDN.json" {
		t.Errorf("request path = %q, want /FDN.json", gotPath)
	}
}

func TestFetchSetList(t *testing.T) {
	fixture := mustReadFile(t, "testdata/setlist.json")

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	entries, err := mtgjson.FetchSetList(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("FetchSetList: %v", err)
	}
	if gotPath != "/SetList.json" {
		t.Errorf("request path = %q, want /SetList.json", gotPath)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Code != "FDN" || entries[0].ReleaseDate != "2024-11-15" {
		t.Errorf("entries[0] = %+v, want FDN 2024-11-15", entries[0])
	}
}

func TestFetchSet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := mtgjson.FetchSet(context.Background(), srv.Client(), srv.URL, "NOPE")
	if err == nil {
		t.Fatal("expected an error for a 404 response, got nil")
	}
}

func TestParseSetFile_McmIDs(t *testing.T) {
	data, err := os.ReadFile("testdata/fdn.json")
	if err != nil {
		t.Fatal(err)
	}
	sf, err := mtgjson.ParseSetFile(data)
	if err != nil {
		t.Fatal(err)
	}

	if got := sf.Data.Cards[0].Identifiers.MCMID; got != "796513" {
		t.Errorf("cards[0].Identifiers.MCMID = %q, want 796513", got)
	}

	var pack *mtgjson.SealedProduct
	for i := range sf.Data.SealedProduct {
		if sf.Data.SealedProduct[i].Category == "booster_pack" {
			pack = &sf.Data.SealedProduct[i]
		}
	}
	if pack == nil {
		t.Fatal("no booster_pack sealed product in fixture")
	}
	if pack.Identifiers.MCMID != "781936" {
		t.Errorf("booster_pack MCMID = %q, want 781936", pack.Identifiers.MCMID)
	}
	if pack.Identifiers.TCGplayerProductID != "562116" {
		t.Errorf("booster_pack TCGplayerProductID = %q, want 562116", pack.Identifiers.TCGplayerProductID)
	}
}
