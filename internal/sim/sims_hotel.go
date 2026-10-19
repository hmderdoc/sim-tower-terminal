package sim

import "sort"

// Port of reference/tower-together/sim/sims/hotel.ts: hotel-family
// (families 3/4/5) per-state refresh handlers and arrival dispatch.

// floorLocalRoomRank TS: sims/hotel.ts floorLocalRoomRank. Binary
// subtype_index (sim+1) = floor-local ROOM rank in column-ascending order
// (IDIV at 1228:3493).
func floorLocalRoomRank(world *WorldState, sim *SimRecord) int {
	floorY := GridHeight - 1 - sim.FloorAnchor
	columns := []int{}
	for _, e := range world.PlacedObjects.Entries() {
		if !HotelFamilies[e.Val.ObjectTypeCode] {
			continue
		}
		x, y := parseKeyXY(e.Key)
		if y != floorY {
			continue
		}
		columns = append(columns, x)
	}
	sort.SliceStable(columns, func(i, j int) bool { return columns[i] < columns[j] })
	idx := -1
	for i, c := range columns {
		if c == sim.HomeColumn {
			idx = i
			break
		}
	}
	return max(0, idx)
}

// hotelArrivalState TS: sims/hotel.ts hotelArrivalState
func hotelArrivalState(world *WorldState, sim *SimRecord) int {
	if floorLocalRoomRank(world, sim)&1 == 0 {
		return StateActive
	}
	return StateCheckoutQueue
}

// activateHotelUnit TS: sims/hotel.ts activateHotelUnit
// (activate_family_345_unit 1180:0e72).
func activateHotelUnit(world *WorldState, object *PlacedObjectRecord, time *TimeState) {
	if object.UnitStatus >= 0x18 {
		if time.DaypartIndex < 4 {
			object.UnitStatus = 0
		} else {
			object.UnitStatus = 8
		}
		object.ActivationTickCount = 0
		amount := 2
		if object.ObjectTypeCode == FamilyHotelSingle {
			amount = 1
		}
		AddToPopulationBucket(world, object.ObjectTypeCode, amount)
	}
}

// incrementStayPhase345 TS: sims/hotel.ts incrementStayPhase345
// (increment_stay_phase_345 1228:6a56).
func incrementStayPhase345(object *PlacedObjectRecord, time *TimeState) {
	if object.UnitStatus == 0x10 {
		if time.DaypartIndex < 4 {
			object.UnitStatus = 1
		} else {
			object.UnitStatus = 9
		}
	} else {
		object.UnitStatus = (object.UnitStatus + 1) & 0xff
	}
}

// routeHotelToLobbyNoVenue TS: sims/hotel.ts routeHotelToLobbyNoVenue
// (route_sim_to_commercial_venue 1238:0000 no-venue path).
func routeHotelToLobbyNoVenue(world *WorldState, time *TimeState, sim *SimRecord) {
	directionFlag := 1
	if sim.FloorAnchor > LobbyFloor {
		directionFlag = 0
	}
	result := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor, directionFlag, time, nil)
	if result == -1 || result == 0 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	sim.VenueReturnState = StateCheckoutQueue
	if result == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}
	sim.OriginFloor = sim.FloorAnchor
	sim.StateCode = StateActiveTransit
}

// activateHotelStay TS: sims/hotel.ts activateHotelStay
func activateHotelStay(world *WorldState, sim *SimRecord, time *TimeState) {
	object := FindObjectForSim(world, sim)
	if object == nil {
		return
	}

	directionFlag := dirFlag(sim.FloorAnchor, LobbyFloor)
	result := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor, directionFlag, time, nil)
	if result == -1 || result == 0 {
		return
	}

	sim.OriginFloor = LobbyFloor

	// Binary 1228:33ba (en-route) + 1228:3434 (same-floor).
	activateHotelUnit(world, object, time)

	if result == 3 {
		incrementStayPhase345(object, time)
		sim.StateCode = hotelArrivalState(world, sim)
		sim.SelectedFloor = sim.FloorAnchor
	} else {
		sim.StateCode = StateMorningTransit
	}
}

