package sim

// Port of reference/tower-together/sim/tick/day-scheduler.ts
// (1208:0196 run_simulation_day_scheduler) and its re-export shim
// scheduler.ts (no Go file needed: same package).
//
// Advances g_day_tick and fires the checkpoint handlers. The event rolls
// (VIP visitor, bomb, fire, random news) are free-standing calls in the same
// relative order as the TS.

// SimState is the scheduler's state bundle. TS: tick/day-scheduler.ts SimState
type SimState struct {
	Time   *TimeState
	World  *WorldState
	Ledger *LedgerState
}

// ─── Checkpoint bodies ────────────────────────────────────────────────────────

// checkpointStartOfDay (0x000). TS: tick/day-scheduler.ts checkpointStartOfDay
func checkpointStartOfDay(s *SimState) {
	// update_periodic_facility_progress_override (1020:0df8): every 8th day
	// (dayCounter % 8 == 4) below 5 stars, enable the override seed slot and
	// set bit 0x10 in g_game_state_flags. Cleared at 0x640 by
	// clear_facility_progress_override (1020:0e1c).
	if s.Time.DayCounter%8 == 4 &&
		s.World.StarCount < 5 &&
		s.World.EventState.GameStateFlags&0x10 == 0 {
		s.World.GateFlags.FacilityProgressOverride = 1
		s.World.EventState.GameStateFlags |= 0x10
	}
	// Medical daily flag: latched to 1 at day-start when starCount > 2.
	if s.World.StarCount > 2 {
		s.World.GateFlags.OfficeServiceOkMedical = 1
	}
	// Activate cathedral guest sims.
	ActivateEvalSims(s.World)
}

// checkpointRecyclingReset (0x020). TS: tick/day-scheduler.ts checkpointRecyclingReset
func checkpointRecyclingReset(s *SimState) {
	ResetRecyclingCenterDutyTier(s.World)
}

// checkpointFacilityLedgerRebuild (0x0f0). TS: tick/day-scheduler.ts checkpointFacilityLedgerRebuild
func checkpointFacilityLedgerRebuild(s *SimState) {
	CheckDailyEvents(s.World, s.Ledger, s.Time)
	RebuildFacilityLedger(s.Ledger, s.World)
	RebuildCommercialVenueRuntime(s.World, s.Time)
	SeedEntertainmentBudgets(s.World)
}

// checkpointEntertainmentHalf1 (0x3e8): activate_entertainment_link_half_runtime_phase.
// TS: tick/day-scheduler.ts checkpointEntertainmentHalf1
func checkpointEntertainmentHalf1(s *SimState) {
	ActivateEntertainmentUpperHalf(s.World)
}

// checkpointHotelSaleReset (0x4b0). TS: tick/day-scheduler.ts checkpointHotelSaleReset
func checkpointHotelSaleReset(s *SimState) {
	s.World.GateFlags.Family345SaleCount = 0
	DispatchEvalMiddayReturn(s.World)
	PromoteCinemaAndActivatePartyHall(s.World)
	// Lookup-only count map (TS Map, never iterated).
	hotelFamilyCounts := map[int]int{}
	for _, object := range s.World.PlacedObjects.Values() {
		if object.ObjectTypeCode == FamilyHotelSingle || object.ObjectTypeCode == FamilyHotelTwin {
			hotelFamilyCounts[object.ObjectTypeCode]++
		}
	}
	if len(s.World.Carriers) == 0 {
		for _, sim := range s.World.Sims {
			if sim.StateCode == StateHotelParked &&
				(sim.FamilyCode == FamilyHotelSingle || sim.FamilyCode == FamilyHotelTwin) &&
				hotelFamilyCounts[sim.FamilyCode] == 1 {
				sim.StateCode = StateMorningGate
			}
		}
	}
}

// checkpointEntertainmentHalf2 (0x578). TS: tick/day-scheduler.ts checkpointEntertainmentHalf2
func checkpointEntertainmentHalf2(s *SimState) {
	ActivateEntertainmentLowerHalf(s.World)
}

