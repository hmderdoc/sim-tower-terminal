package sim

import "testing"

// Port of reference/tower-together/sim/sims/housekeeping.test.ts.

func hkTestWorld() *WorldState {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	return w
}

func hkAddHotelRoom(world *WorldState, x, floor, unitStatus int) *PlacedObjectRecord {
	y := FloorToY(floor)
	object := &PlacedObjectRecord{
		LeftTileIndex:     x,
		RightTileIndex:    x + 3,
		ObjectTypeCode:    FamilyHotelSingle,
		UnitStatus:        unitStatus,
		LinkedRecordIndex: -1,
		EvalScore:         -1,
		RentLevel:         1,
	}
	world.PlacedObjects.Set(cellKey(x, y), object)
	return object
}

func hkMakeHousekeeper(floor int) *SimRecord {
	return &SimRecord{
		FloorAnchor:         floor,
		FamilyCode:          FamilyHousekeeping,
		StateCode:           HkStateSearch,
		Route:               RouteIdle,
		SelectedFloor:       floor,
		OriginFloor:         floor,
		DestinationFloor:    -1,
		LastDemandTick:      -1,
		TargetRoomFloor:     HkSearchingSentinel,
		TargetRoomColumn:    -1,
		SpawnFloor:          floor,
		CommercialVenueSlot: -1,
	}
}

func hkTimeAt(dayTick int) *TimeState {
	t := CreateTimeState()
	t.DayTick = dayTick
	return t
}

func TestHousekeepingStaysSearchingWithoutTurnoverRooms(t *testing.T) {
	world := hkTestWorld()
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(100), sim)
	if sim.TargetRoomFloor != HkSearchingSentinel {
		t.Fatalf("targetRoomFloor = %d", sim.TargetRoomFloor)
	}
	if sim.PostClaimCountdown != 0 {
		t.Fatalf("postClaimCountdown = %d", sim.PostClaimCountdown)
	}
}

func TestHousekeepingSkipsWrongFloorClass(t *testing.T) {
	world := hkTestWorld()
	room := hkAddHotelRoom(world, 20, 13, 0x28)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(100), sim)
	if room.HousekeepingClaimedFlag != 0 {
		t.Fatal("room claimed")
	}
	if sim.TargetRoomFloor != HkSearchingSentinel {
		t.Fatalf("targetRoomFloor = %d", sim.TargetRoomFloor)
	}
}

func TestHousekeepingClaimsSameFloorRoom(t *testing.T) {
	world := hkTestWorld()
	room := hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(100), sim)
	if sim.StateCode != HkStateCountdown {
		t.Fatalf("state = %d", sim.StateCode)
	}
	if sim.TargetRoomFloor != 12 || sim.PostClaimCountdown != HkPostClaimCountdown {
		t.Fatalf("target %d countdown %d", sim.TargetRoomFloor, sim.PostClaimCountdown)
	}
	if room.HousekeepingClaimedFlag != 1 || room.UnitStatus != 0x18 {
		t.Fatalf("claimed %d status %#x", room.HousekeepingClaimedFlag, room.UnitStatus)
	}
	if sim.TargetRoomColumn != 20 {
		t.Fatalf("targetRoomColumn = %d", sim.TargetRoomColumn)
	}
}

func TestHousekeepingRefusesAtCutoff(t *testing.T) {
	world := hkTestWorld()
	room := hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(1500), sim)
	if sim.StateCode != HkStateSearch || room.HousekeepingClaimedFlag != 0 || room.UnitStatus != 0x28 {
		t.Fatalf("state %d claimed %d status %#x", sim.StateCode, room.HousekeepingClaimedFlag, room.UnitStatus)
	}
}

func TestHousekeepingUpwardScanPrefersSpawnFloor(t *testing.T) {
	world := hkTestWorld()
	spawnFloorRoom := hkAddHotelRoom(world, 10, 12, 0x28)
	downwardRoom := hkAddHotelRoom(world, 10, 6, 0x28)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(50), sim)
	if sim.StateCode != HkStateCountdown {
		t.Fatalf("state = %d", sim.StateCode)
	}
	if spawnFloorRoom.HousekeepingClaimedFlag != 1 || downwardRoom.HousekeepingClaimedFlag != 0 {
		t.Fatal("wrong room claimed")
	}
}

