package sim

// Port of reference/tower-together/sim/carriers/arrival.ts.
//
// 1098:13cc clear_floor_requests_on_arrival
// 1098:12c9 cancel_stale_floor_assignment
// 1098:0192 reset_out_of_range_car
//
// Per-floor arrival bookkeeping: clears primary/secondary route-status slots
// that match the arriving car's direction, decrements the owning car's
// pendingAssignmentCount inline, and handles out-of-range / stale-assignment
// housekeeping.

// decrementOwnerPacAndRecompute mirrors binary
// decrement_car_pending_assignment_count (1098:151c): decrements the owner's
// pendingAssignmentCount and re-runs target/direction recompute so the owner
// re-evaluates its sweep (the recompute may flip the owner's direction at an
// endpoint, chaining into another ClearFloorRequestsOnArrival; binary chain
// observed at d3 t428 dense_office).
// TS: carriers/arrival.ts decrementOwnerPacAndRecompute (binary 1098:151c)
func decrementOwnerPacAndRecompute(carrier *CarrierRecord, ownerIndex int) {
	if ownerIndex < 0 || ownerIndex >= len(carrier.Cars) {
		return
	}
	owner := carrier.Cars[ownerIndex]
	if owner == nil {
		return
	}
	if owner.PendingAssignmentCount > 0 {
		owner.PendingAssignmentCount--
	}
	RecomputeCarTargetAndDirection(carrier, owner, ownerIndex)
}

// carIndexOf is the JS `carrier.cars.indexOf(car)` (pointer identity).
func carIndexOf(carrier *CarrierRecord, car *CarrierCar) int {
	for i, c := range carrier.Cars {
		if c == car {
			return i
		}
	}
	return -1
}

// ClearFloorRequestsOnArrival mirrors binary clear_floor_requests_on_arrival
// (1098:13cc). Called when a car arrives at a floor. Each clause gates on
// (schedF != 0 || dir matches) && table[floor] != 0, then zeros the slot and
// decrements the owning car's pendingAssignmentCount. Same-car decrement is
// inline; cross-car goes through decrementOwnerPacAndRecompute.
// TS: carriers/arrival.ts clearFloorRequestsOnArrival (binary 1098:13cc)
func ClearFloorRequestsOnArrival(carrier *CarrierRecord, car *CarrierCar, floor int) {
	slot := FloorToSlot(carrier, floor)
	if slot < 0 {
		return
	}
	carIndex := carIndexOf(carrier, car)

	if car.ScheduleFlag != 0 || car.DirectionFlag != 0 {
		pri := trAt(carrier.PrimaryRouteStatusByFloor, slot, 0)
		if pri != 0 {
			trSet(&carrier.PrimaryRouteStatusByFloor, slot, 0)
			ownerIndex := pri - 1
			if ownerIndex == carIndex {
				if car.PendingAssignmentCount > 0 {
					car.PendingAssignmentCount--
				}
			} else {
				decrementOwnerPacAndRecompute(carrier, ownerIndex)
			}
		}
	}

	if car.ScheduleFlag != 0 || car.DirectionFlag == 0 {
		sec := trAt(carrier.SecondaryRouteStatusByFloor, slot, 0)
		if sec != 0 {
			trSet(&carrier.SecondaryRouteStatusByFloor, slot, 0)
			ownerIndex := sec - 1
			if ownerIndex == carIndex {
				if car.PendingAssignmentCount > 0 {
					car.PendingAssignmentCount--
				}
			} else {
				decrementOwnerPacAndRecompute(carrier, ownerIndex)
			}
		}
	}
}

// clearRouteSlot resets one active route slot to the idle sentinel.
func clearRouteSlot(slot *CarrierRouteSlot) {
	slot.RouteID = ""
	slot.SourceFloor = 0xff
	slot.DestinationFloor = 0xff
	slot.Boarded = false
	slot.Active = false
}

// ResetOutOfRangeCar mirrors binary reset_out_of_range_car (1098:0192).
// Forces a car home when its position or target falls outside the
// served-floor range. All counters and active route slots reset to idle.
// TS: carriers/arrival.ts resetOutOfRangeCar (binary 1098:0192)
func ResetOutOfRangeCar(carrier *CarrierRecord, car *CarrierCar) {
	homeFloor := min(carrier.TopServedFloor, max(carrier.BottomServedFloor, car.HomeFloor))
	car.CurrentFloor = homeFloor
	car.TargetFloor = homeFloor
	car.PrevFloor = homeFloor
	car.SettleCounter = 0
	car.DwellCounter = 0
	car.AssignedCount = 0
	car.PendingAssignmentCount = 0
	car.DwellStartPendingAssignmentCount = 0
	car.DirectionFlag = 1
	car.ArrivalSeen = 0
	car.ArrivalTick = 0
	car.ArrivalDispatchThisTick = false
	car.ArrivalDispatchStartingAssignedCount = 0
	car.SuppressDwellOppositeDirectionFlip = false
	// Binary 1098:0192 sets car[-0x51] (nearestWorkFloor) to homeFloor.
	car.NearestWorkFloor = homeFloor
	trFill(car.DestinationCountByFloor, 0)
	car.NonemptyDestinationCount = 0
	for _, slot := range car.ActiveRouteSlots {
		clearRouteSlot(slot)
	}
	car.PendingRouteIDs = []string{}
}

// CancelStaleFloorAssignment mirrors binary cancel_stale_floor_assignment
// (1098:12c9). Invoked from advance_carrier_car_state's A2 (motion) branch
// when a car LEAVES a floor. Clears the per-direction route-status slot if
// it was assigned to THIS car (carIndex+1) and the gate (scheduleFlag != 0
// || direction match) holds, then decrements this car's
// pendingAssignmentCount.
// TS: carriers/arrival.ts cancelStaleFloorAssignment (binary 1098:12c9)
func CancelStaleFloorAssignment(carrier *CarrierRecord, car *CarrierCar, floor int, carIndex int) {
	slot := FloorToSlot(carrier, floor)
	if slot < 0 {
		return
	}
	carTag := carIndex + 1

	// Clause 1: clear primary[floor] if (sched != 0 || dir != 0) AND primary[floor] == carIndex+1
	if car.ScheduleFlag != 0 || car.DirectionFlag != 0 {
		if trAt(carrier.PrimaryRouteStatusByFloor, slot, 0) == carTag {
			trSet(&carrier.PrimaryRouteStatusByFloor, slot, 0)
			if car.PendingAssignmentCount > 0 {
				car.PendingAssignmentCount--
			}
		}
	}

	// Clause 2: clear secondary[floor] if (sched != 0 || dir == 0) AND secondary[floor] == carIndex+1
	if car.ScheduleFlag != 0 || car.DirectionFlag == 0 {
		if trAt(carrier.SecondaryRouteStatusByFloor, slot, 0) == carTag {
			trSet(&carrier.SecondaryRouteStatusByFloor, slot, 0)
			if car.PendingAssignmentCount > 0 {
				car.PendingAssignmentCount--
			}
		}
	}
}
