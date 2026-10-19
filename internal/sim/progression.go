package sim

// Port of reference/tower-together/sim/progression.ts: star advancement
// (binary check_and_advance_star_rating @ 1148:002d /
// compute_tower_tier_from_ledger @ 1148:041d / reset_star_gate_state) and the
// primary family ledger buckets.
//
// tryAdvanceStarCount runs every tick from FUN_1098_03ab (mirrored at the
// top of carrierTick in tick/carrier-tick.ts). Both the ledger-tier check
// and the qualitative gate must pass; starCount is capped at 5 here (rank 6
// "Tower" comes only from the cathedral path).

import "strconv"

// ComputeStarCountFromPopulation TS: progression.ts computeStarCountFromPopulation
// (binary compute_tower_tier_from_ledger 1148:041d). Thresholds
// [300, 1000, 5000, 10000, 15000], compared with >=.
func ComputeStarCountFromPopulation(world *WorldState) int {
	total := world.CurrentPopulation
	tier := 1
	for index := 0; index < len(StarThresholds); index++ {
		if total >= StarThresholds[index] {
			tier = index + 2
		}
	}
	return tier
}

// AddToPopulationBucket mirrors add_to_primary_family_ledger_bucket
// (1068:07f7; 1068:0812-082b): bucket[family] += amount and total += amount.
// TS: progression.ts addToPopulationBucket
func AddToPopulationBucket(world *WorldState, familyCode int, amount int) {
	if world.CurrentPopulationBuckets == nil {
		world.CurrentPopulationBuckets = map[int]int{}
	}
	world.CurrentPopulationBuckets[familyCode] += amount
	world.CurrentPopulation += amount
}

// ClearPopulationBucket mirrors clear_primary_family_ledger_bucket
// (1068:07b3; 1068:07d3-07e2): subtract the bucket from the total, then zero
// it. TS: progression.ts clearPopulationBucket
func ClearPopulationBucket(world *WorldState, familyCode int) {
	if world.CurrentPopulationBuckets == nil {
		world.CurrentPopulationBuckets = map[int]int{}
	}
	current := world.CurrentPopulationBuckets[familyCode]
	world.CurrentPopulation -= current
	world.CurrentPopulationBuckets[familyCode] = 0
}

// CheckStarAdvancementConditions TS: progression.ts checkStarAdvancementConditions
func CheckStarAdvancementConditions(world *WorldState, time *TimeState) bool {
	flags := &world.GateFlags
	switch world.StarCount {
	case 1:
		return true
	case 2:
		return flags.SecurityPlaced != 0
	case 3:
		return flags.OfficePlaced != 0 &&
			flags.RecyclingAdequate != 0 &&
			flags.OfficeServiceOk != 0 &&
			flags.OfficeServiceOkMedical != 0 &&
			flags.RoutesViable != 0 &&
			time.DaypartIndex >= 4 &&
			time.WeekendFlag == 0
	case 4:
		return flags.MetroPlaced != 0 &&
			flags.RecyclingAdequate != 0 &&
			flags.OfficeServiceOkMedical != 0 &&
			flags.RoutesViable != 0 &&
			time.DaypartIndex >= 4 &&
			time.WeekendFlag == 0
	default:
		return false
	}
}

// ResetStarGateState TS: progression.ts resetStarGateState (binary reset_star_gate_state)
func ResetStarGateState(world *WorldState) {
	world.GateFlags.OfficeServiceOk = 0
}

// TryAdvanceStarCount TS: progression.ts tryAdvanceStarCount
// (binary check_and_advance_star_rating 1148:002d).
func TryAdvanceStarCount(world *WorldState, time *TimeState) bool {
	if world.StarCount >= 5 {
		return false
	}
	if ComputeStarCountFromPopulation(world) <= world.StarCount {
		return false
	}
	if !CheckStarAdvancementConditions(world, time) {
		return false
	}
	world.StarCount++
	ResetStarGateState(world)
	world.PendingNotifications = append(world.PendingNotifications, SimNotification{
		Kind:    "star_advanced",
		Message: strconv.Itoa(world.StarCount),
	})
	return true
}
