package sim

// Port of reference/tower-together/sim/route-scoring/score-special-link.ts.
//
// 11b8:0be2 scoreSpecialLinkRoute: pass/fail viability check for one
// specialLinkRecords[recordIndex]. Cost 0 with a direction on success,
// otherwise RouteCostInfinite with direction 0.

// SpecialLinkRouteResult TS: route-scoring/score-special-link.ts SpecialLinkRouteResult
type SpecialLinkRouteResult struct {
	Cost      int
	Direction int
}

var specialLinkRouteFail = SpecialLinkRouteResult{Cost: RouteCostInfinite, Direction: 0}

// ScoreSpecialLinkRoute TS: route-scoring/score-special-link.ts scoreSpecialLinkRoute (binary 11b8:0be2)
func ScoreSpecialLinkRoute(world *WorldState, recordIndex, fromFloor, toFloor int) SpecialLinkRouteResult {
	if recordIndex < 0 || recordIndex >= len(world.SpecialLinkRecords) {
		return specialLinkRouteFail
	}
	record := world.SpecialLinkRecords[recordIndex]
	if record == nil || !record.Active {
		return specialLinkRouteFail
	}

	srcInSpan := fromFloor >= record.LowerFloor && fromFloor <= record.UpperFloor
	if !srcInSpan {
		return specialLinkRouteFail
	}

	dstInSpan := toFloor >= record.LowerFloor && toFloor <= record.UpperFloor
	if dstInSpan {
		dir := 0
		if fromFloor < toFloor {
			dir = 1
		}
		return SpecialLinkRouteResult{Cost: 0, Direction: dir}
	}

	dstMask := trAt(record.ReachabilityMasksByFloor, toFloor, 0)
	if dstMask != 0 {
		srcMask := trAt(record.ReachabilityMasksByFloor, fromFloor, 0)
		if srcMask != 0 {
			// Inside the span the table stores a 1-based index into
			// transferGroupEntries.
			idx := srcMask - 1
			if idx >= 0 && idx < len(world.TransferGroupEntries) {
				srcEntry := world.TransferGroupEntries[idx]
				if srcEntry != nil && srcEntry.Active && srcEntry.CarrierMask&dstMask == dstMask {
					return SpecialLinkRouteResult{Cost: 0, Direction: 0}
				}
			}
		}
	}

	// Final fallback: peer-record reachability through every transfer-group
	// entry this record participates in.
	recordBit := trBit(24 + recordIndex)
	for _, entry := range world.TransferGroupEntries {
		if !entry.Active {
			continue
		}
		if entry.CarrierMask&recordBit == 0 {
			continue
		}
		if TestSpecialLinkTransferReachability(world, entry, toFloor) {
			return SpecialLinkRouteResult{Cost: 0, Direction: 0}
		}
	}
	return specialLinkRouteFail
}
