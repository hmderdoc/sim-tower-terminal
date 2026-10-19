package sim

// Port of reference/tower-together/sim/tick/carrier-tick.ts.
//
// 1098:03ab carrier_tick (FUN_1098_03ab). Order matches the binary:
//
//	check_and_advance_star_rating              (1148:002d)
//	refresh_runtime_entities_for_tick_stride   (1228:0d64)
//	for each carrier:
//	  for each active car: advance_carrier_car_state     (1098:06fb)
//	  for each active car: dispatch_carrier_car_arrivals (1218:07a6)
//	  for each active car: process_unit_travel_queue     (1218:0351)

// CarrierTick TS: tick/carrier-tick.ts carrierTick (binary 1098:03ab)
func CarrierTick(world *WorldState, ledger *LedgerState, time *TimeState) {
	// Binary check_and_advance_star_rating runs before
	// refresh_runtime_entities_for_tick_stride and may bump starCount.
	TryAdvanceStarCount(world, time)

	RefreshRuntimeEntitiesForTickStride(world, ledger, time)

	// PORT-NOTE: `for (const carrier of world.carriers)` is a live array
	// iterator; nothing in the tick reassigns world.carriers, so ranging
	// over the slice header is equivalent. Same for carrier.cars.
	for _, carrier := range world.Carriers {
		ResetCarrierTickBookkeeping(carrier)
		for carIndex, car := range carrier.Cars {
			AdvanceCarrierCarState(car, carrier, carIndex, time, world.LobbyMode)
		}
		for _, car := range carrier.Cars {
			DispatchCarrierCarArrivals(world, ledger, time, carrier, car)
		}
		for carIndex, car := range carrier.Cars {
			ProcessUnitTravelQueue(world, carrier, car, carIndex, time)
		}
	}

	ReconcileSimTransport(world, ledger, time)
}
