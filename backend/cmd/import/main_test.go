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
