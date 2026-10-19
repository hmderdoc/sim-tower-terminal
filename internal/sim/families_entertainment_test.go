package sim

import (
	"reflect"
	"testing"
)

// B2 unit tests for families/entertainment.ts, families/recycling.ts and
// families/parking.ts gates.

func TestClassifyPathBucketIndex(t *testing.T) {
	cases := map[int]int{5: 0, 14: 0, 15: -1, 19: -1, 20: 1, 29: 1, 30: -1, 95: 6, 104: 6, 110: -1, 4: 0, -10: -1}
	for floor, want := range cases {
		if got := classifyPathBucketIndex(floor); got != want {
			t.Errorf("classifyPathBucketIndex(%d) = %d, want %d", floor, got, want)
		}
	}
}

func TestRecomputeCommercialVenueBuckets(t *testing.T) {
	w := b2World(t)
	add := func(x, floor, family int) int {
		idx := len(w.Sidecars)
		w.Sidecars = append(w.Sidecars, &CommercialVenueRecord{OwnerSubtypeIndex: 0})
		b2PlaceObject(w, x, floor, &PlacedObjectRecord{ObjectTypeCode: family, LinkedRecordIndex: idx})
		return idx
	}
	r1 := add(10, 12, FamilyRetail)
	ff := add(20, 12, FamilyFastFood)
	rest := add(30, 22, FamilyRestaurant)
	r2 := add(40, 22, FamilyRetail)
	dead := add(50, 12, FamilyRetail)
	w.Sidecars[dead].(*CommercialVenueRecord).OwnerSubtypeIndex = 0xff
	add(60, 17, FamilyRetail) // floor 17: offset 12 % 15 > 9 -> not bucketed

	RecomputeCommercialVenueBuckets(w)
	b := w.CommercialVenueBuckets
	if !reflect.DeepEqual(b.Retail[0], []int{r1}) || !reflect.DeepEqual(b.Retail[1], []int{r2}) ||
		!reflect.DeepEqual(b.FastFood[0], []int{ff}) || len(b.Restaurant[1]) != 0 {
		t.Fatalf("buckets after rebuild: %+v", b)
	}
	RecomputeRestaurantVenueBuckets(w)
	if !reflect.DeepEqual(w.CommercialVenueBuckets.Restaurant[1], []int{rest}) {
		t.Fatalf("restaurant bucket: %+v", w.CommercialVenueBuckets.Restaurant)
	}
	RecomputeRestaurantVenueBuckets(w)
	if len(w.CommercialVenueBuckets.Restaurant[1]) != 1 {
		t.Fatal("restaurant rebuild must clear before append")
	}

	// Picker: draws RNG even when the pick is dormant, then rejects it.
	w.Sidecars[r1].(*CommercialVenueRecord).AvailabilityState = VenueDormant
	calls := w.RngCallCount
	if got := selectRandomCommercialVenueRecordFromBucket(w, 0, 0); got != -1 || w.RngCallCount != calls+1 {
		t.Fatalf("dormant pick = %d (rng %d)", got, w.RngCallCount-calls)
	}
	// Empty zone row falls back to row 0.
	w.Sidecars[r1].(*CommercialVenueRecord).AvailabilityState = VenueAvailable
	if got := selectRandomCommercialVenueRecordFromBucket(w, 0, 5); got != r1 {
		t.Fatalf("row fallback pick = %d", got)
	}
	if got := selectRandomCommercialVenueRecordFromBucket(w, 3, 0); got != -1 {
		t.Fatal("unknown bucket -> -1")
	}
}

func TestAcquireCommercialVenueSlot(t *testing.T) {
	w := b2World(t)
	venue := &CommercialVenueRecord{OwnerSubtypeIndex: FamilyFastFood, AvailabilityState: VenueAvailable}
	w.Sidecars = append(w.Sidecars, venue)
	tm := &TimeState{DayTick: 777}
	sim := b2Sim(20, 57, FamilyCinemaLower, entStateServiceAcquire)
	if acquireCommercialVenueSlot(w, tm, sim, -1) != 3 || sim.LastDemandTick != 777 {
		t.Fatal("lobby fallback")
	}
	if acquireCommercialVenueSlot(w, tm, sim, 0) != 3 || venue.CurrentPopulation != 1 || venue.AcquireCount != 1 {
		t.Fatalf("acquire: %+v", venue)
	}
	venue.CurrentPopulation = 40
	if acquireCommercialVenueSlot(w, tm, sim, 0) != 2 {
		t.Fatal("full")
	}
	venue.AvailabilityState = VenueClosed
	trips := sim.TripCount
	if acquireCommercialVenueSlot(w, tm, sim, 0) != -1 || sim.TripCount != trips+1 {
		t.Fatal("closed -> -1 with a trip-counter advance")
	}
}

