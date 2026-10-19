package sim

// Port of reference/tower-together/sim/families/finalize.ts (binary 1228:1481
// finalize_runtime_route_state).

// FinalizeRuntimeRouteState TS: families/finalize.ts finalizeRuntimeRouteState
// (binary 1228:1481). Strips only the 0x40 in-transit bit: the TS state
// constants encode 0x20 as part of the phase byte.
func FinalizeRuntimeRouteState(_ *WorldState, sim *SimRecord) {
	wasActive := IsSimInTransit(sim.StateCode) ||
		IsSimWaiting(sim.StateCode) ||
		sim.Route.Mode != RouteModeIdle
	if wasActive {
		AdvanceSimTripCounters(sim)
	}
	ClearSimRoute(sim)
	SetSimInTransit(sim, false)
	sim.TransitTicksRemaining = 0
}
