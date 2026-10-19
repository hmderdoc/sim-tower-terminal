package sim

import "sort"

// Port of reference/tower-together/sim/resources.ts. Every table is kept
// verbatim, including values docs/mechanics.md flags as wrong for real play
// (party hall width 27, metro $45,000, lobby $3,000, ...): the trace
// fixtures run in freeBuild so costs do not affect parity. The verified
// gameplay costs live in catalog.go.

const StartingCash = 2_000_000

// ─── Tile registry ────────────────────────────────────────────────────────────

// TileWidths is the width in grid cells for each placeable tile type.
var TileWidths = map[string]int{
	// Infrastructure
	"floor":           1,
	"lobby":           1,
	"stairs":          8,
	"elevator":        4,
	"elevatorExpress": 6,
	"elevatorService": 4,
	"escalator":       8,
	// Hotels (families 3/4/5)
	"hotelSingle": 4,
	"hotelTwin":   6,
	"hotelSuite":  10,
	// Commercial (families 6/0x0a/0x0c)
	"restaurant": 24,
	"fastFood":   16,
	"retail":     12,
	// Office (family 7)
	"office": 9,
	// Condo (family 9)
	"condo": 16,
	// Entertainment (families 0x12/0x1d)
	"cinema":    31,
	"partyHall": 27,
	"cathedral": 28,
	// Services
	"recyclingCenter":      25, // virtual two-floor stack placement
	"recyclingCenterUpper": 25, // family 0x14 upper slice
	"recyclingCenterLower": 25, // family 0x15 lower slice
	"parking":              4,  // family 0x18
	"parkingRamp":          15, // family 0x2c
	"security":             16, // family 0x0e
	"metro":                30, // families 0x1f/0x20/0x21, three-floor stack
	"housekeeping":         15, // family 0x0f
	"medical":              26, // family 0x0d
}

// TileCosts is the one-time construction cost in dollars (TS verbatim).
var TileCosts = map[string]int{
	"floor":                500,
	"lobby":                3_000,
	"stairs":               5_000,
	"elevator":             200_000,
	"elevatorExpress":      200_000,
	"elevatorService":      200_000,
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
	"recyclingCenter":      500_000, // combined cost
	"recyclingCenterUpper": 500_000,
	"recyclingCenterLower": 0,
	"parking":              5_000,
	"parkingRamp":          50_000,
	"security":             100_000,
	"metro":                45_000,
	"housekeeping":         50_000,
	"medical":              500_000,
}

// CarrierCarConstructionCost is the one-time cost to add a car, by carrier mode.
var CarrierCarConstructionCost = map[int]int{
	0: 150_000,
	1: 80_000,
	2: 50_000,
}

// CarrierExtendFloorCost is the per-floor cost of extending an existing
// shaft (binary charge_floor_range_construction_cost 1180:02e5).
var CarrierExtendFloorCost = map[int]int{
	0: 7_500, // express (width 6)
	1: 5_000, // standard (width 4)
	2: 5_000, // service (width 4)
}

// TileStarRequirements: minimum star rating before the build-menu entry
// appears. Tiles omitted are always available.
var TileStarRequirements = map[string]int{
	"lobby":                1,
	"floor":                1,
	"stairs":               1,
	"elevator":             1,
	"office":               1,
	"fastFood":             1,
	"condo":                1,
	"elevatorService":      2,
	"hotelSingle":          2,
	"hotelTwin":            2,
	"hotelSuite":           2,
	"housekeeping":         2,
	"security":             2,
	"escalator":            3,
	"elevatorExpress":      3,
	"restaurant":           3,
	"retail":               3,
	"partyHall":            3,
	"cinema":               3,
	"parking":              3,
	"parkingRamp":          3,
	"recyclingCenter":      3,
	"recyclingCenterUpper": 3,
	"recyclingCenterLower": 3,
	"medical":              3,
	"metro":                4,
	"cathedral":            5,
}

// GetTileStarRequirement TS: resources.ts getTileStarRequirement
func GetTileStarRequirement(tileType string) int {
	if v, ok := TileStarRequirements[tileType]; ok {
		return v
	}
	return 1
}

