package sim

// Port of reference/tower-together/sim/families/force-dispatch.ts (binary
// 1228:1614 force_dispatch_sim_state_by_family).

// ForceDispatchSimStateByFamily TS: families/force-dispatch.ts
// forceDispatchSimStateByFamily (binary 1228:1614). Strips only the 0x40
// in-transit bit (the TS phase bytes 0x20..0x27 carry 0x20), then re-enters
// the family refresh handler.
func ForceDispatchSimStateByFamily(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if sim.StateCode&SimStateInTransitBit != 0 {
		SetSimInTransit(sim, false)
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
		// TODO (TS): entertainment (18/29), recycling (33), parking (36).
	}
}