// CheckoutHotelStay TS: sims/hotel.ts checkoutHotelStay
// (deactivate_family_hotel_unit_with_income 1180:0f24).
func CheckoutHotelStay(world *WorldState, ledger *LedgerState, time *TimeState, _ *SimRecord, object *PlacedObjectRecord) {
	var tileName string
	switch object.ObjectTypeCode {
	case FamilyHotelSingle:
		tileName = "hotelSingle"
	case FamilyHotelTwin:
		tileName = "hotelTwin"
	default:
		tileName = "hotelSuite"
	}
	AddCashflowFromFamilyResource(ledger, tileName, object.RentLevel, object.ObjectTypeCode)
	world.GateFlags.Family345SaleCount++
	saleCount := world.GateFlags.Family345SaleCount
	if (saleCount < 20 && saleCount%2 == 0) || (saleCount >= 20 && saleCount%8 == 0) {
		world.GateFlags.NewspaperTrigger = 1
	} else {
		world.GateFlags.NewspaperTrigger = 0
	}
	// Binary 1228:2fa7 does NOT touch sibling state bytes at checkout.
	if time.DaypartIndex < 4 {
		object.UnitStatus = 0x28
	} else {
		object.UnitStatus = 0x30
	}
	// 1180:0f7d dirty, 1180:0f8e occupied cleared.
	object.DirtyFlag = 1
	object.OccupiedFlag = 0
	object.ActivationTickCount = 0
	populationAmount := 2
	if object.ObjectTypeCode == FamilyHotelSingle {
		populationAmount = 1
	}
	AddToPopulationBucket(world, object.ObjectTypeCode, -populationAmount)
}

// --- Per-state handlers ---

// handleHotelParked TS: sims/hotel.ts handleHotelParked (hotel_refresh_0x24).
// Binary: state 0x24 is NOT in the hotel jump table, a no-op.
func handleHotelParked(_ *WorldState, _ *LedgerState, _ *TimeState, _ *SimRecord, _ *PlacedObjectRecord) {
}

// handleHotelMorningGate TS: sims/hotel.ts handleHotelMorningGate (hotel_refresh_0x20)
func handleHotelMorningGate(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	// Binary 1228:2c8c: occupied flag (+0x14) gate.
	if object.OccupiedFlag == 0 {
		return
	}
	if time.DaypartIndex == 4 {
		if SampleRng(world)%12 != 0 {
			return
		}
		if sim.FamilyCode == FamilyHotelSuite && world.StarCount <= 2 {
			sim.StateCode = StateNightB
			return
		}
		activateHotelStay(world, sim, time)
		return
	}
	// Binary 1228:2c63: daypart > 4 AND day_tick < 2300 dispatches without RNG.
	if time.DaypartIndex > 4 && time.DayTick < DayTickNewDay {
		if sim.FamilyCode == FamilyHotelSuite && world.StarCount <= 2 {
			sim.StateCode = StateNightB
			return
		}
		activateHotelStay(world, sim, time)
		return
	}
}

// handleHotelActive TS: sims/hotel.ts handleHotelActive (hotel_refresh_0x01)
func handleHotelActive(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	if time.DaypartIndex <= 3 {
		return
	}
	if time.DaypartIndex > 4 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	if SampleRng(world)%6 != 0 {
		return
	}
	if sim.FamilyCode == FamilyHotelSuite && world.StarCount > 2 && object.UnitStatus != 0 {
		TryAssignParkingService(world, time, sim)
	}
	trip := StateActiveTransit
	dispatched := DispatchCommercialVenueVisit(world, time, sim, CommercialVenueVisitOptions{
		VenueFamilies:               HotelRoomSelector,
		ReturnState:                 StateActive,
		TripState:                   &trip,
		SkipPenaltyOnUnavailable:    true,
		AdvanceBeforeSameFloorDwell: true,
		OnVenueReserved: func() {
			object.ActivationTickCount = min(ActivationTickCap, object.ActivationTickCount+1)
		},
	})
	if dispatched && sim.StateCode == CommercialDwellState {
		// Hotel same-floor venue: binary writes state=0x22, not 0x62.
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		sim.LastDemandTick = -1
		return
	}
	if !dispatched {
		routeHotelToLobbyNoVenue(world, time, sim)
	}
}

// handleHotelActiveTransit TS: sims/hotel.ts handleHotelActiveTransit
// (0x41 alias of 0x01, handler 0x3126).
func handleHotelActiveTransit(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
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
		// Inline handle_hotel_guest_venue_acquisition (1228:4fab): state 0x22.
		sim.DestinationFloor = -1
		sim.SelectedFloor = targetFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
	}
}

// handleHotelMorningTransit TS: sims/hotel.ts handleHotelMorningTransit
// (0x60 alias of 0x20, handler 0x317b @ 0x327d).
func handleHotelMorningTransit(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.FloorAnchor
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if routeResult == -1 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	if routeResult == 3 {
		// Binary 1228:3434 path.
		activateHotelUnit(world, object, time)
		incrementStayPhase345(object, time)
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = hotelArrivalState(world, sim)
		return
	}
}

