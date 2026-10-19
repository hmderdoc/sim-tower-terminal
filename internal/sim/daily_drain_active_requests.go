package sim

// Port of reference/tower-together/sim/daily/drain-active-requests.ts.

// DispatchActiveRequestsByFamily mirrors 1190:0977
// dispatch_active_requests_by_family: once per day at the 0x9c4 checkpoint,
// every sim with the 0x40 in-transit bit is routed through
// dispatchSimBehavior (1228:186c) so the route-failure timeout can fire.
// TS: daily/drain-active-requests.ts dispatchActiveRequestsByFamily
//
// PORT-NOTE: the TS `for..of world.sims` is a live array iteration, while
// Go's range snapshots the slice length. They differ only if the dispatch
// appends to world.Sims in place (reassigning world.Sims behaves the same in
// both). Revisit if a trace diverges here.
func DispatchActiveRequestsByFamily(world *WorldState, ledger *LedgerState, time *TimeState) {
	for _, sim := range world.Sims {
		if !IsSimInTransit(sim.StateCode) {
			continue
		}
		DispatchSimBehavior(world, ledger, time, sim)
	}
}
