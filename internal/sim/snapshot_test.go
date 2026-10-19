package sim

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestInitialSnapshotShape(t *testing.T) {
	snap := CreateInitialSnapshot("t", "T", StartingCash, "")
	w := snap.World
	if w.LobbyMode != LobbyModeModern || snap.Time.DayTick != NewGameDayTick || snap.Time.DaypartIndex != 6 {
		t.Fatalf("defaults %+v %+v", w.LobbyMode, snap.Time)
	}
	if len(w.SpecialLinks) != MaxSpecialLinks || len(w.SpecialLinkRecords) != MaxSpecialLinkRecords ||
		len(w.SpecialLinkRecords[0].ReachabilityMasksByFloor) != GridHeight ||
		len(w.TransferGroupEntries) != MaxTransferGroups || w.TransferGroupEntries[0].TaggedFloor != -1 ||
		len(w.MedicalServiceSlots) != 10 || w.MedicalServiceSlots[0].SourceFloor != -1 {
		t.Fatal("tables")
	}
	if w.GateFlags.UnknownC198 != 0xffffffff || w.GateFlags.VipSuiteFloor != 0xffff || w.GateFlags.MetroStationFloorIndex != -1 {
		t.Fatalf("gates %+v", w.GateFlags)
	}
	if len(w.EventState.FireLeftPos) != GridHeight || w.EventState.FireLeftPos[5] != 0xffff || w.EventState.LcgState != 1 {
		t.Fatal("event state")
	}
	if snap.Ledger.CashBalance != StartingCash || len(snap.Ledger.ExpenseLedger) != 256 || len(snap.Ledger.PopulationLedger) != 10 {
		t.Fatal("ledger")
	}
}

// A populated snapshot survives the JSON round trip byte-for-byte, with
// map order, sidecar kinds, rings and the int32 RNG intact.
func TestSnapshotJSONRoundTrip(t *testing.T) {
	snap := CreateInitialSnapshot("t", "T", StartingCash, LobbyModePerfectParity)
	w := snap.World
	setRow(w, GroundY, 90, 100, "lobby")
	w.Cells.Set("5,5", "office")
	w.Cells.Delete("95,109")
	w.Cells.Set("95,109", "lobby") // moved to the end
	addObject(w, 100, 11, FamilyOffice, 9)
	addObject(w, 50, 11, FamilyHotelSingle, 4)
	RebuildRuntimeSims(w)
	w.Sims[0].Route = RouteState{Mode: RouteModeCarrier, CarrierID: 2, Direction: "up", Source: 10}
	w.Sidecars = SidecarList{
		&CommercialVenueRecord{OwnerSubtypeIndex: 4, AvailabilityState: VenueDormant},
		&ServiceRequestEntry{OwnerSubtypeIndex: 3, FloorIndex: intPtr(9), CoverageFlag: intPtr(0)},
		&ServiceRequestEntry{OwnerSubtypeIndex: 2},
		&EntertainmentLinkRecord{FamilySelectorOrSingleLinkFlag: 0xff},
		&MedicalCenterRecord{PendingVisitorsCount: 2},
	}
	ring := NewRouteRequestRing()
	for i := 0; i < 41; i++ {
		ring.Push(cellKey(i, i))
	}
	w.Carriers = []*CarrierRecord{{Column: 7, FloorQueues: []*CarrierFloorQueue{{Up: ring, Down: NewRouteRequestRing()}},
		Cars: []*CarrierCar{{Active: true, ActiveRouteSlots: []*CarrierRouteSlot{{RouteID: "a"}}}}}}
	for i := 0; i < 5; i++ {
		SampleRng(w) // drive rngState negative-capable int32
	}
	w.CurrentPopulationBuckets[7] = 12

	b1, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	back := CloneSnapshot(snap)
	b2, _ := json.Marshal(back)
	if !bytes.Equal(b1, b2) {
		t.Fatalf("round trip differs:\n%s\n%s", b1, b2)
	}
	if back.World.Cells.Keys()[back.World.Cells.Len()-1] != "95,109" {
		t.Fatal("cell order lost")
	}
	if _, ok := back.World.Sidecars[2].(*ServiceRequestEntry); !ok || back.World.Sidecars[2].(*ServiceRequestEntry).FloorIndex != nil {
		t.Fatal("optional floorIndex must stay undefined")
	}
	if back.World.Carriers[0].FloorQueues[0].Up.PeekAll()[0] != "1,1" {
		t.Fatal("ring wrap lost")
	}
	if back.World.Sims[0].Route.Direction != "up" || back.World.RngState != w.RngState {
		t.Fatal("route / rng lost")
	}
}

func TestNormalizeSnapshotRepairs(t *testing.T) {
	snap := CreateInitialSnapshot("t", "T", StartingCash, LobbyModeModern)
	w := snap.World
	w.LobbyMode = ""
	w.SpecialLinks = nil
	w.FloorWalkabilityFlags = []int{1}
	w.GateFlags.MetroPlaced = 1
	w.GateFlags.MetroStationFloorIndex = 3
	addObject(w, 100, 9, FamilyMetroTop, 30)
	w.Cells.Set("10,10", "vipSuite")
	addObject(w, 10, FloorToY(10), 33, 10).ObjectTypeCode = 33
	w.Carriers = []*CarrierRecord{{FloorQueues: []*CarrierFloorQueue{{Up: &RouteRequestRing{Items: []string{"a", "b"}, Head: 41, Count: 99}}}}}
	NormalizeSnapshot(snap)
	if w.LobbyMode != LobbyModePerfectParity || len(w.SpecialLinks) != MaxSpecialLinks || len(w.FloorWalkabilityFlags) != GridHeight {
		t.Fatal("repairs")
	}
	if w.GateFlags.MetroPlaced != 1 || w.GateFlags.MetroStationFloorIndex != 9 {
		t.Fatalf("metro gate %+v", w.GateFlags)
	}
	if w.Cells.At("10,10") != "hotelSuite" || w.PlacedObjects.At("10,10").ObjectTypeCode != 5 || !w.PlacedObjects.At("10,10").VipFlag {
		t.Fatal("vip migration")
	}
	q := w.Carriers[0].FloorQueues[0]
	if len(q.Up.Items) != 40 || q.Up.Head != 1 || q.Up.Count != 40 || q.Down == nil {
		t.Fatalf("ring normalize %+v", q.Up)
	}
}

func TestHydrateReachesA2Stubs(t *testing.T) {
	// TowerSim.Create hydrates, which needs carriers/reachability (A2).
	expectTODOPanic(t, "carriers.ts initCarrierState", func() { Create("t", "T", "") })
}
