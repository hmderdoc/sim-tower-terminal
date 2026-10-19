package sim

import "testing"

// B2 unit tests for cathedral.ts (spec: specs/facility/EVALUATION.md).

func b2CathedralWorld(t *testing.T) (*WorldState, []*PlacedObjectRecord) {
	w := b2World(t)
	w.GateFlags.EvalSimIndex = 0
	var objs []*PlacedObjectRecord
	for i := 0; i < 5; i++ {
		obj := &PlacedObjectRecord{ObjectTypeCode: FamilyCathedralBase + i, LinkedRecordIndex: -1}
		floor := EvalZoneFloor + i
		b2PlaceObject(w, 100, floor, obj)
		objs = append(objs, obj)
		for s := 0; s < 8; s++ {
			sim := b2Sim(floor, 100, 0x24, StateParked)
			sim.BaseOffset = s
			w.Sims = append(w.Sims, sim)
		}
	}
	return w, objs
}

func TestActivateEvalSimsAndMiddayReturn(t *testing.T) {
	w, objs := b2CathedralWorld(t)
	w.GateFlags.EvalSimIndex = NoEvalEntity
	ActivateEvalSims(w)
	if w.Sims[0].StateCode != StateParked {
		t.Fatal("no cathedral placed: no activation")
	}
	w.GateFlags.EvalSimIndex = 0
	ActivateEvalSims(w)
	for _, s := range w.Sims {
		if s.StateCode != StateMorningGate || s.SelectedFloor != LobbyFloor || s.OriginFloor != s.FloorAnchor {
			t.Fatalf("activation: %+v", s)
		}
	}
	w.Sims[0].StateCode = StateArrived
	objs[2].AuxValueOrTimer = 3
	DispatchEvalMiddayReturn(w)
	if w.Sims[0].StateCode != StateDeparture || w.Sims[0].SelectedFloor != EvalZoneFloor ||
		w.Sims[1].StateCode != StateMorningGate || objs[2].AuxValueOrTimer != 0 || objs[2].DirtyFlag != 1 {
		t.Fatalf("midday return: %+v", w.Sims[0])
	}
}

func TestCheckEvalCompletionAndAward(t *testing.T) {
	w, objs := b2CathedralWorld(t)
	w.StarCount = 5
	w.CurrentPopulation = 15_000 // tier 6
	for _, s := range w.Sims[:39] {
		s.StateCode = StateArrived
	}
	CheckEvalCompletionAndAward(w, &TimeState{DayTick: 300}, w.Sims[0])
	if w.StarCount != 5 || objs[0].AuxValueOrTimer != 3 {
		t.Fatalf("39 arrived: star=%d aux=%d", w.StarCount, objs[0].AuxValueOrTimer)
	}
	w.Sims[39].StateCode = StateArrived
	CheckEvalCompletionAndAward(w, &TimeState{DayTick: 800}, w.Sims[39])
	if w.StarCount != 5 {
		t.Fatal("dayTick >= 800 never awards")
	}
	CheckEvalCompletionAndAward(w, &TimeState{DayTick: 799}, w.Sims[39])
	if w.StarCount != 6 {
		t.Fatalf("all arrived with tier 6 > 5: star = %d", w.StarCount)
	}
	for _, o := range objs {
		if o.AuxValueOrTimer != 2 || o.DirtyFlag != 1 {
			t.Fatalf("award stamp: %+v", o)
		}
	}
}

func TestProcessCathedralSimGates(t *testing.T) {
	w, _ := b2CathedralWorld(t)
	sim := w.Sims[0]
	sim.StateCode = StateMorningGate
	calls := w.RngCallCount
	ProcessCathedralSim(w, &TimeState{WeekendFlag: 0, DayTick: 300}, sim)
	if sim.StateCode != StateMorningGate || w.RngCallCount != calls {
		t.Fatal("weekday: no-op")
	}
	ProcessCathedralSim(w, &TimeState{WeekendFlag: 1, DayTick: 0x40}, sim)
	if w.RngCallCount != calls {
		t.Fatal("tick <= 0x50: no RNG draw")
	}
	ProcessCathedralSim(w, &TimeState{WeekendFlag: 1, DaypartIndex: 1, DayTick: 450}, sim)
	if sim.StateCode != StateParked {
		t.Fatal("daypart >= 1 parks")
	}
	sim.StateCode = StateMorningGate
	skipOnTODO(t, func() {
		ProcessCathedralSim(w, &TimeState{WeekendFlag: 1, DayTick: 0xf1}, sim)
		if sim.StateCode == StateMorningGate {
			t.Fatal("tick > 0xf0 always dispatches")
		}
	})
}

func TestHandleCathedralSimArrival(t *testing.T) {
	w, _ := b2CathedralWorld(t)
	sim := w.Sims[0]
	sim.StateCode = StateEvalOutbound
	HandleCathedralSimArrival(w, &TimeState{DayTick: 300}, sim, EvalZoneFloor)
	if sim.StateCode != StateArrived {
		t.Fatalf("outbound arrival = %#x", sim.StateCode)
	}
	sim.StateCode = StateEvalReturn
	HandleCathedralSimArrival(w, &TimeState{}, sim, LobbyFloor)
	if sim.StateCode != StateParked {
		t.Fatalf("return arrival = %#x", sim.StateCode)
	}
}