func TestGateEntertainmentGuestState(t *testing.T) {
	w := b2World(t)
	ledger := CreateLedgerState(0)
	sim := b2Sim(20, 50, FamilyCinema, entStatePhaseConsume)
	calls := w.RngCallCount
	GateEntertainmentGuestState(w, ledger, &TimeState{DaypartIndex: 4, DayTick: 1700}, sim)
	if sim.StateCode != entStateParked || w.RngCallCount != calls {
		t.Fatalf("daypart > 3 parks without RNG: %#x", sim.StateCode)
	}
	other := b2Sim(20, 50, FamilyOffice, entStatePhaseConsume)
	GateEntertainmentGuestState(w, ledger, &TimeState{DaypartIndex: 4}, other)
	if other.StateCode != entStatePhaseConsume {
		t.Fatal("non-entertainment family ignored")
	}
	// Phase consumption with an exhausted budget returns before routing.
	link := &EntertainmentLinkRecord{OwnerSubtypeIndex: 50, FamilySelectorOrSingleLinkFlag: 3}
	w.Sidecars = append(w.Sidecars, link)
	sim.StateCode = entStatePhaseConsume
	DispatchEntertainmentGuestState(w, &TimeState{}, sim)
	if sim.StateCode != entStatePhaseConsume {
		t.Fatal("zero budget: stay idle")
	}
	link.UpperBudget = 1
	skipOnTODO(t, func() {
		DispatchEntertainmentGuestState(w, &TimeState{}, sim)
		if sim.StateCode == entStatePhaseConsume {
			// Route -1 on the fresh state: budget refunded, origin cleared.
			if link.UpperBudget != 1 || sim.OriginFloor != 0 {
				t.Fatalf("refund path: budget=%d origin=%d", link.UpperBudget, sim.OriginFloor)
			}
		} else if link.UpperBudget != 0 {
			t.Fatalf("routed: budget must stay consumed, got %d", link.UpperBudget)
		}
	})
}

func TestRecyclingLowerGate(t *testing.T) {
	w := b2World(t)
	ledger := CreateLedgerState(0)
	sim := b2Sim(5, 10, FamilyRecyclingCenterLower, stateRecyclingParked)
	GateObjectFamilyRecyclingCenterLowerStateHandler(w, ledger, &TimeState{DayTick: 0x8fc}, sim)
	if sim.StateCode != stateRecyclingParked {
		t.Fatal("parked before 0x8fd")
	}
	GateObjectFamilyRecyclingCenterLowerStateHandler(w, ledger, &TimeState{DayTick: 0x8fd, DaypartIndex: 5}, sim)
	if sim.StateCode != stateRecyclingActive {
		t.Fatal("reset to active at 0x8fd")
	}
	calls := w.RngCallCount
	GateObjectFamilyRecyclingCenterLowerStateHandler(w, ledger, &TimeState{DayTick: 0xf0}, sim)
	if w.RngCallCount != calls {
		t.Fatal("dayTick < 0xf1: no RNG")
	}
	// Gate passes (RNG%36 == 0) -> the inner 0x530d gate always draws again.
	w.RngState = b2SeedFor(t, 36, 0)
	todoPanic(func() {
		GateObjectFamilyRecyclingCenterLowerStateHandler(w, ledger, &TimeState{DayTick: 0xf1}, sim)
	})
	if w.RngCallCount < calls+2 {
		t.Fatalf("gate + inner gate draws expected, got %d", w.RngCallCount-calls)
	}
	sim.StateCode = stateRecyclingActive
	// Dispatch at daypart > 3 parks.
	DispatchObjectFamilyRecyclingCenterLowerStateHandler(w, &TimeState{DaypartIndex: 4}, sim)
	if sim.StateCode != stateRecyclingParked {
		t.Fatal("daypart > 3 parks")
	}
}

func TestParkingFamilyGate(t *testing.T) {
	w := b2World(t)
	sim := b2Sim(EvalZoneFloor, 100, 0x24, StateMorningGate)
	GateObjectFamilyParkingStateHandler(w, &TimeState{WeekendFlag: 0, DayTick: 300}, sim)
	if sim.StateCode != StateMorningGate {
		t.Fatal("weekday no-op")
	}
	GateObjectFamilyParkingStateHandler(w, &TimeState{WeekendFlag: 1, DaypartIndex: 2}, sim)
	if sim.StateCode != StateParked {
		t.Fatal("missed window parks")
	}
	sim.StateCode = StateEvalOutbound
	sim.Route = RouteState{Mode: RouteModeCarrier}
	GateObjectFamilyParkingStateHandler(w, &TimeState{}, sim)
	if sim.StateCode != StateEvalOutbound {
		t.Fatal("carrier-queued transit sim: no-op")
	}
}
