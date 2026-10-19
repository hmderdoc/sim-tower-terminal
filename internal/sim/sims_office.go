package sim

// Port of reference/tower-together/sim/sims/office.ts: office-family
// (family 7) per-state refresh handlers and arrival dispatch.

// fastFoodFamilies is the TS `new Set([FAMILY_FAST_FOOD])` venue selector.
var fastFoodFamilies = map[int]bool{FamilyFastFood: true}

// resolveNoFeedback is the TS `{ emitDistanceFeedback: false }` option bag.
func resolveNoFeedback() *ResolveSimRouteOptions {
	f := false
	return &ResolveSimRouteOptions{EmitDistanceFeedback: &f}
}

// dirFlag is the TS `a > b ? 1 : 0` direction idiom.
func dirFlag(a, b int) int {
	if a > b {
		return 1
	}
	return 0
}

// AdvanceOfficePresenceCounter TS: sims/office.ts advanceOfficePresenceCounter
func AdvanceOfficePresenceCounter(object *PlacedObjectRecord) {
	if object.ObjectTypeCode != FamilyOffice {
		return
	}
	if object.UnitStatus > UnitStatusOfficeOccupied {
		return
	}
	if object.UnitStatus >= 8 {
		object.UnitStatus = 1
	} else {
		object.UnitStatus++
	}
}

// decrementOfficePresenceCounter TS: sims/office.ts decrementOfficePresenceCounter
func decrementOfficePresenceCounter(object *PlacedObjectRecord, time *TimeState) {
	if object.ObjectTypeCode != FamilyOffice {
		return
	}
	if object.UnitStatus > UnitStatusOfficeOccupied {
		return
	}
	object.UnitStatus = max(0, object.UnitStatus-1)
	if object.UnitStatus == 0 && time.DaypartIndex >= 4 {
		object.UnitStatus = 8
	}
}

// activateOfficeCashflow TS: sims/office.ts activateOfficeCashflow (binary
// activate_office_cashflow 1180:0d2e, is_reopening==0 path).
func activateOfficeCashflow(world *WorldState, object *PlacedObjectRecord, sim *SimRecord) {
	if object.UnitStatus <= UnitStatusOfficeOccupied {
		return
	}
	object.UnitStatus = 0
	// Binary 1180:0d93: dirty flag (+0x13) set.
	object.DirtyFlag = 1
	// Binary: add_to_primary_family_ledger_bucket(7, 6).
	AddToPopulationBucket(world, FamilyOffice, 6)

	ResetFacilitySimTripCounters(world, sim)
}

// routeFailureStateForOffice TS: sims/office.ts routeFailureStateForOffice
func routeFailureStateForOffice(object *PlacedObjectRecord) int {
	if object.UnitStatus > UnitStatusOfficeOccupied {
		return StateMorningGate
	}
	return StateNightA
}

// failOfficeRoute TS: sims/office.ts failOfficeRoute
func failOfficeRoute(world *WorldState, sim *SimRecord, failureState int) {
	ReleaseServiceRequest(world, sim)
	sim.StateCode = failureState
}

// finalizeOfficeFloorArrival TS: sims/office.ts finalizeOfficeFloorArrival
// (object may be nil, TS `undefined`).
func finalizeOfficeFloorArrival(sim *SimRecord, object *PlacedObjectRecord, nextState int) {
	if object != nil {
		AdvanceOfficePresenceCounter(object)
	}
	sim.DestinationFloor = -1
	sim.SelectedFloor = sim.FloorAnchor
	sim.VenueReturnState = 0
	sim.StateCode = nextState
}

// nextOfficeMorningState TS: sims/office.ts nextOfficeMorningState (binary
// 1228:23bb). RNG is only consumed on the non-base-0 branch with starCount >= 3.
func nextOfficeMorningState(world *WorldState, sim *SimRecord) int {
	if sim.BaseOffset == 0 {
		return StateCommute
	}
	if world.StarCount >= 3 && SampleRng(world)%10 == 0 {
		return StateActiveAlt
	}
	return StateActive
}

// NextOfficeReturnState TS: sims/office.ts nextOfficeReturnState
func NextOfficeReturnState(sim *SimRecord) int {
	if sim.BaseOffset == 1 {
		return StateCommute
	}
	return StateDeparture
}

