package sim

import "testing"

// B2 unit tests for sims/commercial.ts and the families/retail.ts +
// families/restaurant.ts gates.

func b2CommercialWorld(t *testing.T, family int) (*WorldState, *SimRecord, *PlacedObjectRecord, *CommercialVenueRecord) {
	w := b2World(t)
	venue := &CommercialVenueRecord{OwnerSubtypeIndex: 0, AvailabilityState: VenueAvailable, RemainingCapacity: 5}
	w.Sidecars = append(w.Sidecars, venue)
	obj := &PlacedObjectRecord{ObjectTypeCode: family, LinkedRecordIndex: 0}
	b2PlaceObject(w, 30, 12, obj)
	sim := b2Sim(12, 30, family, StateNightA)
	w.Sims = append(w.Sims, sim)
	return w, sim, obj, venue
}

func TestCommercialNightToMorningGate(t *testing.T) {
	w, sim, _, _ := b2CommercialWorld(t, FamilyRetail)
	ledger := CreateLedgerState(0)
	ProcessCommercialSim(w, ledger, &TimeState{DayTick: 2300}, sim)
	if sim.StateCode != StateNightA {
		t.Fatal("dayTick 2300 stays parked")
	}
	ProcessCommercialSim(w, ledger, &TimeState{DayTick: 2301}, sim)
	if sim.StateCode != StateMorningGate {
		t.Fatal("dayTick > 2300 -> morning gate")
	}
}

func TestRetailDormantGateSkipsRng(t *testing.T) {
	w, sim, obj, venue := b2CommercialWorld(t, FamilyRetail)
	venue.AvailabilityState = VenueDormant
	obj.OccupiedFlag = 0
	sim.StateCode = StateMorningGate
	calls := w.RngCallCount
	GateObjectFamilyRetailStateHandler(w, CreateLedgerState(0), &TimeState{DayTick: 500, DaypartIndex: 1}, sim)
	if w.RngCallCount != calls || sim.StateCode != StateMorningGate {
		t.Fatal("dormant unoccupied retail returns before RNG")
	}
}

func TestRestaurantGateWindows(t *testing.T) {
	w, sim, _, venue := b2CommercialWorld(t, FamilyRestaurant)
	sim.StateCode = StateMorningGate
	ledger := CreateLedgerState(0)
	calls := w.RngCallCount
	GateObjectFamilyRestaurantFastFoodStateHandler(w, ledger, &TimeState{DaypartIndex: 2, DayTick: 900}, sim)
	if w.RngCallCount != calls {
		t.Fatal("restaurant daypart < 4 returns without RNG")
	}
	GateObjectFamilyRestaurantFastFoodStateHandler(w, ledger, &TimeState{DaypartIndex: 5, DayTick: 2200}, sim)
	if w.RngCallCount != calls {
		t.Fatal("daypart 5 after 2199 returns")
	}
	// Daypart 5 before 2199 dispatches without RNG; CLOSED silent-parks.
	venue.AvailabilityState = VenueClosed
	GateObjectFamilyRestaurantFastFoodStateHandler(w, ledger, &TimeState{DaypartIndex: 5, DayTick: 2100}, sim)
	if sim.StateCode != StateParked || w.RngCallCount != calls || venue.RemainingCapacity != 5 {
		t.Fatalf("closed restaurant silent-park: state=%#x cap=%d", sim.StateCode, venue.RemainingCapacity)
	}
}

func TestCommercialMorningGateConsumesCapacity(t *testing.T) {
	w, sim, _, venue := b2CommercialWorld(t, FamilyFastFood)
	sim.StateCode = StateMorningGate
	w.RngState = b2SeedFor(t, 6, 0)
	// try_consume runs before the route call, so the bookkeeping holds
	// whether or not queue/resolve.ts is ported yet.
	todoPanic(func() {
		ProcessCommercialSim(w, CreateLedgerState(0), &TimeState{DaypartIndex: 4, DayTick: 1700}, sim)
	})
	if venue.RemainingCapacity != 4 || venue.TodayVisitCount != 1 || venue.AcquireCount != 1 ||
		venue.VisitCount != 1 || venue.LastAcquireTick != 1700 {
		t.Fatalf("try_consume bookkeeping: %+v", venue)
	}
}

func TestHandleCommercialSimArrivalFallback(t *testing.T) {
	w, sim, _, _ := b2CommercialWorld(t, FamilyRetail)
	sim.StateCode = StateMorningTransit
	sim.Route = RouteState{Mode: RouteModeCarrier}
	HandleCommercialSimArrival(w, sim, 40, &TimeState{})
	if sim.StateCode != StateNightB || sim.Route.Mode != RouteModeIdle {
		t.Fatal("wrong-floor arrival parks to NIGHT_B")
	}
}
