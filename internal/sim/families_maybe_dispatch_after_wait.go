package sim

// Port of reference/tower-together/sim/families/maybe-dispatch-after-wait.ts
// (binary 1228:15a0 maybe_dispatch_queued_route_after_wait).

// routeWaitTimeoutTicks TS: ROUTE_WAIT_TIMEOUT_TICKS. Binary
// g_route_delay_table_base @ 1288:e5ee (resource 0xff05 id 1000 word 0).
const routeWaitTimeoutTicks = 300

// officeWaitTimeoutToNightBStates TS: OFFICE_WAIT_TIMEOUT_TO_NIGHT_B_STATES
// (family-7 jumptable 1228:1c51 -> 1228:193d). Lookup-only.
// STATE_VENUE_HOME_TRANSIT == STATE_VENUE_TRIP_TRANSIT (0x62).
var officeWaitTimeoutToNightBStates = map[int]bool{
	StateDepartureTransit:   true,
	StateMorningTransit:     true,
	StateAtWorkTransit:      true,
	StateVenueHomeTransit:   true,
	StateDwellReturnTransit: true,
}

// hotelWaitTimeoutToNightBStates TS: HOTEL_WAIT_TIMEOUT_TO_NIGHT_B_STATES (cs:1c41 0x45 -> 1228:19f4).
var hotelWaitTimeoutToNightBStates = map[int]bool{StateDepartureTransit: true}

// HOTEL_FAMILY_CODES: same set as sims/trip-counters.ts, reused from
// sims_trip_counters.go (hotelFamilyCodes).

// commercialFamilyCodes TS: COMMERCIAL_FAMILY_CODES
var commercialFamilyCodes = map[int]bool{FamilyRestaurant: true, FamilyRetail: true, FamilyFastFood: true}

// commercialWaitTimeoutToParkedStates TS: COMMERCIAL_WAIT_TIMEOUT_TO_PARKED_STATES (1228:4d37 -> 1228:4c9e).
var commercialWaitTimeoutToParkedStates = map[int]bool{StateDepartureTransit: true}

// dispatchTimedOutOfficeQueueEntry TS: maybe-dispatch-after-wait.ts dispatchTimedOutOfficeQueueEntry
func dispatchTimedOutOfficeQueueEntry(world *WorldState, time *TimeState, sim *SimRecord) {
	RebaseSimElapsedFromClock(sim, time)
	AdvanceSimTripCounters(sim)

	if sim.StateCode == StateCommuteTransit ||
		sim.StateCode == StateActiveTransit ||
		sim.StateCode == StateVenueTripTransit {
		if object := FindObjectForSim(world, sim); object != nil {
			AdvanceOfficePresenceCounter(object)
		}
		sim.SelectedFloor = sim.FloorAnchor
		sim.DestinationFloor = -1
		sim.StateCode = StateDeparture
		ClearSimRoute(sim)
		return
	}

	if officeWaitTimeoutToNightBStates[sim.StateCode] {
		sim.StateCode = StateNightB
	}
	sim.DestinationFloor = -1
	ClearSimRoute(sim)
}

// dispatchTimedOutHotelQueueEntry TS: maybe-dispatch-after-wait.ts dispatchTimedOutHotelQueueEntry
func dispatchTimedOutHotelQueueEntry(_ *WorldState, time *TimeState, sim *SimRecord) {
	// Mirror the dispatch_sim_behavior (1228:186c) prologue.
	RebaseSimElapsedFromClock(sim, time)
	AdvanceSimTripCounters(sim)
	if hotelWaitTimeoutToNightBStates[sim.StateCode] {
		sim.StateCode = StateNightB
	}
	sim.DestinationFloor = -1
	ClearSimRoute(sim)
}

// dispatchTimedOutCommercialQueueEntry TS: maybe-dispatch-after-wait.ts dispatchTimedOutCommercialQueueEntry
func dispatchTimedOutCommercialQueueEntry(world *WorldState, time *TimeState, sim *SimRecord) {
	RebaseSimElapsedFromClock(sim, time)
	AdvanceSimTripCounters(sim)
	// Binary 1228:4c9e: state 0x45 -> PARKED (0x27) and release.
	if commercialWaitTimeoutToParkedStates[sim.StateCode] {
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
	}
	sim.DestinationFloor = -1
	ClearSimRoute(sim)
}

// findWaitCarrier is the TS `world.carriers.find(c => c.carrierId === id)`
// (first match in carrier order; nil when absent).
func findWaitCarrier(world *WorldState, id int) *CarrierRecord {
	for _, c := range world.Carriers {
		if c.CarrierID == id {
			return c
		}
	}
	return nil
}

