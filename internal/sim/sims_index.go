package sim

// Port of reference/tower-together/sim/sims/index.ts: the runtime sims
// facade (stride refresh, arrival dispatch, legacy segment reconcile) and
// the shared commercial-venue helpers used by the office/condo/hotel and
// commercial families. The TS file's re-exports need no Go shims: the
// package is flat.

// ReleaseServiceRequest TS: sims/index.ts releaseServiceRequest
func ReleaseServiceRequest(_ *WorldState, sim *SimRecord) {
	sim.DestinationFloor = -1
	ClearSimRoute(sim)
}

// recomputeRoutesViableFlag TS: sims/index.ts recomputeRoutesViableFlag.
// Binary-grounded: rebuild_path_seed_bucket_table unconditionally latches
// routesViable = 1 whenever star_count > 2.
func recomputeRoutesViableFlag(world *WorldState) {
	if world.StarCount > 2 {
		world.GateFlags.RoutesViable = 1
	} else {
		world.GateFlags.RoutesViable = 0
	}
}

// venueSelection TS: sims/index.ts VenueSelection
type venueSelection struct {
	record       *CommercialVenueRecord
	floor        int
	heightMetric int
	// ownerFamily: PlacedObjectRecord.objectTypeCode of the venue's owning
	// facility (binary record[+0] sim_type, acquire_commercial_venue_slot 11b0:0d92).
	ownerFamily int
}

// commercialVenueAt returns the sidecar at idx when it is a commercial
// venue (TS `record && record.kind === "commercial_venue"`).
func commercialVenueAt(world *WorldState, idx int) *CommercialVenueRecord {
	rec, _ := GetSidecar(world, idx).(*CommercialVenueRecord)
	return rec
}

// FindCommercialVenueAtFloor TS: sims/index.ts findCommercialVenueAtFloor.
// Returns the first venue (placedObjects insertion order) at `floor` whose
// family is in `families` and whose record is not INVALID; nil otherwise.
func FindCommercialVenueAtFloor(world *WorldState, floor int, families map[int]bool) *CommercialVenueRecord {
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if !families[object.ObjectTypeCode] {
			continue
		}
		if object.LinkedRecordIndex < 0 {
			continue
		}
		_, y := parseKeyXY(e.Key)
		if YToFloor(y) != floor {
			continue
		}
		record := commercialVenueAt(world, object.LinkedRecordIndex)
		if record == nil {
			continue
		}
		if record.OwnerSubtypeIndex == InvalidFloor {
			continue
		}
		return record
	}
	return nil
}

// Result codes mirroring binary acquire_commercial_venue_slot (11b0:0d92).
const (
	VenueSlotAcquired    = 3
	VenueSlotFull        = 2
	VenueSlotUnavailable = -1
)

// TryAcquireOfficeVenueSlot TS: sims/index.ts tryAcquireOfficeVenueSlot
// (binary acquire_commercial_venue_slot 11b0:0d92). Capacity gate is
// currentPopulation > 39; acquireCount (+0x10) is bumped only when the
// visitor's family differs from the venue owner's (11b0:0f3a-0f55).
func TryAcquireOfficeVenueSlot(venue *CommercialVenueRecord, sim *SimRecord, time *TimeState, venueOwnerFamily int) int {
	if venue.OwnerSubtypeIndex == 0xff ||
		venue.AvailabilityState == VenueDormant ||
		venue.AvailabilityState == VenueClosed {
		return VenueSlotUnavailable
	}
	if venue.CurrentPopulation > 39 {
		// Binary 11b0:0f6c+ writes entity[+0xa] on the full path; the TS
		// deliberately does not mirror that latch.
		return VenueSlotFull
	}
	venue.CurrentPopulation++
	if sim.FamilyCode != venueOwnerFamily {
		venue.AcquireCount++
	}
	sim.LastDemandTick = time.DayTick
	return VenueSlotAcquired
}

