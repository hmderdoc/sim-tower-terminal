package sim

// Port of reference/tower-together/sim/progression.test.ts.

import "testing"

func progressionWorld() *WorldState {
	return newEmptyWorld("t", "T", LobbyModePerfectParity)
}

// addObjectWithActivation mirrors the TS helper: places a dummy object and
// bumps currentPopulation (g_primary_family_ledger_total) directly.
func addObjectWithActivation(w *WorldState, key string, ledgerContribution int) *PlacedObjectRecord {
	object := &PlacedObjectRecord{LinkedRecordIndex: -1, EvalScore: -1, RentLevel: 1}
	w.PlacedObjects.Set(key, object)
	w.CurrentPopulation += ledgerContribution
	return object
}

func lateAfternoonTime() *TimeState {
	t := CreateTimeState()
	t.DayTick = 2000
	t.DaypartIndex = 5
	return t
}

func TestComputeTowerTierFromLedger(t *testing.T) {
	cases := []struct {
		name    string
		amounts []int
		want    int
	}{
		{"empty tower is tier 1", nil, 1},
		{"tier 2 once first threshold is met", []int{StarThresholds[0]}, 2},
		{"stays below the threshold", []int{StarThresholds[0] - 1}, 1},
		{"top threshold gives tier 6", []int{StarThresholds[4]}, 6},
	}
	for _, c := range cases {
		w := progressionWorld()
		for i, a := range c.amounts {
			addObjectWithActivation(w, string(rune('a'+i)), a)
		}
		if got := ComputeStarCountFromPopulation(w); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
	// sums contributions to the primary ledger total
	w := progressionWorld()
	addObjectWithActivation(w, "a", 200)
	if got := ComputeStarCountFromPopulation(w); got != 1 {
		t.Fatalf("200: got %d", got)
	}
	addObjectWithActivation(w, "b", 100)
	if got := ComputeStarCountFromPopulation(w); got != 2 {
		t.Fatalf("300: got %d", got)
	}
}

func TestCheckStarAdvancementConditions(t *testing.T) {
	// 1->2 has no qualitative gate
	w := progressionWorld()
	if !CheckStarAdvancementConditions(w, lateAfternoonTime()) {
		t.Fatal("1->2 should pass")
	}

	// 2->3 requires securityPlaced
	w = progressionWorld()
	w.StarCount = 2
	if CheckStarAdvancementConditions(w, lateAfternoonTime()) {
		t.Fatal("2->3 without security should fail")
	}
	w.GateFlags.SecurityPlaced = 1
	if !CheckStarAdvancementConditions(w, lateAfternoonTime()) {
		t.Fatal("2->3 with security should pass")
	}

	// 3->4 requires all office-tier gates + daypart + weekendFlag
	w = progressionWorld()
	w.StarCount = 3
	w.GateFlags.OfficePlaced = 1
	w.GateFlags.RecyclingAdequate = 1
	w.GateFlags.OfficeServiceOk = 1
	w.GateFlags.OfficeServiceOkMedical = 1
	w.GateFlags.RoutesViable = 1
	tm := lateAfternoonTime()
	if !CheckStarAdvancementConditions(w, tm) {
		t.Fatal("3->4 all gates should pass")
	}
	w.GateFlags.OfficeServiceOk = 0
	if CheckStarAdvancementConditions(w, tm) {
		t.Fatal("3->4 missing officeServiceOk should fail")
	}
	w.GateFlags.OfficeServiceOk = 1
	early := *tm
	early.DaypartIndex = 3
	if CheckStarAdvancementConditions(w, &early) {
		t.Fatal("3->4 daypart < 4 should fail")
	}
	weekend := *tm
	weekend.WeekendFlag = 1
	if CheckStarAdvancementConditions(w, &weekend) {
		t.Fatal("3->4 weekend should fail")
	}

	// 4->5 requires metroPlaced (not officeServiceOk)
	w = progressionWorld()
	w.StarCount = 4
	w.GateFlags.MetroPlaced = 1
	w.GateFlags.RecyclingAdequate = 1
	w.GateFlags.OfficeServiceOkMedical = 1
	w.GateFlags.RoutesViable = 1
	if !CheckStarAdvancementConditions(w, tm) {
		t.Fatal("4->5 should pass")
	}
	w.GateFlags.MetroPlaced = 0
	if CheckStarAdvancementConditions(w, tm) {
		t.Fatal("4->5 without metro should fail")
	}

	// false at starCount >= 5
	w = progressionWorld()
	w.StarCount = 5
	if CheckStarAdvancementConditions(w, lateAfternoonTime()) {
		t.Fatal("5 should fail")
	}
}

func TestTryAdvanceStarCount(t *testing.T) {
	// advances 1->2 once the activity threshold is met
	w := progressionWorld()
	addObjectWithActivation(w, "a", StarThresholds[0])
	if !TryAdvanceStarCount(w, lateAfternoonTime()) || w.StarCount != 2 {
		t.Fatalf("1->2: star %d", w.StarCount)
	}
	if len(w.PendingNotifications) != 1 || w.PendingNotifications[0] != (SimNotification{Kind: "star_advanced", Message: "2"}) {
		t.Fatalf("notifications %+v", w.PendingNotifications)
	}

	// does nothing if activity threshold not met
	w = progressionWorld()
	addObjectWithActivation(w, "a", 10)
	if TryAdvanceStarCount(w, lateAfternoonTime()) || w.StarCount != 1 || len(w.PendingNotifications) != 0 {
		t.Fatal("below threshold should not advance")
	}

	// does nothing if qualitative gate fails (2->3 without security)
	w = progressionWorld()
	w.StarCount = 2
	addObjectWithActivation(w, "a", StarThresholds[1])
	if TryAdvanceStarCount(w, lateAfternoonTime()) || w.StarCount != 2 {
		t.Fatal("2->3 without security should not advance")
	}

	// resets officeServiceOk on advancement
	w = progressionWorld()
	w.StarCount = 3
	addObjectWithActivation(w, "a", StarThresholds[2])
	w.GateFlags.OfficePlaced = 1
	w.GateFlags.RecyclingAdequate = 1
	w.GateFlags.OfficeServiceOk = 1
	w.GateFlags.OfficeServiceOkMedical = 1
	w.GateFlags.RoutesViable = 1
	if !TryAdvanceStarCount(w, lateAfternoonTime()) || w.StarCount != 4 || w.GateFlags.OfficeServiceOk != 0 {
		t.Fatalf("3->4: star %d ok %d", w.StarCount, w.GateFlags.OfficeServiceOk)
	}

	// caps at 5 stars
	w = progressionWorld()
	w.StarCount = 5
	addObjectWithActivation(w, "a", StarThresholds[4])
	if TryAdvanceStarCount(w, lateAfternoonTime()) || w.StarCount != 5 {
		t.Fatal("5 should stay 5")
	}
}

// Not in the TS suite: bucket bookkeeping (binary 1068:07f7 / 1068:07b3).
func TestPopulationBuckets(t *testing.T) {
	w := progressionWorld()
	AddToPopulationBucket(w, FamilyRetail, 40)
	AddToPopulationBucket(w, FamilyRetail, 10)
	AddToPopulationBucket(w, FamilyFastFood, 7)
	if w.CurrentPopulation != 57 || w.CurrentPopulationBuckets[FamilyRetail] != 50 {
		t.Fatalf("after add: total %d buckets %v", w.CurrentPopulation, w.CurrentPopulationBuckets)
	}
	ClearPopulationBucket(w, FamilyRetail)
	if w.CurrentPopulation != 7 || w.CurrentPopulationBuckets[FamilyRetail] != 0 {
		t.Fatalf("after clear: total %d buckets %v", w.CurrentPopulation, w.CurrentPopulationBuckets)
	}
}
