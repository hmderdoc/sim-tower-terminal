package sim

// Unit tests for events.go. The TS has no dedicated events test file (the
// sim.test.ts named in AGENTS.md does not exist upstream); fire/bomb parity
// is covered by the fire fixtures in trace_test.go. These pin the gates,
// RNG draw counts and prompt plumbing.

import (
	"strings"
	"testing"
)

func eventsWorld() *WorldState {
	return newEmptyWorld("t", "T", LobbyModePerfectParity)
}

// buildFloorsCells fills floor-support cells for floors [fromFloor, toFloor]
// over columns [left, right].
func buildFloorsCells(w *WorldState, fromFloor, toFloor, left, right int) {
	for f := fromFloor; f <= toFloor; f++ {
		setRow(w, FloorToY(f), left, right, "floor")
	}
}

// rngPeek returns the next n SampleRng outputs without advancing w.
func rngPeek(w *WorldState, n int) []int {
	c := &WorldState{RngState: w.RngState}
	out := make([]int, n)
	for i := range out {
		out[i] = SampleRng(c)
	}
	return out
}

func TestJsToLocaleString(t *testing.T) {
	for v, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 200000: "200,000", 1000000: "1,000,000", -12345: "-12,345"} {
		if got := jsToLocaleString(v); got != want {
			t.Errorf("%d: got %q want %q", v, got, want)
		}
	}
}

func TestFloorBoundsAndLiveFloorScan(t *testing.T) {
	w := eventsWorld()
	if _, _, ok := floorTileBoundsForFloor(w, 21); ok {
		t.Fatal("empty floor has bounds")
	}
	buildFloorsCells(w, 21, 23, 100, 140)
	l, r, ok := floorTileBoundsForFloor(w, 22)
	if !ok || l != 100 || r != 140 {
		t.Fatalf("bounds %d %d %v", l, r, ok)
	}
	// No live floor at/above 30: no RNG draw.
	if got := selectRandomLiveFloor(w, 30); got != -1 || w.RngCallCount != 0 {
		t.Fatalf("got %d draws %d", got, w.RngCallCount)
	}
	// From 11: run 21..23, range = 23-11+1 = 13.
	want := 11 + rngPeek(w, 1)[0]%13
	if got := selectRandomLiveFloor(w, 11); got != want || w.RngCallCount != 1 {
		t.Fatalf("got %d want %d draws %d", got, want, w.RngCallCount)
	}
}

func bombReadyWorld() (*WorldState, *TimeState) {
	w := eventsWorld()
	w.StarCount = 3
	// Floors 11..13 contiguous; with lobbyHeight 1 the scan starts at 11.
	buildFloorsCells(w, 11, 13, 100, 140)
	w.PlacedObjects.Set(cellKey(100, FloorToY(11)), &PlacedObjectRecord{
		LeftTileIndex: 100, RightTileIndex: 100, ObjectTypeCode: 0x15, LinkedRecordIndex: -1,
	})
	tm := CreateTimeState()
	tm.DayTick = 0xf0
	tm.DayCounter = 59
	return w, tm
}

func TestTryTriggerBombEvent(t *testing.T) {
	w, tm := bombReadyWorld()
	draws := rngPeek(w, 2)
	wantFloor := 11 + draws[0]%3
	wantTile := 100 + draws[1]%(140-100-4+1)
	CheckDailyEvents(w, CreateLedgerState(0), tm)
	es := w.EventState
	if es.GameStateFlags&1 == 0 || es.BombFloor != wantFloor || es.BombTile != wantTile {
		t.Fatalf("flags %x floor %d/%d tile %d/%d", es.GameStateFlags, es.BombFloor, wantFloor, es.BombTile, wantTile)
	}
	if es.BombDeadline != 1200 || es.BombSearchLowerBound != wantFloor-1 || es.BombSearchUpperBound != wantFloor {
		t.Fatalf("deadline %d bounds %d %d", es.BombDeadline, es.BombSearchLowerBound, es.BombSearchUpperBound)
	}
	if es.BombSearchCurrentFloor != -1 || es.BombSearchScanTile != -1 {
		t.Fatal("no security: search cursor should be unset")
	}
	if len(w.PendingPrompts) != 1 {
		t.Fatalf("prompts %+v", w.PendingPrompts)
	}
	p := w.PendingPrompts[0]
	if p.PromptID != "bomb_59" || p.PromptKind != PromptBombRansom || p.Cost == nil || *p.Cost != 300_000 ||
		p.Message != "A bomb threat has been received! Pay $300,000 ransom to avoid detonation?" {
		t.Fatalf("prompt %+v", p)
	}
	// Second trigger is blocked by the active flag (no RNG draw).
	before := w.RngCallCount
	TryTriggerBombEvent(w, nil, tm)
	if w.RngCallCount != before || len(w.PendingPrompts) != 1 {
		t.Fatal("re-trigger while active")
	}
}