// ReleaseOfficeVenueSlot TS: sims/index.ts releaseOfficeVenueSlot (binary
// release_commercial_venue_slot 11b0:0fae). Returns false while the 60-tick
// dwell gate has not expired (unless skipDwellGate).
func ReleaseOfficeVenueSlot(venue *CommercialVenueRecord, sim *SimRecord, time *TimeState, skipDwellGate bool) bool {
	if venue.OwnerSubtypeIndex == 0xff ||
		venue.AvailabilityState == VenueDormant ||
		venue.AvailabilityState == VenueClosed {
		return true
	}
	if !skipDwellGate && time.DayTick-sim.LastDemandTick < CommercialVenueDwellTicks {
		return false
	}
	if venue.CurrentPopulation > 0 {
		venue.CurrentPopulation--
	}
	return true
}

// pickAvailableVenue TS: sims/index.ts pickAvailableVenue (binary
// select_random_commercial_venue_record_from_bucket 11b0:1361). The bucket
// holds every placed venue of the families, in placedObjects insertion
// order; sampleRng fires whenever the bucket is non-empty, even if the
// chosen venue is then rejected.
func pickAvailableVenue(world *WorldState, _ int, allowedFamilies map[int]bool) *venueSelection {
	bucket := []venueSelection{}
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if !allowedFamilies[object.ObjectTypeCode] {
			continue
		}
		if object.LinkedRecordIndex < 0 {
			continue
		}
		record := commercialVenueAt(world, object.LinkedRecordIndex)
		if record == nil {
			continue
		}
		if record.OwnerSubtypeIndex == InvalidFloor {
			continue
		}
		_, y := parseKeyXY(e.Key)
		bucket = append(bucket, venueSelection{
			record:       record,
			floor:        YToFloor(y),
			heightMetric: object.LeftTileIndex,
			ownerFamily:  object.ObjectTypeCode,
		})
	}

	if len(bucket) == 0 {
		return nil
	}
	picked := bucket[SampleRng(world)%len(bucket)]

	if picked.record.AvailabilityState == VenueClosed ||
		picked.record.AvailabilityState == VenueDormant {
		return nil
	}
	// Capacity and route viability are checked later (acquire at arrival,
	// resolve failure delay), not at pick time.
	return &picked
}

// beginCommercialVenueDwell TS: sims/index.ts beginCommercialVenueDwell
func beginCommercialVenueDwell(sim *SimRecord, arrivalFloor, returnState int, time *TimeState) {
	sim.DestinationFloor = -1
	sim.SelectedFloor = arrivalFloor
	ClearSimRoute(sim)
	sim.VenueReturnState = returnState
	sim.StateCode = CommercialDwellState
	sim.LastDemandTick = time.DayTick
}

// beginCommercialVenueTrip TS: sims/index.ts beginCommercialVenueTrip
func beginCommercialVenueTrip(sim *SimRecord, destinationFloor, tripState int) {
	sim.DestinationFloor = destinationFloor
	// Office + hotel + condo keep the resolve-written selectedFloor (per-leg);
	// other families reset to floorAnchor (legacy behavior).
	usesPerLeg := sim.FamilyCode == FamilyOffice ||
		sim.FamilyCode == FamilyCondo ||
		sim.FamilyCode == FamilyHotelSingle ||
		sim.FamilyCode == FamilyHotelTwin ||
		sim.FamilyCode == FamilyHotelSuite
	if !usesPerLeg {
		sim.SelectedFloor = sim.FloorAnchor
	}
	sim.StateCode = tripState
}

// FinishCommercialVenueDwell TS: sims/index.ts finishCommercialVenueDwell
func FinishCommercialVenueDwell(sim *SimRecord, time *TimeState, defaultState int) bool {
	if sim.StateCode != CommercialDwellState {
		return false
	}
	if time.DayTick-sim.LastDemandTick < CommercialVenueDwellTicks {
		return true
	}
	sim.SelectedFloor = sim.FloorAnchor
	// TS `sim.venueReturnState || defaultState`: 0 falls back.
	if sim.VenueReturnState != 0 {
		sim.StateCode = sim.VenueReturnState
	} else {
		sim.StateCode = defaultState
	}
	sim.VenueReturnState = 0
	return true
}

