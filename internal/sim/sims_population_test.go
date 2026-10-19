package sim

import "testing"

func addObject(w *WorldState, x, floor, family, width int) *PlacedObjectRecord {
	o := &PlacedObjectRecord{LeftTileIndex: x, RightTileIndex: x + width - 1, ObjectTypeCode: family, EvalLevel: 0xff, EvalScore: -1, LinkedRecordIndex: -1}
	w.PlacedObjects.Set(cellKey(x, FloorToY(floor)), o)
	return o
}

func TestRebuildRuntimeSimsOrderAndStates(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	// Insertion order deliberately scrambled; rebuild sorts above-grade floors
	// ascending, then below-grade, then x.
	addObject(w, 120, 12, FamilyOffice, 9)
	addObject(w, 100, 12, FamilyCondo, 16)
	addObject(w, 100, 11, FamilyHotelSingle, 4)
	addObject(w, 100, 9, FamilyRestaurant, 24) // underground
	addObject(w, 200, 11, FamilyParking, 4)    // population 0: no sims
	RebuildRuntimeSims(w)

	if len(w.Sims) != 2+3+6+48 {
		t.Fatalf("sim count %d", len(w.Sims))
	}
	s0 := w.Sims[0]
	if s0.FamilyCode != FamilyHotelSingle || s0.StateCode != StateHotelParked || w.Sims[1].StateCode != StateMorningGate {
		t.Fatalf("hotel sims first: %+v", s0)
	}
	if w.Sims[2].FamilyCode != FamilyCondo || w.Sims[2].FacilitySlot != 0 {
		t.Fatalf("condo next: %+v", w.Sims[2])
	}
	if w.Sims[5].FamilyCode != FamilyOffice || w.Sims[5].FacilitySlot != 1 || w.Sims[5].BaseOffset != 0 {
		t.Fatalf("office slot: %+v", w.Sims[5])
	}
	if w.Sims[11].FamilyCode != FamilyRestaurant || w.Sims[11].FloorAnchor != 9 {
		t.Fatalf("underground last: %+v", w.Sims[11])
	}

	// Runtime state survives a rebuild (keyed by simKey).
	w.Sims[5].TripCount = 7
	w.Sims[5].StateCode = StateAtWork
	RebuildRuntimeSims(w)
	if w.Sims[5].TripCount != 7 || w.Sims[5].StateCode != StateAtWork {
		t.Fatalf("prior state lost: %+v", w.Sims[5])
	}
}

func TestRebuildRuntimeSimsMedicalPlaceholder(t *testing.T) {
	// A medical center reserves 6 slots, freed when its queue entry is
	// finalized after 10 more placements; a later office reclaims them.
	w := newEmptyWorld("t", "t", LobbyModeModern)
	addObject(w, 10, 11, FamilyMedical, 26)
	for i := 0; i < 11; i++ {
		addObject(w, 40+i*10, 11, FamilyOffice, 9)
	}
	RebuildRuntimeSims(w)
	if len(w.Sims) != 66 {
		t.Fatalf("sims %d", len(w.Sims))
	}
	// The 11th office (x=140) is placed after the medical block is zeroed and
	// lands in slots 0..5, i.e. FIRST in the sim table.
	if w.Sims[0].HomeColumn != 140 || w.Sims[6].HomeColumn != 40 {
		t.Fatalf("medical reclaim order: first=%d seventh=%d", w.Sims[0].HomeColumn, w.Sims[6].HomeColumn)
	}
}

func TestCleanupSimsForRemovedTile(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	addObject(w, 100, 11, FamilyOffice, 9)
	addObject(w, 120, 11, FamilyOffice, 9)
	RebuildRuntimeSims(w)
	victim := SimKey(w.Sims[0])
	car := &CarrierCar{Active: true, AssignedCount: 1, PendingRouteIDs: []string{victim, "other"},
		ActiveRouteSlots: []*CarrierRouteSlot{{RouteID: victim, Active: true, Boarded: true}}}
	c := testCarrier(50, 1, 10, 12, car)
	c.PendingRoutes = []*CarrierPendingRoute{{SimID: victim}, {SimID: "other"}}
	w.Carriers = []*CarrierRecord{c}
	CleanupSimsForRemovedTile(w, 100, FloorToY(11))
	if len(w.Sims) != 6 || w.Sims[0].HomeColumn != 120 {
		t.Fatalf("sims left %d", len(w.Sims))
	}
	if len(c.PendingRoutes) != 1 || car.AssignedCount != 0 || car.ActiveRouteSlots[0].Active ||
		car.ActiveRouteSlots[0].SourceFloor != 0xff || len(car.PendingRouteIDs) != 1 {
		t.Fatalf("carrier not cleaned: %+v %+v", c.PendingRoutes, car)
	}
}

