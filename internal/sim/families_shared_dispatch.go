package sim

// Port of reference/tower-together/sim/families/shared-dispatch.ts (binary
// 1228:650e, the shared destination-floor resolver for families
// 3/4/5/6/7/9/10/0c, jump tables CS:65a1 / CS:65b1).

// Binary jump-table keys at CS:65a1 (file-local TS constants).
const (
	sharedDispatchCommuteTransit   = 0x40 // outbound to lobby
	sharedDispatchActiveTransit    = 0x41 // outbound to commercial venue
	sharedDispatchMedicalTransit   = 0x42 // outbound for medical
	sharedDispatchDepartureTransit = 0x45 // end-of-day return / service
	sharedDispatchHomeTransitBase  = 0x60 // 0x60-0x63: inbound to home floor
)

// DispatchObjectFamilyHotelRestaurantOfficeCondoRetailFastFoodStateHandler TS:
// families/shared-dispatch.ts (binary 1228:650e). Returns the logical
// destination floor for the sim's in-transit state, or -1.
func DispatchObjectFamilyHotelRestaurantOfficeCondoRetailFastFoodStateHandler(world *WorldState, sim *SimRecord) int {
	stateCode := sim.StateCode

	// 0x40: hardcoded lobby.
	if stateCode == sharedDispatchCommuteTransit {
		return LobbyFloor
	}

	// 0x41: get_current_commercial_venue_destination_floor (11b0:10fe).
	if stateCode == sharedDispatchActiveTransit {
		if sim.DestinationFloor >= 0 {
			return sim.DestinationFloor
		}
		return LobbyFloor
	}

	// 0x42: lookup_medical_service_source_floor (1178:0522); first active
	// slot for this sim in slot order.
	if stateCode == sharedDispatchMedicalTransit {
		id := SimKey(sim)
		for _, slot := range world.MedicalServiceSlots {
			if slot.Active && slot.SimID == id {
				return slot.SourceFloor
			}
		}
		return LobbyFloor
	}

	// 0x45: get_sim_assigned_floor (11a0:0650).
	if stateCode == sharedDispatchDepartureTransit {
		if sim.DestinationFloor >= 0 {
			return sim.DestinationFloor
		}
		return LobbyFloor
	}

	// 0x60-0x63: get_current_sim_type (1228:681d) -> sim[+0] = floorAnchor.
	if stateCode >= sharedDispatchHomeTransitBase && stateCode <= sharedDispatchHomeTransitBase+3 {
		return sim.FloorAnchor
	}

	// No match: the caller's local stays at -1.
	return -1
}
