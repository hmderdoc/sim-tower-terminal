package sim

// Port of reference/tower-together/sim/sims/commercial.ts: the shared
// restaurant / fast-food / retail owner-sim state machine.

// ─── B2 helpers (not in the TS) ──────────────────────────────────────────────

// b2CommercialVenueAt is the TS
// `world.sidecars[i] as CommercialVenueRecord | undefined` followed by a
// `?.kind === "commercial_venue"` check: nil when out of range or another kind.
func b2CommercialVenueAt(world *WorldState, index int) *CommercialVenueRecord {
	rec, _ := GetSidecar(world, index).(*CommercialVenueRecord)
	return rec
}

// resolveOptsFeedback builds `{ emitDistanceFeedback: emit }`.
func resolveOptsFeedback(emit bool) *ResolveSimRouteOptions {
	return &ResolveSimRouteOptions{EmitDistanceFeedback: &emit}
}

// resolveOptsFeedbackHeight builds
// `{ emitDistanceFeedback: emit, targetHeightMetric: height }`.
func resolveOptsFeedbackHeight(emit bool, height int) *ResolveSimRouteOptions {
	return &ResolveSimRouteOptions{EmitDistanceFeedback: &emit, TargetHeightMetric: &height}
}

// b2Bool01 is the TS `cond ? 1 : 0` direction-flag idiom.
func b2Bool01(cond bool) int {
	if cond {
		return 1
	}
	return 0
}

// ─── sims/commercial.ts ──────────────────────────────────────────────────────

