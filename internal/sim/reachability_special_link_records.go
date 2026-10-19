package sim

import "sort"

// Port of reference/tower-together/sim/reachability/special-link-records.ts.
//
// 11b8:06a4 rebuildSpecialLinkRouteRecords
// 11b8:0763 scanSpecialLinkSpanBound
//
// Rebuilds the eight derived specialLinkRecord transfer zones from the set
// of stairs/escalator overlays, and scans a center floor outward to find the
// top or bottom of a walkable span.

// derivedRecordCenters: lobby/sky-lobby centers track the express-stop
// cadence. perfect-parity: 10, 24, 39, 54, ...; modern: 10, 25, 40, 55, ...
// TS: reachability/special-link-records.ts derivedRecordCenters
func derivedRecordCenters(lobbyMode LobbyMode) []int {
	cycleOffset := 14
	if lobbyMode == LobbyModeModern {
		cycleOffset = 15
	}
	return []int{
		10,
		10 + cycleOffset,
		25 + cycleOffset,
		40 + cycleOffset,
		55 + cycleOffset,
		70 + cycleOffset,
		85 + cycleOffset,
	}
}

// RebuildSpecialLinkRouteRecords: per-tile placement matches binary
// 1200:149c. flags = (extent_minus_one << 1) | stairsBit; entryFloor is the
// lower landing (the overlay coordinate is the upper landing).
// TS: reachability/special-link-records.ts rebuildSpecialLinkRouteRecords (binary 11b8:06a4)
//
// Iteration order: overlays are read through the OrderedMap snapshot and
// then stable-sorted by (column, entryFloor); ties (none in practice: one
// overlay per cell) keep insertion order.
func RebuildSpecialLinkRouteRecords(world *WorldState) {
	world.SpecialLinks = make([]*SpecialLinkSegment, MaxSpecialLinks)
	for i := range world.SpecialLinks {
		world.SpecialLinks[i] = &SpecialLinkSegment{}
	}
	world.SpecialLinkRecords = make([]*SpecialLinkRecord, MaxSpecialLinkRecords)
	for i := range world.SpecialLinkRecords {
		world.SpecialLinkRecords[i] = &SpecialLinkRecord{ReachabilityMasksByFloor: make([]int, GridHeight)}
	}

	type tileEntry struct {
		column, entryFloor int
		typ                string
	}
	tileEntries := []tileEntry{}
	for _, e := range world.Overlays.Entries() {
		typ := e.Val
		if typ != "stairs" && typ != "escalator" {
			continue
		}
		column, y := parseKeyXY(e.Key)
		upperFloor := YToFloor(y)
		// Overlay coordinate is the upper landing; entry_floor is F - 1.
		tileEntries = append(tileEntries, tileEntry{column: column, entryFloor: upperFloor - 1, typ: typ})
	}

	sort.SliceStable(tileEntries, func(i, j int) bool {
		a, b := tileEntries[i], tileEntries[j]
		if a.column == b.column {
			return a.entryFloor < b.entryFloor
		}
		return a.column < b.column
	})

	segmentIndex := 0
	for _, tile := range tileEntries {
		if segmentIndex >= MaxSpecialLinks {
			break
		}
		stairsBit := 0
		if tile.typ == "stairs" {
			stairsBit = 1
		}
		// extent_minus_one = 0 for a 2-floor tile, so flags == stairsBit.
		world.SpecialLinks[segmentIndex] = &SpecialLinkSegment{
			Active:       true,
			Flags:        stairsBit,
			HeightMetric: tile.column,
			EntryFloor:   tile.entryFloor,
		}
		segmentIndex++
	}

	// Refresh floorWalkabilityFlags from the just-rebuilt specialLinks so the
	// span scan below sees up-to-date walkability.
	world.FloorWalkabilityFlags = make([]int, GridHeight)
	markSpecialLinkWalkability(world)

	for recordIndex, center := range derivedRecordCenters(world.LobbyMode) {
		if recordIndex >= MaxSpecialLinkRecords {
			break
		}
		lowerFloor := ScanSpecialLinkSpanBound(world, center, 0)
		upperFloor := ScanSpecialLinkSpanBound(world, center, 1)
		if lowerFloor >= upperFloor {
			continue
		}
		world.SpecialLinkRecords[recordIndex] = &SpecialLinkRecord{
			Active:                   true,
			LowerFloor:               lowerFloor,
			UpperFloor:               upperFloor,
			ReachabilityMasksByFloor: make([]int, GridHeight),
		}
	}
}

// ScanSpecialLinkSpanBound scans from centerFloor up (dir 1) or down (dir
// 0) for the walkable-span bound.
// TS: reachability/special-link-records.ts scanSpecialLinkSpanBound (binary 11b8:0763)
func ScanSpecialLinkSpanBound(world *WorldState, centerFloor int, dir int) int {
	seenGap := false

	if dir != 0 {
		for floor := centerFloor; floor < centerFloor+6; floor++ {
			flags := trAt(world.FloorWalkabilityFlags, floor, 0)
			if flags == 0 {
				return floor
			}
			if flags&1 == 0 {
				seenGap = true
			}
			if seenGap && floor >= centerFloor+3 {
				return floor
			}
		}
		return centerFloor + 6
	}

	// Binary 11b8:0763 starts at centerFloor-1 and returns floor+1 on the
	// zero-flag exit.
	for floor := centerFloor - 1; floor > centerFloor-6; floor-- {
		flags := trAt(world.FloorWalkabilityFlags, floor, 0)
		if flags == 0 {
			return floor + 1
		}
		if flags&1 == 0 {
			seenGap = true
		}
		if seenGap && floor <= centerFloor-3 {
			return floor
		}
	}
	return centerFloor - 5
}
