package sim

import "testing"

// The gameplay catalogue (catalog.go) must agree with the verbatim TS
// tables (resources.go) except for the known, documented errors.
func TestCatalogAgreesWithTSExceptKnownErrors(t *testing.T) {
	for tile, tsCost := range TileCosts {
		gameCost, ok := CatalogCosts[tile]
		if !ok {
			t.Errorf("catalog missing %q", tile)
			continue
		}
		if CatalogKnownCostErrors[tile] {
			if gameCost == tsCost {
				t.Errorf("%q listed as a known error but costs agree (%d)", tile, tsCost)
			}
			continue
		}
		if gameCost != tsCost {
			t.Errorf("%q: catalog %d vs TS %d (not a listed known error)", tile, gameCost, tsCost)
		}
	}
	for tile := range CatalogCosts {
		if _, ok := TileCosts[tile]; !ok {
			t.Errorf("catalog has extra tile %q", tile)
		}
	}
	for mode, cost := range CarrierCarConstructionCost {
		if CatalogCarCosts[mode] != cost {
			t.Errorf("car cost mode %d: %d vs %d", mode, CatalogCarCosts[mode], cost)
		}
	}
	want := map[string]int{"lobby": 5_000, "elevatorExpress": 400_000, "elevatorService": 100_000, "parking": 3_000, "metro": 1_000_000}
	for tile, c := range want {
		if CatalogCost(tile) != c {
			t.Errorf("%q gameplay cost %d want %d", tile, CatalogCost(tile), c)
		}
	}
}
