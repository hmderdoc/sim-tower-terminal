package sim

// Port of reference/tower-together/sim/families/retail.ts.
// 1228:3ed9 gate_object_family_retail_state_handler (family 10)
// 1228:40c0 dispatch_object_family_retail_state_handler (family 10)
//
// The TS re-exports handleCommercialSimArrival / processCommercialSim from
// sims/commercial.ts; in the flat Go package those are already visible
// (HandleCommercialSimArrival, ProcessCommercialSim in sims_commercial.go).

// GateObjectFamilyRetailStateHandler TS: families/retail.ts
// gateObjectFamilyRetailStateHandler (binary 1228:3ed9)
func GateObjectFamilyRetailStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if sim.FamilyCode != FamilyRetail {
		return
	}
	if IsSimInTransit(sim.StateCode) {
		// Commercial families do NOT dispatch through dispatch_sim_behavior
		// in the binary; the queued-wait timeout is not wired for them.
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		return
	}
	ProcessCommercialSim(world, ledger, time, sim)
}

// DispatchObjectFamilyRetailStateHandler TS: families/retail.ts
// dispatchObjectFamilyRetailStateHandler (binary 1228:40c0)
func DispatchObjectFamilyRetailStateHandler(world *WorldState, sim *SimRecord, arrivalFloor int, time *TimeState) {
	HandleCommercialSimArrival(world, sim, arrivalFloor, time)
}