// findPendingRoute is the TS `carrier.pendingRoutes.find(r => r.simId === id)`.
func findPendingRoute(carrier *CarrierRecord, id string) *CarrierPendingRoute {
	for _, r := range carrier.PendingRoutes {
		if r.SimID == id {
			return r
		}
	}
	return nil
}

// dispatchQueuedRouteUntilRequest TS: maybe-dispatch-after-wait.ts
// dispatchQueuedRouteUntilRequest (binary 1218:1981
// dispatch_queued_route_until_request): pop the floor queue in FIFO order,
// dispatching each popped sim, until the requesting sim itself is popped.
func dispatchQueuedRouteUntilRequest(world *WorldState, time *TimeState, sim *SimRecord) {
	routeState := sim.Route
	if routeState.Mode != RouteModeCarrier {
		switch {
		case sim.FamilyCode == FamilyOffice:
			dispatchTimedOutOfficeQueueEntry(world, time, sim)
		case hotelFamilyCodes[sim.FamilyCode]:
			dispatchTimedOutHotelQueueEntry(world, time, sim)
		case commercialFamilyCodes[sim.FamilyCode]:
			dispatchTimedOutCommercialQueueEntry(world, time, sim)
		}
		return
	}
	carrier := findWaitCarrier(world, routeState.CarrierID)
	if carrier == nil {
		return
	}
	routeID := SimKey(sim)
	route := findPendingRoute(carrier, routeID)
	if route == nil || route.Boarded {
		return
	}
	slot := FloorToSlot(carrier, route.SourceFloor)
	if slot < 0 {
		return
	}
	if slot >= len(carrier.FloorQueues) {
		return
	}
	queue := carrier.FloorQueues[slot]
	if queue == nil {
		return
	}

	for {
		poppedRouteID, ok := PopUnitQueueRequest(queue, route.DirectionFlag)
		if !ok || poppedRouteID == "" {
			break
		}
		kept := make([]*CarrierPendingRoute, 0, len(carrier.PendingRoutes))
		for _, candidate := range carrier.PendingRoutes {
			if candidate.SimID != poppedRouteID {
				kept = append(kept, candidate)
			}
		}
		carrier.PendingRoutes = kept
		var poppedSim *SimRecord
		for _, candidate := range world.Sims {
			if SimKey(candidate) == poppedRouteID {
				poppedSim = candidate
				break
			}
		}
		if poppedSim != nil {
			switch {
			case poppedSim.FamilyCode == FamilyOffice:
				dispatchTimedOutOfficeQueueEntry(world, time, poppedSim)
			case hotelFamilyCodes[poppedSim.FamilyCode]:
				dispatchTimedOutHotelQueueEntry(world, time, poppedSim)
			case commercialFamilyCodes[poppedSim.FamilyCode]:
				dispatchTimedOutCommercialQueueEntry(world, time, poppedSim)
			}
		}
		if poppedRouteID == routeID {
			break
		}
	}
	SyncAssignmentStatus(carrier)
}

// MaybeDispatchQueuedRouteAfterWait TS: families/maybe-dispatch-after-wait.ts
// maybeDispatchQueuedRouteAfterWait (binary 1228:15a0). Fires only for
// in-transit sims sitting on a carrier queue (state&0x40 and
// encoded_route_target >= 0x40) once 300 ticks have elapsed since enqueue.
func MaybeDispatchQueuedRouteAfterWait(world *WorldState, _ *LedgerState, time *TimeState, sim *SimRecord) {
	if !IsSimInTransit(sim.StateCode) {
		return
	}
	if sim.Route.Mode != RouteModeCarrier {
		return
	}
	switch {
	case sim.FamilyCode == FamilyOffice:
		// No office-state filter in 1228:15a0 itself.
	case hotelFamilyCodes[sim.FamilyCode]:
		if !hotelWaitTimeoutToNightBStates[sim.StateCode] {
			return
		}
	case commercialFamilyCodes[sim.FamilyCode]:
		if !commercialWaitTimeoutToParkedStates[sim.StateCode] {
			return
		}
	default:
		return
	}
	if sim.LastDemandTick < 0 {
		return
	}
	// 16-bit unsigned subtraction across the day wrap.
	elapsed := (time.DayTick - sim.LastDemandTick + DayTickMax) % DayTickMax
	if elapsed <= routeWaitTimeoutTicks {
		return
	}
	carrier := findWaitCarrier(world, sim.Route.CarrierID)
	if carrier == nil {
		return
	}
	route := findPendingRoute(carrier, SimKey(sim))
	if route == nil || route.Boarded {
		return
	}
	dispatchQueuedRouteUntilRequest(world, time, sim)
}
