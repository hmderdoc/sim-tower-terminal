package sim

// Port of reference/tower-together/sim/queue/process-travel.ts.
//
// 1218:0351 process_unit_travel_queue
// 1218:0d4e assign_request_to_runtime_route
//
// Per-tick queue-drain + boarding pass, called from the carrier tick after
// dispatch_carrier_car_arrivals.

// processTravelStateBitFamilies TS: STATE_BIT_FAMILIES (queue/process-travel.ts).
// Lookup-only.
var processTravelStateBitFamilies = map[int]bool{
	FamilyHotelSingle: true,
	FamilyHotelTwin:   true,
	FamilyHotelSuite:  true,
	FamilyRestaurant:  true,
	FamilyOffice:      true,
	FamilyCondo:       true,
	FamilyFastFood:    true,
	FamilyRetail:      true,
}

// requeueFailureDelay TS: REQUEUE_FAILURE_DELAY (g_requeue_failure_delay)
const requeueFailureDelay = 0

// applyAssignmentStressUpdate: binary assign_request_to_runtime_route
// (1218:0d4e) invokes accumulate_elapsed_delay_into_current_sim for the
// assigned sim. Service carriers (mode 2) do not update sim stress.
// TS: queue/process-travel.ts applyAssignmentStressUpdate
func applyAssignmentStressUpdate(world *WorldState, time *TimeState, carrier *CarrierRecord, routeID string, sourceFloor int) {
	if carrier.CarrierMode == 2 {
		return
	}
	sim := findSimByKey(world, routeID)
	if sim == nil {
		return
	}
	AccumulateElapsedDelayIntoCurrentSim(world, time, sim, sourceFloor)
}

// clearSimRouteByID returns the first sim keyed simID to idle and strips
// only the 0x40 in-transit bit (for the dispatch_sim_behavior families).
// TS: queue/process-travel.ts clearSimRouteById
func clearSimRouteByID(world *WorldState, simID string) {
	for _, sim := range world.Sims {
		if SimKey(sim) != simID {
			continue
		}
		sim.Route = RouteIdle
		if processTravelStateBitFamilies[sim.FamilyCode] {
			SetSimInTransit(sim, false)
		}
		return
	}
}

// AssignRequestToRuntimeRoute moves a popped request from the floor ring
// into one of the car's active-route slots. Runs the transfer-floor
// resolver; on failure applies g_requeue_failure_delay (0) and clears the
// sim's route.
// TS: queue/process-travel.ts assignRequestToRuntimeRoute (binary 1218:0d4e)
// PORT-NOTE: the optional TS `carIndex?: number` is an int where a negative
// value means undefined (every TS caller passes a real index).
func AssignRequestToRuntimeRoute(world *WorldState, time *TimeState, carrier *CarrierRecord, car *CarrierCar, route *CarrierPendingRoute, carIndex int) bool {
	resolvedFloor := ChooseTransferFloorFromCarrierReachability(world, carrier.CarrierID, car.CurrentFloor, route.DestinationFloor)
	if resolvedFloor < 0 {
		carrier.PendingRoutes = filterOutPendingRoute(carrier.PendingRoutes, route.SimID)
		// Binary: on transfer-floor failure, adds g_requeue_failure_delay
		// then calls force_dispatch_sim_state_by_family.
		if failedSim := findSimByKey(world, route.SimID); failedSim != nil {
			AddDelayToCurrentSim(failedSim, requeueFailureDelay)
		}
		clearSimRouteByID(world, route.SimID)
		return false
	}
	route.DestinationFloor = resolvedFloor
	// Binary 1218:0d4e: store_request_in_active_route_slot writes into THIS
	// car's slot table; slot ownership transfers to the boarding car.
	if carIndex >= 0 {
		route.AssignedCarIndex = carIndex
	}
	stored := AddRouteSlot(carrier, car, route)
	if stored {
		applyAssignmentStressUpdate(world, time, carrier, route.SimID, route.SourceFloor)
	}
	if stored && carIndex >= 0 && car.DwellCounter == 0 {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}
	return stored
}

