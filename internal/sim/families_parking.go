package sim

// Port of reference/tower-together/sim/families/parking.ts.
// 1228:5b5a gate_object_family_parking_state_handler (family 36)
// 1228:5cd2 dispatch_object_family_parking_state_handler (family 36)
//
// The TS re-exports rebuildParkingDemandLog / tryAssignParkingService from
// sims/parking.ts (sims_parking.go in Go).
//
// Binary quirk: the dispatch jump table at cs:5f29 lists exactly four state
// codes. States 0x05/0x45 -> handle_family_parking_return_route (1228:5e7e);
// 0x20/0x60 -> handle_family_parking_outbound_route (1228:5ddd).

// handleFamilyParkingOutboundRoute TS: families/parking.ts
// handleFamilyParkingOutboundRoute (binary 1228:5ddd). Call site 1228:5e04
// passes is_passenger_route = 1, emit_distance_feedback = (state == 0x20).
func handleFamilyParkingOutboundRoute(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == StateMorningGate
	sourceFloor := sim.OriginFloor
	directionFlag := 0
	if isFreshDispatch {
		sourceFloor = LobbyFloor
		directionFlag = 1
	}

	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, EvalZoneFloor, directionFlag, time,
		resolveOptsFeedback(isFreshDispatch))

	if result == 3 {
		sim.StateCode = StateArrived
		CheckEvalCompletionAndAward(world, time, sim)
	} else if result == 0 || result == 1 || result == 2 {
		sim.StateCode = StateEvalOutbound // 0x60: in-transit to eval zone
	} else {
		sim.StateCode = StateParked // 0x27: route failure
	}
}

// handleFamilyParkingReturnRoute TS: families/parking.ts
// handleFamilyParkingReturnRoute (binary 1228:5e7e). Call site 1228:5ea5
// passes is_passenger_route = 1, emit_distance_feedback = (state == 0x05).
func handleFamilyParkingReturnRoute(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == StateDeparture
	sourceFloor := sim.OriginFloor
	directionFlag := 0
	if isFreshDispatch {
		sourceFloor = EvalZoneFloor
		directionFlag = 1
	}

	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor, directionFlag, time,
		resolveOptsFeedback(isFreshDispatch))

	if result == 0 || result == 1 || result == 2 {
		sim.StateCode = StateDepartureTransit // 0x45: in-transit back to lobby
	} else {
		// Binary quirk: both 3 (same-floor) and -1 (failure) park to 0x27.
		sim.StateCode = StateParked
	}
}

// runParkingDispatchJumpTable TS: families/parking.ts runDispatchJumpTable
// (renamed: the flat Go package needs a family-specific name).
func runParkingDispatchJumpTable(world *WorldState, time *TimeState, sim *SimRecord) {
	// Prologue: in-transit on a special-link segment -> decrement_route_queue_direction_load.
	// PORT-NOTE: the TS passes `world.carriers[0] ?? ({} as CarrierRecord)`;
	// Go passes nil when there is no carrier. The TS callee ignores it.
	if IsSimInTransit(sim.StateCode) && sim.Route.Mode == RouteModeSegment {
		var carrier *CarrierRecord
		if len(world.Carriers) > 0 {
			carrier = world.Carriers[0]
		}
		DecrementRouteQueueDirectionLoad(carrier, sim.OriginFloor, 0)
	}

	// 4-entry jump table at cs:5f29.
	switch sim.StateCode {
	case StateDeparture, StateDepartureTransit: // 0x05, 0x45
		handleFamilyParkingReturnRoute(world, time, sim)
	case StateMorningGate, StateEvalOutbound: // 0x20, 0x60
		handleFamilyParkingOutboundRoute(world, time, sim)
	}
}

// GateObjectFamilyParkingStateHandler TS: families/parking.ts
// gateObjectFamilyParkingStateHandler (binary 1228:5b5a)
func GateObjectFamilyParkingStateHandler(world *WorldState, time *TimeState, sim *SimRecord) {
	stateCode := sim.StateCode

	if IsSimInTransit(stateCode) {
		// Carrier-queued: maybe_dispatch_queued_route_after_wait exits
		// immediately for this family, so the TS skips the call.
		if sim.Route.Mode == RouteModeCarrier {
			return
		}
		runParkingDispatchJumpTable(world, time, sim)
		return
	}

	if stateCode == StateDeparture {
		// 0x05: unconditional dispatch (return route)
		runParkingDispatchJumpTable(world, time, sim)
		return
	}

	if stateCode == StateMorningGate {
		// 0x20: morning activation gate - weekendFlag must be 1
		if time.WeekendFlag != 1 {
			return
		}

		if time.DaypartIndex == 0 {
			// Staggered dispatch: 1/12 chance after tick 0x50, guaranteed after 0xf0.
			if time.DayTick > 0x50 {
				if SampleRng(world)%12 == 0 {
					runParkingDispatchJumpTable(world, time, sim)
				}
			}
			if time.DayTick > 0xf0 {
				runParkingDispatchJumpTable(world, time, sim)
			}
			return
		}

		// daypartIndex >= 1: missed dispatch window - park the sim
		if time.DaypartIndex >= 1 {
			sim.StateCode = StateParked
		}
		return
	}

	// All other states: no-op.
}

// DispatchObjectFamilyParkingStateHandler TS: families/parking.ts
// dispatchObjectFamilyParkingStateHandler (binary 1228:5cd2). Called by
// dispatch_destination_queue_entries (1218:0883) on carrier arrival.
func DispatchObjectFamilyParkingStateHandler(world *WorldState, time *TimeState, sim *SimRecord) {
	runParkingDispatchJumpTable(world, time, sim)
}
