package sim

// Port of reference/tower-together/sim/recycling.ts: recycling-center
// daily duty-tier reset, adequacy calculation and unit-status updates.

// ResetRecyclingCenterDutyTier TS: recycling.ts resetRecyclingCenterDutyTier
// (binary reset_recycling_center_daily 0x109001d1, tick 32).
func ResetRecyclingCenterDutyTier(world *WorldState) {
	for _, object := range world.PlacedObjects.Values() {
		if object.ObjectTypeCode == FamilyRecyclingCenterLower && object.UnitStatus == 6 {
			object.UnitStatus = 0
		}
	}
}

// UpdateRecyclingCenterState TS: recycling.ts updateRecyclingCenterState
// (binary update_recycling_center_state 0x10900000).
func UpdateRecyclingCenterState(world *WorldState, ledger *LedgerState, param int) {
	if world.StarCount <= 2 {
		world.GateFlags.RecyclingAdequate = 0
		return
	}

	if param == 0 {
		world.GateFlags.RecyclingAdequate = 0
		if world.GateFlags.RecyclingCenterCount == 0 {
			return
		}
		for _, object := range world.PlacedObjects.Values() {
			if object.ObjectTypeCode == FamilyRecyclingCenterUpper || object.ObjectTypeCode == FamilyRecyclingCenterLower {
				if object.UnitStatus == 5 {
					continue
				}
				object.UnitStatus = 0
			}
		}
		return
	}

	scale := max(0, world.GateFlags.RecyclingCenterCount)
	if scale == 0 {
		world.GateFlags.RecyclingAdequate = 0
		return
	}

	primaryTotal := 0
	for _, value := range ledger.PopulationLedger {
		primaryTotal += value
	}
	scaled := primaryTotal / scale // Math.trunc
	var requiredTier int
	switch {
	case scaled < 500:
		requiredTier = 1
	case scaled < 1000:
		requiredTier = 2
	case scaled < 1500:
		requiredTier = 3
	case scaled < 2000:
		requiredTier = 4
	case scaled < 2500:
		requiredTier = 5
	default:
		requiredTier = 6
	}
	adequate := 0
	if param >= requiredTier {
		adequate = 1
	}
	dutyTier := param
	if adequate != 0 {
		dutyTier = min(requiredTier, param)
	}

	world.GateFlags.RecyclingAdequate = adequate
	for _, object := range world.PlacedObjects.Values() {
		if object.ObjectTypeCode == FamilyRecyclingCenterUpper || object.ObjectTypeCode == FamilyRecyclingCenterLower {
			if adequate == 0 && object.UnitStatus == 5 {
				continue
			}
			object.UnitStatus = dutyTier
		}
	}
}
