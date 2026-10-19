package sim

// Port of reference/tower-together/sim/sims/condo.ts: condo-family
// (family 9) per-state refresh handlers and arrival dispatch.

// finalizeCondoSale TS: sims/condo.ts finalizeCondoSale (finalize_condo_sale
// family-9 helper): YEN #1001 payout, occupied band 0 pre-day-4 / 8 after.
func finalizeCondoSale(world *WorldState, ledger *LedgerState, time *TimeState, object *PlacedObjectRecord) {
	AddCashflowFromFamilyResource(ledger, "condo", object.RentLevel, object.ObjectTypeCode)
	if PreDay4(time) {
		object.UnitStatus = 0x00
	} else {
		object.UnitStatus = 0x08
	}
	AddToPopulationBucket(world, FamilyCondo, 3)
}

// dispatchCondoMorningGate TS: sims/condo.ts dispatchCondoMorningGate
// (binary 1228:3b71 state-0x20 handler: source=LOBBY, target=floorAnchor).
func dispatchCondoMorningGate(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	directionFlag := dirFlag(sim.FloorAnchor, LobbyFloor)
	result := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor, directionFlag, time, nil)
	wasVacant := object.UnitStatus >= UnitStatusCondoVacant

	if result == -1 {
		if wasVacant {
			sim.StateCode = StateMorningTransit
		} else {
			sim.StateCode = StateCheckoutQueue
		}
		return
	}

	if wasVacant {
		finalizeCondoSale(world, ledger, time, object)
	}

	if result == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}

	// rc=0/1/2: in transit. Resolve owns sim+7/sim+0x12.
	sim.StateCode = StateMorningTransit
}

// --- Per-state handlers ---

// handleCondoMorningGate TS: sims/condo.ts handleCondoMorningGate (condo_refresh_0x20)
func handleCondoMorningGate(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	// Binary 1228:375e: occupied flag (+0x14) gate.
	if object.OccupiedFlag == 0 {
		return
	}
	if time.DaypartIndex >= 5 {
		return
	}
	dispatchCondoMorningGate(world, ledger, time, sim, object)
}

// handleCondoMorningTransit TS: sims/condo.ts handleCondoMorningTransit (condo_refresh_0x60)
func handleCondoMorningTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.FloorAnchor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	wasVacant := object.UnitStatus >= UnitStatusCondoVacant

	if result == -1 {
		if wasVacant {
			sim.StateCode = StateMorningTransit
		} else {
			sim.StateCode = StateCheckoutQueue
		}
		return
	}

	if wasVacant {
		finalizeCondoSale(world, ledger, time, object)
	}

	if result == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
}

// handleCondoCommuteTransit TS: sims/condo.ts handleCondoCommuteTransit (condo_refresh_0x40)
func handleCondoCommuteTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := LobbyFloor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if result == -1 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	if result == 3 {
		DispatchSimArrival(world, ledger, time, sim, targetFloor)
	}
}

// handleCondoAtWorkTransit TS: sims/condo.ts handleCondoAtWorkTransit (condo_refresh_0x61)
func handleCondoAtWorkTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.FloorAnchor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if result == -1 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
	if result == 3 {
		DispatchSimArrival(world, ledger, time, sim, targetFloor)
	}
}

// handleCondoActiveTransit TS: sims/condo.ts handleCondoActiveTransit (condo_refresh_0x41)
func handleCondoActiveTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	if sim.DestinationFloor < 0 {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.DestinationFloor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if result == -1 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	if result == 3 {
		DispatchSimArrival(world, ledger, time, sim, targetFloor)
	}
}

// handleCondoVenueHomeTransit TS: sims/condo.ts handleCondoVenueHomeTransit (condo_refresh_0x62)
func handleCondoVenueHomeTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.Route.Mode == RouteModeCarrier {
		return
	}
	if sim.DestinationFloor < 0 {
		return
	}
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.DestinationFloor
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		dirFlag(targetFloor, sourceFloor), time, resolveNoFeedback())
	if result == -1 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
	if result == 3 {
		DispatchSimArrival(world, ledger, time, sim, targetFloor)
	}
}

