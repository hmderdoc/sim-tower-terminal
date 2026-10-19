package sim

// Port of reference/tower-together/sim/stress/rebase-elapsed.ts.

// RebaseSimElapsedFromClock syncs per-trip elapsed ticks with the day clock
// (clamped to 300) and clears lastDemandTick. Housekeeping excluded.
// TS: stress/rebase-elapsed.ts rebaseSimElapsedFromClock (binary 11e0:00fc)
func RebaseSimElapsedFromClock(sim *SimRecord, time *TimeState) {
	if sim.FamilyCode == FamilyHousekeeping {
		return
	}
	if sim.LastDemandTick >= 0 {
		sim.ElapsedTicks = min(300, sim.ElapsedTicks+time.DayTick-sim.LastDemandTick)
	}
	sim.LastDemandTick = -1
}
