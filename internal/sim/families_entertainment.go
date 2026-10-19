package sim

// Port of reference/tower-together/sim/families/entertainment.ts.
// 1228:5231 gate_entertainment_guest_state (family 18/29)
// 1228:53ad dispatch_entertainment_guest_state (family 18/29)
//
// Entertainment guest state machine shared by Movie Theater (family 18) and
// Party Hall (family 29) visitors, plus the commercial-venue bucket rebuild
// (rebuild_linked_facility_records 11b0:0184).
//
// Binary jump table at cs:0x5b3a (dispatch, 8 entries):
//   0x01/0x41 -> handle_entertainment_service_acquisition  (1228:57e2)
//   0x05/0x45 -> handle_entertainment_linked_half_routing  (1228:5746)
//   0x20/0x60 -> handle_entertainment_phase_consumption    (1228:54b8)
//   0x22/0x62 -> handle_entertainment_service_release_return (1228:5a23)
//
// Binary selectors:
//   sim[+5]  = stateCode
//   sim[+6]  = selectedFloor (venue sub-selector byte; 0xb0 = no venue chosen)
//   sim[+7]  = originFloor   (return-to floor written at route time)

// entertainmentGuestFamilies TS: ENTERTAINMENT_GUEST_FAMILIES (lookup only)
var entertainmentGuestFamilies = map[int]bool{
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
	FamilyPartyHall:         true,
	FamilyPartyHallLower:    true,
}

// Entertainment state codes (binary sim[+5] values).
const (
	entStateServiceAcquire        = 0x01 // select venue and route to it
	entStateLinkedHalf            = 0x05 // route to lobby (linked-half transition)
	entStatePhaseConsume          = 0x20 // consume phase budget; route to venue
	entStateVenueDwell            = 0x22 // at venue; release slot and return
	entStateServiceAcquireTransit = 0x41
	entStateLinkedHalfTransit     = 0x45
	entStatePhaseConsumeTransit   = 0x60
	entStateVenueDwellTransit     = 0x62
	entStateParked                = 0x27
)

const (
	entGateDayTickMin     = 0xf0 // dayTick must be > this for state-0x20 to fire
	entGateRngModulo      = 6    // 1-in-6 RNG gate
	entVenueBucketModulo  = 3    // 0=retail, 1=restaurant, 2=fast-food
	entRouteFailVenue     = 0xff // sentinel in commercialVenueSlot / originFloor
	entVenueDwellDuration = 60   // release_commercial_venue_slot dwell gate (TS literal 60)
)

// findEntertainmentLink TS: families/entertainment.ts findEntertainmentLink.
// First link in sidecar order whose owner matches homeColumn (or +7).
func findEntertainmentLink(world *WorldState, sim *SimRecord) *EntertainmentLinkRecord {
	for _, sc := range world.Sidecars {
		link, ok := sc.(*EntertainmentLinkRecord)
		if !ok || link == nil {
			continue
		}
		if link.OwnerSubtypeIndex == sim.HomeColumn || link.OwnerSubtypeIndex+7 == sim.HomeColumn {
			return link
		}
	}
	return nil
}

// entertainmentTargetHeightMetric TS: `link?.ownerSubtypeIndex ?? sim.homeColumn`
func entertainmentTargetHeightMetric(link *EntertainmentLinkRecord, sim *SimRecord) int {
	if link != nil {
		return link.OwnerSubtypeIndex
	}
	return sim.HomeColumn
}

// entertainmentBudgetUsesUpperByte TS: families/entertainment.ts
// entertainmentBudgetUsesUpperByte (try_consume_entertainment_phase_budget 1188:0ce9):
// 0x12 / 0x22 / 0x1d use the upper budget (offset 4), others the lower (offset 5).
func entertainmentBudgetUsesUpperByte(entityType int) bool {
	return entityType == FamilyCinema || entityType == FamilyCinemaStairsUpper || entityType == FamilyPartyHall
}

