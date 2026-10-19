package sim

// Port of reference/tower-together/sim/families/tile-spans.ts (binary
// 1228:0fc2 rebuild_all_sim_tile_spans, 1228:1018 update_sim_tile_span).
// The binary's per-(floor, type) slot loop maps onto a world.sims sweep in
// array order.

// UpdateSimTileSpan TS: families/tile-spans.ts updateSimTileSpan (binary
// 1228:1018, apply_updates=0).
func UpdateSimTileSpan(world *WorldState, sim *SimRecord) {
	if sim.FamilyCode == FamilyHousekeeping {
		if sim.StateCode > 2 {
			FinalizeRuntimeRouteState(world, sim)
		}
	} else {
		if IsSimInTransit(sim.StateCode) {
			FinalizeRuntimeRouteState(world, sim)
		}
		if sim.FamilyCode == FamilyOffice {
			ReleaseServiceRequest(world, sim)
		}
	}
}

// RebuildAllSimTileSpans TS: families/tile-spans.ts rebuildAllSimTileSpans (binary 1228:0fc2)
func RebuildAllSimTileSpans(world *WorldState) {
	for _, sim := range world.Sims {
		UpdateSimTileSpan(world, sim)
	}
}
