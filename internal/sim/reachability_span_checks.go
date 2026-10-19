package sim

// Port of reference/tower-together/sim/reachability/span-checks.ts.
//
// 11b8:12d2 isFloorSpanWalkableForLocalRoute
// 11b8:1392 isFloorSpanWalkableForHousekeepingRoute
// 11b8:0ccf isFloorWithinSpecialLinkSpan
//
// floorWalkabilityFlags[floor]: bit 0x1 = escalator, bit 0x2 = stairs.

// IsFloorSpanWalkableForLocalRoute: escalator-tolerant gate (bit 0x1,
// <=2-floor stair gap).
// TS: reachability/span-checks.ts isFloorSpanWalkableForLocalRoute (binary 11b8:12d2)
func IsFloorSpanWalkableForLocalRoute(world *WorldState, fromFloor, toFloor int) bool {
	lower := min(fromFloor, toFloor)
	upper := max(fromFloor, toFloor)
	if upper-lower >= 7 {
		return false
	}
	seenGap := false
	// Binary 11b8:12d2 iterates [lower, upper): the target floor is not tested.
	for floor := lower; floor < upper; floor++ {
		flags := trAt(world.FloorWalkabilityFlags, floor, 0)
		if flags == 0 {
			return false
		}
		if flags&1 == 0 {
			seenGap = true
		}
		if seenGap && floor-lower > 2 {
			return false
		}
	}
	return true
}

// IsFloorSpanWalkableForHousekeepingRoute demands continuous stairs (bit
// 0x2). Binary quirk: the counter fails at >= 3, so floors beyond the 3rd
// are not validated.
// TS: reachability/span-checks.ts isFloorSpanWalkableForHousekeepingRoute (binary 11b8:1392)
func IsFloorSpanWalkableForHousekeepingRoute(world *WorldState, fromFloor, toFloor int) bool {
	if absInt(toFloor-fromFloor) >= 7 {
		return false
	}
	count := 0
	lo, hi := fromFloor, toFloor
	if fromFloor >= toFloor {
		lo, hi = toFloor, fromFloor
	}
	for floor := lo; floor < hi; floor++ {
		if trAt(world.FloorWalkabilityFlags, floor, 0)&2 == 0 {
			return false
		}
		if count >= 3 {
			return false
		}
		count++
	}
	return true
}

// IsFloorWithinSpecialLinkSpan: top_floor = entry_floor + (flags >> 1) + 1.
// TS: reachability/span-checks.ts isFloorWithinSpecialLinkSpan (binary 11b8:0ccf)
func IsFloorWithinSpecialLinkSpan(segment *SpecialLinkSegment, floor int) bool {
	topFloor := segment.EntryFloor + (segment.Flags >> 1) + 1
	return floor >= segment.EntryFloor && floor <= topFloor
}