func TestHousekeepingPicksLeftmostSlot(t *testing.T) {
	world := hkTestWorld()
	hkAddHotelRoom(world, 40, 12, 0x28)
	leftmost := hkAddHotelRoom(world, 10, 12, 0x30)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(50), sim)
	if sim.StateCode != HkStateCountdown || leftmost.HousekeepingClaimedFlag != 1 {
		t.Fatalf("state %d leftmost claimed %d", sim.StateCode, leftmost.HousekeepingClaimedFlag)
	}
}

func TestHousekeepingPostClaimCountdown(t *testing.T) {
	world := hkTestWorld()
	room := hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(50), sim)
	if sim.PostClaimCountdown != 3 {
		t.Fatalf("countdown = %d", sim.PostClaimCountdown)
	}
	// Check-before-decrement: 4 strides in state 2 for a countdown of 3.
	for i := 1; i <= 4; i++ {
		ProcessHousekeepingSim(world, hkTimeAt(50+i), sim)
	}
	if sim.StateCode != HkStateSearch || sim.PostClaimCountdown != 0 {
		t.Fatalf("state %d countdown %d", sim.StateCode, sim.PostClaimCountdown)
	}
	if room.HousekeepingClaimedFlag != 1 {
		t.Fatal("flag cleared")
	}
}

func TestHousekeepingCrossFloorRouteFailResets(t *testing.T) {
	skipOnTODO(t, func() {
		world := hkTestWorld()
		hkAddHotelRoom(world, 20, 18, 0x28)
		sim := hkMakeHousekeeper(12)
		world.Sims = append(world.Sims, sim)
		ProcessHousekeepingSim(world, hkTimeAt(50), sim)
		// No carriers/routes in this world -> route-fail -> reset.
		if sim.StateCode != HkStateSearch {
			t.Fatalf("state = %d", sim.StateCode)
		}
	})
}

func TestHousekeepingArrivalPromotesAtTargetFloor(t *testing.T) {
	world := hkTestWorld()
	room := hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(18)
	sim.StateCode = HkStateRouteToCandidateTransit
	sim.SpawnFloor = 12
	sim.TargetRoomFloor = 12
	sim.TargetRoomColumn = 20
	sim.FloorAnchor = 12
	world.Sims = append(world.Sims, sim)
	HandleHousekeepingSimArrival(world, hkTimeAt(50), sim, 12)
	if sim.StateCode != HkStateCountdown || room.HousekeepingClaimedFlag != 1 {
		t.Fatalf("state %d claimed %d", sim.StateCode, room.HousekeepingClaimedFlag)
	}
}

func TestHousekeepingIgnoresUnrelatedArrivals(t *testing.T) {
	world := hkTestWorld()
	hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(18)
	sim.StateCode = HkStateRouteToCandidateTransit
	sim.SpawnFloor = 12
	sim.TargetRoomFloor = 12
	world.Sims = append(world.Sims, sim)
	HandleHousekeepingSimArrival(world, hkTimeAt(50), sim, 30)
	if sim.StateCode != HkStateRouteToCandidateTransit {
		t.Fatalf("state = %d", sim.StateCode)
	}
}

func TestHousekeepingRouteToTargetExpiredResets(t *testing.T) {
	world := hkTestWorld()
	hkAddHotelRoom(world, 20, 12, 0x28)
	sim := hkMakeHousekeeper(12)
	sim.StateCode = HkStateRouteToTarget
	sim.TargetRoomFloor = 12
	world.Sims = append(world.Sims, sim)
	ProcessHousekeepingSim(world, hkTimeAt(1600), sim)
	if sim.StateCode != HkStateSearch {
		t.Fatalf("state = %d", sim.StateCode)
	}
}

// Go-only: the nil-time activation path from MaybeStartHousekeepingRoomClaim
// (state-0 target) must not dereference time.
func TestActivateHousekeepingConnectionStateNilTime(t *testing.T) {
	world := hkTestWorld()
	sim := hkMakeHousekeeper(12)
	world.Sims = append(world.Sims, sim)
	ActivateObjectFamilyHousekeepingConnectionState(world, nil, sim, 15)
	if sim.SelectedFloor != 15 || sim.DestinationFloor != -1 || sim.StateCode != HkStateSearch {
		t.Fatalf("selected %d dest %d state %d", sim.SelectedFloor, sim.DestinationFloor, sim.StateCode)
	}
}