// drainFloorQueueForCar TS: queue/process-travel.ts drainFloorQueueForCar
// PORT-NOTE: the __DRAIN_TRACE__ console logging is not ported.
func drainFloorQueueForCar(world *WorldState, carrier *CarrierRecord, car *CarrierCar, carIndex int, time *TimeState) {
	// When schedule is disabled, car does not pick up passengers.
	scheduleIndex := carrierScheduleIndex(time)
	if trAt(carrier.ServiceScheduleFlags, scheduleIndex, 1) == 0 {
		return
	}

	remainingSlots := max(0, carCapacity(carrier)-car.AssignedCount)
	if remainingSlots == 0 {
		return
	}

	floorQueue := queueState(carrier, car.CurrentFloor)
	if floorQueue == nil {
		return
	}

	// Binary 1218:0351 flip-during-dwell: when the current-direction queue
	// is empty, pac==0 and ndc==0, flip toward a non-empty opposite queue.
	qCurBuf := directionQueue(floorQueue, car.DirectionFlag)
	if qCurBuf.Size() == 0 && car.PendingAssignmentCount == 0 && car.NonemptyDestinationCount == 0 {
		oppositeDirection := 1
		if car.DirectionFlag == 1 {
			oppositeDirection = 0
		}
		qOppBuf := directionQueue(floorQueue, oppositeDirection)
		if qOppBuf.Size() != 0 {
			car.DirectionFlag = oppositeDirection
		}
	}

	// Binary 1218:0351: pop cap per direction is 1 unless dwellCounter == 1
	// exactly (then cap = remainingSlots).
	popCap := 1
	if car.DwellCounter == 1 {
		popCap = remainingSlots
	}

	// Strict-FIFO drain matching the binary: pop from the head and attempt
	// assign_request_to_runtime_route. Stale duplicates (boarded or already
	// slotted) are popped without consuming an attempt.
	drainDirection := func(directionFlag int) {
		buf := directionQueue(floorQueue, directionFlag)
		maxAssignments := min(popCap, remainingSlots)
		assignmentAttempts := 0
		for assignmentAttempts < maxAssignments && buf.Size() > 0 {
			routeID, ok := buf.Pop()
			if !ok {
				break
			}
			route := FindRouteByID(carrier, routeID)
			if route == nil {
				continue
			}
			if route.Boarded || HasActiveSlot(car, route.SimID) {
				continue
			}
			assignmentAttempts++
			if AssignRequestToRuntimeRoute(world, time, carrier, car, route, carIndex) {
				remainingSlots--
			}
		}
	}

	primaryDirection := car.DirectionFlag
	drainDirection(primaryDirection)

	// Binary: alternate-direction opportunistic drain when scheduleFlag != 0.
	if car.ScheduleFlag != 0 && remainingSlots > 0 {
		oppositeDirection := 1
		if primaryDirection == 1 {
			oppositeDirection = 0
		}
		drainDirection(oppositeDirection)
	}

	SyncAssignmentStatus(carrier)
	if car.DwellCounter == 0 {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}
}

// boardWaitingRoutes is the boarding-only half of process_unit_travel_queue
// (1218:0351): moves riders from active-route slots onto the car.
// TS: queue/process-travel.ts boardWaitingRoutes
func boardWaitingRoutes(carrier *CarrierRecord, car *CarrierCar, carIndex int) bool {
	changed := false
	limit := activeSlotLimit(carrier)

	for index := 0; index < limit; index++ {
		var slot *CarrierRouteSlot
		if index < len(car.ActiveRouteSlots) {
			slot = car.ActiveRouteSlots[index]
		}
		if car.AssignedCount >= carCapacity(carrier) {
			break
		}
		if slot == nil || !slot.Active || slot.Boarded {
			continue
		}
		route := FindRouteByID(carrier, slot.RouteID)
		if route == nil || route.Boarded {
			continue
		}
		if route.AssignedCarIndex != carIndex && route.AssignedCarIndex != -1 {
			continue
		}
		if route.SourceFloor != car.CurrentFloor {
			continue
		}
		route.AssignedCarIndex = carIndex
		route.Boarded = true
		slot.Boarded = true
		car.AssignedCount++
		destinationSlot := FloorToSlot(carrier, route.DestinationFloor)
		if destinationSlot >= 0 {
			prev := trAt(car.DestinationCountByFloor, destinationSlot, 0)
			trSet(&car.DestinationCountByFloor, destinationSlot, prev+1)
			if prev == 0 {
				car.NonemptyDestinationCount++
			}
		}
		changed = true
	}

	if changed {
		NormalizeInactiveSlots(car)
		SyncPendingRouteIDs(car)
		SyncAssignmentStatus(carrier)
		if car.DwellCounter == 0 {
			RecomputeCarTargetAndDirection(carrier, car, carIndex)
		}
	}
	return changed
}

// ProcessUnitTravelQueue pops riders from the carrier's per-floor queue
// into this car's active-route slots, then boards any slot whose source is
// the current floor. The binary gates the queue pop on
// (car[-0x5c] & 1) != 0 (odd dwellCounter only).
// TS: queue/process-travel.ts processUnitTravelQueue (binary 1218:0351)
func ProcessUnitTravelQueue(world *WorldState, carrier *CarrierRecord, car *CarrierCar, carIndex int, time *TimeState) {
	if !car.Active {
		return
	}
	if car.DwellCounter&1 != 0 {
		drainFloorQueueForCar(world, carrier, car, carIndex, time)
	}
	boardWaitingRoutes(carrier, car, carIndex)
	if car.DwellCounter == 1 &&
		time.DayCounter == 1 &&
		car.DirectionFlag == 0 &&
		car.CurrentFloor <= carrier.BottomServedFloor+2 &&
		car.DwellStartPendingAssignmentCount > 0 &&
		car.PendingAssignmentCount == 0 {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}
	if car.DwellCounter == 3 &&
		time.DayCounter > 0 &&
		car.DirectionFlag == 0 &&
		car.CurrentFloor == carrier.BottomServedFloor &&
		car.TargetFloor == car.CurrentFloor &&
		car.AssignedCount == 0 &&
		car.DwellStartPendingAssignmentCount > 0 &&
		car.PendingAssignmentCount == 0 {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
		drainFloorQueueForCar(world, carrier, car, carIndex, time)
		boardWaitingRoutes(carrier, car, carIndex)
	}
	if car.DwellCounter == 0 &&
		car.SettleCounter > 0 &&
		car.CurrentFloor == car.TargetFloor {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}
	if car.DwellCounter == 0 &&
		car.SettleCounter > 0 &&
		car.AssignedCount == 0 &&
		car.PendingAssignmentCount == 0 &&
		car.NonemptyDestinationCount == 0 {
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}
}