// ProcessCommercialSim TS: sims/commercial.ts processCommercialSim
// (binary gates 1228:3ed9 retail / 1228:466d restaurant+fast-food).
func ProcessCommercialSim(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	object := FindObjectForSim(world, sim)
	if object == nil {
		return
	}

	state := sim.StateCode

	// --- Night / parked states ---
	if state == StateNightA || state == StateNightB || state == StateParked {
		if time.DayTick > 2300 {
			sim.StateCode = StateMorningGate
		}
		return
	}

	// --- Morning activation gate ---
	// Binary: gate_object_family_retail_state_handler (1228:3ed9) and
	// gate_object_family_restaurant_fast_food_state_handler (1228:466d).
	// Retail has an early DORMANT+occupiedFlag exit *before* any RNG;
	// restaurant/fast-food has no occupied-flag check.
	if state == StateMorningGate {
		if sim.FamilyCode == FamilyRetail {
			// Binary 1228:4014/4044: CMP byte ptr ES:[BX+0x14],0x0.
			if object.LinkedRecordIndex >= 0 {
				venue := b2CommercialVenueAt(world, object.LinkedRecordIndex)
				if venue != nil && venue.AvailabilityState == VenueDormant && object.OccupiedFlag == 0 {
					return
				}
			}
		}

		if sim.FamilyCode == FamilyRestaurant {
			// Binary 1228:466d restaurant branch:
			// dp4 -> 1/12 RNG gate -> dispatch; dp<5 -> return;
			// dp5 with dayTick<=2199 -> dispatch; else return.
			if time.DaypartIndex == 4 {
				if SampleRng(world)%12 != 0 {
					return
				}
			} else if time.DaypartIndex < 5 {
				return
			} else if time.DayTick > 2199 {
				return
			}
		} else {
			// Binary 1228:3ed9 / 1228:466d fast-food+retail branch:
			// dp>=5 -> return; dayTick<241 -> return;
			// dp0-3 -> 1/36 RNG gate; dp4 -> 1/6 RNG gate.
			if time.DaypartIndex >= 5 {
				return
			}
			if time.DayTick < 0xf1 {
				return
			}
			if time.DaypartIndex <= 3 {
				if SampleRng(world)%36 != 0 {
					return
				}
			} else {
				if SampleRng(world)%6 != 0 {
					return
				}
			}
		}

		// Dispatch. Restaurant/fast-food share dispatch_object_family_6_0c
		// (1228:4851), which silent-parks when availabilityState == CLOSED.
		// Retail's dispatch (1228:40c0) only consumes capacity.
		if object.LinkedRecordIndex < 0 {
			return
		}
		record := b2CommercialVenueAt(world, object.LinkedRecordIndex)
		if record == nil {
			return
		}

		if sim.FamilyCode != FamilyRetail {
			// Restaurant/fast-food state-0x20 silent-park (1228:499c).
			if record.AvailabilityState == VenueClosed {
				sim.StateCode = StateParked
				return
			}
		}

		// try_consume_commercial_venue_capacity (11b0:1150).
		if record.RemainingCapacity <= 0 {
			return
		}
		if record.EligibilityThreshold < 0 && sim.BaseOffset > 1-record.EligibilityThreshold {
			return
		}

		// Binary try_consume decrements +6, increments +7 and +0x10; the
		// currentPopulation (+9) increment is deferred to the arrival site
		// (acquire_commercial_venue_slot 11b0:0d92).
		record.RemainingCapacity--
		record.LastAcquireTick = time.DayTick
		record.TodayVisitCount++
		record.AcquireCount++
		record.VisitCount++

		// Route to home floor. Binary state-0x20 dispatch (1228:41cb).
		routeResult := ResolveSimRouteBetweenFloors(world, sim, LobbyFloor, sim.FloorAnchor,
			b2Bool01(sim.FloorAnchor > LobbyFloor), time, nil)
		if routeResult == -1 {
			// Binary 1228:4297 route_result==-1 handler:
			//   UNTENANTED (1228:42d8): clear sim[+9]/[+0xc]/[+0xe] and revert
			//   try_consume via 11b0:11de; sim stays in MORNING_GATE.
			//   TENANTED (1228:45dd): sim[+5]=0x27 (PARKED).
			if record.AvailabilityState == VenueDormant {
				sim.TripCount = 0
				sim.ElapsedTicks = 0
				sim.AccumulatedTicks = 0
				record.RemainingCapacity++
				record.TodayVisitCount--
				record.AcquireCount--
				record.VisitCount--
			} else {
				sim.StateCode = StateParked
			}
			return
		}
		if sim.FamilyCode == FamilyRetail && record.AvailabilityState == VenueDormant {
			ActivateRetailShop(world, object, record, ledger)
		}
		if routeResult == 3 {
			// Same-floor venue: binary state-0x20 handler at rc=3
			// (1228:4415 retail / 1228:4b37 ff) calls acquire immediately.
			sim.DestinationFloor = -1
			sim.SelectedFloor = sim.FloorAnchor
			acquireResult := TryAcquireOfficeVenueSlot(record, sim, time, sim.FamilyCode)
			if acquireResult == VenueSlotFull {
				sim.StateCode = StateMorningTransit
			} else {
				sim.StateCode = StateDeparture
				sim.ElapsedTicks = 0
				sim.LastDemandTick = time.DayTick
			}
		} else {
			sim.StateCode = StateMorningTransit
		}
		return
	}

	// --- Departure ---
	// Binary state-5 handler (1228:4517 retail / 1228:4bd7 ff+restaurant).
	// release_commercial_venue_slot (11b0:0fae) gate:
	// g_day_tick - sim[+0x0a] < service_duration -> still dwelling.
	if state == StateDeparture {
		if time.DayTick-sim.LastDemandTick < CommercialVenueDwellTicks {
			return
		}
		// release_commercial_venue_slot decrements record+9 on success.
		if object.LinkedRecordIndex >= 0 {
			venue := b2CommercialVenueAt(world, object.LinkedRecordIndex)
			if venue != nil && venue.AvailabilityState != VenueDormant &&
				venue.AvailabilityState != VenueClosed && venue.CurrentPopulation > 0 {
				venue.CurrentPopulation--
			}
		}
		// src = sim+7, tgt = LOBBY. rc=-1 / rc=3 -> fail tail (1228:45a8 / 1228:4c68).
		sourceFloor := sim.SelectedFloor
		routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor,
			b2Bool01(LobbyFloor > sourceFloor), time, nil)
		if routeResult == -1 || routeResult == 3 {
			// Binary 1228:4c9e: rc=-1 and rc=3 route to the PARKED (0x27) writer.
			ReleaseServiceRequest(world, sim)
			sim.StateCode = StateParked
			// clamp_object_type_limit fires on the rc=3 path only
			// (1228:4cbf -> 1228:4d24).
			if routeResult == 3 && object.LinkedRecordIndex >= 0 {
				if venue := b2CommercialVenueAt(world, object.LinkedRecordIndex); venue != nil {
					IncrementVenueSeed(venue, sim.FamilyCode, sim, world, time)
				}
			}
			return
		}
		sim.StateCode = StateDepartureTransit
		return
	}

	// --- Per-stride in-transit handlers ---
	// Binary jump table aliases: 0x60 -> state 0x20 handler; 0x45 -> 0x05.
	if state == StateMorningTransit {
		handleCommercialMorningTransit(world, ledger, time, sim)
		return
	}
	if state == StateDepartureTransit {
		handleCommercialDepartureTransit(world, time, sim)
		return
	}
}

