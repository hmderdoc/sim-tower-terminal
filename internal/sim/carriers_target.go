package sim

// Port of reference/tower-together/sim/carriers/target.ts.
//
// 1098:0bcf recompute_car_target_and_direction
// 1098:1553 select_next_target_floor
// 1098:1d2f update_car_direction_flag
// 1098:1f4c find_nearest_work_floor

// RecomputeCarTargetAndDirection TS: carriers/target.ts recomputeCarTargetAndDirection (binary 1098:0bcf)
func RecomputeCarTargetAndDirection(carrier *CarrierRecord, car *CarrierCar, carIndex int) {
	next := SelectNextTargetFloor(car, carrier, carIndex, car.ScheduleFlag)
	if next < carrier.BottomServedFloor || next > carrier.TopServedFloor {
		ResetOutOfRangeCar(carrier, car)
		return
	}
	car.TargetFloor = next
	UpdateCarDirectionFlag(carrier, car)
	// Binary 1098:0bcf tail: refresh nearest_work_floor (car[-0x51]) for use
	// as the wrap turn-floor by FindBestAvailableCarForFloor.
	car.NearestWorkFloor = FindNearestWorkFloor(carrier, car, carIndex)
}

// UpdateCarDirectionFlag TS: carriers/target.ts updateCarDirectionFlag (binary 1098:1d2f)
func UpdateCarDirectionFlag(carrier *CarrierRecord, car *CarrierCar) {
	floor := car.CurrentFloor
	prevDir := car.DirectionFlag

	if floor != car.TargetFloor {
		// Binary 1098:1d2f: floor != target branch sets direction from
		// relation and goto-s past the clear-on-flip check.
		if floor < car.TargetFloor {
			car.DirectionFlag = 1
		} else {
			car.DirectionFlag = 0
		}
		return
	}
	if car.ArrivalSeen == 0 {
		return
	}

	if floor == carrier.TopServedFloor && car.DirectionFlag == 1 {
		car.DirectionFlag = 0
	} else if floor == carrier.BottomServedFloor && car.DirectionFlag == 0 {
		car.DirectionFlag = 1
	} else if car.ScheduleFlag == 0 {
		// Bidirectional flip gated on schedF == 0. Express modes skip.
		slot := FloorToSlot(carrier, floor)
		if slot >= 0 {
			upCalls := trAt(carrier.PrimaryRouteStatusByFloor, slot, 0) != 0
			downCalls := trAt(carrier.SecondaryRouteStatusByFloor, slot, 0) != 0
			if car.DirectionFlag == 0 && !downCalls && upCalls {
				car.DirectionFlag = 1
			} else if car.DirectionFlag == 1 && !upCalls && downCalls {
				car.DirectionFlag = 0
			}
		}
	}

	// Binary 1098:1d2f tail: if direction changed during this call,
	// clear_floor_requests_on_arrival fires at the current floor with the
	// NEW direction.
	if car.DirectionFlag != prevDir {
		ClearFloorRequestsOnArrival(carrier, car, floor)
	}
}

