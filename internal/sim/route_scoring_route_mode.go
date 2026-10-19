package sim

// Port of reference/tower-together/sim/route-scoring/route-mode.ts.
//
// 11b8:1422 getCurrentSimRouteMode. Not called by the TS sim (sim.homeColumn
// fills the height-metric role); kept as a reference mapping.

// Sim route modes. TS: SimRouteMode
const (
	SimRouteModePassenger = "passenger"
	SimRouteModeService   = "service"
)

// GetCurrentSimRouteMode TS: route-scoring/route-mode.ts getCurrentSimRouteMode (binary 11b8:1422)
func GetCurrentSimRouteMode(sim *SimRecord) string {
	if sim.FamilyCode == FamilyHousekeeping { // TS local FAMILY_HOUSEKEEPING = 0x0f
		return SimRouteModeService
	}
	return SimRouteModePassenger
}

// SimPrefersLocalMode TS: route-scoring/route-mode.ts simPrefersLocalMode
func SimPrefersLocalMode(sim *SimRecord) bool {
	return GetCurrentSimRouteMode(sim) == SimRouteModePassenger
}
