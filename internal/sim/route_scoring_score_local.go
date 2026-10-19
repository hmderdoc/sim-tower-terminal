package sim

// Port of reference/tower-together/sim/route-scoring/score-local.ts.
//
// 11b8:18fb scoreLocalRouteSegment
// 11b8:19a8 scoreHousekeepingRouteSegment

// ScoreLocalRouteSegment is the cost of a direct stairs/escalator segment.
// TS: route-scoring/score-local.ts scoreLocalRouteSegment (binary 11b8:18fb)
func ScoreLocalRouteSegment(segment *SpecialLinkSegment, fromFloor, toFloor, targetHeightMetric int) int {
	if !segment.Active {
		return RouteCostInfinite
	}
	// Binary 11b8:18fb only validates the source landing.
	if !segmentCoversFloor(segment, fromFloor) {
		return RouteCostInfinite
	}
	if !canEnterSegmentFromFloor(segment, fromFloor, toFloor) {
		return RouteCostInfinite
	}
	isStairs := segment.Flags&1 != 0
	distance := absInt(segment.HeightMetric-targetHeightMetric) * 8
	if isStairs {
		return distance + StairsRouteExtraCost
	}
	return distance
}

// ScoreHousekeepingRouteSegment is the stairs-only housekeeping variant.
// TS: route-scoring/score-local.ts scoreHousekeepingRouteSegment (binary 11b8:19a8)
func ScoreHousekeepingRouteSegment(segment *SpecialLinkSegment, fromFloor, toFloor, targetHeightMetric int) int {
	if !segment.Active {
		return RouteCostInfinite
	}
	if segment.Flags&1 == 0 {
		return RouteCostInfinite
	}
	if !segmentCoversFloor(segment, fromFloor) {
		return RouteCostInfinite
	}
	if !canEnterSegmentFromFloor(segment, fromFloor, toFloor) {
		return RouteCostInfinite
	}
	return absInt(segment.HeightMetric-targetHeightMetric)*8 + StairsRouteExtraCost
}

// segmentTopFloor: binary encoding top_floor = entry_floor + (flags >> 1) + 1.
// TS: route-scoring/score-local.ts getSegmentTopFloor
func segmentTopFloor(segment *SpecialLinkSegment) int {
	return segment.EntryFloor + (segment.Flags >> 1) + 1
}

// TS: route-scoring/score-local.ts segmentCoversFloor
func segmentCoversFloor(segment *SpecialLinkSegment, floor int) bool {
	return floor >= segment.EntryFloor && floor <= segmentTopFloor(segment)
}

// canEnterSegmentFromFloor: terminal-landing entry gate. Going up: source
// must equal the entry floor. Going down: source must equal the top floor.
// TS: route-scoring/score-local.ts canEnterSegmentFromFloor
func canEnterSegmentFromFloor(segment *SpecialLinkSegment, fromFloor, toFloor int) bool {
	if toFloor > fromFloor {
		return fromFloor == segment.EntryFloor
	}
	return fromFloor == segmentTopFloor(segment)
}