// SelectNextTargetFloor is binary select_next_target_floor @ 1098:1553.
// Multi-phase directional scan that distinguishes queued riders
// (destination queue) from assignment slots (up/down call pickups) and gates
// assignment targets on capacity.
// TS: carriers/target.ts selectNextTargetFloor (binary 1098:1553)
func SelectNextTargetFloor(car *CarrierCar, carrier *CarrierRecord, carIndex int, expressDirectionFlag int) int {
	// Binary idle check: arrivalTick == 0 && pendingDestCount == 0. Also
	// gate on pendingAssignmentCount (dayTick can be 0 at the start of a day).
	if car.ArrivalTick == 0 && car.NonemptyDestinationCount == 0 && car.PendingAssignmentCount == 0 {
		return car.HomeFloor
	}

	// When a normal-mode car has finished all work, the binary keeps it
	// parked at home.
	if expressDirectionFlag == 0 && car.NonemptyDestinationCount == 0 && car.PendingAssignmentCount == 0 {
		return car.HomeFloor
	}

	underCapacity := car.AssignedCount != carCapacity(carrier)
	carTag := carIndex + 1

	hasQueuedRider := func(floor int) bool {
		slot := FloorToSlot(carrier, floor)
		return slot >= 0 && trAt(car.DestinationCountByFloor, slot, 0) > 0
	}
	// PORT-NOTE: TS compares `table[slot] === carIndex + 1`; an
	// out-of-range read is undefined and never equal, and carTag >= 1, so
	// the 0 default is equivalent.
	hasUpSlot := func(floor int) bool {
		slot := FloorToSlot(carrier, floor)
		return slot >= 0 && trAt(carrier.PrimaryRouteStatusByFloor, slot, 0) == carTag
	}
	hasDownSlot := func(floor int) bool {
		slot := FloorToSlot(carrier, floor)
		return slot >= 0 && trAt(carrier.SecondaryRouteStatusByFloor, slot, 0) == carTag
	}

	// schedMode 1: express-up. Scan downward for work, fallback = top.
	if expressDirectionFlag == 1 {
		gate := (car.DirectionFlag != 0 &&
			(car.CurrentFloor != carrier.TopServedFloor || car.SettleCounter != 0)) ||
			(car.DirectionFlag == 0 &&
				car.CurrentFloor == carrier.BottomServedFloor &&
				car.SettleCounter == 0)
		if gate {
			for f := car.CurrentFloor; f >= carrier.BottomServedFloor; f-- {
				if hasQueuedRider(f) {
					return f
				}
				if underCapacity && (hasDownSlot(f) || hasUpSlot(f)) {
					return f
				}
			}
		}
		return carrier.TopServedFloor
	}

	// schedMode 2: express-down. Scan upward for work, fallback = bottom.
	if expressDirectionFlag == 2 {
		gate := (car.DirectionFlag == 0 &&
			(car.CurrentFloor != carrier.BottomServedFloor || car.SettleCounter != 0)) ||
			(car.DirectionFlag != 0 &&
				car.CurrentFloor == carrier.TopServedFloor &&
				car.SettleCounter == 0)
		if gate {
			for f := car.CurrentFloor; f <= carrier.TopServedFloor; f++ {
				if hasQueuedRider(f) {
					return f
				}
				if underCapacity && (hasUpSlot(f) || hasDownSlot(f)) {
					return f
				}
			}
		}
		return carrier.BottomServedFloor
	}

	// schedMode 0: bidirectional sweep
	if car.DirectionFlag == 0 {
		// Going down - phase 1: cur->bottom for queued OR downSlot
		for f := car.CurrentFloor; f >= carrier.BottomServedFloor; f-- {
			if hasQueuedRider(f) {
				return f
			}
			if underCapacity && hasDownSlot(f) {
				return f
			}
		}
		// Phase 2: bottom->cur for upSlot (capacity-gated)
		if underCapacity {
			for f := carrier.BottomServedFloor; f <= car.CurrentFloor; f++ {
				if hasUpSlot(f) {
					return f
				}
			}
		}
		// Phase 3: cur+1->top for upSlot (capacity-gated) OR queued
		for f := car.CurrentFloor + 1; f <= carrier.TopServedFloor; f++ {
			if underCapacity && hasUpSlot(f) {
				return f
			}
			if hasQueuedRider(f) {
				return f
			}
		}
		// Phase 4: top->cur+1 for downSlot (capacity-gated)
		if underCapacity {
			for f := carrier.TopServedFloor; f > car.CurrentFloor; f-- {
				if hasDownSlot(f) {
					return f
				}
			}
		}
	} else {
		// Going up - phase 1: cur->top for queued OR upSlot
		for f := car.CurrentFloor; f <= carrier.TopServedFloor; f++ {
			if hasQueuedRider(f) {
				return f
			}
			if underCapacity && hasUpSlot(f) {
				return f
			}
		}
		// Phase 2: top->cur for downSlot (capacity-gated)
		if underCapacity {
			for f := carrier.TopServedFloor; f >= car.CurrentFloor; f-- {
				if hasDownSlot(f) {
					return f
				}
			}
		}
		// Phase 3: cur-1->bottom for downSlot (capacity-gated) OR queued
		for f := car.CurrentFloor - 1; f >= carrier.BottomServedFloor; f-- {
			if underCapacity && hasDownSlot(f) {
				return f
			}
			if hasQueuedRider(f) {
				return f
			}
		}
		// Phase 4: bottom->cur-1 for upSlot (capacity-gated)
		if underCapacity {
			for f := carrier.BottomServedFloor; f < car.CurrentFloor; f++ {
				if hasUpSlot(f) {
					return f
				}
			}
		}
	}

	return -1
}

// FindNearestWorkFloor is binary find_nearest_work_floor @ 1098:1f4c. Scans
// the current direction for the nearest floor with pending work for THIS
// car: a queued rider OR a primary/secondary route-status slot tagged with
// this car. dir=0 scans bottom..current; dir=1 scans top..current. Falls
// back to car.HomeFloor.
// TS: carriers/target.ts findNearestWorkFloor (binary 1098:1f4c)
func FindNearestWorkFloor(carrier *CarrierRecord, car *CarrierCar, carIndex int) int {
	dir := car.DirectionFlag
	cur := car.CurrentFloor
	bottom := carrier.BottomServedFloor
	top := carrier.TopServedFloor
	carTag := carIndex + 1

	hasWork := func(floor int) bool {
		slot := FloorToSlot(carrier, floor)
		if slot < 0 {
			return false
		}
		if trAt(car.DestinationCountByFloor, slot, 0) != 0 {
			return true
		}
		if trAt(carrier.PrimaryRouteStatusByFloor, slot, 0) == carTag {
			return true
		}
		if trAt(carrier.SecondaryRouteStatusByFloor, slot, 0) == carTag {
			return true
		}
		return false
	}

	if dir == 0 {
		for floor := bottom; floor <= cur; floor++ {
			if hasWork(floor) {
				return floor
			}
		}
	} else {
		for floor := top; floor >= cur; floor-- {
			if hasWork(floor) {
				return floor
			}
		}
	}
	return car.HomeFloor
}
