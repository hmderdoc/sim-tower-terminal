package sim

// Port of reference/tower-together/sim/families/restaurant.ts.
// 1228:466d gate_object_family_restaurant_fast_food_state_handler (family 6/12)
// 1228:4851 dispatch_object_family_restaurant_fast_food_state_handler (family 6/12)
//
// The dispatch body stays in ProcessCommercialSim (sims_commercial.go), which
// gates on familyCode for the restaurant/fast-food quirks.

// GateObjectFamilyRestaurantFastFoodStateHandler TS: families/restaurant.ts
// gateObjectFamilyRestaurantFastFoodStateHandler (binary 1228:466d)
func GateObjectFamilyRestaurantFastFoodStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if sim.FamilyCode != FamilyRestaurant && sim.FamilyCode != FamilyFastFood {
		return
	}
	if IsSimInTransit(sim.StateCode) {
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		return
	}
	ProcessCommercialSim(world, ledger, time, sim)
}

// DispatchObjectFamilyRestaurantFastFoodStateHandler TS: families/restaurant.ts
// dispatchObjectFamilyRestaurantFastFoodStateHandler (binary 1228:4851)
func DispatchObjectFamilyRestaurantFastFoodStateHandler(world *WorldState, sim *SimRecord, arrivalFloor int, time *TimeState) {
	HandleCommercialSimArrival(world, sim, arrivalFloor, time)
}
