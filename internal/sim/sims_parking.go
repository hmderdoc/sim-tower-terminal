package sim

// Port of reference/tower-together/sim/sims/parking.ts: parking-space
// coverage propagation, the parking demand log, and service assignment.

type parkingSpace struct {
	x, y         int
	sidecarIndex int
}

type parkingRampAnchor struct {
	x, y, right int
}

// RebuildParkingCoverage TS: sims/parking.ts rebuildParkingCoverage.
// For each ramp anchor, walk left/right across same-floor tiles crossing
// empty runs of at most 3 tiles; reachable parking-space tiles are covered
// (1), every other parking space is uncovered (0).
func RebuildParkingCoverage(world *WorldState) {
	// parkingByKey: TS Map. Lookups by key, plus one `.values()` pass that
	// only writes 0, so iteration order is irrelevant; the slice keeps the
	// insertion order anyway.
	parkingByKey := map[string]parkingSpace{}
	var parkingOrder []string
	var rampAnchors []parkingRampAnchor

	for _, e := range world.PlacedObjects.Entries() {
		x, y := parseKeyXY(e.Key)
		obj := e.Val
		if obj.ObjectTypeCode == FamilyParking {
			if _, seen := parkingByKey[e.Key]; !seen {
				parkingOrder = append(parkingOrder, e.Key)
			}
			parkingByKey[e.Key] = parkingSpace{x: x, y: y, sidecarIndex: obj.LinkedRecordIndex}
		} else if obj.ObjectTypeCode == FamilyParkingRamp {
			rampAnchors = append(rampAnchors, parkingRampAnchor{x: x, y: y, right: obj.RightTileIndex})
		}
	}

	for _, k := range parkingOrder {
		space := parkingByKey[k]
		if rec, ok := GetSidecar(world, space.sidecarIndex).(*ServiceRequestEntry); ok && rec != nil {
			rec.CoverageFlag = intPtr(0)
		}
	}

	// `world.cellToAnchor[k] ?? k`
	anchorOf := func(k string) string {
		if a, ok := world.CellToAnchor.Get(k); ok {
			return a
		}
		return k
	}
	isParkingTile := func(x, y int) bool {
		_, ok := parkingByKey[anchorOf(cellKey(x, y))]
		return ok
	}
	isEmpty := func(x, y int) bool {
		k := cellKey(x, y)
		return !hasStr(&world.Cells, k) && !hasStr(&world.CellToAnchor, k)
	}
	cover := func(x, y int) {
		space, ok := parkingByKey[anchorOf(cellKey(x, y))]
		if !ok {
			return
		}
		if rec, ok := GetSidecar(world, space.sidecarIndex).(*ServiceRequestEntry); ok && rec != nil {
			rec.CoverageFlag = intPtr(1)
		}
	}

	const maxEmptyGap = 3

	walk := func(startX, y, direction, bound int) {
		inBounds := func(x int) bool {
			if direction == 1 {
				return x <= bound
			}
			return x >= bound
		}
		x := startX
		for inBounds(x) {
			if isParkingTile(x, y) {
				cover(x, y)
				x += direction
				continue
			}
			if isEmpty(x, y) {
				run := 0
				for inBounds(x) && isEmpty(x, y) && run <= maxEmptyGap {
					x += direction
					run++
				}
				if run > maxEmptyGap {
					return
				}
				continue
			}
			return
		}
	}

	for _, ramp := range rampAnchors {
		// Cover the ramp footprint's parking neighbors immediately to the sides.
		walk(ramp.x-1, ramp.y, -1, 0)
		walk(ramp.right+1, ramp.y, 1, world.Width-1)
	}
}

// RebuildParkingDemandLog TS: sims/parking.ts rebuildParkingDemandLog
func RebuildParkingDemandLog(world *WorldState) {
	world.ParkingDemandLog = []int{}
	for i := 0; i < len(world.Sidecars); i++ {
		rec, ok := world.Sidecars[i].(*ServiceRequestEntry)
		if !ok || rec == nil {
			continue
		}
		if rec.OwnerSubtypeIndex == 0xff {
			continue
		}
		if rec.FloorIndex == nil {
			continue
		}
		if rec.CoverageFlag != nil && *rec.CoverageFlag == 1 {
			continue
		}

		isParking := false
		for _, obj := range world.PlacedObjects.Values() {
			if obj.ObjectTypeCode == FamilyParking && obj.LinkedRecordIndex == i {
				isParking = true
				break
			}
		}
		if isParking {
			world.ParkingDemandLog = append(world.ParkingDemandLog, i)
		}
	}
}

// TryAssignParkingService TS: sims/parking.ts tryAssignParkingService
func TryAssignParkingService(world *WorldState, time *TimeState, sim *SimRecord) bool {
	if len(world.ParkingDemandLog) == 0 {
		return false
	}
	idx := world.ParkingDemandLog[SampleRng(world)%len(world.ParkingDemandLog)]
	if rec, ok := GetSidecar(world, idx).(*ServiceRequestEntry); !ok || rec == nil {
		return false
	}
	// Binary parity: process_family_parking_destination_arrival (1048:00f0)
	// is NOT one of the 6 advance_sim_trip_counters call sites.
	RebaseSimElapsedFromClock(sim, time)
	return true
}
