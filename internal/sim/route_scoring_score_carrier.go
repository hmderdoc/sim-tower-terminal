package sim

// Port of reference/tower-together/sim/route-scoring/score-carrier.ts.
//
// 11b8:168e scoreCarrierTransferRoute, split upstream into direct and
// transfer variants (TODO(11b8:168e) upstream: confirm whether the binary
// folds them).

// trFindCarrier is world.carriers.find(c => c.carrierId === id); nil when absent.
func trFindCarrier(world *WorldState, carrierID int) *CarrierRecord {
	for _, c := range world.Carriers {
		if c.CarrierID == carrierID {
			return c
		}
	}
	return nil
}

// carrierEligibleFloor: binary score_carrier_transfer_route (11b8:168e)
// gates eligibility on served_floor_flags[floor] != 0, which TS splits into
// CarrierSpansFloor AND stopFloorEnabled[slot], plus IsExpressStopFloor for
// express carriers.
// TS: route-scoring/score-carrier.ts carrierEligibleFloor
func carrierEligibleFloor(carrier *CarrierRecord, floor int, lobbyMode LobbyMode) bool {
	if !CarrierSpansFloor(carrier, floor) {
		return false
	}
	slot := floor - carrier.BottomServedFloor
	if trAt(carrier.StopFloorEnabled, slot, 1) == 0 {
		return false
	}
	if carrier.CarrierMode == 0 {
		return IsExpressStopFloor(floor, lobbyMode)
	}
	return true
}

// TS: route-scoring/score-carrier.ts getFloorQueueCount
func floorQueueCount(carrier *CarrierRecord, floor, directionFlag int) int {
	slot := floor - carrier.BottomServedFloor
	if slot < 0 || slot >= len(carrier.FloorQueues) {
		return 0
	}
	queue := carrier.FloorQueues[slot]
	if queue == nil {
		return 0
	}
	return min(directionQueue(queue, directionFlag).Size(), QueueFullCount)
}

// ScoreCarrierDirectRoute scores a carrier route where the carrier directly
// serves both endpoints ("carrier directly serves floor" branch of 11b8:168e).
// TS: route-scoring/score-carrier.ts scoreCarrierDirectRoute (binary 11b8:168e)
func ScoreCarrierDirectRoute(world *WorldState, carrierID, fromFloor, toFloor, targetHeightMetric int) int {
	carrier := trFindCarrier(world, carrierID)
	if carrier == nil {
		return RouteCostInfinite
	}
	if !carrierEligibleFloor(carrier, fromFloor, world.LobbyMode) {
		return RouteCostInfinite
	}
	if !carrierEligibleFloor(carrier, toFloor, world.LobbyMode) {
		return RouteCostInfinite
	}
	dir := 0
	if toFloor > fromFloor {
		dir = 1
	}
	qCount := floorQueueCount(carrier, fromFloor, dir)
	// 11b8:168e direct branch. Express ignores distance: qCount + 640.
	// Others: hypot distance plus base, 1000 only when the queue is
	// saturated (==40).
	if carrier.CarrierMode == 0 {
		return qCount + DirectRouteBaseCost
	}
	distance := absInt(carrier.Column-targetHeightMetric) * 8
	if qCount == QueueFullCount {
		return distance + DirectRouteFullQueueCost
	}
	return distance + DirectRouteBaseCost
}

// ScoreCarrierTransferRoute scores a multi-leg carrier route: the carrier
// covers the source floor and a transfer group reaches the destination.
// TS: route-scoring/score-carrier.ts scoreCarrierTransferRoute (binary 11b8:168e)
func ScoreCarrierTransferRoute(world *WorldState, carrierID, fromFloor, toFloor int, preferLocalMode bool, targetHeightMetric int) int {
	carrier := trFindCarrier(world, carrierID)
	if carrier == nil {
		return RouteCostInfinite
	}
	if !carrierEligibleFloor(carrier, fromFloor, world.LobbyMode) {
		return RouteCostInfinite
	}
	if !TestCarrierTransferReachability(world, carrierID, toFloor, preferLocalMode) {
		return RouteCostInfinite
	}
	dir := 0
	if toFloor > fromFloor {
		dir = 1
	}
	qCount := floorQueueCount(carrier, fromFloor, dir)
	// 11b8:168e transfer branch: base=3000, full-queue penalty=6000.
	if carrier.CarrierMode == 0 {
		return qCount + TransferRouteBaseCost
	}
	distance := absInt(carrier.Column-targetHeightMetric) * 8
	if qCount == QueueFullCount {
		return distance + TransferRouteFullQueueCost
	}
	return distance + TransferRouteBaseCost
}
