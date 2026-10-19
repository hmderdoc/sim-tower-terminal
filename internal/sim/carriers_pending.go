package sim

// Port of reference/tower-together/sim/carriers/pending.ts.
//
// 1098:0b10 decrement_car_pending_assignment_count
//
// Binary post-pass invoked per carrier at the end of carrier_tick. The TS
// folds this into the inline decrement inside clearFloorRequestsOnArrival,
// so the function is intentionally empty (TODO(1098:0b10) upstream).

// DecrementCarPendingAssignmentCount TS: carriers/pending.ts decrementCarPendingAssignmentCount (binary 1098:0b10)
func DecrementCarPendingAssignmentCount(_ *CarrierRecord, _ *CarrierCar) {
	// Intentionally empty: binary post-pass semantics not yet separated from
	// the inline decrement in ClearFloorRequestsOnArrival.
}
