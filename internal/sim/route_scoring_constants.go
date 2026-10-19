package sim

// Port of reference/tower-together/sim/route-scoring/constants.ts: the
// cost-infinite sentinel and route-cost constants used by the scorers.

const (
	RouteCostInfinite          = 0x7fff
	StairsRouteExtraCost       = 0x280  // 640
	DirectRouteBaseCost        = 0x280  // 640 (11b8:168e direct branch)
	DirectRouteFullQueueCost   = 0x3e8  // 1000 (replaces base when qByte == 0x28, mode != 0)
	TransferRouteBaseCost      = 0xbb8  // 3000 (11b8:168e transfer branch)
	TransferRouteFullQueueCost = 0x1770 // 6000 (replaces base when qByte == 0x28, mode != 0)
	QueueFullCount             = 0x28   // 40 (ring buffer capacity)
)