// runOfficeServiceEvaluation TS: sims/office.ts runOfficeServiceEvaluation
func runOfficeServiceEvaluation(world *WorldState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	if world.StarCount != 3 || time.DayCounter%9 != 3 {
		return
	}
	if world.GateFlags.OfficeServiceOk != 0 {
		return
	}
	if world.GateFlags.EvalSimIndex >= 0 && world.GateFlags.EvalSimIndex != NoEvalEntity {
		return
	}
	if sim == nil || object == nil {
		return
	}
	if sim.FamilyCode != FamilyOffice || sim.StateCode != StateActive {
		return
	}
	if object.EvalLevel <= 0 {
		return
	}
	world.GateFlags.OfficeServiceOk = 1
}

// --- Per-state handlers (1228:1e45 etc.) ---

// handleOfficeCommute TS: sims/office.ts handleOfficeCommute (1228:1e45 office_refresh_0x00)
func handleOfficeCommute(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	if time.DaypartIndex >= 4 {
		sim.StateCode = StateDeparture
		return
	}
	if sim.BaseOffset == 0 {
		if time.DaypartIndex == 0 && SampleRng(world)%12 != 0 {
			return
		}
	} else {
		if time.DaypartIndex < 3 {
			return
		}
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	// Binary 1228:1e45 / 1228:266b: source=floor_anchor, target=LOBBY (OUTBOUND).
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor,
		dirFlag(LobbyFloor, sim.FloorAnchor), time, nil)
	if routeResult == -1 {
		sim.StateCode = StateNightB
		return
	}
	decrementOfficePresenceCounter(facility, time)
	if routeResult == 3 {
		AdvanceOfficePresenceCounter(facility)
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateAtWork
	} else {
		sim.StateCode = StateCommuteTransit
	}
}

// handleOfficeActive TS: sims/office.ts handleOfficeActive (1228:1ed5
// office_refresh_0x01/0x02).
func handleOfficeActive(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	runOfficeServiceEvaluation(world, time, sim, facility)
	if time.DaypartIndex >= 4 {
		if TryStartMedicalTrip(world, time, sim) {
			return
		}
		sim.StateCode = StateDeparture
		sim.DestinationFloor = LobbyFloor
		sim.SelectedFloor = sim.FloorAnchor
		return
	}
	if time.DaypartIndex == 0 {
		return
	}
	if time.DaypartIndex == 1 && SampleRng(world)%12 != 0 {
		return
	}

	trip := StateActiveTransit
	dispatched := DispatchCommercialVenueVisit(world, time, sim, CommercialVenueVisitOptions{
		VenueFamilies:               fastFoodFamilies,
		ReturnState:                 StateAtWork,
		TripState:                   &trip,
		SkipPenaltyOnUnavailable:    true,
		AdvanceBeforeSameFloorDwell: true,
	})
	if dispatched && sim.StateCode == CommercialDwellState {
		// Binary route_sim_to_commercial_venue (1238:022a) writes 0x22 on the
		// same-floor + acquire success path.
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}
	if !dispatched {
		// Spec No Fast Food Available: route to lobby for a fake lunch.
		routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor,
			dirFlag(LobbyFloor, sim.FloorAnchor), time, nil)
		if routeResult == -1 {
			AdvanceOfficePresenceCounter(facility)
			sim.StateCode = StateDeparture
			return
		}
		if routeResult == 3 {
			sim.VenueReturnState = 0
			sim.StateCode = StateVenueTrip
			sim.SelectedFloor = LobbyFloor
			sim.DestinationFloor = -1
			sim.LastDemandTick = time.DayTick
			ClearSimRoute(sim)
		} else {
			sim.StateCode = StateActiveTransit
		}
	}
}

// handleOfficeDeparture TS: sims/office.ts handleOfficeDeparture (1228:1fac office_refresh_0x05)
func handleOfficeDeparture(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	if time.DaypartIndex < 4 {
		return
	}
	if time.DaypartIndex == 4 && SampleRng(world)%6 != 0 {
		return
	}
	decrementOfficePresenceCounter(facility, time)
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor, 0, time, nil)
	if routeResult == -1 {
		failOfficeRoute(world, sim, StateNightB)
		return
	}
	if routeResult == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
	} else {
		sim.StateCode = StateDepartureTransit
	}
}

