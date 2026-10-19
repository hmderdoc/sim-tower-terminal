package sim

import (
	"reflect"
	"testing"
)

// Port of reference/tower-together/sim/carriers.test.ts plus Go-only
// transport sanity tests (marked "Go-only"; no TS counterpart).

func a2World() *WorldState {
	return CreateInitialSnapshot("a2", "A2", StartingCash, LobbyModePerfectParity).World
}

// TS: carriers.test.ts "flushCarriersEndOfDay preserves route-ring heads
// while clearing queue counts"
func TestFlushCarriersEndOfDayPreservesRingHeads(t *testing.T) {
	carrier := MakeCarrier(0, 100, 1, 10, 19, 1)
	queue := carrier.FloorQueues[0].Up
	for i := 0; i < 15; i++ {
		queue.Push("r" + string(rune('0'+i%10)) + string(rune('a'+i)))
	}
	for i := 0; i < 11; i++ {
		queue.Pop()
	}
	if queue.Head != 11 || queue.Size() != 4 {
		t.Fatalf("before flush head=%d size=%d", queue.Head, queue.Size())
	}

	FlushCarriersEndOfDay(&WorldState{Carriers: []*CarrierRecord{carrier}})

	if queue.Head != 11 || queue.Size() != 0 {
		t.Fatalf("after flush head=%d size=%d", queue.Head, queue.Size())
	}
	queue.Push("next")
	if queue.Head != 11 {
		t.Fatalf("head moved to %d", queue.Head)
	}
	if got := queue.PeekAll(); !reflect.DeepEqual(got, []string{"next"}) {
		t.Fatalf("peekAll = %v", got)
	}
}

// Go-only: constructor defaults mirror makeCarrier / makeCarrierCar.
func TestMakeCarrierDefaults(t *testing.T) {
	c := MakeCarrier(3, 50, 0, 10, 24, 12)
	if len(c.Cars) != 8 || c.AssignmentCapacity != 0x2a || len(c.FloorQueues) != 15 {
		t.Fatalf("cars=%d cap=%d queues=%d", len(c.Cars), c.AssignmentCapacity, len(c.FloorQueues))
	}
	for _, car := range c.Cars {
		if car.HomeFloor != 10 || car.DirectionFlag != 1 || len(car.ActiveRouteSlots) != ActiveSlotCapacity {
			t.Fatalf("car %+v", car)
		}
	}
	if MakeCarrier(0, 1, 1, 10, 12, 0).AssignmentCapacity != 0x15 || len(MakeCarrier(0, 1, 1, 10, 12, 0).Cars) != 1 {
		t.Fatal("standard carrier defaults")
	}
}

// Go-only: rebuildCarrierList splits a column at a vertical gap, numbers
// carriers in first-seen column order, and keeps the matched record.
func TestRebuildCarrierListRunsAndOrder(t *testing.T) {
	w := a2World()
	addShaft := func(x int, typ string, floors ...int) {
		for _, f := range floors {
			w.Overlays.Set(cellKey(x, FloorToY(f)), typ)
		}
	}
	addShaft(40, "elevator", 10, 11, 12, 15, 16)
	addShaft(20, "elevatorExpress", 10, 11, 12, 13)
	RebuildCarrierList(w)
	if len(w.Carriers) != 3 {
		t.Fatalf("carriers = %d", len(w.Carriers))
	}
	want := [][4]int{{40, 1, 10, 12}, {40, 1, 15, 16}, {20, 0, 10, 13}}
	for i, c := range w.Carriers {
		got := [4]int{c.Column, c.CarrierMode, c.BottomServedFloor, c.TopServedFloor}
		if got != want[i] || c.CarrierID != i {
			t.Fatalf("carrier %d = %v id %d, want %v", i, got, c.CarrierID, want[i])
		}
	}
	first := w.Carriers[0]
	addShaft(40, "elevator", 13) // extend the lower run; it merges with nothing above (14 missing)
	RebuildCarrierList(w)
	if w.Carriers[0] != first || w.Carriers[0].TopServedFloor != 13 {
		t.Fatal("existing carrier not preserved on extend")
	}
}