// tryConsumeEntertainmentPhaseBudget TS: families/entertainment.ts
// tryConsumeEntertainmentPhaseBudget (binary 1188:0ce9)
func tryConsumeEntertainmentPhaseBudget(link *EntertainmentLinkRecord, entityType int) bool {
	if entertainmentBudgetUsesUpperByte(entityType) {
		if link.UpperBudget == 0 {
			return false
		}
		link.UpperBudget--
	} else {
		if link.LowerBudget == 0 {
			return false
		}
		link.LowerBudget--
	}
	return true
}

// incrementEntertainmentHalfPhase TS: families/entertainment.ts
// incrementEntertainmentHalfPhase (binary increment_entertainment_half_phase,
// refund path). Saturates at 0xff.
func incrementEntertainmentHalfPhase(link *EntertainmentLinkRecord, entityType int) {
	if entertainmentBudgetUsesUpperByte(entityType) {
		link.UpperBudget = min(0xff, link.UpperBudget+1)
	} else {
		link.LowerBudget = min(0xff, link.LowerBudget+1)
	}
}

// incrementEntertainmentLinkRuntimeCounters TS: families/entertainment.ts
// incrementEntertainmentLinkRuntimeCounters
func incrementEntertainmentLinkRuntimeCounters(link *EntertainmentLinkRecord) {
	link.ActiveRuntimeCount = min(0xff, link.ActiveRuntimeCount+1)
	link.AttendanceCounter = min(0xff, link.AttendanceCounter+1)
	if link.LinkPhaseState == 1 {
		link.LinkPhaseState = 2
	}
}

// getEntertainmentLinkVenueFloor TS: families/entertainment.ts
// getEntertainmentLinkVenueFloor (binary 1188:0d98). The TS deliberately
// returns sim.floorAnchor (see its comment).
func getEntertainmentLinkVenueFloor(_ *EntertainmentLinkRecord, sim *SimRecord) int {
	return sim.FloorAnchor
}

// getEntertainmentLinkRoutingSourceFloor TS: families/entertainment.ts
// getEntertainmentLinkRoutingSourceFloor (binary 1188:0dce). Returns -1 (the
// sign-extended 0xff lower-half sentinel) when a link is bound.
func getEntertainmentLinkRoutingSourceFloor(link *EntertainmentLinkRecord, sim *SimRecord) int {
	if link == nil {
		return sim.FloorAnchor
	}
	return -1
}

// bucketRowsForFamily TS: families/entertainment.ts bucketRowsForFamily.
// Returns a pointer so callers can append into a row.
func bucketRowsForFamily(world *WorldState, serviceBucket int) *[][]int {
	switch serviceBucket {
	case 0:
		return &world.CommercialVenueBuckets.Retail
	case 1:
		return &world.CommercialVenueBuckets.Restaurant
	case 2:
		return &world.CommercialVenueBuckets.FastFood
	}
	return nil
}

// selectRandomCommercialVenueRecordFromBucket TS: families/entertainment.ts
// selectRandomCommercialVenueRecordFromBucket (binary 11b0:1361). The LCG is
// sampled BEFORE the availability check (LAB_11b0_14c1).
func selectRandomCommercialVenueRecordFromBucket(world *WorldState, serviceBucket, zoneRow int) int {
	rowsPtr := bucketRowsForFamily(world, serviceBucket)
	if rowsPtr == nil {
		return -1
	}
	rows := *rowsPtr
	var row []int
	if zoneRow >= 0 && zoneRow < len(rows) {
		row = rows[zoneRow]
	}
	if len(row) == 0 && len(rows) > 0 {
		row = rows[0]
	}
	if len(row) == 0 {
		return -1
	}
	pick := absInt(SampleRng(world)) % len(row)
	recordIdx := row[pick]
	sidecar := GetSidecar(world, recordIdx)
	if sidecar == nil {
		return -1
	}
	// PORT-NOTE: the TS casts without a kind check; a non-venue sidecar has
	// `availabilityState === undefined`, which passes the test, so only a
	// commercial venue can be rejected here.
	if venue, ok := sidecar.(*CommercialVenueRecord); ok {
		if venue.AvailabilityState == 0xff || venue.AvailabilityState == 3 {
			return -1
		}
	}
	return recordIdx
}

