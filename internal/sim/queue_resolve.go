package sim

// Port of reference/tower-together/sim/queue/resolve.ts.
//
// 1218:0000 resolve_sim_route_between_floors
//
// Binary-faithful route resolver. Consults the route scorer, then either
// (a) walks the sim over a special-link segment, (b) enqueues onto a
// carrier queue, or (c) reports no-route / same-floor.

// sharedRouteSelectorFamilies TS: SHARED_ROUTE_SELECTOR_FAMILIES (lookup-only)
var sharedRouteSelectorFamilies = map[int]bool{
	FamilyHotelSingle:       true,
	FamilyHotelTwin:         true,
	FamilyHotelSuite:        true,
	FamilyRestaurant:        true,
	FamilyOffice:            true,
	FamilyCondo:             true,
	FamilyFastFood:          true,
	FamilyRetail:            true,
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
	FamilyPartyHall:         true,
	FamilyPartyHallLower:    true,
}

// resolveStateBitFamilies TS: STATE_BIT_FAMILIES (queue/resolve.ts; same
// set as SHARED_ROUTE_SELECTOR_FAMILIES). Families whose state machine reads
// the 0x40 / 0x20 bits through dispatch_sim_behavior (1228:186c).
var resolveStateBitFamilies = map[int]bool{
	FamilyHotelSingle:       true,
	FamilyHotelTwin:         true,
	FamilyHotelSuite:        true,
	FamilyRestaurant:        true,
	FamilyOffice:            true,
	FamilyCondo:             true,
	FamilyFastFood:          true,
	FamilyRetail:            true,
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
	FamilyPartyHall:         true,
	FamilyPartyHallLower:    true,
}

// TS: queue/resolve.ts isCathedralFamily
func isCathedralFamily(familyCode int) bool {
	return familyCode >= FamilyCathedralBase && familyCode <= FamilyCathedralMax
}

// TS: queue/resolve.ts familyUsesStateBits
func familyUsesStateBits(familyCode int) bool {
	return resolveStateBitFamilies[familyCode]
}

// TS: queue/resolve.ts selectRouteForFamily
func selectRouteForFamily(world *WorldState, familyCode, fromFloor, toFloor, targetHeightMetric int) *RouteCandidate {
	if familyCode == FamilyHousekeeping {
		return SelectHousekeepingRoute(world, fromFloor, toFloor, targetHeightMetric)
	}
	if isCathedralFamily(familyCode) {
		return SelectCathedralRoute(world, fromFloor, toFloor, targetHeightMetric)
	}
	if sharedRouteSelectorFamilies[familyCode] {
		return SelectBestRouteCandidate(world, fromFloor, toFloor, true /* preferLocalMode */, targetHeightMetric)
	}
	return nil
}

// RouteResolution values (TS RouteResolution = -1 | 0 | 1 | 2 | 3):
//
//	-1 no viable route (sim remains unrouted)
//	 0 carrier queue full; sim parks on the source floor and retries
//	 1 direct special-link leg accepted
//	 2 queued onto a carrier
//	 3 same-floor success (binary quirk: 3, not 2)
const (
	RouteResolutionNoRoute   = -1
	RouteResolutionQueueFull = 0
	RouteResolutionSegment   = 1
	RouteResolutionQueued    = 2
	RouteResolutionSameFloor = 3
)

// ResolveSimRouteOptions mirrors the binary's two short arguments to
// resolve_sim_route_between_floors plus the height metric. nil fields take
// the TS defaults: TargetHeightMetric = sim.HomeColumn, IsPassengerRoute =
// true, EmitDistanceFeedback = true. Use trBoolPtr / intPtr to set them; a
// nil *ResolveSimRouteOptions is the TS default parameter `= {}`.
// TS: queue/resolve.ts ResolveSimRouteOptions
type ResolveSimRouteOptions struct {
	TargetHeightMetric   *int
	IsPassengerRoute     *bool
	EmitDistanceFeedback *bool
}

