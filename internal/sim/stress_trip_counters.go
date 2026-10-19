package sim

// Port of reference/tower-together/sim/stress/trip-counters.ts.

// AdvanceSimTripCounters captures a completed trip into the trip counters
// and resets the per-trip window. Housekeeping excluded.
// TS: stress/trip-counters.ts advanceSimTripCounters (binary 11e0:0000)
func AdvanceSimTripCounters(sim *SimRecord) {
	if sim.FamilyCode == FamilyHousekeeping {
		return
	}
	sim.TripCount++
	sim.AccumulatedTicks += sim.ElapsedTicks
	sim.ElapsedTicks = 0
	sim.LastDemandTick = -1
}
