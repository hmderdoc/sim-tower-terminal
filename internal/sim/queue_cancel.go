package sim

// Port of reference/tower-together/sim/queue/cancel.ts.
//
// 1218:1a86 cancel_runtime_route_request
// 1218:1981 dispatch_queued_route_until_request
// 1218:0fc4 decrement_route_queue_direction_load

// DecrementRouteQueueDirectionLoad is intentionally empty upstream
// (TODO(1218:0fc4): the TS scorer does not read these load counters yet).
// TS: queue/cancel.ts decrementRouteQueueDirectionLoad (binary 1218:0fc4)
func DecrementRouteQueueDirectionLoad(_ *CarrierRecord, _ int, _ int) {
	// Intentionally empty.
}

// DispatchQueuedRouteUntilRequest: upstream only removes the target from
// the ring (TODO(1218:1981): pipe preceding entries to family handlers).
// TS: queue/cancel.ts dispatchQueuedRouteUntilRequest (binary 1218:1981)
func DispatchQueuedRouteUntilRequest(carrier *CarrierRecord, simID string, sourceFloor, directionFlag int) bool {
	return RemoveRequestFromUnitQueue(carrier, simID, sourceFloor, directionFlag)
}

// filterOutPendingRoute is `pendingRoutes.filter(c => c.simId !== simID)`:
// a fresh slice (the old one is not mutated, like the JS filter).
func filterOutPendingRoute(routes []*CarrierPendingRoute, simID string) []*CarrierPendingRoute {
	out := make([]*CarrierPendingRoute, 0, len(routes))
	for _, r := range routes {
		if r.SimID != simID {
			out = append(out, r)
		}
	}
	return out
}

// CancelRuntimeRouteRequest is the full cancel path: removes the request id
// from every car's active slots (or, if none held it, from the floor ring),
// drops it from pendingRoutes and resyncs derived per-car state. Called from
// maybe_dispatch_queued_route_after_wait (1228:15a0).
// TS: queue/cancel.ts cancelRuntimeRouteRequest (binary 1218:1a86)
func CancelRuntimeRouteRequest(carrier *CarrierRecord, simID string) {
	route := FindRouteByID(carrier, simID)
	if route == nil {
		return
	}
	clearedSlots := RemoveRequestFromActiveRouteSlots(carrier, simID)
	if clearedSlots == 0 {
		RemoveRequestFromUnitQueue(carrier, simID, route.SourceFloor, route.DirectionFlag)
	}
	carrier.PendingRoutes = filterOutPendingRoute(carrier.PendingRoutes, simID)
	SyncAssignmentStatus(carrier)
}
