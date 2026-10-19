package sim

// Port of reference/tower-together/sim/entertainment.ts: cinema / party hall
// link state machines - budget seeding, phase advance, attendance payouts.

// entertainmentCinemaFamilyCodes TS: entertainment.ts CINEMA_FAMILY_CODES
// (lookup only). Family codes emitted by cinema placement; all 4 share one
// sidecar. (commands.go has its own copy of the commands.ts set.)
var entertainmentCinemaFamilyCodes = map[int]bool{
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
}

// pairedBudget TS: entertainment.ts pairedBudget. Tiers indexed by
// trunc(linkAgeCounter / 3): selectors 0..6 -> [40,40,40,20],
// selectors 7..13 -> [60,60,40,20].
func pairedBudget(linkAgeCounter, selector int) int {
	ageTier := min(3, linkAgeCounter/3)
	lowSelectorTable := []int{40, 40, 40, 20}
	highSelectorTable := []int{60, 60, 40, 20}
	table := highSelectorTable
	if selector >= 0 && selector < 7 {
		table = lowSelectorTable
	}
	// `table[ageTier] ?? table[table.length - 1] ?? 20`
	if ageTier >= 0 && ageTier < len(table) {
		return table[ageTier]
	}
	return table[len(table)-1]
}

// isPairedSidecar TS: entertainment.ts isPairedSidecar. Paired (cinema) iff
// the selector is a real bucket; party-hall records store 0xff.
func isPairedSidecar(sidecar *EntertainmentLinkRecord) bool {
	return sidecar.FamilySelectorOrSingleLinkFlag != 0xff
}

// simMatchesEntertainmentSidecar TS: entertainment.ts simMatchesEntertainmentSidecar
func simMatchesEntertainmentSidecar(sim *SimRecord, sidecar *EntertainmentLinkRecord) bool {
	return sim.HomeColumn == sidecar.OwnerSubtypeIndex || sim.HomeColumn == sidecar.OwnerSubtypeIndex+7
}

// liveEntertainmentLinks is the recurring TS loop head
// `for (const sidecar of world.sidecars) { if kind !== entertainment_link
// continue; if ownerSubtypeIndex === 0xff continue; ... }`, in sidecar order.
func liveEntertainmentLinks(world *WorldState) []*EntertainmentLinkRecord {
	var out []*EntertainmentLinkRecord
	for _, sc := range world.Sidecars {
		link, ok := sc.(*EntertainmentLinkRecord)
		if !ok || link == nil || link.OwnerSubtypeIndex == 0xff {
			continue
		}
		out = append(out, link)
	}
	return out
}

// b2AddIncome is `ledger.incomeLedger[i] = (ledger.incomeLedger[i] ?? 0) + amount`.
// PORT-NOTE: a fresh ledger has 10 income slots, so the JS write at index 18
// or 29 extends the array (holes read as undefined -> 0). Go grows the slice
// with zeros, which reads identically.
func b2AddIncome(ledger *LedgerState, index, amount int) {
	for len(ledger.IncomeLedger) <= index {
		ledger.IncomeLedger = append(ledger.IncomeLedger, 0)
	}
	ledger.IncomeLedger[index] += amount
}

// SeedEntertainmentBudgets TS: entertainment.ts seedEntertainmentBudgets
// (binary rebuild_entertainment_family_ledger 1188:05af). Called at tick
// 0x0F0. Does NOT clear link_phase_state (mirrors the binary).
func SeedEntertainmentBudgets(world *WorldState) {
	cinemaPopulation := 0
	partyHallPopulation := 0
	for _, sidecar := range liveEntertainmentLinks(world) {
		sidecar.AttendanceCounter = 0
		sidecar.ActiveRuntimeCount = 0
		sidecar.PendingTransitionFlag = 0
		sidecar.LinkAgeCounter = min(0x7f, sidecar.LinkAgeCounter+1)

		if isPairedSidecar(sidecar) {
			budget := pairedBudget(sidecar.LinkAgeCounter, sidecar.FamilySelectorOrSingleLinkFlag)
			sidecar.UpperBudget = budget
			sidecar.LowerBudget = budget
			cinemaPopulation += sidecar.UpperBudget + sidecar.LowerBudget
		} else {
			sidecar.UpperBudget = 0
			sidecar.LowerBudget = 50
			partyHallPopulation += sidecar.LowerBudget
		}
	}

	ClearPopulationBucket(world, FamilyCinema)
	if cinemaPopulation > 0 {
		AddToPopulationBucket(world, FamilyCinema, cinemaPopulation)
	}
	ClearPopulationBucket(world, FamilyPartyHall)
	if partyHallPopulation > 0 {
		AddToPopulationBucket(world, FamilyPartyHall, partyHallPopulation)
	}
}

// ActivateEntertainmentUpperHalf TS: entertainment.ts activateEntertainmentUpperHalf.
// Sets upper phase to 1 for idle paired links and seeds the matching cinema
// sims to STATE_MORNING_GATE.
func ActivateEntertainmentUpperHalf(world *WorldState) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if !isPairedSidecar(sidecar) {
			continue
		}
		if sidecar.LinkPhaseState == 0 {
			sidecar.LinkPhaseState = 1
		}
		for _, sim := range world.Sims {
			if sim.FamilyCode != FamilyCinema {
				continue
			}
			if !simMatchesEntertainmentSidecar(sim, sidecar) {
				continue
			}
			sim.StateCode = StateMorningGate
		}
	}
}