// handleHotelDepartureTransit TS: sims/hotel.ts handleHotelDepartureTransit
// (0x45 alias of 0x05, handler 0x2fa7 @ 0x2fd9). The 0x45 path skips the
// checkout block (CMP/JNZ at 1228:30bd).
func handleHotelDepartureTransit(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	sourceFloor := sim.SelectedFloor
	targetFloor := LobbyFloor
	if sim.DestinationFloor >= 0 {
		targetFloor = sim.DestinationFloor
	}
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if routeResult == -1 {
		sim.StateCode = StateMorningGate
		return
	}
	if routeResult == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		if sim.BaseOffset == 0 {
			sim.StateCode = StateHotelParked
		} else {
			sim.StateCode = StateMorningGate
		}
		return
	}
}

// handleHotelCheckoutQueue TS: sims/hotel.ts handleHotelCheckoutQueue (hotel_refresh_0x04)
func handleHotelCheckoutQueue(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DaypartIndex < 5 {
		return
	}
	if time.DayTick <= 2400 {
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	sim.StateCode = StateTransition
}

// handleHotelTransition TS: sims/hotel.ts handleHotelTransition (hotel_refresh_0x10)
func handleHotelTransition(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	if time.DaypartIndex >= 5 {
		if time.DayTick <= 2566 {
			return
		}
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	if object.UnitStatus == 0x10 {
		if object.ObjectTypeCode == FamilyHotelSingle {
			object.UnitStatus = 1
		} else {
			object.UnitStatus = 2
		}
	}
	sim.StateCode = StateDeparture
}

// handleHotelDeparture TS: sims/hotel.ts handleHotelDeparture (hotel_refresh_0x05, 1228:2c43)
func handleHotelDeparture(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	if time.DaypartIndex == 0 {
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	if time.DaypartIndex >= 6 {
		return
	}
	directionFlag := 1
	if sim.FloorAnchor > LobbyFloor {
		directionFlag = 0
	}
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor, directionFlag, time, nil)
	if routeResult == 0 {
		return
	}
	if routeResult == -1 {
		// Binary mapping: rc=-1 -> 0x20 (MORNING_GATE) + service-eval-fail.
		sim.StateCode = StateMorningGate
		return
	}
	sim.OriginFloor = sim.FloorAnchor
	// trip_prep_for_checkout: payout booked at dispatch, not lobby arrival.
	object.UnitStatus--
	ready := object.UnitStatus&0x07 == 0
	if routeResult == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		if ready {
			CheckoutHotelStay(world, ledger, time, sim, object)
		} else {
			if sim.BaseOffset == 0 {
				sim.StateCode = StateHotelParked
			} else {
				sim.StateCode = StateMorningGate
			}
		}
	} else {
		if ready {
			CheckoutHotelStay(world, ledger, time, sim, object)
		}
		sim.StateCode = StateDepartureTransit
	}
}

// handleHotelVenueTrip TS: sims/hotel.ts handleHotelVenueTrip (hotel_refresh_0x22,
// shared 0x22/0x62 handler 1228:50ef).
func handleHotelVenueTrip(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.VenueReturnState == StateCheckoutQueue {
		dir := dirFlag(sim.FloorAnchor, LobbyFloor)
		result := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor, dir, time, nil)
		if result == 3 {
			sim.StateCode = StateCheckoutQueue
			sim.VenueReturnState = 0
			return
		}
		// rc 0/1/2 all map to 0x62; queue-full (0) only set the waiting bit.
		if result == 0 {
			SetSimInTransit(sim, true)
		}
		return
	}
	// Real-venue dwell: stay in 0x22 until service_duration elapsed.
	if time.DayTick-sim.QueueTick < 64 {
		return
	}
	if sim.SelectedFloor == sim.FloorAnchor {
		// release_venue_slot -> resolve(floor->floor)=3 -> advance.
		AdvanceSimTripCounters(sim)
		sim.StateCode = StateCheckoutQueue
		sim.VenueReturnState = 0
		return
	}
	sim.StateCode = CommercialDwellState
	sim.QueueTick = time.DayTick
}

// handleHotelCommercialDwell TS: sims/hotel.ts handleHotelCommercialDwell (hotel_refresh_0x62)
func handleHotelCommercialDwell(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.VenueReturnState == StateCheckoutQueue {
		dir := dirFlag(sim.FloorAnchor, LobbyFloor)
		result := ResolveSimRouteBetweenFloors(world, sim, sim.SelectedFloor, sim.FloorAnchor, dir, time, nil)
		if result == 3 {
			sim.StateCode = StateCheckoutQueue
			sim.VenueReturnState = 0
		}
		return
	}
	FinishCommercialVenueDwell(sim, time, StateActive)
}

