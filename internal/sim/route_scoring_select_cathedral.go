package sim

// Port of reference/tower-together/sim/route-scoring/select-cathedral.ts.
//
// Cathedral guests (families 0x24-0x28) route through
// handle_family_parking_outbound_route (1228:5ddd) and
// handle_family_parking_return_route (1228:5e7e), both with
// is_passenger_route = 1, i.e. plain passenger routing.

// SelectCathedralRoute TS: route-scoring/select-cathedral.ts selectCathedralRoute
func SelectCathedralRoute(world *WorldState, fromFloor, toFloor, targetHeightMetric int) *RouteCandidate {
	return SelectBestRouteCandidate(world, fromFloor, toFloor, true /* preferLocalMode */, targetHeightMetric)
}
