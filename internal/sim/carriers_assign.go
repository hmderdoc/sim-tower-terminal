package sim

import "math"

// Port of reference/tower-together/sim/carriers/assign.ts.
//
// 1098:0a4c assign_car_to_floor_request
// 1098:0dfc find_best_available_car_for_floor
//
// Floor-call assignment: picks the best car for a floor-call and writes the
// primary (up) / secondary (down) route-status slot with car+1.

// FindBestAvailableCarForFloor mirrors binary
// find_best_available_car_for_floor (1098:0dfc). The binary returns
// (int return_value, int *param_4); the caller gates all its writes on
// return_value != 0. Cases that return fullAssign=false: A/B (same-floor,
// doors closed, scheduleFlag||dirMatch), C (idle-home candidate already at
// floor). Normal path and degenerate: fullAssign=true.
//
// The wrap-cost "turn floor" uses car.NearestWorkFloor (binary -0x51). The
// idle-home test uses the static per-car HomeFloor (binary
// reachability_masks_by_floor[carIdx-8]), NOT NearestWorkFloor.
//
// The binary has NO assignedCount-vs-capacity early-skip.
// TS: carriers/assign.ts findBestAvailableCarForFloor (binary 1098:0dfc)
// PORT-NOTE: the TS returns {carIndex, fullAssign}; Go returns the pair.
// Number.POSITIVE_INFINITY is math.MaxInt (only compared with <).
func FindBestAvailableCarForFloor(carrier *CarrierRecord, floor int, directionFlag int) (carIndex int, fullAssign bool) {
	bestIdleHomeCost := math.MaxInt
	bestIdleHomeIndex := -1
	bestForwardCost := math.MaxInt
	bestForwardIndex := -1
	bestWrapCost := math.MaxInt
	bestWrapIndex := -1

	for ci, car := range carrier.Cars {
		if !car.Active {
			continue
		}

		// Cases A/B: car at floor with doors closed, and either scheduleFlag
		// is set or direction already matches. Binary returns 0 immediately.
		if car.CurrentFloor == floor && car.SettleCounter == 0 {
			if car.ScheduleFlag != 0 || car.DirectionFlag == directionFlag {
				return ci, false
			}
			// else fall through to idle-home / forward / wrap accumulation
		}

		turnFloor := car.NearestWorkFloor
		distance := absInt(floor - car.CurrentFloor)

		// Idle-home candidate: parked at static home floor with no work
		// pending. Binary 1098:0ee9..0efe.
		isIdleHome := car.PendingAssignmentCount == 0 &&
			car.NonemptyDestinationCount == 0 &&
			car.SettleCounter == 0 &&
			car.CurrentFloor == car.HomeFloor

		if isIdleHome {
			// Case C: idle-home candidate already at floor
			if distance == 0 {
				return ci, false
			}
			if distance < bestIdleHomeCost {
				bestIdleHomeCost = distance
				bestIdleHomeIndex = ci
			}
			continue
		}

		if car.DirectionFlag == directionFlag {
			var forward int
			if directionFlag == 1 {
				forward = floor - car.CurrentFloor
			} else {
				forward = car.CurrentFloor - floor
			}
			if forward >= 0 {
				if forward == 0 && car.SettleCounter == 0 {
					// Case C alternate: same-floor same-direction with doors open.
					return ci, false
				}
				if forward < bestForwardCost {
					bestForwardCost = forward
					bestForwardIndex = ci
				}
				continue
			}
			// Past sweep end: wrap, with nearest_work_floor as turn floor.
			var cost int
			if directionFlag == 1 {
				cost = turnFloor*2 - car.CurrentFloor - floor
			} else {
				cost = car.CurrentFloor + floor - turnFloor*2
			}
			if cost < bestWrapCost {
				bestWrapCost = cost
				bestWrapIndex = ci
			}
			continue
		}

		// Opposite direction: wrap via nearest_work as turn floor. Binary
		// 1098:0fe0 branches on (request floor vs turn), not (request floor
		// vs current).
		var cost int
		if directionFlag == 1 {
			if turnFloor < floor {
				cost = car.CurrentFloor + floor - 2*turnFloor
			} else {
				cost = car.CurrentFloor - floor
			}
		} else {
			if floor < turnFloor {
				cost = 2*turnFloor - car.CurrentFloor - floor
			} else {
				cost = floor - car.CurrentFloor
			}
		}
		if cost < bestWrapCost {
			bestWrapCost = cost
			bestWrapIndex = ci
		}
	}

	var bestMovingCost, bestMovingIndex int
	if bestForwardIndex >= 0 {
		bestMovingCost = bestForwardCost
		bestMovingIndex = bestForwardIndex
	} else {
		bestMovingCost = bestWrapCost
		bestMovingIndex = bestWrapIndex
	}

	if bestIdleHomeIndex >= 0 && bestMovingIndex >= 0 {
		// Binary quirk: equality breaks toward the idle-home car (strict
		// `(moving - idle) < threshold`).
		if bestMovingCost-bestIdleHomeCost < carrier.WaitingCarResponseThreshold {
			return bestMovingIndex, true
		}
		return bestIdleHomeIndex, true
	}
	if bestMovingIndex >= 0 {
		return bestMovingIndex, true
	}

	// Binary quirk (1098:10d0): degenerate fallback writes car index 0 and
	// returns fullAssign=true whenever there is NO forward and NO wrap
	// candidate, even when an idle-home candidate is tracked (observed at
	// dense_office tick 1975). Mirror the quirk.
	return 0, true
}

// AssignCarToFloorRequest mirrors binary assign_car_to_floor_request
// (1098:0a4c). Top gate: for direction=1 (UP) requires primary[floor]==0;
// for direction=0 (DOWN) requires secondary[floor]==0. On fullAssign, writes
// the table slot, increments pac, and recomputes target+direction.
// TS: carriers/assign.ts assignCarToFloorRequest (binary 1098:0a4c)
func AssignCarToFloorRequest(carrier *CarrierRecord, floor int, directionFlag int) {
	slot := FloorToSlot(carrier, floor)
	if slot < 0 {
		return
	}
	table := &carrier.SecondaryRouteStatusByFloor
	if directionFlag == 1 {
		table = &carrier.PrimaryRouteStatusByFloor
	}
	if trAt(*table, slot, 0) != 0 {
		return
	}

	carIndex, fullAssign := FindBestAvailableCarForFloor(carrier, floor, directionFlag)
	if carIndex < 0 {
		return
	}
	if !fullAssign {
		return
	}

	// Binary 1098:0a4c body is straight-line; the per-route assignedCarIndex
	// is populated lazily by AssignRequestToRuntimeRoute at drain time.
	// PORT-NOTE: the degenerate fallback can return index 0 for a carrier
	// with zero cars; the TS then throws on `car.pendingAssignmentCount`
	// (undefined). Go panics the same way (index out of range).
	car := carrier.Cars[carIndex]
	trSet(table, slot, carIndex+1)
	car.PendingAssignmentCount++
	RecomputeCarTargetAndDirection(carrier, car, carIndex)
}
