package sim

// Port of reference/tower-together/sim/sims/hotel-facilities.ts: end-of-day
// unit-status normalization, cockroach spread, vacancy expiry and stay-phase
// tier advance. All loops walk placedObjects in insertion order
// (Object.values / Object.entries snapshots).

// NormalizeUnitStatusEndOfDay TS: sims/hotel-facilities.ts normalizeUnitStatusEndOfDay
func NormalizeUnitStatusEndOfDay(world *WorldState) {
	for _, object := range world.PlacedObjects.Values() {
		if HotelFamilies[object.ObjectTypeCode] {
			switch {
			case object.UnitStatus < 0x18:
				// Occupied: clamp to sync sentinel.
				object.UnitStatus = 0x10
			case object.UnitStatus >= 0x18 && object.UnitStatus < 0x28:
				if object.UnitStatus < 0x20 {
					object.UnitStatus = 0x20
				} else {
					object.UnitStatus = 0x18
				}
			case object.UnitStatus >= 0x28 && object.UnitStatus < 0x38:
				if object.UnitStatus < 0x30 {
					object.UnitStatus = 0x30
				} else {
					object.UnitStatus = 0x28
				}
			case object.UnitStatus >= 0x38 && object.UnitStatus <= 0x40:
				if object.UnitStatus < 0x40 {
					object.UnitStatus = 0x40
				} else {
					object.UnitStatus = 0x38
				}
			}
		} else if object.ObjectTypeCode == FamilyCondo {
			if object.UnitStatus < 0x18 {
				object.UnitStatus = 0x10
			} else if object.UnitStatus < 0x20 {
				object.UnitStatus = UnitStatusCondoVacantEvening
			} else {
				object.UnitStatus = UnitStatusCondoVacant
			}
		}
	}
}

// SpreadCockroachInfestation TS: sims/hotel-facilities.ts spreadCockroachInfestation
// (checkpoint 0x640). Both loops use the same entries snapshot, so rooms
// infected earlier in the outer loop spread again later in the same pass,
// exactly as in the TS.
func SpreadCockroachInfestation(world *WorldState, time *TimeState) {
	objectsByKey := world.PlacedObjects.Entries()
	for _, e := range objectsByKey {
		object := e.Val
		if !HotelFamilies[object.ObjectTypeCode] {
			continue
		}
		if object.UnitStatus < 0x38 {
			continue
		}

		x, y := parseKeyXY(e.Key)
		for _, n := range objectsByKey {
			neighbor := n.Val
			if neighbor == object {
				continue
			}
			if !HotelFamilies[neighbor.ObjectTypeCode] {
				continue
			}
			nx, ny := parseKeyXY(n.Key)
			if ny != y {
				continue
			}
			if nx < x && neighbor.RightTileIndex >= object.LeftTileIndex-1 {
				infectHotelRoom(neighbor, time)
			}
			if nx > x && neighbor.LeftTileIndex <= object.RightTileIndex+1 && neighbor.UnitStatus < 0x38 {
				infectHotelRoom(neighbor, time)
			}
		}
	}
}

// infectHotelRoom TS: sims/hotel-facilities.ts infectHotelRoom
func infectHotelRoom(neighbor *PlacedObjectRecord, time *TimeState) {
	if time.DaypartIndex < 4 {
		neighbor.UnitStatus = 0x40
	} else {
		neighbor.UnitStatus = 0x38
	}
	neighbor.EvalLevel = 0xff
	neighbor.HousekeepingClaimedFlag = 0
}

// HandleExtendedVacancyExpiry TS: sims/hotel-facilities.ts handleExtendedVacancyExpiry
// (handle_extended_vacancy_expiry, checkpoint 0x640): three-strikes infestation.
func HandleExtendedVacancyExpiry(world *WorldState, time *TimeState) {
	for _, object := range world.PlacedObjects.Values() {
		if !HotelFamilies[object.ObjectTypeCode] {
			continue
		}
		if object.UnitStatus <= 0x27 {
			continue
		}
		if object.UnitStatus >= 0x38 {
			continue
		}

		if object.HousekeepingClaimedFlag != 0 {
			object.EvalLevel = 0xff
			object.ActivationTickCount = 0
			object.HousekeepingClaimedFlag = 0
		} else {
			object.ActivationTickCount++
			if object.ActivationTickCount >= 3 {
				if time.DaypartIndex < 4 {
					object.UnitStatus = 0x40
				} else {
					object.UnitStatus = 0x38
				}
			}
		}
	}
}

// AdvanceObjectStayPhaseTiers TS: sims/hotel-facilities.ts advanceObjectStayPhaseTiers
// (binary advance_object_stay_phase_tiers 1230:0b5f, checkpoint 0x640).
func AdvanceObjectStayPhaseTiers(world *WorldState) {
	for _, object := range world.PlacedObjects.Values() {
		status := object.UnitStatus
		if HotelFamilies[object.ObjectTypeCode] {
			switch status {
			case 0x18:
				object.UnitStatus = 0x20
			case 0x28:
				object.UnitStatus = 0x30
			case 0x38:
				object.UnitStatus = 0x40
			}
		} else if object.ObjectTypeCode == FamilyOffice {
			switch status {
			case 0x00:
				object.UnitStatus = 0x08
			case 0x10:
				object.UnitStatus = 0x18
			}
		} else if object.ObjectTypeCode == FamilyCondo {
			if status == 0x18 {
				object.UnitStatus = 0x20
			} else if status&0xf8 == 0 {
				object.UnitStatus = (status & 7) + 0x08
			}
		}
	}
}

// UpdateHotelOperationalAndOccupancy TS: sims/hotel-facilities.ts
// updateHotelOperationalAndOccupancy (update_hotel_operational_and_pairings).
func UpdateHotelOperationalAndOccupancy(world *WorldState, time *TimeState) {
	for _, sim := range world.Sims {
		object := FindObjectForSim(world, sim)
		if object == nil || !HotelFamilies[object.ObjectTypeCode] {
			continue
		}
		if sim.BaseOffset != 0 {
			continue
		}
		RecomputeObjectOperationalStatus(world, sim, object)
	}
	HandleExtendedVacancyExpiry(world, time)
	for _, sim := range world.Sims {
		object := FindObjectForSim(world, sim)
		if object == nil || !HotelFamilies[object.ObjectTypeCode] {
			continue
		}
		if sim.BaseOffset != 0 {
			continue
		}
		// Pass 2: rooms >= 0x28 keep their occupiedFlag.
		if object.UnitStatus >= 0x28 {
			continue
		}
		RefreshOccupiedFlagAndTripCounters(world, sim, object)
	}
}
