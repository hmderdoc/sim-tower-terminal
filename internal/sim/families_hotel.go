package sim

// Port of reference/tower-together/sim/families/hotel.ts (binary 1228:2aec
// refresh_object_family_hotel_state_handler, 1228:2dae
// dispatch_object_family_hotel_state_handler).

// RefreshObjectFamilyHotelStateHandler TS: families/hotel.ts
// refreshObjectFamilyHotelStateHandler (binary 1228:2aec; cs:1c41).
func RefreshObjectFamilyHotelStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if IsSimInTransit(sim.StateCode) {
		if carrier := prologueCarrierForSim(world, sim); carrier != nil {
			DecrementRouteQueueDirectionLoad(carrier, sim.OriginFloor, 0)
		}
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		if !IsSimInTransit(sim.StateCode) {
			return
		}
	}
	ProcessHotelSim(world, ledger, time, sim)
}

// DispatchObjectFamilyHotelStateHandler TS: families/hotel.ts
// dispatchObjectFamilyHotelStateHandler (binary 1228:2dae, arrival dispatch).
func DispatchObjectFamilyHotelStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	HandleHotelSimArrival(world, ledger, time, sim, arrivalFloor)
}
