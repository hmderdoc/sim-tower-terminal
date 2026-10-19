package sim

// Port of reference/tower-together/sim/sim-access/selectors.ts: binary
// sim-level selector / accessor helpers (1228:681d..688c + 1228:6700 +
// 1228:65c1 + 1228:6757 + 1228:662a + 1228:640c + 1228:67d7).

// GetCurrentSimType: byte at sim+4 (family_code).
// TS: sim-access/selectors.ts getCurrentSimType (binary 1228:681d)
func GetCurrentSimType(sim *SimRecord) int { return sim.FamilyCode }

// GetCurrentSimVariant: byte at sim+1 (object_subtype_index).
// TS: sim-access/selectors.ts getCurrentSimVariant (binary 1228:6854)
func GetCurrentSimVariant(sim *SimRecord) int { return sim.FacilitySlot }

// GetCurrentSimStateWord: word at sim+2.
// TS: sim-access/selectors.ts getCurrentSimStateWord (binary 1228:688c)
func GetCurrentSimStateWord(sim *SimRecord) int { return sim.BaseOffset }

// ResolveFamilyParkingSelectorValue
// TS: sim-access/selectors.ts resolveFamilyParkingSelectorValue (binary 1228:6700)
func ResolveFamilyParkingSelectorValue(_ *WorldState, sim *SimRecord) int {
	switch sim.StateCode {
	case 0x45:
		return 10
	case 0x60:
		// Binary quirk: 0x6d = 109, outbound cathedral zone constant.
		return 0x6d
	default:
		return -1
	}
}

// ResolveFamilyRecyclingCenterLowerSelectorValue
// TS: sim-access/selectors.ts resolveFamilyRecyclingCenterLowerSelectorValue (binary 1228:65c1)
func ResolveFamilyRecyclingCenterLowerSelectorValue(_ *WorldState, sim *SimRecord) int {
	switch sim.StateCode {
	case 0x41:
		return sim.DestinationFloor
	case 0x62:
		return GetCurrentSimType(sim) + 2
	default:
		return -1
	}
}

// GetHousekeepingRoomClaimSelector
// TS: sim-access/selectors.ts getHousekeepingRoomClaimSelector (binary 1228:6757)
func GetHousekeepingRoomClaimSelector(_ *WorldState, sim *SimRecord) int {
	switch sim.StateCode {
	case 0x03:
		return sim.SelectedFloor
	case 0x04:
		return GetCurrentSimType(sim)
	default:
		return -1
	}
}

// DispatchEntertainmentGuestSubstate: 4-entry jump table for {0x41, 0x45,
// 0x60, 0x62}; only 0x41 (attendance increment on the first matching
// entertainment link, in sidecar order) has a TS-visible effect.
// TS: sim-access/selectors.ts dispatchEntertainmentGuestSubstate (binary 1228:662a)
func DispatchEntertainmentGuestSubstate(world *WorldState, sim *SimRecord) {
	if sim.StateCode == 0x41 {
		for _, sidecar := range world.Sidecars {
			if link, ok := sidecar.(*EntertainmentLinkRecord); ok && link.OwnerSubtypeIndex == sim.HomeColumn {
				link.AttendanceCounter++
				break
			}
		}
	}
	// States 0x45 / 0x62 return 0xa with no side effects; 0x60 has no
	// TS-visible mutation.
}

// MaybeStartHousekeepingRoomClaim
// TS: sim-access/selectors.ts maybeStartHousekeepingRoomClaim (binary 1228:640c)
// PORT-NOTE: the TS passes `null as never` for the TimeState; the Go port
// passes a nil *TimeState, so the callee must not dereference it on this path.
func MaybeStartHousekeepingRoomClaim(world *WorldState, sim *SimRecord) bool {
	targetIndex := ComputeObjectOccupantRuntimeIndex(world, sim)
	if targetIndex < 0 || targetIndex >= len(world.Sims) {
		return false
	}
	targetSim := world.Sims[targetIndex]
	if targetSim == nil {
		return false
	}
	if targetSim.StateCode != 0 {
		return false
	}
	// Binary quirk: (starRatingFlags & 9) != 0.
	if world.StarCount&9 == 0 {
		return false
	}
	ActivateObjectFamilyHousekeepingConnectionState(world, nil, targetSim, sim.SelectedFloor)
	return true
}

// ComputeObjectOccupantRuntimeIndex returns the index in world.Sims of the
// occupant identified by (floorAnchor, facilitySlot, baseOffset): the first
// sim with the same floor/slot and baseOffset 0, plus baseOffset.
// TS: sim-access/selectors.ts computeObjectOccupantRuntimeIndex (binary 1228:67d7)
func ComputeObjectOccupantRuntimeIndex(world *WorldState, sim *SimRecord) int {
	for i, s := range world.Sims {
		if s.FloorAnchor == sim.FloorAnchor && s.FacilitySlot == sim.FacilitySlot && s.BaseOffset == 0 {
			return i + sim.BaseOffset
		}
	}
	return -1
}
