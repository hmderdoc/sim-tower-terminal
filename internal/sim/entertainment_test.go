package sim

import "testing"

// B2 unit tests for entertainment.ts.

func b2EntWorld(t *testing.T) (*WorldState, *EntertainmentLinkRecord, *EntertainmentLinkRecord) {
	w := b2World(t)
	if w.CurrentPopulationBuckets == nil {
		w.CurrentPopulationBuckets = map[int]int{}
	}
	cinema := &EntertainmentLinkRecord{OwnerSubtypeIndex: 50, FamilySelectorOrSingleLinkFlag: 3, PairedSubtypeIndex: 0xff}
	party := &EntertainmentLinkRecord{OwnerSubtypeIndex: 120, FamilySelectorOrSingleLinkFlag: 0xff, PairedSubtypeIndex: 0xff}
	dead := &EntertainmentLinkRecord{OwnerSubtypeIndex: 0xff, FamilySelectorOrSingleLinkFlag: 3}
	w.Sidecars = append(w.Sidecars, cinema, &CommercialVenueRecord{}, party, dead)
	return w, cinema, party
}

func TestPairedBudgetTiers(t *testing.T) {
	cases := []struct{ age, sel, want int }{
		{0, 0, 40}, {5, 6, 40}, {8, 6, 40}, {9, 6, 20}, {100, 6, 20},
		{0, 7, 60}, {5, 13, 60}, {6, 13, 40}, {9, 13, 20},
	}
	for _, c := range cases {
		if got := pairedBudget(c.age, c.sel); got != c.want {
			t.Errorf("pairedBudget(%d,%d) = %d, want %d", c.age, c.sel, got, c.want)
		}
	}
}

func TestSeedEntertainmentBudgets(t *testing.T) {
	skipOnTODO(t, func() {
		w, cinema, party := b2EntWorld(t)
		cinema.AttendanceCounter, cinema.ActiveRuntimeCount, cinema.LinkAgeCounter = 9, 9, 0
		party.LinkAgeCounter = 0x7f
		startPop := w.CurrentPopulation
		SeedEntertainmentBudgets(w)
		if cinema.LinkAgeCounter != 1 || cinema.UpperBudget != 40 || cinema.LowerBudget != 40 ||
			cinema.AttendanceCounter != 0 || cinema.ActiveRuntimeCount != 0 {
			t.Fatalf("cinema = %+v", cinema)
		}
		if party.LinkAgeCounter != 0x7f || party.UpperBudget != 0 || party.LowerBudget != 50 {
			t.Fatalf("party = %+v", party)
		}
		if w.CurrentPopulationBuckets[FamilyCinema] != 80 || w.CurrentPopulationBuckets[FamilyPartyHall] != 50 ||
			w.CurrentPopulation != startPop+130 {
			t.Fatalf("buckets %v pop %d", w.CurrentPopulationBuckets, w.CurrentPopulation)
		}
		// Re-seeding nets out (clear then add).
		SeedEntertainmentBudgets(w)
		if w.CurrentPopulation != startPop+130 {
			t.Fatalf("reseed pop %d", w.CurrentPopulation)
		}
	})
}

func TestEntertainmentPhaseCycle(t *testing.T) {
	w, cinema, party := b2EntWorld(t)
	guest := b2Sim(20, 57, FamilyCinemaLower, StateParked) // homeColumn = owner + 7
	upper := b2Sim(21, 50, FamilyCinema, StateParked)
	w.Sims = append(w.Sims, guest, upper)

	ActivateEntertainmentUpperHalf(w)
	if cinema.LinkPhaseState != 1 || upper.StateCode != StateMorningGate || guest.StateCode != StateParked {
		t.Fatalf("upper activation: phase=%d upper=%#x guest=%#x", cinema.LinkPhaseState, upper.StateCode, guest.StateCode)
	}
	if party.LinkPhaseState != 0 {
		t.Fatal("party hall untouched by upper activation")
	}
	ActivateEntertainmentLowerHalf(w)
	if cinema.LinkPhaseState != 2 {
		t.Fatalf("lower activation phase = %d", cinema.LinkPhaseState)
	}
	PromoteCinemaAndActivatePartyHall(w)
	if cinema.LinkPhaseState != 3 || party.LinkPhaseState != 1 {
		t.Fatalf("promote: cinema=%d party=%d", cinema.LinkPhaseState, party.LinkPhaseState)
	}

	cinema.ActiveRuntimeCount, cinema.UpperBudget = 5, 3
	guest.StateCode = StateArrived
	AdvanceEntertainmentUpperPhase(w)
	if cinema.ActiveRuntimeCount != 2 || cinema.LinkPhaseState != 2 || guest.StateCode != StateActive {
		t.Fatalf("upper advance: %+v guest=%#x", cinema, guest.StateCode)
	}
}

func TestEntertainmentPayouts(t *testing.T) {
	w, cinema, party := b2EntWorld(t)
	ledger := CreateLedgerState(1000)
	partyGuest := b2Sim(30, 120, FamilyPartyHallLower, StateArrived)
	cinemaGuest := b2Sim(20, 50, FamilyCinemaStairsUpper, StateArrived)
	w.Sims = append(w.Sims, partyGuest, cinemaGuest)

	party.LinkPhaseState, party.AttendanceCounter, party.ActiveRuntimeCount = 1, 4, 1
	AdvancePartyHallPhaseAndAccrue(w, ledger, 10)
	if ledger.CashBalance != 21_000 || ledger.IncomeLedger[FamilyPartyHall] != 20_000 {
		t.Fatalf("party payout: cash=%d income=%v", ledger.CashBalance, ledger.IncomeLedger)
	}
	if party.LinkPhaseState != 0 || party.ActiveRuntimeCount != 0 || partyGuest.StateCode != StateDeparture {
		t.Fatalf("party drain: %+v guest=%#x", party, partyGuest.StateCode)
	}

	// Calendar-edge day: state reset still happens, no cash.
	cinema.LinkPhaseState, cinema.AttendanceCounter = 3, 85
	AdvanceEntertainmentLowerPairedPhaseAndAccrue(w, ledger, 59)
	if ledger.CashBalance != 21_000 || cinema.LinkPhaseState != 0 || cinemaGuest.StateCode != StateDeparture {
		t.Fatalf("skip day: cash=%d phase=%d", ledger.CashBalance, cinema.LinkPhaseState)
	}
	cinema.LinkPhaseState = 1
	AdvanceEntertainmentLowerPairedPhaseAndAccrue(w, ledger, 60)
	if ledger.CashBalance != 31_000 || ledger.IncomeLedger[FamilyCinema] != 10_000 {
		t.Fatalf("cinema payout: cash=%d", ledger.CashBalance)
	}
	for _, c := range []struct{ att, want int }{{39, 0}, {40, 2000}, {80, 10_000}, {100, 15_000}} {
		if got := movieTheaterPayout(c.att); got != c.want {
			t.Errorf("movieTheaterPayout(%d) = %d", c.att, got)
		}
	}
	if !isCalendarEdgePayoutSkipDay(83) || !isCalendarEdgePayoutSkipDay(119) || isCalendarEdgePayoutSkipDay(60) {
		t.Error("calendar edge days")
	}
}