// handleCommercialMorningTransit TS: sims/commercial.ts handleCommercialMorningTransit.
// Binary alias of state 0x20 (1228:41cb retail / 1228:495c ff+restaurant)
// with emit_distance_feedback = 0.
func handleCommercialMorningTransit(world *WorldState, ledger *LedgerState, time *TimeState, sim *SimRecord) {
	object := FindObjectForSim(world, sim)
	sourceFloor := sim.SelectedFloor
	targetFloor := sim.FloorAnchor
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, targetFloor,
		b2Bool01(targetFloor > sourceFloor), time, resolveOptsFeedback(false))
	if routeResult == -1 {
		// Binary 1228:4297 -1 path: TENANTED venue -> STATE_PARKED.
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
		return
	}
	if routeResult == 3 {
		// Arrived at home venue floor (1228:4415 retail / 1228:4b37 ff).
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		// Activate retail shop before acquire (binary 1228:42d0).
		var venue *CommercialVenueRecord
		if object != nil && object.LinkedRecordIndex >= 0 {
			if candidate := b2CommercialVenueAt(world, object.LinkedRecordIndex); candidate != nil {
				venue = candidate
				if sim.FamilyCode == FamilyRetail && venue.AvailabilityState == VenueDormant {
					ActivateRetailShop(world, object, venue, ledger)
				}
			}
		}
		if venue == nil {
			sim.StateCode = StateDeparture
			sim.ElapsedTicks = 0
			sim.LastDemandTick = time.DayTick
			return
		}
		acquireResult := TryAcquireOfficeVenueSlot(venue, sim, time, sim.FamilyCode)
		if acquireResult == VenueSlotFull {
			// Stay in STATE_MORNING_TRANSIT; next stride retries acquire.
			return
		}
		sim.StateCode = StateDeparture
		sim.ElapsedTicks = 0
		sim.LastDemandTick = time.DayTick
	}
	// rc=0/1/2: stay in STATE_MORNING_TRANSIT.
}

// handleCommercialDepartureTransit TS: sims/commercial.ts handleCommercialDepartureTransit.
// Binary alias of state 0x05 (1228:4517 retail / 1228:4bd7 ff+restaurant)
// with emit_distance_feedback = 0.
func handleCommercialDepartureTransit(world *WorldState, time *TimeState, sim *SimRecord) {
	sourceFloor := sim.SelectedFloor
	routeResult := ResolveSimRouteBetweenFloors(world, sim, sourceFloor, LobbyFloor,
		b2Bool01(LobbyFloor > sourceFloor), time, resolveOptsFeedback(false))
	if routeResult == -1 {
		// Binary 1228:4c9e: state-0x45 rc=-1 writes PARKED (0x27).
		ReleaseServiceRequest(world, sim)
		sim.StateCode = StateParked
		return
	}
	if routeResult == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
		// clamp_object_type_limit after the rc=3 state->0x27 transition
		// (1228:4cbf -> 1228:4d24).
		object := FindObjectForSim(world, sim)
		if object != nil && object.LinkedRecordIndex >= 0 {
			if venue := b2CommercialVenueAt(world, object.LinkedRecordIndex); venue != nil {
				IncrementVenueSeed(venue, sim.FamilyCode, sim, world, time)
			}
		}
	}
	// rc=0/1/2: stay in STATE_DEPARTURE_TRANSIT.
}

// HandleCommercialSimArrival TS: sims/commercial.ts handleCommercialSimArrival.
// Carrier-arrival entry (dispatch_destination_queue_entries 1218:0883).
func HandleCommercialSimArrival(world *WorldState, sim *SimRecord, arrivalFloor int, time *TimeState) {
	if sim.StateCode == StateMorningTransit && arrivalFloor == sim.FloorAnchor {
		// Same-floor resolve branch (1218:0046) advances trip counters
		// before rc=3; dispatchSimArrival shortcuts the per-stride re-entry.
		AdvanceSimTripCounters(sim)
		sim.DestinationFloor = -1
		sim.SelectedFloor = sim.FloorAnchor
		object := FindObjectForSim(world, sim)
		var venue *CommercialVenueRecord
		if object != nil && object.LinkedRecordIndex >= 0 {
			venue = b2CommercialVenueAt(world, object.LinkedRecordIndex)
		}
		if venue == nil {
			sim.StateCode = StateDeparture
			sim.ElapsedTicks = 0
			sim.LastDemandTick = time.DayTick
			return
		}
		acquireResult := TryAcquireOfficeVenueSlot(venue, sim, time, sim.FamilyCode)
		if acquireResult == VenueSlotFull {
			// Stay in STATE_MORNING_TRANSIT; next stride re-attempts acquire.
			sim.DestinationFloor = -1
			return
		}
		sim.StateCode = StateDeparture
		sim.ElapsedTicks = 0
		sim.LastDemandTick = time.DayTick
		return
	}

	if sim.StateCode == StateDepartureTransit && arrivalFloor == LobbyFloor {
		// Binary 1228:4bd7 (state 0x45 alias of 0x05): same-floor resolve
		// branch (1218:0046) advances trip counters.
		AdvanceSimTripCounters(sim)
		sim.StateCode = StateParked
		sim.SelectedFloor = LobbyFloor
		ReleaseServiceRequest(world, sim)
		// clamp_object_type_limit on the carrier-arrival rc=3 path
		// (dispatch_sim_behavior 1228:1ba6 case 6/10/12).
		object := FindObjectForSim(world, sim)
		if object != nil && object.LinkedRecordIndex >= 0 {
			if venue := b2CommercialVenueAt(world, object.LinkedRecordIndex); venue != nil {
				IncrementVenueSeed(venue, sim.FamilyCode, sim, world, time)
			}
		}
		return
	}

	// Fallback: park
	sim.StateCode = StateNightB
	ClearSimRoute(sim)
}