// classifyPathBucketIndex TS: families/entertainment.ts classifyPathBucketIndex
// (binary 11b0:16f0). Go `/` and `%` truncate like the TS Math.trunc / `%`.
func classifyPathBucketIndex(floor int) int {
	offset := floor - 5
	row := offset / 15
	if row < 0 || row > 6 {
		return -1
	}
	if offset%15 > 9 {
		return -1
	}
	return row
}

// appendBucketsForFamilies TS: families/entertainment.ts appendBucketsForFamilies.
// sidecarToFloor / sidecarToFamily are lookup-only TS Maps (later set wins),
// so Go maps are safe. The append order follows sidecar index order.
func appendBucketsForFamilies(world *WorldState, families map[int]bool) {
	sidecarToFloor := map[int]int{}
	sidecarToFamily := map[int]int{}
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if object.LinkedRecordIndex < 0 {
			continue
		}
		family := object.ObjectTypeCode
		if !families[family] {
			continue
		}
		_, y := parseKeyXY(e.Key)
		sidecarToFloor[object.LinkedRecordIndex] = YToFloor(y)
		sidecarToFamily[object.LinkedRecordIndex] = family
	}
	for idx := 0; idx < len(world.Sidecars); idx++ {
		sidecar, ok := world.Sidecars[idx].(*CommercialVenueRecord)
		if !ok || sidecar == nil {
			continue
		}
		if sidecar.OwnerSubtypeIndex == 0xff {
			continue
		}
		floor, okFloor := sidecarToFloor[idx]
		family, okFamily := sidecarToFamily[idx]
		if !okFloor || !okFamily {
			continue
		}
		row := classifyPathBucketIndex(floor)
		if row < 0 || row >= CommercialVenueBucketRows {
			continue
		}
		var target *[][]int
		switch family {
		case FamilyRetail:
			target = &world.CommercialVenueBuckets.Retail
		case FamilyRestaurant:
			target = &world.CommercialVenueBuckets.Restaurant
		case FamilyFastFood:
			target = &world.CommercialVenueBuckets.FastFood
		}
		if target != nil {
			(*target)[row] = append((*target)[row], idx)
		}
	}
}

// RecomputeCommercialVenueBuckets TS: families/entertainment.ts
// recomputeCommercialVenueBuckets (binary rebuild_linked_facility_records
// 11b0:0184). Clears all three buckets, then appends every family EXCEPT
// restaurant; the restaurant bucket stays empty until RecomputeRestaurantVenueBuckets.
func RecomputeCommercialVenueBuckets(world *WorldState) {
	world.CommercialVenueBuckets = CreateCommercialVenueBuckets()
	appendBucketsForFamilies(world, map[int]bool{FamilyRetail: true, FamilyFastFood: true})
}

// RecomputeRestaurantVenueBuckets TS: families/entertainment.ts
// recomputeRestaurantVenueBuckets (binary rebuild_type6_facility_records
// 11b0:0250). Called at the tick 1600 restaurant restock checkpoint.
func RecomputeRestaurantVenueBuckets(world *WorldState) {
	rows := world.CommercialVenueBuckets.Restaurant
	for i := range rows {
		rows[i] = rows[i][:0] // row.length = 0
	}
	appendBucketsForFamilies(world, map[int]bool{FamilyRestaurant: true})
}

// selectRandomCommercialVenueRecordForFloor TS: families/entertainment.ts
// selectRandomCommercialVenueRecordForFloor (binary 11b0:151d):
// zone_row = max(0, trunc((floor_index - 9) / 15)).
func selectRandomCommercialVenueRecordForFloor(world *WorldState, serviceBucket, floorIndex int) int {
	zoneRow := max(0, (floorIndex-9)/15)
	return selectRandomCommercialVenueRecordFromBucket(world, serviceBucket, zoneRow)
}

