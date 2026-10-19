package sim

// Port of reference/tower-together/sim/sim-access/state-bits.ts: sim
// state_code bit helpers (SimRecord+5: bits 0..3 phase, 0x20 waiting,
// 0x40 route queued / in transit).

const (
	SimStateWaitingBit   = 0x20
	SimStateInTransitBit = 0x40
	SimStateBaseMask     = 0x1f
	SimStateModeMask     = SimStateWaitingBit | SimStateInTransitBit
)

// IsSimWaiting TS: sim-access/state-bits.ts isSimWaiting
func IsSimWaiting(stateCode int) bool { return stateCode&SimStateWaitingBit != 0 }

// IsSimInTransit TS: sim-access/state-bits.ts isSimInTransit
func IsSimInTransit(stateCode int) bool { return stateCode&SimStateInTransitBit != 0 }

// SimBaseState strips the mode bits. TS: sim-access/state-bits.ts simBaseState
func SimBaseState(stateCode int) int { return stateCode & SimStateBaseMask }

// SetSimWaiting TS: sim-access/state-bits.ts setSimWaiting
func SetSimWaiting(sim *SimRecord, on bool) {
	if on {
		sim.StateCode = (sim.StateCode | SimStateWaitingBit) & 0xff
	} else {
		sim.StateCode = sim.StateCode &^ SimStateWaitingBit & 0xff
	}
}

// SetSimInTransit TS: sim-access/state-bits.ts setSimInTransit
func SetSimInTransit(sim *SimRecord, on bool) {
	if on {
		sim.StateCode = (sim.StateCode | SimStateInTransitBit) & 0xff
	} else {
		sim.StateCode = sim.StateCode &^ SimStateInTransitBit & 0xff
	}
}

// ClearSimRouteBits clears both mode bits (0x60).
// TS: sim-access/state-bits.ts clearSimRouteBits
func ClearSimRouteBits(sim *SimRecord) {
	sim.StateCode = sim.StateCode &^ SimStateModeMask & 0xff
}

// SetSimState overwrites the whole state byte. TS: sim-access/state-bits.ts setSimState
func SetSimState(sim *SimRecord, newCode int) {
	sim.StateCode = newCode & 0xff
}

// SetSimBaseState replaces the base-phase bits, keeping the mode bits.
// TS: sim-access/state-bits.ts setSimBaseState
func SetSimBaseState(sim *SimRecord, phase int) {
	sim.StateCode = ((sim.StateCode & SimStateModeMask) | (phase & SimStateBaseMask)) & 0xff
}