// handleOfficeMorningGate TS: sims/office.ts handleOfficeMorningGate (1228:1dc1 office_refresh_0x20)
func handleOfficeMorningGate(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	if time.WeekendFlag != 0 {
		return
	}
	// Binary 1228:1df3: occupied flag (+0x14) gate; unscored facilities
	// must not sample RNG here.
	if facility.OccupiedFlag == 0 {
		return
	}

	if time.DaypartIndex >= 3 {
		return
	}
	if time.DaypartIndex == 0 {
		if SampleRng(world)%12 != 0 {
			return
		}
	}

	if world.StarCount > 2 &&
		(sim.FloorAnchor+sim.HomeColumn)%4 == 1 &&
		facility.UnitStatus == 2 {
		if !TryAssignParkingService(world, time, sim) {
			world.PendingNotifications = append(world.PendingNotifications, SimNotification{
				Kind:    NotificationRouteFailure,
				Message: "Office workers demand Parking",
			})
		}
	}

	routeResult := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor,
		dirFlag(sim.FloorAnchor, LobbyFloor), time, nil)
	if routeResult == -1 {
		// Binary 1228:225f/2285/22ac: zero the counters resolve just bumped.
		sim.TripCount = 0
		sim.ElapsedTicks = 0
		sim.AccumulatedTicks = 0
		sim.StateCode = routeFailureStateForOffice(facility)
		return
	}
	if facility.AuxValueOrTimer != time.DayCounter+1 && time.DayCounter%3 == 0 {
		facility.AuxValueOrTimer = time.DayCounter + 1
		// Binary activate_office_cashflow 1180:0d93 sets +0x13 (dirty).
		facility.DirtyFlag = 1
		AddCashflowFromFamilyResource(ledger, "office", facility.RentLevel, facility.ObjectTypeCode)
	}
	activateOfficeCashflow(world, facility, sim)
	if routeResult == 0 || routeResult == 1 || routeResult == 2 {
		sim.StateCode = StateMorningTransit
		return
	}
	AdvanceOfficePresenceCounter(facility)
	sim.DestinationFloor = -1
	sim.SelectedFloor = sim.FloorAnchor
	sim.StateCode = nextOfficeMorningState(world, sim)
}

// handleOfficeAtWork TS: sims/office.ts handleOfficeAtWork (1228:1f33 office_refresh_0x21)
func handleOfficeAtWork(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	if time.DaypartIndex >= 4 {
		sim.StateCode = StateParked
		sim.DestinationFloor = -1
		ClearSimRoute(sim)
		ReleaseServiceRequest(world, sim)
		return
	}
	if time.DaypartIndex == 3 {
		if SampleRng(world)%12 != 0 {
			return
		}
	} else {
		return
	}

	routeResult := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor,
		dirFlag(sim.FloorAnchor, LobbyFloor), time, nil)
	if routeResult == -1 {
		failOfficeRoute(world, sim, StateNightB)
		return
	}
	if routeResult == 3 {
		finalizeOfficeFloorArrival(sim, facility, StateDeparture)
	} else {
		sim.StateCode = StateAtWorkTransit
	}
}

// handleOfficeVenueTrip TS: sims/office.ts handleOfficeVenueTrip (1228:1f62
// office_refresh_0x22/0x23).
func handleOfficeVenueTrip(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	if time.DaypartIndex >= 4 {
		// Late-day forced park: release the venue slot (binary 11b0:0fae).
		venue := FindCommercialVenueAtFloor(world, sim.SelectedFloor, fastFoodFamilies)
		if venue != nil {
			ReleaseOfficeVenueSlot(venue, sim, time, true)
		}
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
		return
	}
	if time.DaypartIndex < 2 {
		return
	}
	isFakeLunch := sim.SelectedFloor == LobbyFloor
	if sim.StateCode == StateVenueTrip && !isFakeLunch &&
		time.DayTick-sim.QueueTick < CommercialVenueDwellTicks {
		return
	}
	// Binary route_sim_back_from_commercial_venue (1238:0244) releases the
	// slot BEFORE resolving the return route.
	if sim.StateCode == StateVenueTrip && !isFakeLunch {
		venue := FindCommercialVenueAtFloor(world, sim.SelectedFloor, fastFoodFamilies)
		if venue != nil {
			ReleaseOfficeVenueSlot(venue, sim, time, true)
		}
	}
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.SelectedFloor, sim.FloorAnchor,
		dirFlag(sim.FloorAnchor, sim.SelectedFloor), time, nil)
	if routeResult == -1 {
		failOfficeRoute(world, sim, StateNightB)
		return
	}
	// lastDemandTick is left at the resolver's enqueue stamp (see TS comment).
	if routeResult == 3 {
		finalizeOfficeFloorArrival(sim, facility, NextOfficeReturnState(sim))
	} else {
		sim.StateCode = StateVenueHomeTransit
	}
}

