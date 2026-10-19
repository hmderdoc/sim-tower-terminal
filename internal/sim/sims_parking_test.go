package sim

import (
	"reflect"
	"testing"
)

// B2 unit tests for sims/parking.ts (no TS unit test covers it; behavior
// follows specs/facility/PARKING.md "Coverage Propagation").
func TestRebuildParkingCoverageAndDemandLog(t *testing.T) {
	w := b2World(t)
	const floor = 5
	y := FloorToY(floor)
	occupy := func(from, to int) {
		for x := from; x <= to; x++ {
			w.Cells.Set(cellKey(x, y), "x")
		}
	}
	addSpace := func(x int) int {
		idx := len(w.Sidecars)
		w.Sidecars = append(w.Sidecars, &ServiceRequestEntry{OwnerSubtypeIndex: 0, FloorIndex: intPtr(floor)})
		b2PlaceObject(w, x, floor, &PlacedObjectRecord{LeftTileIndex: x, RightTileIndex: x + 1,
			ObjectTypeCode: FamilyParking, LinkedRecordIndex: idx})
		occupy(x, x+1)
		w.CellToAnchor.Set(cellKey(x+1, y), cellKey(x, y))
		return idx
	}
	// Ramp at 20..23.
	b2PlaceObject(w, 20, floor, &PlacedObjectRecord{LeftTileIndex: 20, RightTileIndex: 23,
		ObjectTypeCode: FamilyParkingRamp, LinkedRecordIndex: -1})
	occupy(20, 23)
	right := addSpace(24)     // adjacent to the ramp: covered
	far := addSpace(30)       // 4-tile gap (26..29): uncovered
	left := addSpace(15)      // 15..16, gap 17..19 (3 tiles): covered
	beyond := addSpace(10)    // 10..11, gap 12..14 after a covered space: covered
	tombstone := addSpace(40) // demolished entry: skipped by the log
	w.Sidecars[tombstone].(*ServiceRequestEntry).OwnerSubtypeIndex = 0xff

	RebuildParkingCoverage(w)
	cov := func(i int) int { return *w.Sidecars[i].(*ServiceRequestEntry).CoverageFlag }
	if cov(right) != 1 || cov(left) != 1 || cov(beyond) != 1 {
		t.Fatalf("covered spaces: right=%d left=%d beyond=%d", cov(right), cov(left), cov(beyond))
	}
	if cov(far) != 0 || cov(tombstone) != 0 {
		t.Fatalf("uncovered spaces: far=%d tombstone=%d", cov(far), cov(tombstone))
	}

	RebuildParkingDemandLog(w)
	if !reflect.DeepEqual(w.ParkingDemandLog, []int{far}) {
		t.Fatalf("demand log = %v, want [%d]", w.ParkingDemandLog, far)
	}
}

func TestTryAssignParkingService(t *testing.T) {
	w := b2World(t)
	sim := b2Sim(5, 0, FamilyOffice, 0)
	tm := &TimeState{DayTick: 500}
	calls := w.RngCallCount
	if TryAssignParkingService(w, tm, sim) {
		t.Fatal("empty log must fail")
	}
	if w.RngCallCount != calls {
		t.Fatal("empty log must not draw RNG")
	}
	w.Sidecars = append(w.Sidecars, &ServiceRequestEntry{FloorIndex: intPtr(5)})
	w.ParkingDemandLog = []int{0}
	if !TryAssignParkingService(w, tm, sim) {
		t.Fatal("expected assignment")
	}
	if w.RngCallCount != calls+1 {
		t.Fatal("assignment draws exactly one RNG sample")
	}
}
