package sim

// Port of reference/tower-together/sim/carriers/position.ts.
//
// 1098:10e4 advance_car_position_one_step
//
// Steps the car one position toward its target. When already at target on
// entry, recomputes target+direction first (with prevFloor = cur latched
// before the recompute). Then computes motion mode, possibly arms the settle
// counter, steps by +-1 or +-3 (the +-3 is express-only), and clears
// arrivalSeen.

// departureSequenceTicks TS: DEPARTURE_SEQUENCE_TICKS (carriers/position.ts,
// carriers/advance.ts, queue/dispatch-arrivals.ts all declare it as 5).
const departureSequenceTicks = 5

// AdvanceCarPositionOneStep TS: carriers/position.ts advanceCarPositionOneStep (binary 1098:10e4)
func AdvanceCarPositionOneStep(carrier *CarrierRecord, car *CarrierCar, carIndex int) {
	if car.CurrentFloor == car.TargetFloor {
		car.PrevFloor = car.CurrentFloor
		RecomputeCarTargetAndDirection(carrier, car, carIndex)
	}

	motionMode := ComputeCarMotionMode(carrier, car)
	if motionMode == 0 {
		car.SettleCounter = departureSequenceTicks
	} else if motionMode == 1 {
		car.SettleCounter = 2
	}

	// Binary convention: direction==0 -> down (cur -= step); direction!=0 -> up.
	// Binary quirk: mode 3 (+-3) is express-only.
	stepSize := 1
	if motionMode == 3 {
		stepSize = 3
	}
	stepDir := 1
	if car.DirectionFlag == 0 {
		stepDir = -1
	}
	car.CurrentFloor += stepDir * stepSize

	if car.ArrivalSeen != 0 {
		car.ArrivalSeen = 0
	}
}
