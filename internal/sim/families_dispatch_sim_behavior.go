package sim

// Port of reference/tower-together/sim/families/dispatch-sim-behavior.ts
// (binary 1228:186c dispatch_sim_behavior).

// DispatchSimBehavior TS: families/dispatch-sim-behavior.ts dispatchSimBehavior
// (binary 1228:186c). Gated by FamilyPrologueTable (cs:1c71); in-transit
// sims go to the wait-timeout path only.
func DispatchSimBehavior(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	familyIndex := sim.FamilyCode - 3
	if _, ok := FamilyPrologueTable[familyIndex]; !ok {
		return
	}

	if IsSimInTransit(sim.StateCode) {
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		return
	}

	switch sim.FamilyCode {
	case FamilyHotelSingle, FamilyHotelTwin, FamilyHotelSuite:
		RefreshObjectFamilyHotelStateHandler(world, ledger, time, sim)
	case FamilyOffice:
		RefreshObjectFamilyOfficeStateHandler(world, ledger, time, sim)
	case FamilyCondo:
		RefreshObjectFamilyCondoStateHandler(world, ledger, time, sim)
	case FamilyRetail:
		GateObjectFamilyRetailStateHandler(world, ledger, time, sim)
	case FamilyRestaurant, FamilyFastFood:
		GateObjectFamilyRestaurantFastFoodStateHandler(world, ledger, time, sim)
	case FamilyHousekeeping:
		UpdateObjectFamilyHousekeepingConnectionState(world, time, sim)
	default:
		// Entertainment (18/29), recycling (33), parking (36): legacy TS branches.
	}
}