// PromoteCinemaAndActivatePartyHall TS: entertainment.ts
// promoteCinemaAndActivatePartyHall. Tick 0x4B0 (1200): paired links
// phase 2 -> 3; party hall links 0 -> 1.
// PORT-NOTE: the TS doc comment also mentions seeding lower-half sims to
// 0x20, but the TS body does not; ported as the body reads.
func PromoteCinemaAndActivatePartyHall(world *WorldState) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if isPairedSidecar(sidecar) {
			if sidecar.LinkPhaseState == 2 {
				sidecar.LinkPhaseState = 3
			}
		} else {
			if sidecar.LinkPhaseState == 0 {
				sidecar.LinkPhaseState = 1
			}
		}
	}
}

// PromoteCinemaLinksToReadyPhase TS: entertainment.ts
// promoteCinemaLinksToReadyPhase (binary
// promote_entertainment_links_to_ready_phase(half=1, paired=1) 1188:0826),
// checkpoint 0x640. The audience-sim +0x13 flag write is not mirrored (TS).
func PromoteCinemaLinksToReadyPhase(world *WorldState) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if !isPairedSidecar(sidecar) {
			continue
		}
		if sidecar.LinkPhaseState >= 2 {
			sidecar.LinkPhaseState = 3
		}
	}
}

// ActivateEntertainmentLowerHalf TS: entertainment.ts activateEntertainmentLowerHalf
func ActivateEntertainmentLowerHalf(world *WorldState) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if !isPairedSidecar(sidecar) {
			continue
		}
		if sidecar.LinkPhaseState == 1 {
			sidecar.LinkPhaseState = 2
		}
	}
}

// movieTheaterPayout TS: entertainment.ts movieTheaterPayout
func movieTheaterPayout(attendance int) int {
	if attendance >= 100 {
		return 15_000
	}
	if attendance >= 80 {
		return 10_000
	}
	if attendance >= 40 {
		return 2_000
	}
	return 0
}

// AdvanceEntertainmentUpperPhase TS: entertainment.ts advanceEntertainmentUpperPhase
func AdvanceEntertainmentUpperPhase(world *WorldState) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if !isPairedSidecar(sidecar) {
			continue
		}
		if sidecar.LinkPhaseState < 1 {
			continue
		}

		sidecar.ActiveRuntimeCount = max(0, sidecar.ActiveRuntimeCount-sidecar.UpperBudget)
		if sidecar.ActiveRuntimeCount == 0 {
			sidecar.LinkPhaseState = 1
		} else {
			sidecar.LinkPhaseState = 2
		}

		for _, sim := range world.Sims {
			if !entertainmentCinemaFamilyCodes[sim.FamilyCode] {
				continue
			}
			if !simMatchesEntertainmentSidecar(sim, sidecar) {
				continue
			}
			if sim.StateCode == StateArrived {
				sim.StateCode = StateActive
			}
		}
	}
}

// isCalendarEdgePayoutSkipDay TS: entertainment.ts isCalendarEdgePayoutSkipDay
// (entry check of accrue_facility_income_by_family 1180:12e7).
func isCalendarEdgePayoutSkipDay(dayCounter int) bool {
	return dayCounter%60 == 59 || dayCounter%84 == 83
}

// AdvancePartyHallPhaseAndAccrue TS: entertainment.ts advancePartyHallPhaseAndAccrue
// (binary advance_entertainment_facility_phase(half=1, paired=0), checkpoint 0x640).
func AdvancePartyHallPhaseAndAccrue(world *WorldState, ledger *LedgerState, dayCounter int) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if isPairedSidecar(sidecar) {
			continue
		}

		previousPhaseState := sidecar.LinkPhaseState
		sidecar.LinkPhaseState = 0

		for _, sim := range world.Sims {
			if sim.FamilyCode != FamilyPartyHallLower {
				continue
			}
			if !simMatchesEntertainmentSidecar(sim, sidecar) {
				continue
			}
			if sim.StateCode == StateArrived {
				sim.StateCode = StateDeparture
				sidecar.ActiveRuntimeCount = max(0, sidecar.ActiveRuntimeCount-1)
			}
		}

		if previousPhaseState >= 1 && sidecar.AttendanceCounter > 0 {
			if isCalendarEdgePayoutSkipDay(dayCounter) {
				continue
			}
			payout := 20_000
			ledger.CashBalance = min(99_999_999, ledger.CashBalance+payout)
			b2AddIncome(ledger, FamilyPartyHall, payout)
		}
	}
}

// AdvanceEntertainmentLowerPairedPhaseAndAccrue TS: entertainment.ts
// advanceEntertainmentLowerPairedPhaseAndAccrue (binary
// advance_entertainment_facility_phase(half=1, paired=1), checkpoint 0x76c).
func AdvanceEntertainmentLowerPairedPhaseAndAccrue(world *WorldState, ledger *LedgerState, dayCounter int) {
	for _, sidecar := range liveEntertainmentLinks(world) {
		if !isPairedSidecar(sidecar) {
			continue
		}

		previousPhaseState := sidecar.LinkPhaseState
		sidecar.LinkPhaseState = 0

		for _, sim := range world.Sims {
			if !entertainmentCinemaFamilyCodes[sim.FamilyCode] {
				continue
			}
			if !simMatchesEntertainmentSidecar(sim, sidecar) {
				continue
			}
			if sim.StateCode == StateArrived {
				sim.StateCode = StateDeparture
				sidecar.ActiveRuntimeCount = max(0, sidecar.ActiveRuntimeCount-1)
			}
		}

		if previousPhaseState >= 1 {
			if isCalendarEdgePayoutSkipDay(dayCounter) {
				continue
			}
			payout := movieTheaterPayout(sidecar.AttendanceCounter)
			if payout > 0 {
				ledger.CashBalance = min(99_999_999, ledger.CashBalance+payout)
				b2AddIncome(ledger, FamilyCinema, payout)
			}
		}
	}
}