// getCurrentCommercialVenueDestinationFloor TS: families/entertainment.ts
// getCurrentCommercialVenueDestinationFloor (binary 11b0:10fe): lobby when
// the slot is < 0, else the floor of the first placed object linked to it.
// PORT-NOTE: entRouteFailVenue (0xff) is positive, so a slot of 0xff does a
// real lookup for linkedRecordIndex 255, exactly as the TS does.
func getCurrentCommercialVenueDestinationFloor(world *WorldState, sim *SimRecord) int {
	if sim.CommercialVenueSlot < 0 {
		return LobbyFloor
	}
	for _, e := range world.PlacedObjects.Entries() {
		if e.Val.LinkedRecordIndex != sim.CommercialVenueSlot {
			continue
		}
		_, y := parseKeyXY(e.Key)
		return YToFloor(y)
	}
	return LobbyFloor
}

// ─── Helper state machines (binary 1228:54b8..5a22) ─────────────────────────

// handleEntertainmentPhaseConsumption TS: families/entertainment.ts
// handleEntertainmentPhaseConsumption (binary 1228:54b8)
func handleEntertainmentPhaseConsumption(world *WorldState, time *TimeState, sim *SimRecord) {
	link := findEntertainmentLink(world, sim)

	if sim.StateCode == entStatePhaseConsume {
		if link != nil && !tryConsumeEntertainmentPhaseBudget(link, sim.FamilyCode) {
			return
		}
	}

	venueFloor := getEntertainmentLinkVenueFloor(link, sim)
	isFreshDispatch := sim.StateCode == entStatePhaseConsume
	sourceFloor := sim.SelectedFloor
	if isFreshDispatch {
		sourceFloor = LobbyFloor
	}
	directionFlag := b2Bool01(venueFloor >= sourceFloor)

	// Binary 1228:5592: is_passenger_route = 1, emit_distance_feedback = (state == 0x20).
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, venueFloor, directionFlag, time,
		resolveOptsFeedbackHeight(isFreshDispatch, entertainmentTargetHeightMetric(link, sim)))

	switch result {
	case 0, 1, 2:
		sim.StateCode = entStatePhaseConsumeTransit
	case 3:
		if link != nil {
			incrementEntertainmentLinkRuntimeCounters(link)
		}
		sim.StateCode = 0x03
	default: // -1
		if sim.StateCode == entStatePhaseConsume {
			sim.StateCode = entStatePhaseConsume
			sim.OriginFloor = 0
			sim.ElapsedTicks = 0
			sim.AccumulatedTicks = 0
			if link != nil {
				incrementEntertainmentHalfPhase(link, sim.FamilyCode)
			}
		} else {
			sim.StateCode = entStateParked
		}
	}
}

// handleEntertainmentLinkedHalfRouting TS: families/entertainment.ts
// handleEntertainmentLinkedHalfRouting (binary 1228:5746)
func handleEntertainmentLinkedHalfRouting(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == entStateLinkedHalf
	link := findEntertainmentLink(world, sim)
	sourceFloor := sim.OriginFloor
	if isFreshDispatch {
		sourceFloor = getEntertainmentLinkRoutingSourceFloor(link, sim)
	}
	directionFlag := b2Bool01(LobbyFloor >= sourceFloor)

	// Binary 1228:579d: emit_distance_feedback = (state == 0x05).
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor, directionFlag, time,
		resolveOptsFeedbackHeight(isFreshDispatch, entertainmentTargetHeightMetric(link, sim)))

	switch result {
	case 0, 1, 2:
		sim.StateCode = entStateLinkedHalfTransit
	default: // 3 or -1
		sim.StateCode = entStateParked
	}
}