// FinishCommercialVenueTrip TS: sims/index.ts finishCommercialVenueTrip
func FinishCommercialVenueTrip(sim *SimRecord, returnState int) bool {
	if sim.StateCode != StateVenueTrip {
		return false
	}
	if sim.SelectedFloor != sim.DestinationFloor {
		return true
	}
	sim.DestinationFloor = -1
	sim.SelectedFloor = sim.FloorAnchor
	sim.StateCode = returnState
	return true
}

// CommercialVenueVisitOptions TS: the options bag of
// sims/index.ts dispatchCommercialVenueVisit. TripState / UnavailableState
// are nil when the TS field is undefined.
type CommercialVenueVisitOptions struct {
	VenueFamilies               map[int]bool
	ReturnState                 int
	TripState                   *int
	UnavailableState            *int
	SkipPenaltyOnUnavailable    bool
	AdvanceBeforeSameFloorDwell bool
	OnVenueReserved             func()
}

func (o *CommercialVenueVisitOptions) tripState() int {
	if o.TripState != nil {
		return *o.TripState
	}
	return StateVenueTrip
}

// DispatchCommercialVenueVisit TS: sims/index.ts dispatchCommercialVenueVisit
// (binary route_sim_to_commercial_venue 1238:0000, state-0x01 branch).
func DispatchCommercialVenueVisit(world *WorldState, time *TimeState, sim *SimRecord, options CommercialVenueVisitOptions) bool {
	venue := pickAvailableVenue(world, sim.FloorAnchor, options.VenueFamilies)
	if venue == nil {
		if !options.SkipPenaltyOnUnavailable {
			AddDelayToCurrentSim(sim, 300)
			AdvanceSimTripCounters(sim)
		}
		if options.UnavailableState != nil {
			sim.StateCode = *options.UnavailableState
		}
		return false
	}

	// Route requirement: resolve route before reserving venue.
	if venue.floor != sim.FloorAnchor {
		dirFlag := 0
		if venue.floor > sim.FloorAnchor {
			dirFlag = 1
		}
		routeResult := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, venue.floor, dirFlag, time, nil)
		if routeResult == -1 {
			// Binary 1238:0000 hides the resolve failure from state 0x01: the
			// next-stride 0x41 alias re-resolves to the lobby (0xff sentinel).
			sim.StateCode = options.tripState()
			sim.SelectedFloor = sim.FloorAnchor
			sim.DestinationFloor = LobbyFloor
			return true
		}
		if routeResult == 0 {
			if options.UnavailableState != nil {
				sim.StateCode = *options.UnavailableState
			}
			return false
		}
		// Cross-floor: state 0x41 (tripState); acquire fires at arrival.
		if options.OnVenueReserved != nil {
			options.OnVenueReserved()
		}
		beginCommercialVenueTrip(sim, venue.floor, options.tripState())
		return true
	}

	// Same-floor branch: resolve rc=3 then acquire_commercial_venue_slot.
	if options.AdvanceBeforeSameFloorDwell {
		AdvanceSimTripCounters(sim)
	}
	acquireResult := TryAcquireOfficeVenueSlot(venue.record, sim, time, venue.ownerFamily)
	if acquireResult == VenueSlotFull {
		sim.StateCode = options.tripState()
		sim.SelectedFloor = sim.FloorAnchor
		sim.DestinationFloor = sim.FloorAnchor
		sim.VenueReturnState = options.ReturnState
		return true
	}
	if options.OnVenueReserved != nil {
		options.OnVenueReserved()
	}
	beginCommercialVenueDwell(sim, venue.floor, options.ReturnState, time)
	return true
}

// HandleCommercialVenueArrival TS: sims/index.ts handleCommercialVenueArrival.
// The TS default tripState is STATE_VENUE_TRIP; Go callers pass it explicitly.
func HandleCommercialVenueArrival(sim *SimRecord, arrivalFloor, returnState int, time *TimeState, tripState int) bool {
	if sim.StateCode != tripState || sim.DestinationFloor != arrivalFloor {
		return false
	}
	beginCommercialVenueDwell(sim, arrivalFloor, returnState, time)
	return true
}