func TestBombGuards(t *testing.T) {
	w, tm := bombReadyWorld()
	w.StarCount = 5
	TryTriggerBombEvent(w, nil, tm)
	tm2 := *tm
	tm2.DayTick = 0x4b1
	w.StarCount = 3
	TryTriggerBombEvent(w, nil, &tm2)
	if w.RngCallCount != 0 || w.EventState.GameStateFlags != 0 {
		t.Fatal("guards should block without drawing")
	}
}

func TestBombDetonationAndCleanup(t *testing.T) {
	w, tm := bombReadyWorld()
	ledger := CreateLedgerState(0)
	TryTriggerBombEvent(w, ledger, tm)
	es := &w.EventState
	floor, tile := es.BombFloor, es.BombTile

	tm.DayTick = 1199
	TickBombEvent(w, ledger, tm)
	if es.GameStateFlags&1 == 0 {
		t.Fatal("detonated early")
	}
	tm.DayTick = 1200
	TickBombEvent(w, ledger, tm)
	if es.GameStateFlags&1 != 0 || es.GameStateFlags&0x40 == 0 || es.BombDeadline != 1250 {
		t.Fatalf("flags %x deadline %d", es.GameStateFlags, es.BombDeadline)
	}
	if w.Cells.Has(cellKey(tile, FloorToY(floor))) {
		t.Fatal("blast left the bomb cell")
	}
	tm.DayTick = 1250
	TickBombEvent(w, ledger, tm)
	if es.GameStateFlags&0x60 != 0 || es.BombSearchLowerBound != -1 || tm.DayTick != 1500 || tm.DaypartIndex != 3 {
		t.Fatalf("cleanup: flags %x tick %d daypart %d", es.GameStateFlags, tm.DayTick, tm.DaypartIndex)
	}
}

func TestBombRansomPrompt(t *testing.T) {
	w, tm := bombReadyWorld()
	ledger := CreateLedgerState(1_000_000)
	TryTriggerBombEvent(w, ledger, tm)
	if !HandlePromptResponse(w, ledger, tm, "bomb_59", false) || w.EventState.GameStateFlags&1 == 0 {
		t.Fatal("refusal must leave the bomb armed")
	}
	if !HandlePromptResponse(w, ledger, tm, "bomb_59", true) {
		t.Fatal("not handled")
	}
	// PORT-NOTE (TS behavior): bombCleanup clears only 0x60, so the active
	// search bit 1 stays set after a paid ransom.
	if ledger.CashBalance != 700_000 || tm.DayTick != 1500 {
		t.Fatalf("cash %d tick %d", ledger.CashBalance, tm.DayTick)
	}
	last := w.PendingNotifications[len(w.PendingNotifications)-1]
	if last.Message != "Ransom paid. The bomb threat has been resolved." {
		t.Fatalf("notification %+v", last)
	}
	if HandlePromptResponse(w, ledger, tm, "unknown_1", true) {
		t.Fatal("unknown prompt should return false")
	}
}

func fireReadyWorld() (*WorldState, *TimeState) {
	w := eventsWorld()
	w.StarCount = 3
	buildFloorsCells(w, 11, 12, 80, 139)
	w.PlacedObjects.Set(cellKey(80, FloorToY(11)), &PlacedObjectRecord{
		LeftTileIndex: 80, RightTileIndex: 80, ObjectTypeCode: 0x15, LinkedRecordIndex: -1,
	})
	tm := CreateTimeState()
	tm.DayTick = 0xf0
	tm.DayCounter = 83
	return w, tm
}

func TestFireTriggerPromptAndHelicopter(t *testing.T) {
	w, tm := fireReadyWorld()
	ledger := CreateLedgerState(600_000)
	wantFloor := 11 + rngPeek(w, 1)[0]%2
	CheckDailyEvents(w, ledger, tm)
	es := &w.EventState
	if es.GameStateFlags&8 == 0 || es.FireFloor != wantFloor || es.FireTile != 139-0x20 || es.FireStartTick != 0xf0 {
		t.Fatalf("fire state %+v", es)
	}
	if es.FireLeftPos[wantFloor] != es.FireTile || es.FireRightPos[wantFloor] != es.FireTile || es.RescueCountdown != 0 {
		t.Fatal("fire seed")
	}
	tm.DayTick = 0xf2
	TickFireEvent(w, ledger, tm)
	if len(w.PendingPrompts) != 1 || w.PendingPrompts[0].PromptID != "fire_83" ||
		!strings.Contains(w.PendingPrompts[0].Message, "$500,000?") {
		t.Fatalf("prompts %+v", w.PendingPrompts)
	}
	if !HandlePromptResponse(w, ledger, tm, "fire_83", true) {
		t.Fatal("not handled")
	}
	_, right, _ := floorTileBoundsForFloor(w, es.FireFloor)
	if ledger.CashBalance != 100_000 || es.HelicopterExtinguishPos != right-12 {
		t.Fatalf("cash %d heli %d", ledger.CashBalance, es.HelicopterExtinguishPos)
	}
	tm.DayTick = 2000
	TickFireEvent(w, ledger, tm)
	if es.GameStateFlags&8 != 0 || es.FireLeftPos[wantFloor] != 0xffff || len(es.FireRescueHelpers) != 0 || tm.DayTick != 2000 {
		t.Fatal("fire should resolve at 2000 without fast-forward")
	}
	if !HandlePromptResponse(w, ledger, tm, "fire_83", false) {
		t.Fatal("resolved fire prompt still handled")
	}
}

