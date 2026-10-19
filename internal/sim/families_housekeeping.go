package sim

// Port of reference/tower-together/sim/families/housekeeping.ts: the
// binary-named entry points of the housekeeping helper (family 0x0f) state
// machine, all driven by sims_housekeeping.go.

// GateHousekeepingRoomClaimState TS: families/housekeeping.ts
// gateHousekeepingRoomClaimState (binary 1228:5f39).
func GateHousekeepingRoomClaimState(world *WorldState, time *TimeState, sim *SimRecord) {
	ProcessHousekeepingSim(world, time, sim)
}

// UpdateObjectFamilyHousekeepingConnectionState TS: families/housekeeping.ts
// updateObjectFamilyHousekeepingConnectionState (binary 1228:602b).
func UpdateObjectFamilyHousekeepingConnectionState(world *WorldState, time *TimeState, sim *SimRecord) {
	ProcessHousekeepingSim(world, time, sim)
}

// ActivateObjectFamilyHousekeepingConnectionState TS: families/housekeeping.ts
// activateObjectFamilyHousekeepingConnectionState (binary 1228:6480).
// NOTE: MaybeStartHousekeepingRoomClaim calls this with a nil time (the TS
// passes `null as never`); see HandleHousekeepingSimArrival for why that
// path never dereferences it.
func ActivateObjectFamilyHousekeepingConnectionState(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	HandleHousekeepingSimArrival(world, time, sim, arrivalFloor)
}