// checkpointEntertainmentPhase1 (0x5dc). TS: tick/day-scheduler.ts checkpointEntertainmentPhase1
func checkpointEntertainmentPhase1(s *SimState) {
	AdvanceEntertainmentUpperPhase(s.World)
}

// checkpointMidday (0x640). TS: tick/day-scheduler.ts checkpointMidday
func checkpointMidday(s *SimState) {
	// clear_facility_progress_override (1020:0e1c), gated on bit 0x10.
	if s.World.EventState.GameStateFlags&0x10 != 0 {
		s.World.GateFlags.FacilityProgressOverride = 0
		s.World.EventState.GameStateFlags &^= 0x10
	}
	// 0. rebuild_type6_facility_records: restaurant per-cycle seeding.
	RebuildRestaurantFacilityRecords(s.World, s.Time)
	// 1. Spread existing cockroach infestations.
	SpreadCockroachInfestation(s.World, s.Time)
	// 2. Hotel status + vacancy expiry + occupancy refresh.
	UpdateHotelOperationalAndOccupancy(s.World, s.Time)
	// 3. Normal midday tasks.
	AdvancePartyHallPhaseAndAccrue(s.World, s.Ledger, s.Time.DayCounter)
	// promote_entertainment_links_to_ready_phase(1, 1): cinema link phase 2 -> 3.
	PromoteCinemaLinksToReadyPhase(s.World)
	UpdateRecyclingCenterState(s.World, s.Ledger, 0)
	// 4. advance_object_stay_phase_tiers @ 1230:0b5f.
	AdvanceObjectStayPhaseTiers(s.World)
}

// checkpointNoop (0x708). TS: tick/day-scheduler.ts checkpointNoop
func checkpointNoop(_ *SimState) {}

// checkpointEntertainmentPhase2 (0x76c). TS: tick/day-scheduler.ts checkpointEntertainmentPhase2
func checkpointEntertainmentPhase2(s *SimState) {
	AdvanceEntertainmentLowerPairedPhaseAndAccrue(s.World, s.Ledger, s.Time.DayCounter)
}

// checkpointLateFacility (0x7d0). TS: tick/day-scheduler.ts checkpointLateFacility.
// tryAdvanceStarCount does not fire here; it runs every tick from carrierTick.
func checkpointLateFacility(s *SimState) {
	CloseCommercialVenuesByFamily(s.World, s.Ledger, FamilyRetail)
	CloseCommercialVenuesByFamily(s.World, s.Ledger, FamilyFastFood)
	UpdateRecyclingCenterState(s.World, s.Ledger, 2)
}

// checkpointType6Advance (0x898). TS: tick/day-scheduler.ts checkpointType6Advance
func checkpointType6Advance(s *SimState) {
	CloseCommercialVenuesByFamily(s.World, s.Ledger, FamilyRestaurant)
}

// checkpointDayCounter (0x8fc): no-op, AdvanceOneTick already did the
// dayCounter/weekendFlag update. TS: tick/day-scheduler.ts checkpointDayCounter
func checkpointDayCounter(_ *SimState) {}

// checkpointRuntimeRefresh (0x9c4). TS: tick/day-scheduler.ts checkpointRuntimeRefresh
func checkpointRuntimeRefresh(s *SimState) {
	RebuildAllSimTileSpans(s.World)
	ResetSimRuntimeState(s.World)
	NormalizeUnitStatusEndOfDay(s.World)
	// 1190:0977 dispatch_active_requests_by_family (once per day).
	DispatchActiveRequestsByFamily(s.World, s.Ledger, s.Time)
}

