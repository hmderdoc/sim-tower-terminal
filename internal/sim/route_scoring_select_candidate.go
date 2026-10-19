package sim

// Port of reference/tower-together/sim/route-scoring/select-candidate.ts.
//
// 11b8:1484 selectBestRouteCandidate: family-agnostic route selector.
// Three-stage scan: (1) a 64-segment local/express scan gated by direct
// walkability, (2) when passenger-mode AND no direct segment was found, an
// 8-record special-link pass that can trigger a second 64-segment local
// scan against sourceFloor +- 1, (3) a 24-carrier scan whose threshold is
// whatever local_e was preserved from earlier stages.

// Route candidate kinds.
const (
	RouteCandidateSegment = "segment"
	RouteCandidateCarrier = "carrier"
)

// RouteCandidate TS: route-scoring/select-candidate.ts RouteCandidate
type RouteCandidate struct {
	Kind string // "segment" | "carrier"
	ID   int
	Cost int
}

// SelectBestRouteCandidate returns nil when no route exists (TS null).
// TS defaults: preferLocalMode = true, targetHeightMetric = 0.
// TS: route-scoring/select-candidate.ts selectBestRouteCandidate (binary 11b8:1484)
func SelectBestRouteCandidate(world *WorldState, fromFloor, toFloor int, preferLocalMode bool, targetHeightMetric int) *RouteCandidate {
	if fromFloor == toFloor {
		return nil
	}

	delta := absInt(fromFloor - toFloor)
	// Binary local_e and local_idx. local_e doubles as the running minimum
	// cost AND the carrier-scan threshold.
	bestCost := RouteCostInfinite
	bestIndex := -1

	if !preferLocalMode {
		// Housekeeping path: one 64-segment scan gated by continuous-stairs
		// walkability; on success return immediately.
		if delta == 1 || IsFloorSpanWalkableForHousekeepingRoute(world, fromFloor, toFloor) {
			for segmentIndex, segment := range world.SpecialLinks {
				if segment == nil {
					continue
				}
				cost := ScoreHousekeepingRouteSegment(segment, fromFloor, toFloor, targetHeightMetric)
				if cost < bestCost {
					bestIndex = segmentIndex
					bestCost = cost
				}
			}
			if bestIndex >= 0 {
				return &RouteCandidate{Kind: RouteCandidateSegment, ID: bestIndex, Cost: bestCost}
			}
		}
	} else {
		// Passenger path. First local scan gated by local walkability.
		if delta == 1 || IsFloorSpanWalkableForLocalRoute(world, fromFloor, toFloor) {
			for segmentIndex, segment := range world.SpecialLinks {
				if segment == nil {
					continue
				}
				cost := ScoreLocalRouteSegment(segment, fromFloor, toFloor, targetHeightMetric)
				if cost < bestCost {
					bestIndex = segmentIndex
					bestCost = cost
				}
			}
			// 11b8:1510-1515 early return when the cheapest segment is below
			// the stairs cost-extra threshold (escalator hit).
			if bestIndex >= 0 && bestCost < StairsRouteExtraCost {
				return &RouteCandidate{Kind: RouteCandidateSegment, ID: bestIndex, Cost: bestCost}
			}
		}

		// 11b8:151d Special-link block. Runs only if the first scan found
		// nothing (local_idx < 0).
		if bestIndex < 0 {
			direction := 0
			foundRecord := false
			for recordIndex := 0; recordIndex < MaxSpecialLinkRecords; recordIndex++ {
				result := ScoreSpecialLinkRoute(world, recordIndex, fromFloor, toFloor)
				if result.Cost < bestCost {
					bestIndex = recordIndex
					direction = result.Direction
					bestCost = result.Cost
					foundRecord = true
				}
			}

			if foundRecord {
				// Second 64-segment local scan against sourceFloor +- 1.
				// The binary resets local_e = 0x7fff; local_idx = -1 here.
				adjacentFloor := fromFloor + 1
				if direction == 0 {
					adjacentFloor = fromFloor - 1
				}
				bestCost = RouteCostInfinite
				bestIndex = -1
				for segmentIndex, segment := range world.SpecialLinks {
					if segment == nil {
						continue
					}
					cost := ScoreLocalRouteSegment(segment, fromFloor, adjacentFloor, targetHeightMetric)
					if cost < bestCost {
						bestIndex = segmentIndex
						bestCost = cost
					}
				}
				// Post-scan early return: winner found with cost < 640.
				if bestIndex >= 0 && bestCost < StairsRouteExtraCost {
					return &RouteCandidate{Kind: RouteCandidateSegment, ID: bestIndex, Cost: bestCost}
				}
			}
		}
	}

	// 11b8:1632 Carrier scan. Threshold is the preserved local_e. Carrier
	// IDs are encoded as 0x40 + carrierIndex.
	for carrierIndex := 0; carrierIndex < 24; carrierIndex++ {
		carrier := trFindCarrier(world, carrierIndex)
		if carrier == nil {
			continue
		}
		if preferLocalMode {
			if carrier.CarrierMode == 2 {
				continue
			}
		} else if carrier.CarrierMode != 2 {
			continue
		}
		// Binary calls score_carrier_transfer_route once per carrier; TS
		// splits direct vs transfer and uses the cheaper.
		directCost := ScoreCarrierDirectRoute(world, carrier.CarrierID, fromFloor, toFloor, targetHeightMetric)
		transferCost := ScoreCarrierTransferRoute(world, carrier.CarrierID, fromFloor, toFloor, preferLocalMode, targetHeightMetric)
		carrierCost := min(directCost, transferCost)
		if carrierCost < bestCost {
			bestIndex = 0x40 + carrierIndex
			bestCost = carrierCost
		}
	}

	if bestIndex < 0 {
		return nil
	}
	if bestIndex >= 0x40 {
		return &RouteCandidate{Kind: RouteCandidateCarrier, ID: bestIndex - 0x40, Cost: bestCost}
	}
	return &RouteCandidate{Kind: RouteCandidateSegment, ID: bestIndex, Cost: bestCost}
}