// AdvanceSimRefreshStride TS: sims/index.ts advanceSimRefreshStride
// (binary refresh_runtime_entities_for_tick_stride 1228:0d64). Walks every
// 16th sim in world.sims order.
func AdvanceSimRefreshStride(world *WorldState, ledger *LedgerState, time *TimeState) {
	if len(world.Sims) == 0 {
		return
	}

	stride := time.DayTick % EntityRefreshStride
	for index := 0; index < len(world.Sims); index++ {
		if index%EntityRefreshStride != stride {
			continue
		}
		sim := world.Sims[index]
		// Binary parity: no rebase_sim_elapsed_from_clock here (see TS comment).
		finalizePendingRouteLeg(sim)
		// Capture the queued-carrier-alias gate BEFORE the helper: a timeout
		// rewrites state and route, and the binary (1228:1d59-1d6e) skips the
		// per-state dispatch when the helper ran.
		wasQueuedCarrierAlias := IsSimInTransit(sim.StateCode) && sim.Route.Mode == RouteModeCarrier
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		if wasQueuedCarrierAlias {
			continue
		}
		if sim.Route.Mode == RouteModeCarrier {
			continue
		}
		if sim.Route.Mode == RouteModeSegment &&
			sim.FamilyCode != FamilyOffice &&
			sim.FamilyCode != FamilyCondo &&
			sim.FamilyCode != FamilyHotelSingle &&
			sim.FamilyCode != FamilyHotelTwin &&
			sim.FamilyCode != FamilyHotelSuite &&
			sim.FamilyCode != FamilyHousekeeping &&
			sim.FamilyCode != FamilyRestaurant &&
			sim.FamilyCode != FamilyFastFood &&
			sim.FamilyCode != FamilyRetail &&
			sim.FamilyCode != FamilyCinema &&
			sim.FamilyCode != FamilyCinemaLower &&
			sim.FamilyCode != FamilyCinemaStairsUpper &&
			sim.FamilyCode != FamilyCinemaStairsLower &&
			sim.FamilyCode != FamilyPartyHall &&
			sim.FamilyCode != FamilyPartyHallLower {
			continue
		}
		switch sim.FamilyCode {
		case FamilyHotelSingle, FamilyHotelTwin, FamilyHotelSuite:
			ProcessHotelSim(world, ledger, time, sim)
		case FamilyOffice:
			if sim.StateCode == StateMedicalTrip ||
				sim.StateCode == StateMedicalTripTransit ||
				sim.StateCode == StateMedicalDwell {
				ProcessMedicalSim(world, time, sim)
			} else {
				ProcessOfficeSim(world, ledger, time, sim)
			}
		case FamilyCondo:
			ProcessCondoSim(world, ledger, time, sim)
		case FamilyRestaurant, FamilyFastFood, FamilyRetail:
			ProcessCommercialSim(world, ledger, time, sim)
		case FamilyHousekeeping:
			ProcessHousekeepingSim(world, time, sim)
		case FamilyCinema, FamilyCinemaLower, FamilyCinemaStairsUpper,
			FamilyCinemaStairsLower, FamilyPartyHall, FamilyPartyHallLower:
			GateEntertainmentGuestState(world, ledger, time, sim)
		default:
			if CathedralFamilies[sim.FamilyCode] {
				ProcessCathedralSim(world, time, sim)
			}
		}
	}

	recomputeRoutesViableFlag(world)
}

// shouldFinalizeSegmentTrip TS: sims/index.ts shouldFinalizeSegmentTrip
func shouldFinalizeSegmentTrip(sim *SimRecord) bool {
	switch sim.FamilyCode {
	case FamilyOffice, FamilyCondo,
		FamilyHotelSingle, FamilyHotelTwin, FamilyHotelSuite,
		FamilyRestaurant, FamilyFastFood, FamilyRetail,
		FamilyHousekeeping,
		FamilyCinema, FamilyCinemaLower, FamilyCinemaStairsUpper,
		FamilyCinemaStairsLower, FamilyPartyHall, FamilyPartyHallLower:
		return false
	}
	switch sim.StateCode {
	case StateCommute, StateCommuteTransit, StateActiveTransit, StateVenueTrip,
		StateVenueTripTransit, StateDeparture, StateDepartureTransit,
		StateMorningTransit, StateAtWorkTransit, StateDwellReturnTransit:
		// STATE_VENUE_HOME_TRANSIT == STATE_VENUE_TRIP_TRANSIT (0x62).
		return true
	}
	return false
}