// checkpointLedgerRollover (0x9e5). TS: tick/day-scheduler.ts checkpointLedgerRollover
func checkpointLedgerRollover(s *SimState) {
	// Binary also calls reset_sim_runtime_state at checkpoint 0x9e5.
	ResetSimRuntimeState(s.World)
	DoLedgerRollover(s.Ledger, s.World, s.Time.DayCounter)
	RecomputeAllObjectOperationalStatus(s.World)
	if s.Time.DayCounter%3 == 0 {
		ActivateThreeDayCashflow(s.World, s.Ledger, s.Time.DayCounter)
		DoExpenseSweep(s.Ledger, s.World)
		RefundUnhappyFacilities(s.World, s.Ledger, s.Time)
		// activate_family_cashflow_if_operational resets office/condo trip
		// counters on the 3-day cycle (spec: 2533 step 2).
		for _, sim := range s.World.Sims {
			if sim.FamilyCode == FamilyOffice || sim.FamilyCode == FamilyCondo {
				ResetSimTripCounters(sim)
			}
		}
	}
}

// checkpointEndOfDay (0x9f6). TS: tick/day-scheduler.ts checkpointEndOfDay
func checkpointEndOfDay(s *SimState) {
	FlushCarriersEndOfDay(s.World)
}

// checkpointRecyclingFinal (0xa06). TS: tick/day-scheduler.ts checkpointRecyclingFinal
func checkpointRecyclingFinal(s *SimState) {
	UpdateRecyclingCenterState(s.World, s.Ledger, 5)
}

// ─── Checkpoint table ─────────────────────────────────────────────────────────

type dayCheckpoint struct {
	tick int
	fn   func(*SimState)
}

// dayCheckpoints TS: tick/day-scheduler.ts CHECKPOINTS (order is significant).
var dayCheckpoints = []dayCheckpoint{
	{0x000, checkpointStartOfDay},
	{0x020, checkpointRecyclingReset},
	{0x0f0, checkpointFacilityLedgerRebuild},
	{0x3e8, checkpointEntertainmentHalf1},
	{0x4b0, checkpointHotelSaleReset},
	{0x578, checkpointEntertainmentHalf2},
	{0x5dc, checkpointEntertainmentPhase1},
	{0x640, checkpointMidday},
	{0x708, checkpointNoop},
	{0x76c, checkpointEntertainmentPhase2},
	{0x7d0, checkpointLateFacility},
	{0x898, checkpointType6Advance},
	{0x8fc, checkpointDayCounter},
	{0x9c4, checkpointRuntimeRefresh},
	{0x9e5, checkpointLedgerRollover},
	{0x9f6, checkpointEndOfDay},
	{0x0a06, checkpointRecyclingFinal},
}

// ─── Dispatcher ───────────────────────────────────────────────────────────────

// RunCheckpoints fires every checkpoint whose tick falls in the half-open
// interval (prevTick, currTick], handling the day wrap (currTick < prevTick).
// TS: tick/day-scheduler.ts runCheckpoints
func RunCheckpoints(state *SimState, prevTick, currTick int) {
	wrapped := currTick < prevTick
	for _, cp := range dayCheckpoints {
		if wrapped {
			if cp.tick > prevTick || cp.tick <= currTick {
				cp.fn(state)
			}
		} else {
			if cp.tick > prevTick && cp.tick <= currTick {
				cp.fn(state)
			}
		}
	}
}

// RunSimulationDayScheduler TS: tick/day-scheduler.ts runSimulationDayScheduler
// (binary run_simulation_day_scheduler 1208:0196). Order:
//
//	advanceOneTick, triggerRandomNewsEvent, tickVipSpecialVisitor,
//	runCheckpoints, tickBombEvent, tickFireEvent
//
// Returns the advanced time (fresh pointer). The bomb/fire cleanup paths may
// fast-forward its dayTick in place, as the TS does.
func RunSimulationDayScheduler(world *WorldState, ledger *LedgerState, time *TimeState) *TimeState {
	prevTick := time.DayTick
	nextTime, _ := AdvanceOneTick(time)
	currTick := nextTime.DayTick

	TriggerRandomNewsEvent(world, nextTime)
	TickVipSpecialVisitor(world, nextTime)
	RunCheckpoints(&SimState{Time: nextTime, World: world, Ledger: ledger}, prevTick, currTick)

	TickBombEvent(world, ledger, nextTime)
	TickFireEvent(world, ledger, nextTime)

	return nextTime
}
