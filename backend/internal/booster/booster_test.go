package booster_test

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"

	"github.com/Agmat/rip/backend/internal/booster"
	"github.com/Agmat/rip/backend/internal/mtgjson"
)

// loadFDNConfig reuses the trimmed real-data fixture from internal/mtgjson
// rather than hand-writing a synthetic one: same booster config the import
// CLI's own tests exercise, so the draw engine is tested against exactly
// the shape it will see in production.
func loadFDNConfig(t *testing.T) mtgjson.BoosterConfig {
	t.Helper()
	data, err := os.ReadFile("../mtgjson/testdata/fdn.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	sf, err := mtgjson.ParseSetFile(data)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return sf.Data.Booster["play"]
}

// seedFromIndex builds a distinct, deterministic seed per index so
// statistical tests are reproducible across runs without touching
// crypto/rand.
func seedFromIndex(i int) [32]byte {
	var s [32]byte
	binary.LittleEndian.PutUint64(s[:8], uint64(i))
	return s
}

func TestOpen_ReplayIsExact(t *testing.T) {
	cfg := loadFDNConfig(t)
	seed := seedFromIndex(42)

	variant1, picks1, err := booster.Open(cfg, seed)
	if err != nil {
		t.Fatalf("Open (first): %v", err)
	}
	variant2, picks2, err := booster.Open(cfg, seed)
	if err != nil {
		t.Fatalf("Open (second): %v", err)
	}

	if variant1 != variant2 {
		t.Errorf("variant index differs across replays: %d vs %d", variant1, variant2)
	}
	if !reflect.DeepEqual(picks1, picks2) {
		t.Errorf("picks differ across replays:\n%+v\n%+v", picks1, picks2)
	}
}

func TestOpen_MatchesExpectedShape(t *testing.T) {
	cfg := loadFDNConfig(t)

	_, picks, err := booster.Open(cfg, seedFromIndex(1))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// The fixture's one variant draws exactly 1 card from each of 6 sheets.
	if len(picks) != 6 {
		t.Fatalf("len(picks) = %d, want 6", len(picks))
	}

	seenSheets := make(map[string]bool)
	for _, p := range picks {
		seenSheets[p.SheetName] = true
		if _, ok := cfg.Sheets[p.SheetName].Cards[p.CardUUID]; !ok {
			t.Errorf("picked card %s not present in sheet %s", p.CardUUID, p.SheetName)
		}
	}
	for name, sheet := range cfg.Sheets {
		if !seenSheets[name] {
			t.Errorf("sheet %s never drawn from", name)
		}
		_ = sheet
	}
}

func TestOpen_WithoutReplacementWithinSheet(t *testing.T) {
	cfg := mtgjson.BoosterConfig{
		Boosters: []mtgjson.BoosterPack{{Contents: map[string]int{"common": 2}, Weight: 1}},
		Sheets: map[string]mtgjson.BoosterSheet{
			"common": {Cards: map[string]int{"a": 1, "b": 1, "c": 1}},
		},
	}

	for i := 0; i < 200; i++ {
		_, picks, err := booster.Open(cfg, seedFromIndex(i))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if picks[0].CardUUID == picks[1].CardUUID {
			t.Fatalf("sheet without AllowDuplicates drew %q twice in one pack", picks[0].CardUUID)
		}
	}
}

func TestOpen_AllowDuplicatesCanRepeat(t *testing.T) {
	cfg := mtgjson.BoosterConfig{
		Boosters: []mtgjson.BoosterPack{{Contents: map[string]int{"foil": 2}, Weight: 1}},
		Sheets: map[string]mtgjson.BoosterSheet{
			"foil": {Cards: map[string]int{"only-card": 1}, AllowDuplicates: true, Foil: true},
		},
	}

	_, picks, err := booster.Open(cfg, seedFromIndex(0))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if picks[0].CardUUID != "only-card" || picks[1].CardUUID != "only-card" {
		t.Fatalf("picks = %+v, want both slots drawing the single allowed-duplicate card", picks)
	}
	if !picks[0].Foil || !picks[1].Foil {
		t.Errorf("picks = %+v, want Foil=true carried from the sheet", picks)
	}
}

func TestOpen_SheetExhaustedIsError(t *testing.T) {
	cfg := mtgjson.BoosterConfig{
		Boosters: []mtgjson.BoosterPack{{Contents: map[string]int{"common": 2}, Weight: 1}},
		Sheets: map[string]mtgjson.BoosterSheet{
			"common": {Cards: map[string]int{"only-card": 1}},
		},
	}

	if _, _, err := booster.Open(cfg, seedFromIndex(0)); err == nil {
		t.Fatal("expected an error when a sheet without AllowDuplicates can't fill its slots")
	}
}

func TestOpen_UnknownSheetIsError(t *testing.T) {
	cfg := mtgjson.BoosterConfig{
		Boosters: []mtgjson.BoosterPack{{Contents: map[string]int{"missing": 1}, Weight: 1}},
		Sheets:   map[string]mtgjson.BoosterSheet{},
	}

	if _, _, err := booster.Open(cfg, seedFromIndex(0)); err == nil {
		t.Fatal("expected an error when a variant references a sheet that doesn't exist")
	}
}

// TestOpen_SheetWeightsMatchDeclaredOdds simulates many pack opens and
// checks the observed pick frequency for the fixture's two-card
// rareMythicWithShowcase sheet (real FDN weights: 16 vs 14, i.e. ~53.3% vs
// ~46.7%) lands close to the declared weight ratio.
func TestOpen_SheetWeightsMatchDeclaredOdds(t *testing.T) {
	if testing.Short() {
		t.Skip("statistical simulation skipped under -short")
	}

	cfg := loadFDNConfig(t)
	sheet := cfg.Sheets["rareMythicWithShowcase"]
	if len(sheet.Cards) != 2 {
		t.Fatalf("fixture assumption changed: rareMythicWithShowcase has %d cards, want 2", len(sheet.Cards))
	}

	const n = 20000
	counts := make(map[string]int)
	for i := 0; i < n; i++ {
		_, picks, err := booster.Open(cfg, seedFromIndex(i))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, p := range picks {
			if p.SheetName == "rareMythicWithShowcase" {
				counts[p.CardUUID]++
			}
		}
	}

	for uuid, weight := range sheet.Cards {
		want := float64(weight) / float64(sheet.TotalWeight)
		got := float64(counts[uuid]) / float64(n)
		if diff := got - want; diff < -0.02 || diff > 0.02 {
			t.Errorf("card %s: observed rate %.4f, want ~%.4f (weight %d/%d)", uuid, got, want, weight, sheet.TotalWeight)
		}
	}
}

func TestNewSeed_ReturnsDistinctBytes(t *testing.T) {
	a, err := booster.NewSeed()
	if err != nil {
		t.Fatalf("NewSeed: %v", err)
	}
	b, err := booster.NewSeed()
	if err != nil {
		t.Fatalf("NewSeed: %v", err)
	}
	if a == b {
		t.Fatal("two NewSeed calls returned identical bytes (astronomically unlikely unless crypto/rand is broken)")
	}
}
