// Package booster draws the cards for one opened pack from an MTGJSON
// booster config: pick one weighted pack variant, then for each of its
// slots draw a weighted card from the named sheet. The draw is a pure
// function of (config, seed) - same inputs always produce the same cards,
// which is what makes an open replayable and auditable without trusting
// client input.
package booster

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/Agmat/rip/backend/internal/mtgjson"
)

// Pick is one drawn card within an opened pack.
type Pick struct {
	Slot      int
	SheetName string
	Foil      bool
	CardUUID  string
}

// NewSeed returns 32 cryptographically random bytes to seed a pack open.
// Never derive a seed from anything client-supplied - it's the one piece of
// state that must be unpredictable for the draw to be fair.
func NewSeed() ([32]byte, error) {
	var seed [32]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return seed, fmt.Errorf("generate seed: %w", err)
	}
	return seed, nil
}

// Open deterministically draws a pack from cfg using seed: one pack variant
// chosen by weight, then each of its slots filled by a weighted draw from
// the corresponding sheet, without replacement within that sheet for this
// pack (a real pack can't contain the same physical card twice from one
// print sheet) unless the sheet's AllowDuplicates says otherwise.
//
// balanceColors (Wizards' color-balancing of commons within a pack) is not
// applied - a documented v1 simplification; sheet draws are the plain
// weighted-random MTGJSON describes. Fixed is decoded but unused: it only
// appears on booster types rip doesn't open yet (v1 is "play" only).
func Open(cfg mtgjson.BoosterConfig, seed [32]byte) (variantIndex int, picks []Pick, err error) {
	rng := rand.New(rand.NewChaCha8(seed))

	variantIndex, err = pickVariant(rng, cfg.Boosters)
	if err != nil {
		return 0, nil, err
	}
	variant := cfg.Boosters[variantIndex]

	// Contents is a map; Go randomizes map iteration order per-process, so
	// iterating it directly would consume the rng's sequential output in a
	// different order each run and break replay for the *same* seed. Sort
	// sheet names first so the draw sequence is a pure function of the
	// config, not of map iteration.
	sheetNames := make([]string, 0, len(variant.Contents))
	for name := range variant.Contents {
		sheetNames = append(sheetNames, name)
	}
	sort.Strings(sheetNames)

	slot := 0
	for _, name := range sheetNames {
		count := variant.Contents[name]
		sheet, ok := cfg.Sheets[name]
		if !ok {
			return 0, nil, fmt.Errorf("variant references unknown sheet %q", name)
		}

		uuids, err := drawFromSheet(rng, sheet, count)
		if err != nil {
			return 0, nil, fmt.Errorf("sheet %s: %w", name, err)
		}
		for _, u := range uuids {
			picks = append(picks, Pick{Slot: slot, SheetName: name, Foil: sheet.Foil, CardUUID: u})
			slot++
		}
	}

	return variantIndex, picks, nil
}

func pickVariant(rng *rand.Rand, boosters []mtgjson.BoosterPack) (int, error) {
	if len(boosters) == 0 {
		return 0, errors.New("booster config has no pack variants")
	}

	total := 0
	for _, b := range boosters {
		total += b.Weight
	}
	if total <= 0 {
		return 0, errors.New("booster config pack variants have zero total weight")
	}

	r := rng.IntN(total)
	cum := 0
	for i, b := range boosters {
		cum += b.Weight
		if r < cum {
			return i, nil
		}
	}
	return len(boosters) - 1, nil // unreachable given the loop above, kept as a safe fallback
}

// cardWeight pairs a card uuid with its sheet weight, sorted by uuid so the
// cumulative-weight order (and therefore which random draw picks which
// card) is deterministic rather than dependent on map iteration order.
type cardWeight struct {
	uuid   string
	weight int
}

// drawFromSheet draws `count` cards from sheet, removing each drawn card
// from the pool before the next draw unless the sheet allows duplicates.
func drawFromSheet(rng *rand.Rand, sheet mtgjson.BoosterSheet, count int) ([]string, error) {
	pool := make([]cardWeight, 0, len(sheet.Cards))
	for uuid, weight := range sheet.Cards {
		pool = append(pool, cardWeight{uuid: uuid, weight: weight})
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].uuid < pool[j].uuid })

	drawn := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if len(pool) == 0 {
			return nil, fmt.Errorf("sheet exhausted: need %d more card(s)", count-i)
		}

		total := 0
		for _, c := range pool {
			total += c.weight
		}
		if total <= 0 {
			return nil, errors.New("sheet has zero total weight")
		}

		r := rng.IntN(total)
		cum := 0
		idx := len(pool) - 1
		for j, c := range pool {
			cum += c.weight
			if r < cum {
				idx = j
				break
			}
		}

		drawn = append(drawn, pool[idx].uuid)
		if !sheet.AllowDuplicates {
			pool = append(pool[:idx], pool[idx+1:]...)
		}
	}

	return drawn, nil
}
