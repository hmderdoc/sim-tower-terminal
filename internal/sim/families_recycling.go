package sim

// Port of reference/tower-together/sim/families/recycling.ts.
// 1228:4d5b gate_object_family_recycling_center_lower_state_handler (family 33)
// 1228:4ea0 dispatch_object_family_recycling_center_lower_state_handler (family 33)
//
// Recycling-center lower-slice sims share the entertainment guest machine
// (dispatch_entertainment_guest_state 1228:53ad) behind a coarser gate.
// Binary table at cs:0x5221 (4 entries):
//   state 0x01 / 0x41 -> handler 0x530d (daypart+RNG gate then entertainment dispatch)
//   state 0x22 / 0x62 -> handler 0x537f (unconditional entertainment dispatch)
//
// PORT-NOTE: the TS comment says "family 33 / 0x21" but the code gates on
// FAMILY_RECYCLING_CENTER_LOWER, which is 21 (0x15). Ported as the code reads.

// Binary state constants for family 33 (match the entertainment guest machine).
const (
	stateRecyclingActive        = 0x01 // outbound / venue-seeking
	stateRecyclingVenueDwell    = 0x22 // at venue (pre-release)
	stateRecyclingActiveTransit = 0x41 // 0x01 + in-transit bit
	stateRecyclingDwellTransit  = 0x62 // 0x22 + in-transit bit
	stateRecyclingParked        = 0x27 // parked / idle
)

const (
	recyclingGateDayTickMin    = 0xf1  // dayTick must be >= this for state-0x01 to fire
	recyclingParkDayTick       = 0x8fd // dayTick >= this resets state-0x27 to 0x01
	recyclingGateRngModulo     = 0x24  // 36: gate fires on RNG % 36 == 0
	recyclingDispatchRngModulo = 6     // inner dispatch gate for state-0x01
)

// GateObjectFamilyRecyclingCenterLowerStateHandler TS: families/recycling.ts
// gateObjectFamilyRecyclingCenterLowerStateHandler (binary 1228:4d5b)
func GateObjectFamilyRecyclingCenterLowerStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if sim.FamilyCode != FamilyRecyclingCenterLower {
		return
	}

	if !IsSimInTransit(sim.StateCode) {
		state := sim.StateCode

		if state == stateRecyclingActive {
			// Binary quirk: gate on daypart 0-3, dayTick >= 0xf1, and 1-in-36 RNG.
			if time.DaypartIndex < 0 || time.DaypartIndex > 3 {
				return
			}
			if time.DayTick < recyclingGateDayTickMin {
				return
			}
			if SampleRng(world)%recyclingGateRngModulo != 0 {
				return
			}
		} else if state == stateRecyclingParked {
			// Binary quirk: state 0x27 resets itself to 0x01 once dayTick >= 0x8fd.
			if time.DayTick < recyclingParkDayTick {
				return
			}
			sim.StateCode = stateRecyclingActive
			return
		} else if state != stateRecyclingVenueDwell {
			return
		}
		// state 0x22 falls through to dispatch.
	} else {
		if sim.Route.Mode == RouteModeCarrier {
			MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
			return
		}
	}

	DispatchObjectFamilyRecyclingCenterLowerStateHandler(world, time, sim)
}

// DispatchObjectFamilyRecyclingCenterLowerStateHandler TS: families/recycling.ts
// dispatchObjectFamilyRecyclingCenterLowerStateHandler (binary 1228:4ea0).
// The binary prologue's decrement_route_queue_direction_load is not called
// here in the TS either (carried by the cancel path).
func DispatchObjectFamilyRecyclingCenterLowerStateHandler(world *WorldState, time *TimeState, sim *SimRecord) {
	state := sim.StateCode

	if state == stateRecyclingActive || state == stateRecyclingActiveTransit {
		// Handler 0x530d: second daypart + dayTick > 0xf0 + RNG%6==0 gate.
		// The && short-circuit keeps the RNG draw conditional, as in the TS.
		if time.DaypartIndex >= 0 && time.DaypartIndex <= 3 && time.DayTick > 0xf0 &&
			SampleRng(world)%recyclingDispatchRngModulo == 0 {
			DispatchEntertainmentGuestState(world, time, sim)
		}
		if time.DaypartIndex > 3 {
			sim.StateCode = stateRecyclingParked
		}
		return
	}

	if state == stateRecyclingVenueDwell || state == stateRecyclingDwellTransit {
		// Handler 0x537f: unconditional delegation.
		DispatchEntertainmentGuestState(world, time, sim)
		return
	}
}