// ResolveSimRouteBetweenFloors returns a RouteResolution code (-1..3).
// time may be nil (TS `TimeState | undefined`); options may be nil.
// TS: queue/resolve.ts resolveSimRouteBetweenFloors (binary 1218:0000)
func ResolveSimRouteBetweenFloors(world *WorldState, sim *SimRecord, sourceFloor, destinationFloor, directionFlag int, time *TimeState, options *ResolveSimRouteOptions) int {
	if options == nil {
		options = &ResolveSimRouteOptions{}
	}
	targetHeightMetric := sim.HomeColumn
	if options.TargetHeightMetric != nil {
		targetHeightMetric = *options.TargetHeightMetric
	}
	isPassengerRoute := true
	if options.IsPassengerRoute != nil {
		isPassengerRoute = *options.IsPassengerRoute
	}
	emitDistanceFeedback := true
	if options.EmitDistanceFeedback != nil {
		emitDistanceFeedback = *options.EmitDistanceFeedback
	}

	// Binary 1218:001e/0029: clamp negative source/target to LOBBY_FLOOR.
	if sourceFloor < 0 {
		sourceFloor = LobbyFloor
	}
	if destinationFloor < 0 {
		destinationFloor = LobbyFloor
	}

	if sourceFloor == destinationFloor {
		// Binary 1218:0046: same-floor branch, trip-counter advance gated on
		// is_passenger_route.
		if isPassengerRoute {
			AdvanceSimTripCounters(sim)
		}
		return RouteResolutionSameFloor
	}

	route := selectRouteForFamily(world, sim.FamilyCode, sourceFloor, destinationFloor, targetHeightMetric)
	if route == nil {
		ClearSimRoute(sim)
		// No-route path: failure delay (300) and trip-counter advance are
		// gated on is_passenger_route.
		if isPassengerRoute {
			AddDelayToCurrentSim(sim, 300)
			AdvanceSimTripCounters(sim)
		}
		return RouteResolutionNoRoute
	}

	if route.Kind == RouteCandidateSegment {
		// Binary 1218:0140-019d: one leg per call; with per-tile placement
		// each leg advances exactly one floor.
		var segment *SpecialLinkSegment
		if route.ID >= 0 && route.ID < len(world.SpecialLinks) {
			segment = world.SpecialLinks[route.ID]
		}
		if emitDistanceFeedback {
			routeHeight := targetHeightMetric
			if segment != nil {
				routeHeight = segment.HeightMetric
			}
			MaybeApplyDistanceFeedback(world, sim, routeHeight, targetHeightMetric, true)
		}
		direction := -1
		if destinationFloor > sourceFloor {
			direction = 1
		}
		nextFloor := sourceFloor + direction
		sim.Route = RouteState{Mode: RouteModeSegment, SegmentID: route.ID, Destination: destinationFloor}
		if familyUsesStateBits(sim.FamilyCode) {
			SetSimInTransit(sim, true)
		}
		if time != nil {
			sim.QueueTick = time.DayTick
		}
		sim.SelectedFloor = nextFloor
		sim.DestinationFloor = destinationFloor
		// Per-leg stress penalty (per-stop parity delay x 1 step), gated on
		// is_passenger_route.
		if isPassengerRoute {
			parityBit := 0
			if segment != nil {
				parityBit = segment.Flags & 1
			}
			AddDelayToCurrentSim(sim, PerStopParityDelay[parityBit])
		}
		sim.TransitTicksRemaining = 0
		if time != nil {
			sim.LastDemandTick = time.DayTick
		}
		return RouteResolutionSegment
	}

	carrier := trFindCarrier(world, route.ID)
	if carrier == nil {
		ClearSimRoute(sim)
		if isPassengerRoute {
			AddDelayToCurrentSim(sim, 300)
			AdvanceSimTripCounters(sim)
		}
		return RouteResolutionNoRoute
	}

	queued := EnqueueRequestIntoRouteQueue(carrier, SimKey(sim), sourceFloor, destinationFloor, directionFlag, world.LobbyMode)
	if !queued {
		// Queue-full path: sim+8 = 0xff, sim+7 = source floor, waiting bit;
		// 5-tick g_waiting_state_delay gated on is_passenger_route.
		ClearSimRoute(sim)
		if familyUsesStateBits(sim.FamilyCode) {
			SetSimWaiting(sim, true)
		}
		sim.SelectedFloor = sourceFloor
		sim.DestinationFloor = destinationFloor
		if isPassengerRoute {
			AddDelayToCurrentSim(sim, 5)
		}
		return RouteResolutionQueueFull
	}

	dirName := "down"
	if directionFlag == 1 {
		dirName = "up"
	}
	sim.Route = RouteState{Mode: RouteModeCarrier, CarrierID: route.ID, Direction: dirName, Source: sourceFloor}
	if familyUsesStateBits(sim.FamilyCode) {
		SetSimInTransit(sim, true)
	}
	if time != nil {
		sim.QueueTick = time.DayTick
	}
	// Binary: carrier branch writes sim+7 = source_floor.
	sim.SelectedFloor = sourceFloor
	sim.DestinationFloor = destinationFloor
	if emitDistanceFeedback {
		MaybeApplyDistanceFeedback(world, sim, carrier.Column, targetHeightMetric, carrier.CarrierMode != 0)
	}
	if time != nil {
		sim.LastDemandTick = time.DayTick
	}
	return RouteResolutionQueued
}
