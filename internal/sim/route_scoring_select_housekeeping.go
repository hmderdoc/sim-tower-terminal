package sim

// Port of reference/tower-together/sim/route-scoring/select-housekeeping.ts.
//
// Housekeeping (family 0x0f) calls resolve_sim_route_between_floors
// (1218:0000) with is_passenger_route = 0 (call sites 1228:620f and
// 1228:6320 in update_object_family_housekeeping_connection_state), which
// forwards prefer_local_mode = 0 to select_best_route_candidate.

// SelectHousekeepingRoute TS: route-scoring/select-housekeeping.ts selectHousekeepingRoute
func SelectHousekeepingRoute(world *WorldState, fromFloor, toFloor, targetHeightMetric int) *RouteCandidate {
	return SelectBestRouteCandidate(world, fromFloor, toFloor, false /* preferLocalMode */, targetHeightMetric)
}
