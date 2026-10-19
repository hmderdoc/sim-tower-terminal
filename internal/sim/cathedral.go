package sim

// Port of reference/tower-together/sim/cathedral.ts: cathedral guest sims
// (families 0x24-0x28) - activation, dispatch, return routing, award path.

// evalSimCount TS: cathedral.ts EVAL_SIM_COUNT (5 floor types x 8 slots).
const evalSimCount = 40

// isEvalPlaced TS: cathedral.ts isEvalPlaced
func isEvalPlaced(world *WorldState) bool {
	return world.GateFlags.EvalSimIndex >= 0 && world.GateFlags.EvalSimIndex != NoEvalEntity
}

// ActivateEvalSims TS: cathedral.ts activateEvalSims (binary
// activate_cathedral_evaluation_entities 1048:0000). No star-count gate.
func ActivateEvalSims(world *WorldState) {
	if !isEvalPlaced(world) {
		return
	}
	for _, sim := range world.Sims {
		if !CathedralFamilies[sim.FamilyCode] {
			continue
		}
		sim.StateCode = StateMorningGate
		sim.SelectedFloor = LobbyFloor
		sim.OriginFloor = sim.FloorAnchor
		ClearSimRoute(sim)
		sim.DestinationFloor = -1
		sim.VenueReturnState = 0
	}
}

// DispatchEvalMiddayReturn TS: cathedral.ts dispatchEvalMiddayReturn (binary
// dispatch_evaluation_sim_midday_return 1048:0179, checkpoint 0x04b0).
func DispatchEvalMiddayReturn(world *WorldState) {
	if !isEvalPlaced(world) {
		return
	}
	for _, object := range world.PlacedObjects.Values() {
		if !CathedralFamilies[object.ObjectTypeCode] {
			continue
		}
		object.AuxValueOrTimer = 0
		object.DirtyFlag = 1
	}
	for _, sim := range world.Sims {
		if !CathedralFamilies[sim.FamilyCode] {
			continue
		}
		if sim.StateCode == StateArrived {
			sim.StateCode = StateDeparture
			sim.SelectedFloor = EvalZoneFloor
			sim.DestinationFloor = LobbyFloor
		}
	}
}

// cathedralDispatchOutbound TS: cathedral.ts dispatchOutbound
func cathedralDispatchOutbound(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == StateMorningGate
	sourceFloor := sim.OriginFloor
	directionFlag := 0
	if isFreshDispatch {
		sourceFloor = LobbyFloor
		directionFlag = 1
	}
	sim.SelectedFloor = sourceFloor
	sim.DestinationFloor = EvalZoneFloor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, EvalZoneFloor, directionFlag, time,
		resolveOptsFeedback(isFreshDispatch))
	if result == 3 {
		sim.StateCode = StateArrived
		CheckEvalCompletionAndAward(world, time, sim)
	} else if result >= 0 {
		sim.StateCode = StateEvalOutbound
	} else {
		sim.StateCode = StateParked
	}
}

// cathedralDispatchReturn TS: cathedral.ts dispatchReturn
func cathedralDispatchReturn(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == StateDeparture
	sourceFloor := sim.OriginFloor
	directionFlag := 0
	if isFreshDispatch {
		sourceFloor = EvalZoneFloor
		directionFlag = 1
	}
	sim.SelectedFloor = sourceFloor
	sim.DestinationFloor = LobbyFloor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor, directionFlag, time,
		resolveOptsFeedback(isFreshDispatch))
	if result == 0 || result == 1 || result == 2 {
		sim.StateCode = StateEvalReturn
	} else {
		sim.StateCode = StateParked
	}
}

// ProcessCathedralSim TS: cathedral.ts processCathedralSim
func ProcessCathedralSim(world *WorldState, time *TimeState, sim *SimRecord) {
	switch sim.StateCode {
	case StateMorningGate:
		// Gate: weekendFlag must be 1
		if time.WeekendFlag != 1 {
			return
		}
		if time.DaypartIndex == 0 {
			// && short-circuit: the RNG is drawn only after tick 0x50.
			if time.DayTick > 0x50 && SampleRng(world)%12 == 0 {
				cathedralDispatchOutbound(world, time, sim)
			}
			if time.DayTick > 0xf0 {
				cathedralDispatchOutbound(world, time, sim)
			}
			return
		}
		if time.DaypartIndex >= 1 {
			sim.StateCode = StateParked
		}
		return

	case StateEvalOutbound:
		if sim.Route.Mode != RouteModeCarrier {
			cathedralDispatchOutbound(world, time, sim)
		}
		return

	case StateArrived:
		// Arrived at eval zone; waiting for midday return dispatch
		return

	case StateDeparture:
		cathedralDispatchReturn(world, time, sim)
		return

	case StateEvalReturn:
		if sim.Route.Mode != RouteModeCarrier {
			cathedralDispatchReturn(world, time, sim)
		}
		return

	case StateParked:
		// Parked; will be reset at next day-start
		return
	}
}

// CheckEvalCompletionAndAward TS: cathedral.ts checkEvalCompletionAndAward
// (binary process_family_parking_destination_arrival 1048:00f0 /
// award_star_rating_upgrade 1048:02b5).
func CheckEvalCompletionAndAward(world *WorldState, time *TimeState, arrivedSim *SimRecord) {
	if !isEvalPlaced(world) {
		return
	}
	if time.DayTick >= 800 {
		return
	}

	// Count sims that arrived at eval zone
	arrivedCount := 0
	for _, sim := range world.Sims {
		if !CathedralFamilies[sim.FamilyCode] {
			continue
		}
		if sim.StateCode == StateArrived {
			arrivedCount++
		}
	}

	if arrivedCount < evalSimCount {
		// Not all arrived yet - stamp the arrived sim's placed object
		if object := FindObjectForSim(world, arrivedSim); object != nil {
			object.AuxValueOrTimer = 3
		}
		return
	}

	// All 40 arrived - check ledger tier > star_count for tower promotion.
	tierThresholds := []int{300, 1000, 5000, 10_000, 15_000}
	ledgerTotal := world.CurrentPopulation
	tier := 1
	for index := 0; index < len(tierThresholds); index++ {
		if ledgerTotal >= tierThresholds[index] {
			tier = index + 2
		}
	}

	if tier > world.StarCount {
		// Tower promotion: star_count := 6
		world.StarCount = 6
		for _, object := range world.PlacedObjects.Values() {
			if !CathedralFamilies[object.ObjectTypeCode] {
				continue
			}
			object.AuxValueOrTimer = 2
			object.DirtyFlag = 1
		}
	}
}

// HandleCathedralSimArrival TS: cathedral.ts handleCathedralSimArrival
func HandleCathedralSimArrival(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	if sim.StateCode == StateEvalOutbound && arrivalFloor == EvalZoneFloor {
		sim.StateCode = StateArrived
		sim.DestinationFloor = -1
		CheckEvalCompletionAndAward(world, time, sim)
		return
	}

	if sim.StateCode == StateEvalReturn && arrivalFloor == LobbyFloor {
		sim.StateCode = StateParked
		sim.DestinationFloor = -1
	}
}
