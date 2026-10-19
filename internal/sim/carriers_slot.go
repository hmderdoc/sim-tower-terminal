package sim

// Port of reference/tower-together/sim/carriers/slot.ts.
//
// 10a8:17ee floor_to_carrier_slot_index
//
// Maps (carrier, floor) -> per-floor queue slot index. Returns -1 when the
// floor is outside the served range or not a lobby/express slot on an
// express carrier.

// FloorToSlot TS: carriers/slot.ts floorToSlot (binary 10a8:17ee), re-exported by carriers.ts
func FloorToSlot(carrier *CarrierRecord, floor int) int {
	if floor < carrier.BottomServedFloor || floor > carrier.TopServedFloor {
		return -1
	}
	return floor - carrier.BottomServedFloor
}

// IsExpressStopFloor is the binary 10a8:17ee express branch. Only the
// "express queue slot" floors: basement/ground (binary floors 1..10) and the
// sky-lobby cadence above.
//
//	"perfect-parity": sky lobbies at (floor-10) % 15 == 14
//	  (binary 24, 39, 54, ... = logical 14, 29, 44, ...).
//	"modern": sky lobbies at (floor-10) % 15 == 0 with floor > 10
//	  (binary 25, 40, 55, ... = logical 15, 30, 45, ...).
//
// Any other floor on a mode-0 carrier cannot host a call or enqueue. Used by
// the scorer and enqueue paths to gate floor acceptance without changing
// FloorToSlot's linear slot arithmetic.
// TS: carriers/slot.ts isExpressStopFloor (binary 10a8:17ee)
func IsExpressStopFloor(floor int, mode LobbyMode) bool {
	if floor <= 10 {
		return floor >= 1
	}
	cycleOffset := 14
	if mode == LobbyModeModern {
		cycleOffset = 0
	}
	return (floor-10)%15 == cycleOffset
}

// CarrierServesFloor TS: carriers/slot.ts carrierServesFloor
func CarrierServesFloor(carrier *CarrierRecord, floor int) bool {
	return FloorToSlot(carrier, floor) >= 0
}

// CarrierSpansFloor is the geometric span check floor in
// [bottomServedFloor, topServedFloor]. NOT a substitute for binary
// served_floor_flags[f], which on express carriers is gated by FUN_10a8_1296
// to set the byte only at express-stop floors. Use carrierEligibleFloor /
// IsExpressStopFloor for the served-flag-equivalent gate.
// TS: carriers/slot.ts carrierSpansFloor
func CarrierSpansFloor(carrier *CarrierRecord, floor int) bool {
	return floor >= carrier.BottomServedFloor && floor <= carrier.TopServedFloor
}