// acquireCommercialVenueSlot TS: families/entertainment.ts
// acquireCommercialVenueSlot (binary acquire_commercial_venue_slot 11b0:0d92).
func acquireCommercialVenueSlot(world *WorldState, time *TimeState, sim *SimRecord, recordIdx int) int {
	if recordIdx < 0 {
		sim.LastDemandTick = time.DayTick
		return 3
	}
	sidecar, ok := GetSidecar(world, recordIdx).(*CommercialVenueRecord)
	if !ok || sidecar == nil || sidecar.OwnerSubtypeIndex == 0xff ||
		sidecar.AvailabilityState == 0xff || sidecar.AvailabilityState == 3 {
		AdvanceSimTripCounters(sim)
		sim.LastDemandTick = time.DayTick
		return -1
	}
	if sidecar.CurrentPopulation > 39 {
		sim.LastDemandTick = time.DayTick
		return 2
	}
	sidecar.CurrentPopulation++
	if sim.FamilyCode != sidecar.OwnerSubtypeIndex {
		sidecar.AcquireCount++
	}
	sim.LastDemandTick = time.DayTick
	return 3
}

// handleEntertainmentServiceAcquisition TS: families/entertainment.ts
// handleEntertainmentServiceAcquisition (binary 1228:57e2)
func handleEntertainmentServiceAcquisition(world *WorldState, time *TimeState, sim *SimRecord) {
	isFreshDispatch := sim.StateCode == entStateServiceAcquire
	link := findEntertainmentLink(world, sim)
	// 1228:5402-540b: dispatch reads sim[+7] (selectedFloor) on retry.
	sourceFloor := sim.SelectedFloor
	if isFreshDispatch {
		sourceFloor = getEntertainmentLinkRoutingSourceFloor(link, sim)
	}

	if isFreshDispatch {
		// Binary 1228:5807-5827: bucket selector via lcg15 % 3, then the
		// picker (which samples again when a non-empty row exists).
		bucket := absInt(SampleRng(world)) % entVenueBucketModulo
		recordIdx := selectRandomCommercialVenueRecordForFloor(world, bucket, sourceFloor)
		sim.CommercialVenueSlot = recordIdx
	}

	destFloor := getCurrentCommercialVenueDestinationFloor(world, sim)
	directionFlag := b2Bool01(destFloor >= sourceFloor)

	// Binary 1228:5899: emit_distance_feedback = (state == 0x01).
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, destFloor, directionFlag, time,
		resolveOptsFeedbackHeight(isFreshDispatch, entertainmentTargetHeightMetric(link, sim)))

	switch result {
	case 0, 1, 2:
		sim.StateCode = entStateServiceAcquireTransit
	case 3:
		acquireRc := acquireCommercialVenueSlot(world, time, sim, sim.CommercialVenueSlot)
		if acquireRc == 2 {
			sim.StateCode = entStateServiceAcquireTransit
		} else if acquireRc == -1 || acquireRc == 3 {
			sim.StateCode = entStateVenueDwell
		}
		// acquireRc == 0 -> binary fallthrough: no state change.
	default: // -1
		if isFreshDispatch {
			// 1228:58db-5981: sim[+5]=0x41, sim[+6]=0xff, sim[+7]=lowerHalfFloor, sim[+8]=0xff.
			sim.StateCode = entStateServiceAcquireTransit
			sim.CommercialVenueSlot = entRouteFailVenue
			sim.SelectedFloor = getEntertainmentLinkRoutingSourceFloor(link, sim)
			sim.OriginFloor = entRouteFailVenue
		} else {
			sim.StateCode = entStateParked
		}
	}
}