// Go-only: enqueue assigns a car, the car travels to the call, boards the
// rider, and delivers it (DispatchSimArrival is a B1 function: skip when
// still stubbed).
func TestCarrierEnqueueAssignAndTravel(t *testing.T) {
	c := MakeCarrier(0, 100, 1, 10, 20, 1)
	if !EnqueueRequestIntoRouteQueue(c, "15:100:7:0", 15, 10, 0, LobbyModePerfectParity) {
		t.Fatal("enqueue failed")
	}
	if c.SecondaryRouteStatusByFloor[5] != 1 || c.Cars[0].PendingAssignmentCount != 1 {
		t.Fatalf("secondary=%v pac=%d", c.SecondaryRouteStatusByFloor, c.Cars[0].PendingAssignmentCount)
	}
	if c.Cars[0].TargetFloor != 15 || c.Cars[0].DirectionFlag != 1 {
		t.Fatalf("target=%d dir=%d", c.Cars[0].TargetFloor, c.Cars[0].DirectionFlag)
	}
	// Duplicate enqueue is a no-op success.
	if !EnqueueRequestIntoRouteQueue(c, "15:100:7:0", 15, 10, 0, LobbyModePerfectParity) || len(c.PendingRoutes) != 1 {
		t.Fatal("dedup")
	}

	w := a2World()
	w.Carriers = []*CarrierRecord{c}
	tm := &TimeState{DayTick: 100}
	car := c.Cars[0]
	boarded := false
	for tick := 0; tick < 400 && !boarded; tick++ {
		tm.DayTick++
		AdvanceCarrierCarState(car, c, 0, tm, w.LobbyMode)
		ProcessUnitTravelQueue(w, c, car, 0, tm)
		boarded = len(c.PendingRoutes) == 1 && c.PendingRoutes[0].Boarded
	}
	if !boarded || car.CurrentFloor != 15 || car.AssignedCount != 1 {
		t.Fatalf("not boarded: floor=%d assigned=%d routes=%+v", car.CurrentFloor, car.AssignedCount, c.PendingRoutes[0])
	}
	skipOnTODO(t, func() {
		for tick := 0; tick < 400 && len(c.PendingRoutes) > 0; tick++ {
			tm.DayTick++
			ResetCarrierTickBookkeeping(c)
			AdvanceCarrierCarState(car, c, 0, tm, w.LobbyMode)
			DispatchCarrierCarArrivals(w, nil, tm, c, car)
			ProcessUnitTravelQueue(w, c, car, 0, tm)
		}
		if len(c.PendingRoutes) != 0 || car.CurrentFloor != 10 || car.AssignedCount != 0 {
			t.Fatalf("not delivered: floor=%d assigned=%d", car.CurrentFloor, car.AssignedCount)
		}
	})
}

// Go-only: cancel removes the request from the ring and the pending table.
func TestCancelRuntimeRouteRequest(t *testing.T) {
	c := MakeCarrier(0, 100, 1, 10, 20, 1)
	EnqueueRequestIntoRouteQueue(c, "a", 12, 18, 1, LobbyModePerfectParity)
	EnqueueRequestIntoRouteQueue(c, "b", 12, 19, 1, LobbyModePerfectParity)
	CancelRuntimeRouteRequest(c, "a")
	if got := c.FloorQueues[2].Up.PeekAll(); !reflect.DeepEqual(got, []string{"b"}) || len(c.PendingRoutes) != 1 {
		t.Fatalf("queue=%v routes=%d", got, len(c.PendingRoutes))
	}
	if EvictCarrierRoute(c, "zzz"); len(c.PendingRoutes) != 1 {
		t.Fatal("unknown cancel changed state")
	}
}

// Go-only: express carriers silently accept enqueues at non-stop floors.
func TestExpressEnqueueAtNonStopFloor(t *testing.T) {
	c := MakeCarrier(0, 100, 0, 10, 30, 1)
	if !EnqueueRequestIntoRouteQueue(c, "a", 12, 24, 1, LobbyModePerfectParity) || len(c.PendingRoutes) != 0 {
		t.Fatal("express non-stop enqueue should be a silent no-op")
	}
	if !IsExpressStopFloor(24, LobbyModePerfectParity) || IsExpressStopFloor(25, LobbyModePerfectParity) ||
		!IsExpressStopFloor(25, LobbyModeModern) || IsExpressStopFloor(0, LobbyModeModern) {
		t.Fatal("express stop cadence")
	}
}

// Go-only: JS int32 shift semantics for the record bits.
func TestTrBitInt32(t *testing.T) {
	if trBit(31) != -2147483648 || trBit(32) != 1 || trBit(24+7)&trBit(31) == 0 {
		t.Fatal("trBit")
	}
	if (trBit(31)|trBit(0))&^trBit(31) != 1 {
		t.Fatal("mask clear")
	}
}