// handleCondoCheckoutQueue TS: sims/condo.ts handleCondoCheckoutQueue (condo_refresh_0x04)
func handleCondoCheckoutQueue(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DaypartIndex < 5 {
		return
	}
	if sim.BaseOffset != 2 && time.DayTick < 2401 {
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	sim.StateCode = StateTransition
}

// handleCondoTransition TS: sims/condo.ts handleCondoTransition (condo_refresh_0x10, FUN_1228_397b)
func handleCondoTransition(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DaypartIndex >= 5 {
		if time.DayTick < 2567 {
			return
		}
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	if time.WeekendFlag == 1 {
		if sim.FacilitySlot%2 != 0 {
			sim.StateCode = StateCheckoutQueue
		} else {
			sim.StateCode = StateActive
		}
	} else {
		if sim.BaseOffset == 1 {
			sim.StateCode = StateActive
		} else {
			sim.StateCode = StateCommute
		}
	}
}

// handleCondoCommute TS: sims/condo.ts handleCondoCommute (condo_refresh_0x00)
func handleCondoCommute(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DaypartIndex == 6 {
		return
	}
	if time.DaypartIndex == 0 {
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	dispatchCondoCommute(world, time, sim)
}

// handleCondoActive TS: sims/condo.ts handleCondoActive (condo_refresh_0x01, 1228:3681)
func handleCondoActive(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.WeekendFlag == 1 && sim.FacilitySlot%4 == 0 {
		if time.DaypartIndex == 4 {
			if SampleRng(world)%6 == 0 {
				dispatchCondoActive(world, time, sim)
			}
			return
		}
		if time.DaypartIndex > 4 {
			sim.StateCode = StateCheckoutQueue
			return
		}
		return
	}
	if time.DaypartIndex == 6 {
		return
	}
	if time.DaypartIndex == 0 {
		if time.DayTick < 0xf1 {
			return
		}
		if SampleRng(world)%12 != 0 {
			return
		}
	}
	dispatchCondoActive(world, time, sim)
}

// handleCondoAtWork TS: sims/condo.ts handleCondoAtWork (condo_refresh_0x21)
func handleCondoAtWork(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if sim.BaseOffset == 2 {
		if time.DaypartIndex < 3 {
			return
		}
		if time.DaypartIndex == 3 {
			if SampleRng(world)%12 != 0 {
				return
			}
		}
	} else {
		if time.DaypartIndex < 4 {
			return
		}
		if time.DaypartIndex == 4 {
			if SampleRng(world)%12 != 0 {
				return
			}
		}
	}
	dispatchCondoAtWork(world, time, sim)
}

// handleCondoVenueTrip TS: sims/condo.ts handleCondoVenueTrip (condo_refresh_0x22)
func handleCondoVenueTrip(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) {
	if time.DaypartIndex <= 2 {
		return
	}
	// Binary release_commercial_venue_slot gates the exit on service_duration.
	if sim.VenueReturnState != StateCheckoutQueue &&
		time.DayTick-sim.QueueTick < CommercialVenueDwellTicks {
		return
	}
	dispatchCondoVenueTrip(world, time, sim)
}

// CondoHandler TS: sims/condo.ts CondoHandler
type CondoHandler func(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord)

// CondoRefreshHandlerTable TS: sims/condo.ts CONDO_REFRESH_HANDLER_TABLE
// (family-9 refresh dispatch). Lookup-only.
var CondoRefreshHandlerTable map[int]CondoHandler

func init() {
	CondoRefreshHandlerTable = map[int]CondoHandler{
		StateMorningGate:      handleCondoMorningGate,      // 0x20
		StateMorningTransit:   handleCondoMorningTransit,   // 0x60 alias of 0x20
		StateCheckoutQueue:    handleCondoCheckoutQueue,    // 0x04
		StateTransition:       handleCondoTransition,       // 0x10
		StateCommute:          handleCondoCommute,          // 0x00
		StateCommuteTransit:   handleCondoCommuteTransit,   // 0x40 alias of 0x00
		StateActive:           handleCondoActive,           // 0x01
		StateActiveTransit:    handleCondoActiveTransit,    // 0x41
		StateAtWork:           handleCondoAtWork,           // 0x21
		StateAtWorkTransit:    handleCondoAtWorkTransit,    // 0x61 alias of 0x21
		StateVenueTrip:        handleCondoVenueTrip,        // 0x22
		StateVenueHomeTransit: handleCondoVenueHomeTransit, // 0x62
	}
}

// ProcessCondoSim TS: sims/condo.ts processCondoSim
func ProcessCondoSim(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	object := FindObjectForSim(world, sim)
	if object == nil {
		return
	}
	if handler, ok := CondoRefreshHandlerTable[sim.StateCode]; ok {
		handler(world, ledger, time, sim, object)
	}
}

// dispatchCondoCommute TS: sims/condo.ts dispatchCondoCommute (dispatch_0x00: home -> LOBBY)
func dispatchCondoCommute(world *WorldState, time *TimeState, sim *SimRecord) {
	directionFlag := 1
	if sim.FloorAnchor > LobbyFloor {
		directionFlag = 0
	}
	result := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, LobbyFloor, directionFlag, time, nil)
	if result == -1 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	if result == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateAtWork
		return
	}
	sim.StateCode = StateCommuteTransit
}

// dispatchCondoActive TS: sims/condo.ts dispatchCondoActive (dispatch_0x01
// selector 1228:3b34-3b54).
func dispatchCondoActive(world *WorldState, time *TimeState, sim *SimRecord) {
	var venueFamilies map[int]bool
	switch {
	case time.WeekendFlag == 0:
		venueFamilies = CondoSelectorRetail
	case sim.FacilitySlot%4 == 0:
		venueFamilies = CondoSelectorRestaurant
	default:
		venueFamilies = CondoSelectorFastFood
	}
	trip := StateActiveTransit
	dispatched := DispatchCommercialVenueVisit(world, time, sim, CommercialVenueVisitOptions{
		VenueFamilies:            venueFamilies,
		ReturnState:              StateCheckoutQueue,
		TripState:                &trip,
		SkipPenaltyOnUnavailable: true,
	})
	if !dispatched {
		routeCondoToLobbyNoVenue(world, time, sim)
	}
}

// routeCondoToLobbyNoVenue TS: sims/condo.ts routeCondoToLobbyNoVenue
// (binary route_sim_to_commercial_venue 1238:0000 no-venue lobby path).
func routeCondoToLobbyNoVenue(world *WorldState, time *TimeState, sim *SimRecord) {
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
	sim.StateCode = StateActiveTransit
}

// dispatchCondoAtWork TS: sims/condo.ts dispatchCondoAtWork
func dispatchCondoAtWork(world *WorldState, time *TimeState, sim *SimRecord) {
	directionFlag := dirFlag(sim.FloorAnchor, LobbyFloor)
	result := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor, directionFlag, time, nil)
	if result == -1 || result == 3 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	sim.StateCode = StateAtWorkTransit
}