// finalizePendingRouteLeg TS: sims/index.ts finalizePendingRouteLeg
func finalizePendingRouteLeg(sim *SimRecord) {
	if sim.Route.Mode != RouteModeSegment {
		return
	}
	if sim.TransitTicksRemaining > 0 {
		return
	}
	switch sim.FamilyCode {
	case FamilyOffice, FamilyCondo,
		FamilyHotelSingle, FamilyHotelTwin, FamilyHotelSuite,
		FamilyRestaurant, FamilyFastFood, FamilyRetail,
		FamilyCinema, FamilyCinemaLower, FamilyCinemaStairsUpper,
		FamilyCinemaStairsLower, FamilyPartyHall, FamilyPartyHallLower:
		return
	}
	// PORT-NOTE: housekeeping is NOT in this skip list in the TS (it is in
	// shouldFinalizeSegmentTrip's), so HK segment sims get selectedFloor
	// synced here. Kept verbatim.
	sim.SelectedFloor = sim.Route.Destination
}

// DispatchSimArrival TS: sims/index.ts dispatchSimArrival. Called inline
// from queue/dispatch-arrivals.ts (binary 1218:0883). Arrival does NOT
// advance trip counters here.
func DispatchSimArrival(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	arrivedViaCarrier := sim.Route.Mode == RouteModeCarrier
	sim.SelectedFloor = arrivalFloor
	ClearSimRoute(sim)
	switch sim.FamilyCode {
	case FamilyHotelSingle, FamilyHotelTwin, FamilyHotelSuite:
		HandleHotelSimArrival(world, ledger, time, sim, arrivalFloor)
	case FamilyOffice:
		if sim.StateCode == StateMedicalTripTransit {
			HandleMedicalSimArrival(world, sim, arrivalFloor)
			return
		}
		HandleOfficeSimArrival(world, time, sim, arrivalFloor, false)
	case FamilyCondo:
		HandleCondoSimArrival(sim, arrivalFloor, time, arrivedViaCarrier)
	case FamilyRestaurant, FamilyFastFood, FamilyRetail:
		HandleCommercialSimArrival(world, sim, arrivalFloor, time)
	case FamilyHousekeeping:
		HandleHousekeepingSimArrival(world, time, sim, arrivalFloor)
	case FamilyCinema, FamilyCinemaLower, FamilyCinemaStairsUpper,
		FamilyCinemaStairsLower, FamilyPartyHall, FamilyPartyHallLower:
		HandleEntertainmentSimArrival(world, time, sim, arrivalFloor)
	default:
		if CathedralFamilies[sim.FamilyCode] {
			HandleCathedralSimArrival(world, time, sim, arrivalFloor)
		}
	}
}

// ReconcileSimTransport TS: sims/index.ts reconcileSimTransport. Legacy
// whole-trip finalizer for families not on per-leg progression, then the
// completed-route re-dispatch.
func ReconcileSimTransport(world *WorldState, ledger *LedgerState, time *TimeState) {
	for _, sim := range world.Sims {
		if sim.Route.Mode != RouteModeSegment {
			continue
		}
		if !shouldFinalizeSegmentTrip(sim) {
			continue
		}
		if sim.TransitTicksRemaining > 0 {
			sim.TransitTicksRemaining--
			continue
		}
		DispatchSimArrival(world, ledger, time, sim, sim.Route.Destination)
	}

	// Lookup-only set: insertion order irrelevant.
	completed := map[string]bool{}
	for _, carrier := range world.Carriers {
		for _, routeID := range carrier.CompletedRouteIDs {
			completed[routeID] = true
		}
		carrier.CompletedRouteIDs = []string{}
	}

	for _, sim := range world.Sims {
		if sim.DestinationFloor < 0 {
			continue
		}
		if !completed[SimKey(sim)] {
			continue
		}
		// Skip if the inline arrival already cleared the route (idle) or
		// re-enqueued the next transfer leg (carrier).
		if sim.Route.Mode == RouteModeIdle || sim.Route.Mode == RouteModeCarrier {
			continue
		}
		DispatchSimArrival(world, ledger, time, sim, sim.DestinationFloor)
	}
}
