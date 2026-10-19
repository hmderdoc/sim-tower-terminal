package sim

// Gameplay catalogue: the verified construction costs from
// docs/mechanics.md section 2, keyed by the same tile names as
// resources.go TileCosts. This is what the door's build menu and its
// cost path use for real play. The TS tables in resources.go stay
// verbatim for trace parity (fixtures run with freeBuild, so costs never
// affect parity). Widths are NOT overridden here: TileWidths stays as the
// TS has it until the trace test says otherwise.
//
// Known disagreements with resources.go TileCosts (asserted by
// catalog_test.go):
//   - lobby:           $5,000/seg here vs TS $3,000
//   - elevatorExpress: $400,000 shaft vs TS $200,000
//   - elevatorService: $100,000 shaft vs TS $200,000
//   - parking:         $3,000 vs TS $5,000
//   - metro:           $1,000,000 vs TS $45,000

// CatalogCosts: one-time construction cost in dollars for real play.
var CatalogCosts = map[string]int{
	"floor":                500,
	"lobby":                5_000,
	"stairs":               5_000,
	"elevator":             200_000,
	"elevatorExpress":      400_000,
	"elevatorService":      100_000,
	"escalator":            20_000,
	"hotelSingle":          20_000,
	"hotelTwin":            50_000,
	"hotelSuite":           100_000,
	"restaurant":           200_000,
	"fastFood":             100_000,
	"retail":               100_000,
	"office":               40_000,
	"condo":                80_000,
	"cinema":               500_000,
	"partyHall":            100_000,
	"cathedral":            3_000_000,
	"recyclingCenter":      500_000, // combined two-floor stack
	"recyclingCenterUpper": 500_000,
	"recyclingCenterLower": 0,
	"parking":              3_000,
	"parkingRamp":          50_000,
	"security":             100_000,
	"metro":                1_000_000,
	"housekeeping":         50_000,
	"medical":              500_000,
}

// CatalogCarCosts: per-car cost by carrier mode (0 express, 1 standard,
// 2 service). Matches CarrierCarConstructionCost.
var CatalogCarCosts = map[int]int{
	0: 150_000,
	1: 80_000,
	2: 50_000,
}

// CatalogKnownCostErrors lists the tile names whose TS TileCosts value is
// known to be wrong for real play (mechanics.md section 2).
var CatalogKnownCostErrors = map[string]bool{
	"lobby":           true,
	"elevatorExpress": true,
	"elevatorService": true,
	"parking":         true,
	"metro":           true,
}

// CatalogCost returns the gameplay cost for a tile (0 when unknown).
func CatalogCost(tileType string) int {
	return CatalogCosts[tileType]
}