// handleHotelNightB TS: sims/hotel.ts handleHotelNightB (hotel_refresh_0x26)
func handleHotelNightB(_ *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DayTick <= DayTickNewDay {
		return
	}
	if sim.BaseOffset == 0 {
		sim.StateCode = StateHotelParked
	} else {
		sim.StateCode = StateMorningGate
	}
}

// HotelHandler TS: sims/hotel.ts HotelHandler
type HotelHandler func(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord)

// HotelRefreshHandlerTable TS: sims/hotel.ts HOTEL_REFRESH_HANDLER_TABLE. Lookup-only.
var HotelRefreshHandlerTable map[int]HotelHandler

func init() {
	HotelRefreshHandlerTable = map[int]HotelHandler{
		StateHotelParked:      handleHotelParked,           // 0x24
		StateMorningGate:      handleHotelMorningGate,      // 0x20
		StateActive:           handleHotelActive,           // 0x01
		StateMorningTransit:   handleHotelMorningTransit,   // 0x60 -> 0x317b alias of 0x20
		StateActiveTransit:    handleHotelActiveTransit,    // 0x41 -> 0x3126 alias of 0x01
		StateDepartureTransit: handleHotelDepartureTransit, // 0x45 -> 0x2fa7 alias of 0x05
		StateCheckoutQueue:    handleHotelCheckoutQueue,    // 0x04
		StateTransition:       handleHotelTransition,       // 0x10
		StateDeparture:        handleHotelDeparture,        // 0x05
		StateVenueTrip:        handleHotelVenueTrip,        // 0x22
		CommercialDwellState:  handleHotelCommercialDwell,  // 0x62
		StateNightB:           handleHotelNightB,           // 0x26
	}
}

// ProcessHotelSim TS: sims/hotel.ts processHotelSim. Binary 1228:0d64 gates
// the family 3/4/5 handler on base_offset > 0.
func ProcessHotelSim(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	object := FindObjectForSim(world, sim)
	if object == nil {
		return
	}
	if sim.BaseOffset == 0 {
		return
	}
	if handler, ok := HotelRefreshHandlerTable[sim.StateCode]; ok {
		handler(world, ledger, time, sim, object)
	} else {
		sim.StateCode = StateHotelParked
	}
}

// HandleHotelSimArrival TS: sims/hotel.ts handleHotelSimArrival
func HandleHotelSimArrival(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	object := FindObjectForSim(world, sim)

	if sim.StateCode == StateMorningTransit && arrivalFloor == sim.FloorAnchor {
		// 1218:0883 -> 0x317b: resolve same-floor advance (1218:0046).
		AdvanceSimTripCounters(sim)
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		if object != nil {
			activateHotelUnit(world, object, time)
			incrementStayPhase345(object, time)
			sim.StateCode = hotelArrivalState(world, sim)
		}
		return
	}

	if sim.StateCode == StateActiveTransit && arrivalFloor == LobbyFloor &&
		sim.VenueReturnState == StateCheckoutQueue {
		// 1228:4fab lobby no-venue reentry: resolve(lobby,lobby)=3 advance.
		AdvanceSimTripCounters(sim)
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}

	if sim.StateCode == StateActiveTransit && arrivalFloor == sim.DestinationFloor {
		// Real commercial venue (1228:4fab): state 0x22.
		sim.DestinationFloor = -1
		sim.SelectedFloor = arrivalFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}

	if sim.StateCode == CommercialDwellState && arrivalFloor == sim.FloorAnchor &&
		sim.VenueReturnState == StateCheckoutQueue {
		// 1228:50ef state-0x62: resolve(home->home)=3 advance.
		AdvanceSimTripCounters(sim)
		sim.DestinationFloor = -1
		sim.SelectedFloor = arrivalFloor
		sim.StateCode = StateCheckoutQueue
		sim.VenueReturnState = 0
		return
	}

	if sim.StateCode == StateDepartureTransit && arrivalFloor == LobbyFloor {
		AdvanceSimTripCounters(sim)
		sim.DestinationFloor = -1
		if sim.BaseOffset == 0 {
			sim.StateCode = StateHotelParked
		} else {
			sim.StateCode = StateMorningGate
		}
		return
	}

	if sim.StateCode == StateCheckoutQueue && arrivalFloor == LobbyFloor {
		sim.DestinationFloor = -1
		if object != nil {
			object.UnitStatus--
			if object.UnitStatus&0x07 == 0 {
				CheckoutHotelStay(world, ledger, time, sim, object)
			} else {
				if sim.BaseOffset == 0 {
					sim.StateCode = StateHotelParked
				} else {
					sim.StateCode = StateMorningGate
				}
			}
		}
	}
}