// dispatchCondoVenueTrip TS: sims/condo.ts dispatchCondoVenueTrip
func dispatchCondoVenueTrip(world *WorldState, time *TimeState, sim *SimRecord) {
	directionFlag := dirFlag(sim.FloorAnchor, sim.SelectedFloor)
	result := ResolveSimRouteBetweenFloors(world, sim, sim.SelectedFloor, sim.FloorAnchor, directionFlag, time, nil)
	if result == -1 || result == 3 {
		sim.StateCode = StateCheckoutQueue
		return
	}
	sim.StateCode = StateVenueHomeTransit
}

// HandleCondoSimArrival TS: sims/condo.ts handleCondoSimArrival. The trip
// counter advance fires only for carrier arrivals (the per-stride rc=3 path
// already advanced inside resolve).
func HandleCondoSimArrival(sim *SimRecord, arrivalFloor int, time *TimeState, arrivedViaCarrier bool) {
	if sim.StateCode == StateMorningTransit && arrivalFloor == sim.FloorAnchor {
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
	if sim.StateCode == StateCommuteTransit && arrivalFloor == LobbyFloor {
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateAtWork
		return
	}
	if sim.StateCode == StateActiveTransit && arrivalFloor == LobbyFloor &&
		sim.VenueReturnState == StateCheckoutQueue {
		// No-venue fallback arrival (1238:0000 lobby path) -> 0x22.
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		return
	}
	if sim.StateCode == StateActiveTransit && arrivalFloor != LobbyFloor {
		// Real commercial venue: binary 1228:4fab writes 0x22.
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = arrivalFloor
		sim.StateCode = StateVenueTrip
		sim.QueueTick = time.DayTick
		sim.VenueReturnState = 0
		return
	}
	if sim.StateCode == StateAtWorkTransit && arrivalFloor == sim.FloorAnchor {
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
	if sim.StateCode == StateVenueHomeTransit && arrivalFloor == sim.FloorAnchor {
		if arrivedViaCarrier {
			AdvanceSimTripCounters(sim)
		}
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		sim.StateCode = StateCheckoutQueue
		return
	}
}
