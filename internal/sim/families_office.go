package sim

// Port of reference/tower-together/sim/families/office.ts (binary 1228:1cb5
// refresh_object_family_office_state_handler, 1228:2031
// dispatch_object_family_office_state_handler). The TS re-exports of
// sims/office.ts need no Go shims.

// prologueCarrierForSim is the TS
// `world.carriers.find(c => c.carrierId === (route is carrier ? carrierId : undefined)) ?? world.carriers[0]`
// used by the family refresh prologues. A non-carrier route never matches
// (undefined), so it falls back to the first carrier.
func prologueCarrierForSim(world *WorldState, sim *SimRecord) *CarrierRecord {
	if sim.Route.Mode == RouteModeCarrier {
		for _, c := range world.Carriers {
			if c.CarrierID == sim.Route.CarrierID {
				return c
			}
		}
	}
	if len(world.Carriers) > 0 {
		return world.Carriers[0]
	}
	return nil
}

// RefreshObjectFamilyOfficeStateHandler TS: families/office.ts
// refreshObjectFamilyOfficeStateHandler (binary 1228:1cb5). In-transit
// states run the decrement_route_queue_direction_load prologue (1218:0fc4)
// and the wait-timeout check first.
func RefreshObjectFamilyOfficeStateHandler(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	if IsSimInTransit(sim.StateCode) {
		if carrier := prologueCarrierForSim(world, sim); carrier != nil {
			DecrementRouteQueueDirectionLoad(carrier, sim.OriginFloor, 0)
		}
		MaybeDispatchQueuedRouteAfterWait(world, ledger, time, sim)
		if !IsSimInTransit(sim.StateCode) {
			return
		}
	}
	ProcessOfficeSim(world, ledger, time, sim)
}

// DispatchObjectFamilyOfficeStateHandler TS: families/office.ts
// dispatchObjectFamilyOfficeStateHandler (binary 1228:2031, arrival path
// from dispatch_destination_queue_entries 1218:0883).
func DispatchObjectFamilyOfficeStateHandler(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	_ = SimBaseState(sim.StateCode) // reserved for future table-driven dispatch (TS `void`)
	HandleOfficeSimArrival(world, time, sim, arrivalFloor, false)
}
