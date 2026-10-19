package sim

// Port of reference/tower-together/sim/families/condo.ts (binary 1228:3548
// refresh_object_family_condo_state_handler, 1228:3870
// dispatch_object_family_condo_state_handler).

// RefreshObjectFamilyCondoStateHandler TS: families/condo.ts
// refreshObjectFamilyCondoStateHandler (binary 1228:3548; cs:1c2d maps
// {0x40,0x41,0x60,0x61,0x62} to the 1228:1aba prologue).
func RefreshObjectFamilyCondoStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if IsSimInTransit(sim.StateCode) {
		if carrier := prologueCarrierForSim(world, sim); carrier != nil {
			DecrementRouteQueueDirectionLoad(carrier, sim.OriginFloor, 0)
		}
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		if !IsSimInTransit(sim.StateCode) {
			return
		}
	}
	ProcessCondoSim(world, ledger, time, sim)
}

// DispatchObjectFamilyCondoStateHandler TS: families/condo.ts
// dispatchObjectFamilyCondoStateHandler (binary 1228:3870, arrival dispatch).
func DispatchObjectFamilyCondoStateHandler(sim *SimRecord, arrivalFloor int, time *TimeState, arrivedViaCarrier bool) {
	HandleCondoSimArrival(sim, arrivalFloor, time, arrivedViaCarrier)
}