// handleOfficeNightPark TS: sims/office.ts handleOfficeNightPark (1228:1d8e
// office_refresh_0x25/0x26/0x27).
func handleOfficeNightPark(_ *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DayTick > 2300 {
		sim.StateCode = StateMorningGate
	}
}

// handleOfficeTransit TS: sims/office.ts handleOfficeTransit. Per-stride
// in-transit handler for the +0x40 aliases; resolve rc=3 already advanced
// trip counters, so the arrival is dispatched with alreadyAdvanced=true.
func handleOfficeTransit(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	if sim.DestinationFloor < 0 {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.DestinationFloor
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if routeResult == 3 {
		sim.SelectedFloor = targetFloor
		ClearSimRoute(sim)
		HandleOfficeSimArrival(world, time, sim, targetFloor, true)
	}
}

// handleOfficeMorningTransitRetry TS: sims/office.ts
// handleOfficeMorningTransitRetry (1228:213c, state 0x60 alias of 0x20).
func handleOfficeMorningTransitRetry(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord) {
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.FloorAnchor
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if routeResult == -1 {
		sim.StateCode = routeFailureStateForOffice(facility)
		ReleaseServiceRequest(world, sim)
		return
	}
	if routeResult == 3 {
		AdvanceOfficePresenceCounter(facility)
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = nextOfficeMorningState(world, sim)
	}
	// rc 0/1/2: stay in STATE_MORNING_TRANSIT.
}

// OfficeHandler TS: sims/office.ts OfficeHandler
type OfficeHandler func(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, facility *PlacedObjectRecord)

// OfficeRefreshHandlerTable TS: sims/office.ts OFFICE_REFRESH_HANDLER_TABLE
// (cs:2005 refresh-state dispatch table). Lookup-only.
var OfficeRefreshHandlerTable map[int]OfficeHandler

func init() {
	OfficeRefreshHandlerTable = map[int]OfficeHandler{
		StateCommute:            handleOfficeCommute,             // 0x00 -> 1228:1e45
		StateActive:             handleOfficeActive,              // 0x01 -> 1228:1ed5
		StateActiveAlt:          handleOfficeActive,              // 0x02 -> 1228:1ed5
		StateDeparture:          handleOfficeDeparture,           // 0x05 -> 1228:1fac
		StateMorningGate:        handleOfficeMorningGate,         // 0x20 -> 1228:1dc1
		StateAtWork:             handleOfficeAtWork,              // 0x21 -> 1228:1f33
		StateVenueTrip:          handleOfficeVenueTrip,           // 0x22 -> 1228:1f62
		StateNightA:             handleOfficeNightPark,           // 0x25 -> 1228:1d8e
		StateNightB:             handleOfficeNightPark,           // 0x26
		StateParked:             handleOfficeNightPark,           // 0x27
		StateCommuteTransit:     handleOfficeTransit,             // 0x40
		StateActiveTransit:      handleOfficeTransit,             // 0x41
		StateVenueTripTransit:   handleOfficeTransit,             // 0x62 (the TS comment says 0x42; see PORT-NOTE)
		StateDepartureTransit:   handleOfficeTransit,             // 0x45
		StateMorningTransit:     handleOfficeMorningTransitRetry, // 0x60 -> 1228:213c alias
		StateAtWorkTransit:      handleOfficeTransit,             // 0x61
		StateDwellReturnTransit: handleOfficeTransit,             // 0x63
		// PORT-NOTE: the TS Map also lists [STATE_VENUE_HOME_TRANSIT,
		// handleOfficeTransit]; STATE_VENUE_HOME_TRANSIT == STATE_VENUE_TRIP_TRANSIT
		// (0x62) so it is the same key with the same handler.
	}
}

// ProcessOfficeSim TS: sims/office.ts processOfficeSim
func ProcessOfficeSim(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	facility := FindObjectForSim(world, sim)
	if facility == nil {
		return
	}
	if handler, ok := OfficeRefreshHandlerTable[sim.StateCode]; ok {
		handler(world, ledger, time, sim, facility)
	}
}

// HandleOfficeSimArrival TS: sims/office.ts handleOfficeSimArrival. Carrier
// or segment arrival dispatch for office sims. alreadyAdvanced is the TS
// default-false parameter (true from handleOfficeTransit).
func HandleOfficeSimArrival(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int, alreadyAdvanced bool) {
	object := FindObjectForSim(world, sim)
	advanceIfNeeded := func() {
		if !alreadyAdvanced {
			AdvanceSimTripCounters(sim)
		}
	}

	if sim.StateCode == StateMorningTransit && arrivalFloor == sim.FloorAnchor {
		// 1228:213c alias 0x60 of 0x20.
		advanceIfNeeded()
		finalizeOfficeFloorArrival(sim, object, nextOfficeMorningState(world, sim))
		return
	}

	if sim.StateCode == StateAtWorkTransit && arrivalFloor == sim.FloorAnchor {
		// 1228:2429 alias 0x61 of 0x21.
		advanceIfNeeded()
		finalizeOfficeFloorArrival(sim, object, StateDeparture)
		return
	}

	if (sim.StateCode == StateVenueHomeTransit || sim.StateCode == StateDwellReturnTransit) &&
		arrivalFloor == sim.FloorAnchor {
		// 1228:24cd / 1228:2505 alias 0x62/0x63 (1238:0244).
		advanceIfNeeded()
		finalizeOfficeFloorArrival(sim, object, NextOfficeReturnState(sim))
		return
	}

	if sim.StateCode == StateDepartureTransit && arrivalFloor == LobbyFloor {
		// 1228:2980 alias 0x45 of 0x05.
		advanceIfNeeded()
		sim.StateCode = StateParked
		sim.SelectedFloor = LobbyFloor
		ReleaseServiceRequest(world, sim)
		return
	}

	if sim.StateCode == StateCommuteTransit && arrivalFloor == sim.FloorAnchor {
		// 1228:2644 alias 0x40 of 0x00.
		advanceIfNeeded()
		finalizeOfficeFloorArrival(sim, object, StateAtWork)
		return
	}

	if sim.StateCode == StateCommuteTransit && sim.BaseOffset <= 1 && arrivalFloor == LobbyFloor {
		// 1228:2644 alias 0x40 outbound lobby leg for occupants 0/1.
		advanceIfNeeded()
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateAtWork
		return
	}

	// ACTIVE_TRANSIT (0x41) / VENUE_TRIP_TRANSIT arrival: route_sim_to_commercial_venue
	// (1238:0000) then acquire_commercial_venue_slot (11b0:0d92).
	if (sim.StateCode == StateActiveTransit || sim.StateCode == StateVenueTripTransit) &&
		(sim.DestinationFloor < 0 || arrivalFloor == sim.DestinationFloor) {
		advanceIfNeeded()
		sim.SelectedFloor = arrivalFloor
		var venue *CommercialVenueRecord
		if arrivalFloor != LobbyFloor {
			venue = FindCommercialVenueAtFloor(world, arrivalFloor, fastFoodFamilies)
		}
		if venue != nil {
			result := TryAcquireOfficeVenueSlot(venue, sim, time, FamilyFastFood)
			if result == VenueSlotFull {
				// Stay in STATE_ACTIVE_TRANSIT; retry next stride.
				return
			}
		}
		sim.DestinationFloor = -1
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}

	// Transfer-point drop: re-resolve the next leg from the arrival floor.
	if sim.StateCode == StateMorningTransit ||
		sim.StateCode == StateAtWorkTransit ||
		sim.StateCode == StateVenueHomeTransit ||
		sim.StateCode == StateDwellReturnTransit ||
		sim.StateCode == StateDepartureTransit ||
		sim.StateCode == StateCommuteTransit ||
		sim.StateCode == StateActiveTransit ||
		sim.StateCode == StateVenueTripTransit {
		var targetFloor int
		switch {
		case sim.StateCode == StateMorningTransit ||
			sim.StateCode == StateAtWorkTransit ||
			sim.StateCode == StateVenueHomeTransit ||
			sim.StateCode == StateDwellReturnTransit:
			targetFloor = sim.FloorAnchor
		case sim.StateCode == StateActiveTransit || sim.StateCode == StateVenueTripTransit:
			targetFloor = sim.DestinationFloor
		default:
			targetFloor = LobbyFloor
		}
		sim.SelectedFloor = arrivalFloor
		routeResult := ResolveSimRouteBetweenFloors(world, sim, arrivalFloor, targetFloor,
			dirFlag(targetFloor, arrivalFloor), time, resolveNoFeedback())
		if routeResult == -1 {
			failOfficeRoute(world, sim, StateNightB)
		}
		return
	}

	if sim.StateCode == StateCommute && arrivalFloor == sim.FloorAnchor {
		finalizeOfficeFloorArrival(sim, object, StateActive)
		return
	}

	if sim.StateCode == StateDeparture && arrivalFloor == LobbyFloor {
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateParked
	}
}