// Go-only: routing over a stairs tile and an elevator.
func TestSelectBestRouteCandidate(t *testing.T) {
	w := a2World()
	// Stairs tile at column 60 linking floors 10-11 (overlay sits on the upper landing).
	w.Overlays.Set(cellKey(60, FloorToY(11)), "stairs")
	RebuildSpecialLinkRouteRecords(w)
	if !w.SpecialLinks[0].Active || w.SpecialLinks[0].EntryFloor != 10 || w.SpecialLinks[0].Flags != 1 {
		t.Fatalf("segment %+v", w.SpecialLinks[0])
	}
	if w.FloorWalkabilityFlags[10] != 2 || w.FloorWalkabilityFlags[11] != 2 {
		t.Fatal("walkability")
	}
	got := SelectBestRouteCandidate(w, 10, 11, true, 62)
	if got == nil || got.Kind != RouteCandidateSegment || got.ID != 0 || got.Cost != 2*8+StairsRouteExtraCost {
		t.Fatalf("stairs candidate %+v", got)
	}
	if SelectBestRouteCandidate(w, 10, 10, true, 0) != nil || SelectBestRouteCandidate(w, 10, 14, true, 0) != nil {
		t.Fatal("expected no route")
	}

	// An elevator 10..20 at column 100 beats nothing on 10->15.
	for f := 10; f <= 20; f++ {
		w.Overlays.Set(cellKey(100, FloorToY(f)), "elevator")
	}
	RebuildCarrierList(w)
	RebuildTransferGroupCache(w)
	got = SelectBestRouteCandidate(w, 10, 15, true, 98)
	if got == nil || got.Kind != RouteCandidateCarrier || got.ID != 0 || got.Cost != 2*8+DirectRouteBaseCost {
		t.Fatalf("carrier candidate %+v", got)
	}
	// Housekeeping never takes a standard elevator.
	if hk := SelectHousekeepingRoute(w, 10, 15, 98); hk != nil {
		t.Fatalf("housekeeping got %+v", hk)
	}
	if ChooseTransferFloorFromCarrierReachability(w, 0, 10, 15) != 15 ||
		ChooseTransferFloorFromCarrierReachability(w, 0, 10, 40) != -1 {
		t.Fatal("transfer floor")
	}
}

// Go-only: resolver result codes for the simple branches.
func TestResolveSimRouteBetweenFloorsCodes(t *testing.T) {
	w := a2World()
	sim := &SimRecord{FloorAnchor: 15, HomeColumn: 100, FamilyCode: FamilyOffice}
	if r := ResolveSimRouteBetweenFloors(w, sim, 12, 12, 1, nil, nil); r != RouteResolutionSameFloor || sim.TripCount != 1 {
		t.Fatalf("same floor r=%d trips=%d", r, sim.TripCount)
	}
	if r := ResolveSimRouteBetweenFloors(w, sim, 10, 15, 1, nil, &ResolveSimRouteOptions{IsPassengerRoute: trBoolPtr(false)}); r != RouteResolutionNoRoute || sim.TripCount != 1 {
		t.Fatalf("no route r=%d trips=%d", r, sim.TripCount)
	}
	for f := 10; f <= 20; f++ {
		w.Overlays.Set(cellKey(100, FloorToY(f)), "elevator")
	}
	RebuildCarrierList(w)
	tm := &TimeState{DayTick: 77}
	if r := ResolveSimRouteBetweenFloors(w, sim, 10, 15, 1, tm, nil); r != RouteResolutionQueued {
		t.Fatalf("queued r=%d", r)
	}
	if sim.Route.Mode != RouteModeCarrier || sim.Route.Direction != "up" || sim.QueueTick != 77 || sim.SelectedFloor != 10 {
		t.Fatalf("route %+v", sim.Route)
	}
	if !HasTransitFlag(sim.StateCode) || len(w.Carriers[0].PendingRoutes) != 1 {
		t.Fatal("transit bit / pending route")
	}
}

// Go-only: route-target byte encoding round-trips.
func TestRuntimeRouteTargetEncoding(t *testing.T) {
	for _, b := range []int{0, 0x3f, 0x40, 0x57, 0x58, 0x6f} {
		tg := DecodeEncodedRouteTargetByte(b)
		if tg == nil || EncodeRuntimeRouteTarget(*tg) != b {
			t.Fatalf("byte %#x -> %+v", b, tg)
		}
	}
	if DecodeEncodedRouteTargetByte(0xff) != nil {
		t.Fatal("0xff")
	}
	sim := &SimRecord{Route: RouteState{Mode: RouteModeCarrier, CarrierID: 2, Direction: "down"}}
	if tg := DecodeRuntimeRouteTarget(sim); tg.Kind != RouteTargetKindCarrier || tg.Slot != 2 || tg.DirectionFlag != 0 {
		t.Fatalf("%+v", tg)
	}
}
