package sim

import "slices"

// Port of reference/tower-together/sim/queue/dispatch-arrivals.ts.
//
// 1218:07a6 dispatch_carrier_car_arrivals
// 1218:0883 dispatch_destination_queue_entries
//
// Per-tick arrival pass: at dwellCounter == 5 (first dwell tick after a
// stop), walks the car's active-slot ring and drops any slot whose
// destination matches the current floor, invoking the family handler
// inline (DispatchSimArrival, sims/index.ts).

// findSimByKey is world.sims.find(s => simKey(s) === id).
func findSimByKey(world *WorldState, id string) *SimRecord {
	for _, s := range world.Sims {
		if SimKey(s) == id {
			return s
		}
	}
	return nil
}

// DispatchDestinationQueueEntries iterates the car's active slots up to
// assignment capacity; for each boarded slot whose destination is the
// current floor it (1) clears the slot in place, (2) runs the family
// arrival handler, (3) decrements assignedCount and
// destinationCountByFloor. The handler runs BEFORE the counter decrements
// so an in-handler recompute sees the binary's count snapshot.
// TS: queue/dispatch-arrivals.ts dispatchDestinationQueueEntries (binary 1218:0883)
func DispatchDestinationQueueEntries(world *WorldState, ledger *LedgerState, time *TimeState, carrier *CarrierRecord, car *CarrierCar) bool {
	changed := false
	assignedBeforeArrivals := car.AssignedCount
	limit := activeSlotLimit(carrier)
	destinationSlot := FloorToSlot(carrier, car.CurrentFloor)

	for index := 0; index < limit; index++ {
		if index >= len(car.ActiveRouteSlots) {
			continue
		}
		slot := car.ActiveRouteSlots[index]
		if slot == nil || !slot.Active || !slot.Boarded {
			continue
		}
		if slot.DestinationFloor != car.CurrentFloor {
			continue
		}

		// Step 1: pop_active_route_slot_request - clear the slot in place.
		arrivedRouteID := slot.RouteID
		arrivedFloor := slot.DestinationFloor
		slot.Active = false
		slot.RouteID = ""
		slot.SourceFloor = 0xff
		slot.DestinationFloor = 0xff
		slot.Boarded = false

		// Step 2: family dispatch (may recursively touch this car via
		// cancel_runtime_route_request).
		if sim := findSimByKey(world, arrivedRouteID); sim != nil {
			DispatchSimArrival(world, ledger, time, sim, arrivedFloor)
		}

		// Step 3: decrement counters AFTER the handler.
		car.AssignedCount = max(0, car.AssignedCount-1)
		if destinationSlot >= 0 {
			prev := trAt(car.DestinationCountByFloor, destinationSlot, 0)
			trSet(&car.DestinationCountByFloor, destinationSlot, max(0, prev-1))
			if prev == 1 {
				car.NonemptyDestinationCount = max(0, car.NonemptyDestinationCount-1)
			}
		}

		carrier.PendingRoutes = filterOutPendingRoute(carrier.PendingRoutes, arrivedRouteID)
		if !slices.Contains(carrier.CompletedRouteIDs, arrivedRouteID) {
			carrier.CompletedRouteIDs = append(carrier.CompletedRouteIDs, arrivedRouteID)
		}
		changed = true
	}

	if changed {
		car.ArrivalDispatchThisTick = true
		car.ArrivalDispatchStartingAssignedCount = assignedBeforeArrivals
		SyncPendingRouteIDs(car)
		SyncAssignmentStatus(carrier)
	}
	return changed
}

// DispatchCarrierCarArrivals gates on dwellCounter == 5 and delegates to
// DispatchDestinationQueueEntries.
// TS: queue/dispatch-arrivals.ts dispatchCarrierCarArrivals (binary 1218:07a6)
func DispatchCarrierCarArrivals(world *WorldState, ledger *LedgerState, time *TimeState, carrier *CarrierRecord, car *CarrierCar) {
	if !car.Active {
		return
	}
	if car.DwellCounter != departureSequenceTicks {
		return
	}
	DispatchDestinationQueueEntries(world, ledger, time, carrier, car)
}
