package sim

import "testing"

// Port of sims/scoring.test.ts.

func scoringOffice(w *WorldState, x, floor int) *PlacedObjectRecord {
	o := &PlacedObjectRecord{
		LeftTileIndex: x, RightTileIndex: x + 5, ObjectTypeCode: FamilyOffice, AuxValueOrTimer: 1,
		LinkedRecordIndex: -1, DirtyFlag: 1, OccupiedFlag: 1, EvalLevel: 0xff, EvalScore: -1, RentLevel: 1,
	}
	y := FloorToY(floor)
	w.Cells.Set(cellKey(x, y), "office")
	w.PlacedObjects.Set(cellKey(x, y), o)
	return o
}

func scoringOfficeSim(w *WorldState, x, floor, tripCount, accumulated int) *SimRecord {
	s := makeSim(floor, x, 0, FamilyOffice, 6, 0)
	s.StateCode = StateMorningGate
	s.TripCount = tripCount
	s.AccumulatedTicks = accumulated
	w.Sims = append(w.Sims, s)
	return s
}

func scoringWorld() *WorldState {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	w.SpecialLinks = nil
	w.SpecialLinkRecords = nil
	w.TransferGroupEntries = nil
	return w
}

func TestScoringKeepsOfficesUnscoredWithoutTripData(t *testing.T) {
	w := scoringWorld()
	office := scoringOffice(w, 10, 1)
	scoringOfficeSim(w, 10, 1, 0, 0)
	RecomputeAllObjectOperationalStatus(w)
	if office.EvalScore != -1 || office.EvalLevel != 0xff {
		t.Fatalf("score %d level %d", office.EvalScore, office.EvalLevel)
	}
}

// Direct check of the scoring math the TS 2533-checkpoint test relies on:
// one sim with 20 ticks over 1 trip, divisor 6 -> evalScore 3, level 2.
func TestScoringOfficeWithTripData(t *testing.T) {
	w := scoringWorld()
	office := scoringOffice(w, 10, 1)
	scoringOfficeSim(w, 10, 1, 1, 20)
	RecomputeAllObjectOperationalStatus(w)
	if office.EvalScore != 3 || office.EvalLevel != 2 {
		t.Fatalf("score %d level %d", office.EvalScore, office.EvalLevel)
	}
}

func TestScoringRunsAtTick2533Checkpoint(t *testing.T) {
	skipOnTODO(t, func() {
		w := scoringWorld()
		office := scoringOffice(w, 10, 1)
		scoringOfficeSim(w, 10, 1, 1, 20)
		ledger := CreateLedgerState(2_000_000)
		tm := CreateTimeState()
		tm.DayTick = 0x9e4
		RunSimulationDayScheduler(w, ledger, tm)
		if office.EvalScore != 3 || office.EvalLevel != 2 {
			t.Fatalf("score %d level %d", office.EvalScore, office.EvalLevel)
		}
	})
}

func TestScoringNoiseAndRent(t *testing.T) {
	w := scoringWorld()
	office := scoringOffice(w, 10, 1)
	s := scoringOfficeSim(w, 10, 1, 1, 420) // per-sim 420, /6 divisor -> 70
	office.RentLevel = 0                    // +30 -> 100 -> level 1 (80..149)
	RecomputeObjectOperationalStatus(w, s, office)
	if office.EvalScore != 70 || office.EvalLevel != 1 {
		t.Fatalf("no noise: score %d level %d", office.EvalScore, office.EvalLevel)
	}
	// Fast food within 10 tiles on the same floor -> +60 -> 160 -> level 0.
	addObject(w, 22, 1, FamilyFastFood, 16)
	RecomputeObjectOperationalStatus(w, s, office)
	if office.EvalScore != 70 || office.EvalLevel != 0 {
		t.Fatalf("noise: score %d level %d", office.EvalScore, office.EvalLevel)
	}
}

func TestRefreshOccupiedFlagBranches(t *testing.T) {
	w := scoringWorld()
	a := scoringOffice(w, 10, 1)
	b := scoringOffice(w, 30, 1)
	sa := scoringOfficeSim(w, 10, 1, 4, 40)
	a.EvalLevel = 0
	b.EvalLevel = 2
	RefreshOccupiedFlagAndTripCounters(w, sa, a)
	if a.EvalLevel != 1 || b.EvalLevel != 1 || a.OccupiedFlag != 1 || sa.TripCount != 0 {
		t.Fatalf("branch B: a=%d b=%d trips=%d", a.EvalLevel, b.EvalLevel, sa.TripCount)
	}
	a.EvalLevel = 0
	b.EvalLevel = 0
	sa.TripCount = 2
	RefreshOccupiedFlagAndTripCounters(w, sa, a)
	if a.OccupiedFlag != 0 || sa.TripCount != 2 {
		t.Fatal("branch C must clear occupied without reset")
	}
}

func TestCurrentTripStress(t *testing.T) {
	w := scoringWorld()
	s := makeSim(11, 10, 0, FamilyOffice, 6, 0)
	s.ElapsedTicks = 50
	s.LastDemandTick = 2590
	// Wraps the day: (5 - 2590 + 2600) % 2600 = 15.
	if got := CurrentTripStressTicks(w, &TimeState{DayTick: 5}, s); got != 65 {
		t.Fatalf("stress ticks %d", got)
	}
	s.ElapsedTicks = 140
	if CurrentTripStressLevel(w, &TimeState{DayTick: 5}, s) != StressHigh {
		t.Fatal("stress level")
	}
}
