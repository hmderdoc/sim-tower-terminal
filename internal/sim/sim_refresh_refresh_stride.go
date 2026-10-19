package sim

// Port of reference/tower-together/sim/sim-refresh/refresh-stride.ts, a
// re-export shim of sims/index.ts advanceSimRefreshStride (implemented in
// sims_index.go) under the binary-aligned name.

// RefreshRuntimeEntitiesForTickStride TS: sims/index.ts
// refreshRuntimeEntitiesForTickStride (binary refresh_runtime_entities_for_tick_stride
// 1228:0d64), an alias of advanceSimRefreshStride. Used by tick/carrier-tick.ts.
func RefreshRuntimeEntitiesForTickStride(world *WorldState, ledger *LedgerState, time *TimeState) {
	AdvanceSimRefreshStride(world, ledger, time)
}
