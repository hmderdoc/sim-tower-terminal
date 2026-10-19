package sim

// Port of reference/tower-together/sim/sims/trip-counters.ts. The TS file
// also re-exports the stress/* accessors; in Go they are simply the
// package-level functions in stress_*.go.

// ResetSimTripCounters clears trip counters. Spec: reset_sim_trip_counters.
// TS: sims/trip-counters.ts resetSimTripCounters
func ResetSimTripCounters(sim *SimRecord) {
	sim.TripCount = 0
	sim.AccumulatedTicks = 0
}

// hotelFamilyCodes TS: sims/trip-counters.ts HOTEL_FAMILY_CODES
var hotelFamilyCodes = map[int]bool{FamilyHotelSingle: true, FamilyHotelTwin: true, FamilyHotelSuite: true}

// ResetFacilitySimTripCounters: binary reset_facility_sim_trip_counters @
// 1138:0d07. Hotels skip the baseOffset=0 occupant.
// TS: sims/trip-counters.ts resetFacilitySimTripCounters
func ResetFacilitySimTripCounters(world *WorldState, sim *SimRecord) {
	skipPrimary := hotelFamilyCodes[sim.FamilyCode]
	for _, sibling := range FindSiblingSims(world, sim) {
		if skipPrimary && sibling.BaseOffset == 0 {
			continue
		}
		ResetSimTripCounters(sibling)
	}
}
