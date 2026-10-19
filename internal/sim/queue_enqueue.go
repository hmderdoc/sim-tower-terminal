package sim

// Port of reference/tower-together/sim/queue/enqueue.ts.
//
// 1218:1002 enqueue_request_into_route_queue
//
// Append a route request to the (carrier, floor, direction) ring. The first
// enqueue onto a previously-empty direction queue triggers
// assign_car_to_floor_request to pick a serving car.

// queueState TS: getQueueState (queue/enqueue.ts, queue/process-travel.ts)
func queueState(carrier *CarrierRecord, floor int) *CarrierFloorQueue {
	slot := FloorToSlot(carrier, floor)
	if slot < 0 || slot >= len(carrier.FloorQueues) {
		return nil
	}
	return carrier.FloorQueues[slot]
}

// directionQueue TS: getDirectionQueue (queue/enqueue.ts, queue/process-travel.ts)
func directionQueue(queue *CarrierFloorQueue, directionFlag int) *RouteRequestRing {
	if directionFlag == 1 {
		return queue.Up
	}
	return queue.Down
}

// EnqueueRequestIntoRouteQueue pushes simID onto the direction ring for
// (carrier, sourceFloor, directionFlag), adds the matching
// CarrierPendingRoute, and on a previously empty ring calls
// AssignCarToFloorRequest. Deduplicates on simID. Returns false only when
// there is no queue slot or the ring is full.
// TS: queue/enqueue.ts enqueueRequestIntoRouteQueue (binary 1218:1002)
// PORT-NOTE: the __DRAIN_TRACE__ console logging is not ported.
func EnqueueRequestIntoRouteQueue(carrier *CarrierRecord, simID string, sourceFloor, destinationFloor, directionFlag int, lobbyMode LobbyMode) bool {
	for _, route := range carrier.PendingRoutes {
		if route.SimID == simID {
			return true
		}
	}
	// Binary 1218:1002: express (mode=0) at intermediate (non-lobby) floors
	// silently no-ops while resolve still returns 2 and writes sim+8.
	if carrier.CarrierMode == 0 && !IsExpressStopFloor(sourceFloor, lobbyMode) {
		return true
	}
	floorQueue := queueState(carrier, sourceFloor)
	if floorQueue == nil {
		return false
	}
	dq := directionQueue(floorQueue, directionFlag)
	// Binary: resolve checks count==40 upstream and returns 0 (queue-full)
	// without invoking enqueue.
	if dq.IsFull() {
		return false
	}
	wasDirectionQueueEmpty := dq.IsEmpty()
	dq.Push(simID)
	carrier.PendingRoutes = append(carrier.PendingRoutes, &CarrierPendingRoute{
		SimID:            simID,
		SourceFloor:      sourceFloor,
		DestinationFloor: destinationFloor,
		Boarded:          false,
		DirectionFlag:    directionFlag,
		AssignedCarIndex: -1,
	})
	if wasDirectionQueueEmpty {
		AssignCarToFloorRequest(carrier, sourceFloor, directionFlag)
	}
	SyncAssignmentStatus(carrier)
	return true
}
