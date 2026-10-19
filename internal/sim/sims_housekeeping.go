package sim

import "sort"

// Port of reference/tower-together/sim/sims/housekeeping.ts: housekeeping
// helper (family 0x0f) room-claim state machine.

// hotelTurnoverStatus TS: HOTEL_TURNOVER_STATUS (lookup-only).
var hotelTurnoverStatus = map[int]bool{0x28: true, 0x30: true}

// foundRoom TS: sims/housekeeping.ts FoundRoom
type foundRoom struct {
	floor       int
	object      *PlacedObjectRecord
	subtypeByte int
}

// isClaimableHotelRoom TS: sims/housekeeping.ts isClaimableHotelRoom
func isClaimableHotelRoom(object *PlacedObjectRecord) bool {
	if !HotelFamilies[object.ObjectTypeCode] {
		return false
	}
	return hotelTurnoverStatus[object.UnitStatus]
}

type hkFloorEntry struct {
	x      int
	object *PlacedObjectRecord
}

// findVacantHotelRoomForClaim TS: sims/housekeeping.ts findVacantHotelRoomForClaim.
// Scan upward from spawnFloor to the top, then downward from spawnFloor-1;
// within floors of the right %6 class, return the leftmost turnover-band
// hotel room. Per-floor lists keep placedObjects insertion order and are
// then stably sorted by x (JS sort is stable), so equal-x ties keep
// insertion order. byFloor is lookup-only, so a Go map is fine.
func findVacantHotelRoomForClaim(world *WorldState, spawnFloor, floorClass int) *foundRoom {
	byFloor := map[int][]hkFloorEntry{}
	for _, e := range world.PlacedObjects.Entries() {
		x, y := parseKeyXY(e.Key)
		floor := YToFloor(y)
		byFloor[floor] = append(byFloor[floor], hkFloorEntry{x, e.Val})
	}
	for _, list := range byFloor {
		sort.SliceStable(list, func(i, j int) bool { return list[i].x < list[j].x })
	}

	tryFloor := func(floor int) *foundRoom {
		if floor < 0 || floor >= world.Height {
			return nil
		}
		if ((floor%HkFloorClassMod)+HkFloorClassMod)%HkFloorClassMod != floorClass {
			return nil
		}
		list, ok := byFloor[floor]
		if !ok {
			return nil
		}
		for _, entry := range list {
			if isClaimableHotelRoom(entry.object) {
				return &foundRoom{floor: floor, object: entry.object, subtypeByte: entry.x & 0xff}
			}
		}
		return nil
	}

	for floor := spawnFloor; floor < world.Height; floor++ {
		if hit := tryFloor(floor); hit != nil {
			return hit
		}
	}
	for floor := spawnFloor - 1; floor >= 0; floor-- {
		if hit := tryFloor(floor); hit != nil {
			return hit
		}
	}
	return nil
}

// attemptRouteToFloor TS: sims/housekeeping.ts attemptRouteToFloor. Binary
// 1228:620f / 1228:6320 pass is_passenger_route=0 and emit_distance_feedback=0.
func attemptRouteToFloor(world *WorldState, sim *SimRecord, targetFloor int, time *TimeState) int {
	direction := dirFlag(targetFloor, sim.SelectedFloor)
	f := false
	return ResolveSimRouteBetweenFloors(world, sim, sim.SelectedFloor, targetFloor, direction, time,
		&ResolveSimRouteOptions{IsPassengerRoute: &f, EmitDistanceFeedback: &f})
}

// findHotelFirstOccupant is the TS `world.sims.find(...)` for the
// base-offset-0 occupant of a hotel room (first match in sims order).
func findHotelFirstOccupant(world *WorldState, room *PlacedObjectRecord, floor int) *SimRecord {
	for _, candidate := range world.Sims {
		if candidate.FamilyCode == room.ObjectTypeCode &&
			candidate.FloorAnchor == floor &&
			candidate.HomeColumn == room.LeftTileIndex &&
			candidate.BaseOffset == 0 {
			return candidate
		}
	}
	return nil
}

// promoteClaim TS: sims/housekeeping.ts promoteClaim (binary
// activate_selected_vacant_unit 1158:02e2; the caller at 1228:6264 still
// enters state 2 with countdown 3 when activation bails).
func promoteClaim(world *WorldState, time *TimeState, sim *SimRecord, object *PlacedObjectRecord) {
	claimable := object != nil && isClaimableHotelRoom(object)
	if claimable {
		if time.DaypartIndex < 4 {
			object.UnitStatus = 0x18
		} else {
			object.UnitStatus = 0x20
		}
		object.HousekeepingClaimedFlag = 1
		if newOccupant := findHotelFirstOccupant(world, object, sim.TargetRoomFloor); newOccupant != nil {
			newOccupant.StateCode = StateArrived
		}
	}
	sim.PostClaimCountdown = HkPostClaimCountdown
	sim.StateCode = HkStateCountdown
}

// resetToSearch TS: sims/housekeeping.ts resetToSearch
func resetToSearch(sim *SimRecord) {
	sim.StateCode = HkStateSearch
	sim.TargetRoomFloor = HkSearchingSentinel
	sim.TargetRoomColumn = -1
	sim.PostClaimCountdown = 0
}

