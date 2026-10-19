package sim

// Port of reference/tower-together/sim/stress/add-delay.ts.

// AddDelayToCurrentSim adds a fixed tick penalty to the current-trip elapsed
// window, clamped to 300. Housekeeping excluded.
// TS: stress/add-delay.ts addDelayToCurrentSim (binary 11e0:02f7)
func AddDelayToCurrentSim(sim *SimRecord, delta int) {
	if sim.FamilyCode == FamilyHousekeeping {
		return
	}
	sim.ElapsedTicks = min(300, sim.ElapsedTicks+delta)
	sim.LastDemandTick = -1
}
