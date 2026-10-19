package sim

// Port of reference/tower-together/sim/carriers/sync.ts: carrier-side
// derived-state sync helpers.
//
// The binary maintains per-car counters (waiting_count[],
// destination_count[], pending_route_ids, slot_destination_floors)
// incrementally as requests move through the queue. The TS model keeps
// pendingRoutes as the source of truth and resyncs the derived counters
// after each mutation through SyncAssignmentStatus.

// ActiveSlotCapacity TS: ACTIVE_SLOT_CAPACITY (carriers.ts and
// carriers/sync.ts both declare 42). Exported as ACTIVE_SLOT_CAPACITY_CONST.
const ActiveSlotCapacity = 42

// activeSlotLimit TS: carriers/sync.ts activeSlotLimit (exported as activeSlotLimitFor)
func activeSlotLimit(carrier *CarrierRecord) int {
	return min(ActiveSlotCapacity, carrier.AssignmentCapacity)
}

// FindRouteByID is pendingRoutes.find(route => route.simId === routeId);
// nil when absent.
// TS: carriers/sync.ts findRoute (exported as findRouteById)
func FindRouteByID(carrier *CarrierRecord, routeID string) *CarrierPendingRoute {
	for _, route := range carrier.PendingRoutes {
		if route.SimID == routeID {
			return route
		}
	}
	return nil
}

// newIdleRouteSlot is the TS `{ routeId: "", sourceFloor: 0xff,
// destinationFloor: 0xff, boarded: false, active: false }` literal.
func newIdleRouteSlot() *CarrierRouteSlot {
	return &CarrierRouteSlot{SourceFloor: 0xff, DestinationFloor: 0xff}
}

// SyncPendingRouteIDs TS: carriers/sync.ts syncPendingRouteIds
func SyncPendingRouteIDs(car *CarrierCar) {
	ids := []string{}
	for _, slot := range car.ActiveRouteSlots {
		if slot.Active {
			ids = append(ids, slot.RouteID)
		}
	}
	car.PendingRouteIDs = ids
}

// SyncRouteSlots walks the active slots in place (binary preserves slot
// indices across syncs: pop_active_route_slot_request 1218:1905 clears in
// place, store_request_in_active_route_slot 1218:187b reuses the first
// freed slot). Inactive/orphaned slots are cleared to the sentinel, live
// slots are refreshed from the route table, and the array is padded only
// when shorter than capacity (snapshot hydration).
// TS: carriers/sync.ts syncRouteSlots
func SyncRouteSlots(carrier *CarrierRecord, car *CarrierCar) {
	for _, slot := range car.ActiveRouteSlots {
		if slot == nil {
			continue
		}
		if !slot.Active {
			slot.RouteID = ""
			slot.SourceFloor = 0xff
			slot.DestinationFloor = 0xff
			slot.Boarded = false
			continue
		}
		route := FindRouteByID(carrier, slot.RouteID)
		if route == nil {
			slot.Active = false
			slot.RouteID = ""
			slot.SourceFloor = 0xff
			slot.DestinationFloor = 0xff
			slot.Boarded = false
			continue
		}
		slot.SourceFloor = route.SourceFloor
		slot.DestinationFloor = route.DestinationFloor
		slot.Boarded = route.Boarded
	}
	for len(car.ActiveRouteSlots) < ActiveSlotCapacity {
		car.ActiveRouteSlots = append(car.ActiveRouteSlots, newIdleRouteSlot())
	}
	SyncPendingRouteIDs(car)
}

// HasActiveSlot TS: carriers/sync.ts hasActiveSlot
func HasActiveSlot(car *CarrierCar, routeID string) bool {
	for _, slot := range car.ActiveRouteSlots {
		if slot.Active && slot.RouteID == routeID {
			return true
		}
	}
	return false
}

// AddRouteSlot TS: carriers/sync.ts addRouteSlot
func AddRouteSlot(carrier *CarrierRecord, car *CarrierCar, route *CarrierPendingRoute) bool {
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

// syncWaitingCount TS: carriers/sync.ts syncWaitingCount
func syncWaitingCount(carrier *CarrierRecord, car *CarrierCar, carIndex int) {
	trFill(car.WaitingCount, 0)
	trFill(car.DestinationCountByFloor, 0)
	car.NonemptyDestinationCount = 0

	for slot, queue := range carrier.FloorQueues {
		if queue == nil {
			continue
		}
		trSet(&car.WaitingCount, slot, queue.Up.Size()+queue.Down.Size())
	}

	limit := activeSlotLimit(carrier)
	for index := 0; index < limit; index++ {
		if index >= len(car.ActiveRouteSlots) {
			continue
		}
		slotRef := car.ActiveRouteSlots[index]
		if slotRef == nil || !slotRef.Active || !slotRef.Boarded {
			continue
		}
		slot := FloorToSlot(carrier, slotRef.DestinationFloor)
		if slot < 0 || slot >= len(car.DestinationCountByFloor) {
			continue
		}
		prev := car.DestinationCountByFloor[slot]
		car.DestinationCountByFloor[slot] = prev + 1
		if prev == 0 {
			car.NonemptyDestinationCount++
		}
	}

	for _, route := range carrier.PendingRoutes {
		if !route.Boarded || route.AssignedCarIndex != carIndex {
			continue
		}
		slot := FloorToSlot(carrier, route.DestinationFloor)
		if slot < 0 || slot >= len(car.DestinationCountByFloor) {
			continue
		}
		prev := car.DestinationCountByFloor[slot]
		if prev == 0 {
			car.NonemptyDestinationCount++
		}
		car.DestinationCountByFloor[slot] = prev + 1
	}
}

// SyncAssignmentStatus rebuilds per-car derived state from the
// authoritative route/status tables, recomputing pendingAssignmentCount
// from primary/secondary ownership.
// TS: carriers/sync.ts syncAssignmentStatus
func SyncAssignmentStatus(carrier *CarrierRecord) {
	for _, car := range carrier.Cars {
		car.PendingAssignmentCount = 0
	}
	for _, owner := range carrier.PrimaryRouteStatusByFloor {
		if owner > 0 && owner-1 < len(carrier.Cars) {
			if car := carrier.Cars[owner-1]; car != nil {
				car.PendingAssignmentCount++
			}
		}
	}
	for _, owner := range carrier.SecondaryRouteStatusByFloor {
		if owner > 0 && owner-1 < len(carrier.Cars) {
			if car := carrier.Cars[owner-1]; car != nil {
				car.PendingAssignmentCount++
			}
		}
	}
	for carIndex, car := range carrier.Cars {
		SyncRouteSlots(carrier, car)
		syncWaitingCount(carrier, car, carIndex)
	}
}

// NormalizeInactiveSlots TS: carriers/sync.ts normalizeInactiveSlots
func NormalizeInactiveSlots(car *CarrierCar) {
	for _, slot := range car.ActiveRouteSlots {
		if !slot.Active {
			slot.RouteID = ""
			slot.SourceFloor = 0xff
			slot.DestinationFloor = 0xff
			slot.Boarded = false
		}
	}
}