// flagSelectedUnitUnavailable TS: sims/housekeeping.ts flagSelectedUnitUnavailable
// (binary flag_selected_unit_unavailable 1158:04e2).
func flagSelectedUnitUnavailable(world *WorldState, sim *SimRecord) {
	if sim.TargetRoomFloor < 0 || sim.TargetRoomColumn < 0 {
		return
	}
	y := world.Height - 1 - sim.TargetRoomFloor
	room, ok := world.PlacedObjects.Get(cellKey(sim.TargetRoomColumn, y))
	if !ok || room == nil {
		return
	}
	if !HotelFamilies[room.ObjectTypeCode] {
		return
	}
	if firstOccupant := findHotelFirstOccupant(world, room, sim.TargetRoomFloor); firstOccupant != nil {
		firstOccupant.StateCode = StateHotelParked
	}
}

// ProcessHousekeepingSim TS: sims/housekeeping.ts processHousekeepingSim
func ProcessHousekeepingSim(world *WorldState, time *TimeState, sim *SimRecord) {
	if sim.FamilyCode != FamilyHousekeeping {
		return
	}

	switch sim.StateCode {
	case HkStateSearch:
		// Binary state-0 handler only progresses while day_tick < 1500.
		if time.DayTick >= HkClaimDayTickCutoff {
			return
		}
		if sim.SpawnFloor < 0 {
			sim.SpawnFloor = sim.FloorAnchor
		}
		// Binary find_matching_vacant_unit_floor (1158:0000): baseOffset
		// (sim+2) is the floor-class filter.
		floorClass := ((sim.BaseOffset % HkFloorClassMod) + HkFloorClassMod) % HkFloorClassMod
		candidate := findVacantHotelRoomForClaim(world, sim.SpawnFloor, floorClass)
		if candidate == nil {
			sim.TargetRoomFloor = HkSearchingSentinel
			sim.StateCode = HkStateRouteToCandidate
			return
		}
		sim.TargetRoomFloor = candidate.floor
		sim.TargetRoomColumn = candidate.object.LeftTileIndex
		sim.StateCode = HkStateRouteToTarget
		tryClaimOnCurrentFloor(world, time, sim)
		return

	case HkStateRouteToCandidate, HkStateRouteToCandidateTransit:
		// Binary state-1/4 (jump table 1228:6404 / 640a -> 0x62ec).
		if sim.SelectedFloor == sim.SpawnFloor {
			if sim.TargetRoomFloor == HkSearchingSentinel {
				resetToSearch(sim)
				return
			}
			sim.StateCode = HkStateRouteToTarget
			tryClaimOnCurrentFloor(world, time, sim)
			return
		}
		result := attemptRouteToFloor(world, sim, sim.SpawnFloor, time)
		if result == -1 {
			resetToSearch(sim)
			return
		}
		if result == 3 {
			sim.StateCode = HkStateRouteToTarget
			tryClaimOnCurrentFloor(world, time, sim)
			return
		}
		if result != 0 {
			sim.StateCode = HkStateRouteToCandidateTransit
		}
		return

	case HkStateRouteToTarget:
		tryClaimOnCurrentFloor(world, time, sim)
		return

	case HkStateCountdown:
		// Binary state-2 (1228:602b case 2): check-then-decrement.
		if sim.PostClaimCountdown != 0 {
			sim.PostClaimCountdown--
			return
		}
		flagSelectedUnitUnavailable(world, sim)
		resetToSearch(sim)
		return

	default:
		resetToSearch(sim)
	}
}

// tryClaimOnCurrentFloor TS: sims/housekeeping.ts tryClaimOnCurrentFloor
func tryClaimOnCurrentFloor(world *WorldState, time *TimeState, sim *SimRecord) {
	if sim.SelectedFloor != sim.TargetRoomFloor {
		result := attemptRouteToFloor(world, sim, sim.TargetRoomFloor, time)
		if result == -1 {
			resetToSearch(sim)
			return
		}
		return
	}
	if time.DayTick >= HkClaimDayTickCutoff {
		resetToSearch(sim)
		return
	}
	// Look up the specific room targeted at search time.
	y := world.Height - 1 - sim.TargetRoomFloor
	var room *PlacedObjectRecord
	if sim.TargetRoomColumn >= 0 {
		room = world.PlacedObjects.At(cellKey(sim.TargetRoomColumn, y))
	}
	promoteClaim(world, time, sim, room)
}

// HandleHousekeepingSimArrival TS: sims/housekeeping.ts handleHousekeepingSimArrival.
// PORT-NOTE: reached with a nil *TimeState from MaybeStartHousekeepingRoomClaim
// (TS `null as never`). That caller only passes sims in state 0, which
// touches no time field here; states 1/3/4 would dereference time exactly
// like the TS would throw on `null.dayTick`.
func HandleHousekeepingSimArrival(world *WorldState, time *TimeState, sim *SimRecord, arrivalFloor int) {
	sim.SelectedFloor = arrivalFloor
	sim.DestinationFloor = -1

	if sim.StateCode == HkStateRouteToCandidate || sim.StateCode == HkStateRouteToCandidateTransit {
		if arrivalFloor != sim.SpawnFloor {
			return
		}
		sim.StateCode = HkStateRouteToTarget
		tryClaimOnCurrentFloor(world, time, sim)
		return
	}

	if sim.StateCode == HkStateRouteToTarget {
		if arrivalFloor != sim.TargetRoomFloor {
			return
		}
		tryClaimOnCurrentFloor(world, time, sim)
		return
	}
}