// handleEntertainmentServiceReleaseReturn TS: families/entertainment.ts
// handleEntertainmentServiceReleaseReturn (binary 1228:5a23)
func handleEntertainmentServiceReleaseReturn(world *WorldState, time *TimeState, sim *SimRecord) {
	link := findEntertainmentLink(world, sim)
	if sim.StateCode == entStateVenueDwell {
		// Binary release_commercial_venue_slot (11b0:0fae) dwell gate.
		if sim.CommercialVenueSlot >= 0 && sim.LastDemandTick > 0 &&
			time.DayTick-sim.LastDemandTick < entVenueDwellDuration {
			return
		}
		// Release succeeded: sim[+7] = ownerFloor or 10.
		sim.SelectedFloor = getCurrentCommercialVenueDestinationFloor(world, sim)
	}

	sourceFloor := sim.SelectedFloor
	directionFlag := b2Bool01(LobbyFloor >= sourceFloor)

	// Binary 1228:5aa2: emit_distance_feedback = (state == 0x22).
	isFreshDispatch := sim.StateCode == entStateVenueDwell
	result := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor, directionFlag, time,
		resolveOptsFeedbackHeight(isFreshDispatch, entertainmentTargetHeightMetric(link, sim)))

	switch result {
	case 0, 1, 2:
		sim.StateCode = entStateVenueDwellTransit
	default: // 3 or -1
		sim.StateCode = entStateParked
	}
}

// ─── Gate and dispatch entry points ─────────────────────────────────────────

// HandleEntertainmentSimArrival TS: families/entertainment.ts
// handleEntertainmentSimArrival. Carrier-arrival entry point for cinema /
// party hall guests: re-stamp originFloor and run the retry-state handler.
func HandleEntertainmentSimArrival(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	if !entertainmentGuestFamilies[sim.FamilyCode] {
		return
	}
	sim.OriginFloor = arrivalFloor
	switch sim.StateCode {
	case entStateServiceAcquire:
		sim.StateCode = entStateServiceAcquireTransit
	case entStateLinkedHalf:
		sim.StateCode = entStateLinkedHalfTransit
	case entStatePhaseConsume:
		sim.StateCode = entStatePhaseConsumeTransit
	case entStateVenueDwell:
		sim.StateCode = entStateVenueDwellTransit
	}
	DispatchEntertainmentGuestState(world, time, sim)
}

// DispatchEntertainmentGuestState TS: families/entertainment.ts
// dispatchEntertainmentGuestState (binary 1228:53ad). 8-entry jump table at
// cs:0x5b3a; the prologue's direction-load decrement is omitted, as in the TS.
func DispatchEntertainmentGuestState(world *WorldState, time *TimeState, sim *SimRecord) {
	state := sim.StateCode

	if state == entStateServiceAcquire || state == entStateServiceAcquireTransit {
		handleEntertainmentServiceAcquisition(world, time, sim)
	} else if state == entStateLinkedHalf || state == entStateLinkedHalfTransit {
		handleEntertainmentLinkedHalfRouting(world, time, sim)
	} else if state == entStatePhaseConsume || state == entStatePhaseConsumeTransit {
		handleEntertainmentPhaseConsumption(world, time, sim)
	} else if state == entStateVenueDwell || state == entStateVenueDwellTransit {
		handleEntertainmentServiceReleaseReturn(world, time, sim)
	}
}

// GateEntertainmentGuestState TS: families/entertainment.ts
// gateEntertainmentGuestState (binary 1228:5231). Jump table at cs:0x539d.
func GateEntertainmentGuestState(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if !entertainmentGuestFamilies[sim.FamilyCode] {
		return
	}

	if !IsSimInTransit(sim.StateCode) {
		state := sim.StateCode

		if state == entStatePhaseConsume {
			// Binary handler 0x530d: two independent branches; the &&
			// short-circuit keeps the RNG draw conditional.
			if time.DaypartIndex >= 0 && time.DaypartIndex <= 3 && time.DayTick > entGateDayTickMin &&
				SampleRng(world)%entGateRngModulo == 0 {
				DispatchEntertainmentGuestState(world, time, sim)
			}
			if time.DaypartIndex > 3 {
				sim.StateCode = entStateParked
			}
			return
		} else if state != entStateServiceAcquire && state != entStateLinkedHalf && state != entStateVenueDwell {
			return
		}
	} else {
		if sim.Route.Mode == RouteModeCarrier {
			MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
			return
		}
	}

	DispatchEntertainmentGuestState(world, time, sim)
}
