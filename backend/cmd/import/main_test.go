package main

import (
	"sort"
	"testing"

	"github.com/Agmat/rip/backend/internal/mtgjson"
)

func TestSheetCardUUIDs_DedupesAcrossSheets(t *testing.T) {
	cfg := mtgjson.BoosterConfig{
		Sheets: map[string]mtgjson.BoosterSheet{
			"common": {Cards: map[string]int{"a": 1, "b": 1}},
			// "foil" reuses the same cards as "common", just weighted for foil odds.
			"foil": {Cards: map[string]int{"a": 100, "c": 1}},
		},
	}

	got := sheetCardUUIDs(cfg)
	sort.Strings(got)

	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("sheetCardUUIDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sheetCardUUIDs = %v, want %v", got, want)
		}
	}
}

func TestMcmID(t *testing.T) {
	if got := mcmID("796513"); !got.Valid || got.Int32 != 796513 {
		t.Errorf("mcmID(\"796513\") = %+v, want valid 796513", got)
	}
	if got := mcmID(""); got.Valid {
		t.Errorf("mcmID(\"\") = %+v, want null", got)
	}
	if got := mcmID("abc"); got.Valid {
		t.Errorf("mcmID(\"abc\") = %+v, want null", got)
	}
}

func TestPackMCMID_PicksBoosterPackOfMatchingSubtype(t *testing.T) {
	var sf mtgjson.SetFile
	box := mtgjson.SealedProduct{Category: "booster_box", Subtype: "play"}
	box.Identifiers.MCMID = "781940"
	collector := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "collector"}
	collector.Identifiers.MCMID = "1"
	play := mtgjson.SealedProduct{Category: "booster_pack", Subtype: "play"}
	play.Identifiers.MCMID = "781936"
	sf.Data.SealedProduct = []mtgjson.SealedProduct{box, collector, play}

	if got := packMCMID(&sf, "play"); !got.Valid || got.Int32 != 781936 {
		t.Errorf("packMCMID = %+v, want valid 781936", got)
	}
	if got := packMCMID(&sf, "draft"); got.Valid {
		t.Errorf("packMCMID(draft) = %+v, want null", got)
	}
}