func TestFireDeclineSpawnsHelpers(t *testing.T) {
	w, tm := fireReadyWorld()
	w.PlacedObjects.Set(cellKey(90, FloorToY(12)), &PlacedObjectRecord{
		LeftTileIndex: 90, RightTileIndex: 105, ObjectTypeCode: 0x0e, LinkedRecordIndex: -1,
	})
	TryTriggerFireEvent(w, nil, tm)
	if w.EventState.RescueCountdown != 80 {
		t.Fatalf("security countdown %d", w.EventState.RescueCountdown)
	}
	HandlePromptResponse(w, CreateLedgerState(0), tm, "fire_83", false)
	h := w.EventState.FireRescueHelpers
	if len(h) != 1 || h[0].Floor != 12 || h[0].Column != 139+11 || h[0].Status != 0 {
		t.Fatalf("helpers %+v", h)
	}
}

func TestFireGuardEvalSim(t *testing.T) {
	w, tm := fireReadyWorld()
	w.GateFlags.EvalSimIndex = 3
	TryTriggerFireEvent(w, nil, tm)
	if w.EventState.GameStateFlags != 0 || w.RngCallCount != 0 {
		t.Fatal("cathedral guest dispatch should block fire")
	}
}

func TestTickVipSpecialVisitor(t *testing.T) {
	w := eventsWorld()
	w.GateFlags.MetroStationFloorIndex = 9
	for i, k := range []string{"100,110", "100,111", "100,112"} {
		w.PlacedObjects.Set(k, &PlacedObjectRecord{ObjectTypeCode: []int{FamilyMetroTop, FamilyMetroMiddle, FamilyMetroBottom}[i], LinkedRecordIndex: -1})
	}
	tm := CreateTimeState()
	tm.DayTick = 0xf0
	TickVipSpecialVisitor(w, tm)
	if w.RngCallCount != 0 {
		t.Fatal("tick 0xf0 must not draw")
	}
	// Find a seed whose first draw passes the 1% gate.
	seed := int32(1)
	for ; ; seed++ {
		if SampleRng(&WorldState{RngState: seed})%100 == 0 {
			break
		}
	}
	w.RngState = seed
	tm.DayTick = 0xf1
	TickVipSpecialVisitor(w, tm)
	for _, k := range []string{"100,110", "100,111", "100,112"} {
		o := w.PlacedObjects.At(k)
		if o.UnitStatus != 2 || o.DirtyFlag != 1 {
			t.Fatalf("%s status %d dirty %d", k, o.UnitStatus, o.DirtyFlag)
		}
	}
	if len(w.PendingNotifications) != 1 || w.PendingNotifications[0].Message != "metro_train_arrival" {
		t.Fatalf("notifications %+v", w.PendingNotifications)
	}
	// Second arrival toggles back to 0 with no notification.
	w.RngState = seed
	TickVipSpecialVisitor(w, tm)
	if w.PlacedObjects.At("100,110").UnitStatus != 0 || len(w.PendingNotifications) != 1 {
		t.Fatal("toggle back")
	}
}

func TestTriggerRandomNewsEventDraws(t *testing.T) {
	w := eventsWorld()
	tm := CreateTimeState()
	tm.DayTick = 0xf1
	TriggerRandomNewsEvent(w, tm)
	if w.RngCallCount != 1 {
		t.Fatalf("draws %d", w.RngCallCount)
	}
	w.EventState.DisableNewsEvents = true
	TriggerRandomNewsEvent(w, tm)
	tm.DaypartIndex = 6
	w.EventState.DisableNewsEvents = false
	TriggerRandomNewsEvent(w, tm)
	if w.RngCallCount != 1 {
		t.Fatalf("gated draws %d", w.RngCallCount)
	}
}

func TestCarrierRemovePrompt(t *testing.T) {
	w := eventsWorld()
	w.EventState.PendingCarrierEditColumn = -1
	w.EventState.PendingCarrierEditY = 5
	if !HandlePromptResponse(w, CreateLedgerState(0), CreateTimeState(), "carrier_remove_1", true) {
		t.Fatal("not handled")
	}
	if w.EventState.PendingCarrierEditColumn != -1 || w.EventState.PendingCarrierEditY != -1 {
		t.Fatal("pending edit not cleared")
	}
}
