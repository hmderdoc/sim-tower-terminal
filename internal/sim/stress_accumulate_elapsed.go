package sim

// Port of reference/tower-together/sim/stress/accumulate-elapsed.ts.

// AccumulateElapsedDelayIntoCurrentSim: binary 11e0:01f1
// accumulate_elapsed_delay_into_current_sim. Composed from the rebase and
// lobby-reduction primitives, like the TS.
// TS: stress/accumulate-elapsed.ts accumulateElapsedDelayIntoCurrentSim (binary 11e0:01f1)
func AccumulateElapsedDelayIntoCurrentSim(world *WorldState, time *TimeState, sim *SimRecord, sourceFloor int) {
	RebaseSimElapsedFromClock(sim, time)
	ReduceElapsedForLobbyBoarding(&sim.ElapsedTicks, sourceFloor, world)
}