// ValidTileTypes = Object.keys(TILE_WIDTHS).
var ValidTileTypes = func() map[string]bool {
	out := make(map[string]bool, len(TileWidths))
	for k := range TileWidths {
		out[k] = true
	}
	return out
}()

// UndergroundAllowedTiles: tiles that may be placed at or below the
// underground row.
var UndergroundAllowedTiles = map[string]bool{
	"floor":                true,
	"stairs":               true,
	"escalator":            true,
	"elevator":             true,
	"elevatorExpress":      true,
	"elevatorService":      true,
	"parking":              true,
	"parkingRamp":          true,
	"metro":                true,
	"recyclingCenter":      true,
	"recyclingCenterUpper": true,
	"recyclingCenterLower": true,
	"restaurant":           true,
	"fastFood":             true,
	"retail":               true,
	"security":             true,
	"partyHall":            true,
	"cinema":               true,
	"medical":              true,
}

// ─── Family codes (object-type codes from the spec) ─────────────────────────

const (
	FamilyElevator             = 1
	FamilyEscalator            = 2
	FamilyHotelSingle          = 3
	FamilyHotelTwin            = 4
	FamilyHotelSuite           = 5
	FamilyRestaurant           = 6
	FamilyOffice               = 7
	FamilyCondo                = 9
	FamilyRetail               = 10
	FamilyFastFood             = 12
	FamilySecurity             = 14
	FamilyCinema               = 18
	FamilyCinemaLower          = 19
	FamilyRecyclingCenterUpper = 20
	FamilyRecyclingCenterLower = 21
	FamilyParking              = 24
	FamilyParkingRamp          = 44 // 0x2c
	FamilyMetroTop             = 0x1f
	FamilyMetroMiddle          = 0x20
	FamilyMetroBottom          = 0x21
	FamilyPartyHall            = 29
	FamilyPartyHallLower       = 30
	FamilyCinemaStairsUpper    = 34
	FamilyCinemaStairsLower    = 35
	FamilyCathedralAnchor      = 0x21
	FamilyHousekeeping         = 15
	FamilyMedical              = 13
	FamilyCathedralBase        = 0x24
	FamilyCathedralMax         = 0x28
)

// ─── Family code <-> tile name mappings ──────────────────────────────────────

// FamilyCodeToTile maps object-type codes to tile names. 0x21 is both
// FAMILY_METRO_BOTTOM and FAMILY_CATHEDRAL_ANCHOR; the TS literal only lists
// it under FAMILY_CATHEDRAL_ANCHOR, so 0x21 -> "cathedral".
var FamilyCodeToTile = map[int]string{
	FamilyElevator:             "elevator",
	FamilyEscalator:            "escalator",
	FamilyHotelSingle:          "hotelSingle",
	FamilyHotelTwin:            "hotelTwin",
	FamilyHotelSuite:           "hotelSuite",
	FamilyRestaurant:           "restaurant",
	FamilyOffice:               "office",
	FamilyCondo:                "condo",
	FamilyFastFood:             "fastFood",
	FamilyRetail:               "retail",
	FamilySecurity:             "security",
	FamilyCinema:               "cinema",
	FamilyRecyclingCenterUpper: "recyclingCenterUpper",
	FamilyRecyclingCenterLower: "recyclingCenterLower",
	FamilyParking:              "parking",
	FamilyParkingRamp:          "parkingRamp",
	FamilyPartyHall:            "partyHall",
	FamilyHousekeeping:         "housekeeping",
	FamilyMedical:              "medical",
	FamilyMetroTop:             "metro",
	FamilyMetroMiddle:          "metro",
	FamilyCathedralAnchor:      "cathedral",
}

var LegacyVipTileToStandard = map[string]string{
	"vipSingle": "hotelSingle",
	"vipTwin":   "hotelTwin",
	"vipSuite":  "hotelSuite",
}

var LegacyTileAliases = map[string]string{}

// TileToFamilyCode is built like the TS: Object.entries(FAMILY_CODE_TO_TILE)
// iterates integer keys ascending and Object.fromEntries keeps the last
// write, so "metro" -> 0x20 (not 0x1f). Then lobby -> FAMILY_PARKING (24),
// which is why every lobby tile gets a PlacedObjectRecord of family 0x18.
// PORT-NOTE: both quirks kept verbatim.
var TileToFamilyCode = func() map[string]int {
	codes := make([]int, 0, len(FamilyCodeToTile))
	for k := range FamilyCodeToTile {
		codes = append(codes, k)
	}
	sort.Ints(codes)
	out := make(map[string]int, len(codes)+1)
	for _, c := range codes {
		out[FamilyCodeToTile[c]] = c
	}
	out["lobby"] = FamilyParking
	return out
}()

