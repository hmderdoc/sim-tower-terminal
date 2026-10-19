package sim

// Port of reference/tower-together/sim/queue/scan.ts.
//
// 1218:142a remove_request_from_unit_queue
// 1218:173a remove_request_from_active_route_slots
// 1218:187b store_request_in_active_route_slot
// 1218:1905 pop_active_route_slot_request
//
// Per-car active-slot ring ops + ring-scan removal. The active-slot ring is
// the 42-entry active_request_refs table on each TowerUnitRouteRecord.

// directionRing returns the (sourceFloor, direction) ring, or nil.
// TS: queue/scan.ts getDirectionRing
func directionRing(carrier *CarrierRecord, sourceFloor, directionFlag int) *RouteRequestRing {
	slot := FloorToSlot(carrier, sourceFloor)
	if slot < 0 || slot >= len(carrier.FloorQueues) {
		return nil
	}
	queue := carrier.FloorQueues[slot]
	if queue == nil {
		return nil
	}
	if directionFlag == 1 {
		return queue.Up
	}
	return queue.Down
}

// RemoveRequestFromUnitQueue compacts out the entry whose request id
// matches simID, preserving the relative order of the rest. True on hit.
// TS: queue/scan.ts removeRequestFromUnitQueue (binary 1218:142a)
func RemoveRequestFromUnitQueue(carrier *CarrierRecord, simID string, sourceFloor, directionFlag int) bool {
	ring := directionRing(carrier, sourceFloor, directionFlag)
	if ring == nil {
		return false
	}
	return ring.RemoveFirst(simID)
}

// StoreRequestInActiveRouteSlot writes the request into the first free slot
// of the car's active_request_refs ring. False when every slot up to
// assignment_capacity is occupied; re-uses an already owned slot.
// TS: queue/scan.ts storeRequestInActiveRouteSlot (binary 1218:187b)
func StoreRequestInActiveRouteSlot(carrier *CarrierRecord, car *CarrierCar, route *CarrierPendingRoute) bool {
	if HasActiveSlot(car, route.SimID) {
		return true
	}
	limit := activeSlotLimit(carrier)
	for index := 0; index < limit; index++ {
		if index >= len(car.ActiveRouteSlots) {
			continue
		}
		slot := car.ActiveRouteSlots[index]
		if slot == nil || slot.Active {
			continue
		}
		slot.RouteID = route.SimID
		slot.SourceFloor = route.SourceFloor
		slot.DestinationFloor = route.DestinationFloor
		slot.Boarded = route.Boarded
		slot.Active = true
		SyncPendingRouteIDs(car)
		return true
	}
	return false
}

// PopActiveRouteSlotRequest clears the active slot at slotIndex and returns
// its request id ("" , false when the slot is not active).
// TS: queue/scan.ts popActiveRouteSlotRequest (binary 1218:1905)
func PopActiveRouteSlotRequest(car *CarrierCar, slotIndex int) (string, bool) {
	if slotIndex < 0 || slotIndex >= len(car.ActiveRouteSlots) {
		return "", false
	}
	slot := car.ActiveRouteSlots[slotIndex]
	if slot == nil || !slot.Active {
		return "", false
	}
	id := slot.RouteID
	clearRouteSlot(slot)
	SyncPendingRouteIDs(car)
	return id, true
}

// RemoveRequestFromActiveRouteSlots scans cars in index order; on the FIRST
// car/slot whose routeId matches, clears the slot, recomputes THAT car's
// target and direction and returns 1. Returns 0 if no slot matched.
// The binary also decrements assignedCount and destinationCountByFloor
// inline here; the TS counter model leaves that to SyncAssignmentStatus
// after the cancel (see the TS comment).
// TS: queue/scan.ts removeRequestFromActiveRouteSlots (binary 1218:173a)
func RemoveRequestFromActiveRouteSlots(carrier *CarrierRecord, simID string) int {
	for carIndex := 0; carIndex < len(carrier.Cars); carIndex++ {
		car := carrier.Cars[carIndex]
		if car == nil {
			continue
		}
		limit := activeSlotLimit(carrier)
		for slotIndex := 0; slotIndex < limit; slotIndex++ {
			if slotIndex >= len(car.ActiveRouteSlots) {
				continue
			}
			slot := car.ActiveRouteSlots[slotIndex]
			if slot == nil || !slot.Active || slot.RouteID != simID {
				continue
			}
			slot.Active = false
			NormalizeInactiveSlots(car)
			SyncPendingRouteIDs(car)
			RecomputeCarTargetAndDirection(carrier, car, carIndex)
			return 1
		}
	}
	return 0
}
