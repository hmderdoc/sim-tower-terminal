package sim

// Port of reference/tower-together/sim/carriers/depart.ts.
//
// 1098:23a5 should_car_depart
//
// Gate invoked at dwell-expiry inside advance_carrier_car_state. Forces
// departure when full, when schedule multiplier is 0, or when stopped at a
// non-home, non-lobby/express floor. Otherwise waits until the
// arrival-to-now delta exceeds multiplier * 30 ticks.

// carCapacity TS: getCarCapacity (file-private in carriers/advance.ts,
// depart.ts, target.ts, queue/process-travel.ts).
func carCapacity(carrier *CarrierRecord) int {
	return carrier.AssignmentCapacity
}

// carrierScheduleIndex TS: getScheduleIndex (file-private in
// carriers/advance.ts, depart.ts, queue/process-travel.ts).
func carrierScheduleIndex(time *TimeState) int {
	return time.WeekendFlag*7 + time.DaypartIndex
}

// ShouldCarDepart TS: carriers/depart.ts shouldCarDepart (binary 1098:23a5)
func ShouldCarDepart(carrier *CarrierRecord, car *CarrierCar, time *TimeState, lobbyMode LobbyMode) bool {
	if car.AssignedCount >= carCapacity(carrier) {
		return true
	}
	multiplier := trAt(carrier.DwellDelay, carrierScheduleIndex(time), 1)
	if multiplier == 0 {
		return true
	}
	if car.CurrentFloor != car.HomeFloor {
		// perfect-parity: dwell at floor 10 (ground) and 25, 40, 55, ...
		// (binary's `(currentFloor-10)%15 == 0` express-stop dwell rule).
		// modern: shift the cadence by +1 to follow lobbies that moved up
		// one floor (15, 30, 45, ... above ground).
		cycleOffset := 0
		if lobbyMode == LobbyModeModern {
			cycleOffset = 1
		}
		isLobbyOrExpress := car.CurrentFloor == 10 ||
			(car.CurrentFloor > 10 && (car.CurrentFloor-10)%15 == cycleOffset)
		if !isLobbyOrExpress {
			return true
		}
	}
	return absInt(time.DayTick-car.ArrivalTick) > multiplier*30
}
