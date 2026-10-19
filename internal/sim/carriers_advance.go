package sim

// Port of reference/tower-together/sim/carriers/advance.ts.
//
// 1098:06fb advance_carrier_car_state
//
// Pass 1 of the per-tick carrier update, invoked from carrier_tick
// (1098:03ab) before dispatch_carrier_car_arrivals and
// process_unit_travel_queue. Three-way branch on two countdowns (settle /
// dwell) driving the car state machine (ROUTING-BINARY-MAP.md section 4.5).
//
//	settleCounter <-> -0x5d (settle after a motion step)
//	dwellCounter  <-> -0x5c (dwell/boarding at a stop; 5 = just arrived)

// AdvanceCarrierCarState TS: carriers/advance.ts advanceCarrierCarState (binary 1098:06fb)
func AdvanceCarrierCarState(car *CarrierCar, carrier *CarrierRecord, carIndex int, time *TimeState, lobbyMode LobbyMode) {
	if !car.Active {
		return
	}

	if car.CurrentFloor < carrier.BottomServedFloor || car.CurrentFloor > carrier.TopServedFloor {
		ResetOutOfRangeCar(carrier, car)
		return
	}

	// Branch C (1098:06fb): stabilize countdown. Decrement only if the
	// recomputed mode is still 0; otherwise snap to 0 (fast-cancel).
	if car.SettleCounter > 0 {
		if ComputeCarMotionMode(carrier, car) == 0 {
			car.SettleCounter--
		} else {
			car.SettleCounter = 0
		}
		return
	}

	if car.DwellCounter == 0 {
		car.SuppressDwellOppositeDirectionFlip = false
		// Branch A. A1 (arrival / idle-at-target) fires when target == cur
		// and either the per-car destination queue has riders for this
		// floor, or the car is not full. A1 is level-triggered.
		currentSlot := FloorToSlot(carrier, car.CurrentFloor)
		hasQueuedRider := currentSlot >= 0 && trAt(car.DestinationCountByFloor, currentSlot, 0) > 0
		underCapacity := car.AssignedCount != carCapacity(carrier)

		if car.TargetFloor == car.CurrentFloor && (hasQueuedRider || underCapacity) {
			// A1. Inlined former loadScheduleFlag helper (binary reloads the
			// scheduling_flag inline when a car sits at a terminal floor).
			if car.CurrentFloor == carrier.TopServedFloor || car.CurrentFloor == carrier.BottomServedFloor {
				car.ScheduleFlag = trAt(carrier.ExpressDirectionFlags, carrierScheduleIndex(time), 0)
			}
			ClearFloorRequestsOnArrival(carrier, car, car.CurrentFloor)
			car.DwellStartPendingAssignmentCount = car.PendingAssignmentCount
			car.DwellCounter = departureSequenceTicks
			if car.ArrivalSeen == 0 {
				car.ArrivalTick = time.DayTick
			}
			car.ArrivalSeen = 1
			return
		}

		// A2 - motion step. Binary 1098:06fb A2 calls
		// cancel_stale_floor_assignment (1098:12c9) FIRST, THEN reads the
		// queue + status to decide whether to re-assign. Order matters.
		departFloor := car.CurrentFloor
		departSlot := FloorToSlot(carrier, departFloor)
		CancelStaleFloorAssignment(carrier, car, departFloor, carIndex)
		var queue *CarrierFloorQueue
		if departSlot >= 0 && departSlot < len(carrier.FloorQueues) {
			queue = carrier.FloorQueues[departSlot]
		}
		hasUpRequest := queue != nil && !queue.Up.IsEmpty() &&
			trAt(carrier.PrimaryRouteStatusByFloor, departSlot, 0) == 0
		hasDownRequest := queue != nil && !queue.Down.IsEmpty() &&
			trAt(carrier.SecondaryRouteStatusByFloor, departSlot, 0) == 0
		AdvanceCarPositionOneStep(carrier, car, carIndex)
		if hasUpRequest {
			AssignCarToFloorRequest(carrier, departFloor, 1)
		}
		if hasDownRequest {
			AssignCarToFloorRequest(carrier, departFloor, 0)
		}
		return
	}

	// Branch B - dwell countdown. Decrement unconditionally; on 0
	// transition, latch prevFloor, recompute target+direction, then check
	// the departure gate. If the gate says "wait", pin dwell=1.
	// Binary 1098:06fb: shouldCarDepart runs AFTER the recompute.
	car.DwellCounter--
	if car.DwellCounter == 0 {
		car.SuppressDwellOppositeDirectionFlip = false
		car.PrevFloor = car.CurrentFloor
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
		if !ShouldCarDepart(carrier, car, time, lobbyMode) {
			car.DwellCounter = 1
		}
	}
}
