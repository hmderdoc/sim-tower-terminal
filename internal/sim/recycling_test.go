package sim

import "testing"

// B2 unit tests for recycling.ts (spec: specs/facility/RECYCLING.md).
func TestUpdateRecyclingCenterState(t *testing.T) {
	w := b2World(t)
	ledger := CreateLedgerState(0)
	upper := &PlacedObjectRecord{ObjectTypeCode: FamilyRecyclingCenterUpper, UnitStatus: 3}
	lower := &PlacedObjectRecord{ObjectTypeCode: FamilyRecyclingCenterLower, UnitStatus: 5}
	b2PlaceObject(w, 10, 5, upper)
	b2PlaceObject(w, 10, 4, lower)
	w.GateFlags.RecyclingCenterCount = 1
	w.GateFlags.RecyclingAdequate = 1

	w.StarCount = 2
	UpdateRecyclingCenterState(w, ledger, 2)
	if w.GateFlags.RecyclingAdequate != 0 || upper.UnitStatus != 3 {
		t.Fatal("star <= 2 only clears the flag")
	}

	w.StarCount = 3
	UpdateRecyclingCenterState(w, ledger, 0)
	if upper.UnitStatus != 0 || lower.UnitStatus != 5 {
		t.Fatalf("midday reset: upper=%d lower=%d", upper.UnitStatus, lower.UnitStatus)
	}

	// Population 1200 / 1 center -> required tier 3. Tier-2 check fails.
	ledger.PopulationLedger[0] = 700
	ledger.PopulationLedger[3] = 500
	UpdateRecyclingCenterState(w, ledger, 2)
	if w.GateFlags.RecyclingAdequate != 0 || upper.UnitStatus != 2 || lower.UnitStatus != 5 {
		t.Fatalf("inadequate: flag=%d upper=%d lower=%d", w.GateFlags.RecyclingAdequate, upper.UnitStatus, lower.UnitStatus)
	}
	// Tier-5 check passes; applied tier is the required tier.
	UpdateRecyclingCenterState(w, ledger, 5)
	if w.GateFlags.RecyclingAdequate != 1 || upper.UnitStatus != 3 || lower.UnitStatus != 3 {
		t.Fatalf("adequate: flag=%d upper=%d lower=%d", w.GateFlags.RecyclingAdequate, upper.UnitStatus, lower.UnitStatus)
	}

	lower.UnitStatus, upper.UnitStatus = 6, 6
	ResetRecyclingCenterDutyTier(w)
	if lower.UnitStatus != 0 || upper.UnitStatus != 6 {
		t.Fatal("daily reset touches only the lower slice at status 6")
	}
}
