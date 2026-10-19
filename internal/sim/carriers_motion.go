package sim

// Port of reference/tower-together/sim/carriers/motion.ts.
//
// 1098:209f compute_car_motion_mode
//
// Returns 0=stop (seeds settle=5), 1=slow (seeds settle=2), 2=normal, 3=fast
// (+-3 floors, express only). Binary quirk: mode 3 is gated to express
// carriers (mode==0); standard/service carriers cannot reach it even when
// distances qualify. Do NOT "fix" this.

// ComputeCarMotionMode TS: carriers/motion.ts computeCarMotionMode (binary 1098:209f)
func ComputeCarMotionMode(carrier *CarrierRecord, car *CarrierCar) int {
	distToTarget := absInt(car.CurrentFloor - car.TargetFloor)
	distFromPrev := absInt(car.CurrentFloor - car.PrevFloor)

	// Binary 1098:209f. No firstLeg override: when distFromPrev < 2
	// (including 0 on departure), binary returns mode 0 (stop, stab=5).
	if carrier.CarrierMode == 0 {
		// Express: stop within 2, fast (+-3) when both > 4, normal otherwise.
		if distToTarget < 2 || distFromPrev < 2 {
			return 0
		}
		if distToTarget > 4 && distFromPrev > 4 {
			return 3
		}
		return 2
	}

	// Standard (1) and Service (2): stop within 2, slow-stop within 4.
	if distToTarget < 2 || distFromPrev < 2 {
		return 0
	}
	if distToTarget < 4 || distFromPrev < 4 {
		return 1
	}
	return 2
}