func TestFindObjectForSimCaches(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	o := addObject(w, 100, 11, FamilyOffice, 9)
	RebuildRuntimeSims(w)
	s := w.Sims[0]
	if FindObjectForSim(w, s) != o {
		t.Fatal("owner lookup")
	}
	w.PlacedObjects.Delete(cellKey(100, FloorToY(11)))
	if FindObjectForSim(w, s) != o {
		t.Fatal("TS WeakMap cache semantics: cached owner must be returned")
	}
}

func TestResetSimRuntimeState(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	addObject(w, 100, 11, FamilyOffice, 9)
	addObject(w, 120, 11, FamilyHotelSingle, 4)
	addObject(w, 140, 11, FamilyHousekeeping, 15)
	RebuildRuntimeSims(w)
	for _, s := range w.Sims {
		s.StateCode = 0x41
		s.TripCount = 3
		s.DestinationFloor = 20
	}
	ResetSimRuntimeState(w)
	for _, s := range w.Sims {
		switch s.FamilyCode {
		case FamilyOffice:
			if s.StateCode != StateMorningGate || s.DestinationFloor != -1 || s.TripCount != 3 {
				t.Fatalf("office %+v", s)
			}
		case FamilyHotelSingle:
			if s.BaseOffset == 0 && s.StateCode != StateHotelParked {
				t.Fatalf("hotel primary %+v", s)
			}
			if s.BaseOffset == 1 && (s.StateCode != 0x41 || s.DestinationFloor != 20) {
				t.Fatalf("hotel secondary untouched %+v", s)
			}
		case FamilyHousekeeping:
			if s.StateCode != HkStateSearch || s.TripCount != 0 || s.TargetRoomFloor != -1 {
				t.Fatalf("housekeeping %+v", s)
			}
		}
	}
}

func TestStressAccessors(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	w.LobbyHeight = 3
	s := makeSim(LobbyFloor, 0, 0, FamilyOffice, 6, 0)
	s.LastDemandTick = 100
	AccumulateElapsedDelayIntoCurrentSim(w, &TimeState{DayTick: 180}, s, LobbyFloor)
	if s.ElapsedTicks != 30 || s.LastDemandTick != -1 {
		t.Fatalf("accumulate %+v", s)
	}
	AddDelayToCurrentSim(s, 500)
	if s.ElapsedTicks != 300 {
		t.Fatalf("clamp %d", s.ElapsedTicks)
	}
	AdvanceSimTripCounters(s)
	if s.TripCount != 1 || s.AccumulatedTicks != 300 || s.ElapsedTicks != 0 {
		t.Fatalf("advance %+v", s)
	}
	hk := makeSim(LobbyFloor, 0, 0, FamilyHousekeeping, 6, 0)
	AddDelayToCurrentSim(hk, 50)
	AdvanceSimTripCounters(hk)
	if hk.ElapsedTicks != 0 || hk.TripCount != 0 {
		t.Fatal("housekeeping must be excluded")
	}
}

func TestStateBits(t *testing.T) {
	s := &SimRecord{StateCode: 0x03}
	SetSimInTransit(s, true)
	SetSimWaiting(s, true)
	if s.StateCode != 0x63 || !IsSimInTransit(s.StateCode) || !IsSimWaiting(s.StateCode) || SimBaseState(s.StateCode) != 3 {
		t.Fatalf("bits %#x", s.StateCode)
	}
	SetSimBaseState(s, 5)
	if s.StateCode != 0x65 {
		t.Fatalf("base %#x", s.StateCode)
	}
	ClearSimRouteBits(s)
	if s.StateCode != 0x05 {
		t.Fatalf("clear %#x", s.StateCode)
	}
	SetSimState(s, 0x1ff)
	if s.StateCode != 0xff {
		t.Fatalf("mask %#x", s.StateCode)
	}
}

func TestSelectors(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	addObject(w, 100, 11, FamilyOffice, 9)
	RebuildRuntimeSims(w)
	s := w.Sims[3]
	if ComputeObjectOccupantRuntimeIndex(w, s) != 3 {
		t.Fatalf("occupant index %d", ComputeObjectOccupantRuntimeIndex(w, s))
	}
	w.Sidecars = SidecarList{&EntertainmentLinkRecord{OwnerSubtypeIndex: 7}, &EntertainmentLinkRecord{OwnerSubtypeIndex: 7}}
	DispatchEntertainmentGuestSubstate(w, &SimRecord{StateCode: 0x41, HomeColumn: 7})
	if w.Sidecars[0].(*EntertainmentLinkRecord).AttendanceCounter != 1 || w.Sidecars[1].(*EntertainmentLinkRecord).AttendanceCounter != 0 {
		t.Fatal("attendance must hit only the first matching link")
	}
	if ResolveFamilyParkingSelectorValue(w, &SimRecord{StateCode: 0x60}) != 0x6d {
		t.Fatal("parking selector")
	}
}
