package sim

import "testing"

// Go-only unit checks for B1 helpers that have no TS unit test (the TS
// covers them through the fixture traces).

func TestOfficePresenceCounter(t *testing.T) {
	o := &PlacedObjectRecord{ObjectTypeCode: FamilyOffice, UnitStatus: 8}
	AdvanceOfficePresenceCounter(o)
	if o.UnitStatus != 1 {
		t.Fatalf("8 -> %d, want 1", o.UnitStatus)
	}
	AdvanceOfficePresenceCounter(o)
	if o.UnitStatus != 2 {
		t.Fatalf("1 -> %d, want 2", o.UnitStatus)
	}
	o.UnitStatus = 1
	decrementOfficePresenceCounter(o, &TimeState{DaypartIndex: 4})
	if o.UnitStatus != 8 {
		t.Fatalf("evening decrement to 0 -> %d, want 8", o.UnitStatus)
	}
	o.UnitStatus = 0x10 // vacant band: untouched
	AdvanceOfficePresenceCounter(o)
	if o.UnitStatus != 0x10 {
		t.Fatalf("vacant band changed to %#x", o.UnitStatus)
	}
}

func TestNormalizeUnitStatusEndOfDayBands(t *testing.T) {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	cases := []struct{ family, in, want int }{
		{FamilyHotelSingle, 0x05, 0x10},
		{FamilyHotelSingle, 0x18, 0x20},
		{FamilyHotelSingle, 0x20, 0x18},
		{FamilyHotelTwin, 0x28, 0x30},
		{FamilyHotelTwin, 0x30, 0x28},
		{FamilyHotelSuite, 0x38, 0x40},
		{FamilyHotelSuite, 0x40, 0x38},
		{FamilyCondo, 0x03, 0x10},
		{FamilyCondo, 0x18, UnitStatusCondoVacantEvening},
		{FamilyCondo, 0x20, UnitStatusCondoVacant},
	}
	objs := []*PlacedObjectRecord{}
	for i, c := range cases {
		o := &PlacedObjectRecord{ObjectTypeCode: c.family, UnitStatus: c.in}
		w.PlacedObjects.Set(cellKey(i*4, 100), o)
		objs = append(objs, o)
	}
	NormalizeUnitStatusEndOfDay(w)
	for i, c := range cases {
		if objs[i].UnitStatus != c.want {
			t.Errorf("case %d family %d %#x -> %#x, want %#x", i, c.family, c.in, objs[i].UnitStatus, c.want)
		}
	}
}

func TestSharedDispatchDestinationFloor(t *testing.T) {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	sim := &SimRecord{FloorAnchor: 20, DestinationFloor: -1}
	for _, c := range []struct{ state, want int }{
		{0x40, LobbyFloor}, {0x41, LobbyFloor}, {0x45, LobbyFloor},
		{0x42, LobbyFloor}, {0x60, 20}, {0x63, 20}, {0x64, -1}, {0x01, -1},
	} {
		sim.StateCode = c.state
		if got := DispatchObjectFamilyHotelRestaurantOfficeCondoRetailFastFoodStateHandler(w, sim); got != c.want {
			t.Errorf("state %#x -> %d, want %d", c.state, got, c.want)
		}
	}
	sim.DestinationFloor = 33
	sim.StateCode = 0x41
	if got := DispatchObjectFamilyHotelRestaurantOfficeCondoRetailFastFoodStateHandler(w, sim); got != 33 {
		t.Errorf("0x41 with destination -> %d", got)
	}
}

func TestIncrementVenueSeed(t *testing.T) {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	rec := &CommercialVenueRecord{PhaseASeed: 34}
	sim := &SimRecord{TripCount: 2, AccumulatedTicks: 100} // stress 50 < 80 -> +2
	IncrementVenueSeed(rec, FamilyFastFood, sim, w, &TimeState{})
	if rec.PhaseASeed != 35 { // capped at 35
		t.Fatalf("phaseA = %d, want 35", rec.PhaseASeed)
	}
	sim.AccumulatedTicks = 200 // stress 100 -> +1 on the weekend slot
	IncrementVenueSeed(rec, FamilyFastFood, sim, w, &TimeState{WeekendFlag: 1})
	if rec.PhaseBSeed != 1 {
		t.Fatalf("phaseB = %d, want 1", rec.PhaseBSeed)
	}
}

func TestTryAcquireAndReleaseOfficeVenueSlot(t *testing.T) {
	venue := &CommercialVenueRecord{CurrentPopulation: 39}
	sim := &SimRecord{FamilyCode: FamilyOffice}
	tm := &TimeState{DayTick: 500}
	if r := TryAcquireOfficeVenueSlot(venue, sim, tm, FamilyFastFood); r != VenueSlotAcquired {
		t.Fatalf("acquire = %d", r)
	}
	if venue.CurrentPopulation != 40 || venue.AcquireCount != 1 || sim.LastDemandTick != 500 {
		t.Fatalf("pop %d acquire %d tick %d", venue.CurrentPopulation, venue.AcquireCount, sim.LastDemandTick)
	}
	if r := TryAcquireOfficeVenueSlot(venue, sim, tm, FamilyFastFood); r != VenueSlotFull {
		t.Fatalf("full acquire = %d", r)
	}
	if ReleaseOfficeVenueSlot(venue, sim, &TimeState{DayTick: 559}, false) {
		t.Fatal("released before the 60-tick dwell")
	}
	if !ReleaseOfficeVenueSlot(venue, sim, &TimeState{DayTick: 560}, false) || venue.CurrentPopulation != 39 {
		t.Fatalf("release failed, pop %d", venue.CurrentPopulation)
	}
	venue.AvailabilityState = VenueClosed
	if r := TryAcquireOfficeVenueSlot(venue, sim, tm, FamilyFastFood); r != VenueSlotUnavailable {
		t.Fatalf("closed acquire = %d", r)
	}
}