// ─── Cinema movie titles ─────────────────────────────────────────────────────

var MovieTitles = []string{
	"Revenge of the Big Spider",
	"Northwest Romance",
	"Samurai Cop",
	"Big Wave",
	"Farewell to Morocco",
	"Fear of Shark Teeth",
	"Western Sheriff",
	"Dino Wars",
	"The Making of a Star",
	"Love in N.Y.",
	"Waikiki Moon",
	"My Man of War",
	"Christmas for Both of Us",
	"Casual Friends",
}

const (
	CinemaNewMovieCost     = 300_000
	CinemaClassicMovieCost = 150_000
)

// ─── YEN #1001 — payout table ─────────────────────────────────────────────────

// Yen1001 is income per checkout/activation event by tier (0=best, 3=worst),
// in units of $1,000. Fractional entries are kept as float64 like the JS
// numbers; every entry * 1000 is an exact integer.
var Yen1001 = map[string][]float64{
	"hotelSingle": {3, 2, 1.5, 0.5},
	"hotelTwin":   {4.5, 3, 2, 0.8},
	"hotelSuite":  {9, 6, 4, 1.5},
	"office":      {15, 10, 5, 2},
	"condo":       {200, 150, 100, 40},
	"retail":      {20, 15, 10, 4},
}

// ─── Commercial closure payouts ──────────────────────────────────────────────

// CommercialClosurePayouts: per-venue payout at daily closure by visitor
// band (<25, 25..34, 35..49, >=50), in $1,000 units.
var CommercialClosurePayouts = map[string][]int{
	"restaurant": {-6, 4, 6, 10},
	"fastFood":   {-3, 2, 3, 5},
	"retail":     {0, 0, 0, 0},
}

var CommercialClosureBands = [3]int{25, 35, 50}

// CommercialCapacityCaps: [phaseA, phaseB, override] per family
// (FUN_11b0_17d3).
var CommercialCapacityCaps = map[int][3]int{
	FamilyRestaurant: {35, 50, 25},
	FamilyFastFood:   {35, 50, 25},
	FamilyRetail:     {25, 30, 18},
}

// ─── YEN #1002 — expense table ────────────────────────────────────────────────

// QuarterlyExpenses: operating expenses every 3 days by family code, raw/10
// so that value * 1000 matches the binary's trace scale.
var QuarterlyExpenses = map[int]int{
	1:  10, // elevatorLocal, per active car per 3-day period
	14: 20, // security
	15: 10, // housekeeping
	20: 50, // recyclingCenterUpper
	27: 5,  // escalator
	42: 20, // elevatorExpress
	43: 10, // elevatorService
}

// ─── Operational score thresholds ─────────────────────────────────────────────

// OpScoreThresholds: star -> [low, high] thresholds.
var OpScoreThresholds = map[int][2]int{
	1: {80, 150},
	2: {80, 150},
	3: {80, 150},
	4: {80, 200},
	5: {80, 200},
}

// ParkingExpenseRateByStar: per-star rate; expense = width * rate / 10.
var ParkingExpenseRateByStar = map[int]int{
	1: 0,
	2: 0,
	3: 30,
	4: 100,
	5: 100,
}

// StarThresholds: score must exceed StarThresholds[star-1] to advance.
var StarThresholds = []int{300, 1000, 5000, 10_000, 15_000}

// ─── Route delay values ───────────────────────────────────────────────────────

const (
	DelayWaiting       = 5   // carrier floor-slot status 0x28 (at-capacity)
	DelayRequeueFail   = 0   // assign_request_to_runtime_route finds no transfer floor
	DelayRouteFail     = 300 // select_best_route_candidate returns < 0
	DelayVenueUnavail  = 0   // target venue slot invalid / no path-seed entry
	DelayStopEscalator = 16  // Escalator-branch per-stop delay
	DelayStopStairs    = 35  // Stairs-branch per-stop delay
)
