package sim

import "testing"

func TestCashflowFromFamilyResource(t *testing.T) {
	l := CreateLedgerState(1000)
	AddCashflowFromFamilyResource(l, "hotelTwin", 3, FamilyHotelTwin)
	if l.CashBalance != 1800 || l.IncomeLedger[ledgerHotelTwin] != 800 {
		t.Fatalf("twin tier3: cash %d income %d", l.CashBalance, l.IncomeLedger[ledgerHotelTwin])
	}
	AddCashflowFromFamilyResource(l, "hotelSingle", 4, FamilyHotelSingle) // rent 4 clamps to tier 3
	if l.CashBalance != 2300 {
		t.Fatalf("single rent 4: cash %d", l.CashBalance)
	}
	RemoveCashflowFromFamilyResource(l, "condo", 1, FamilyCondo)
	if l.CashBalance != 0 || l.IncomeLedger[ledgerCondo] != 0 {
		t.Fatalf("condo refund clamps: cash %d", l.CashBalance)
	}
	AddCashflowFromFamilyResource(l, "nope", 0, FamilyOffice)
	l.CashBalance = cashCap - 10
	AddCashflowFromFamilyResource(l, "office", 0, FamilyOffice)
	if l.CashBalance != cashCap {
		t.Fatalf("cap: %d", l.CashBalance)
	}
}

func TestExpenseSweepAndRollover(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	w.StarCount = 3
	// Lobby run (family 0x18 / parking rate): width 10 * 30 / 10 = 30 -> $30,000.
	w.PlacedObjects.Set(cellKey(100, GroundY), &PlacedObjectRecord{LeftTileIndex: 100, RightTileIndex: 109, ObjectTypeCode: 0x18})
	w.PlacedObjects.Set(cellKey(120, GroundY-1), &PlacedObjectRecord{LeftTileIndex: 120, RightTileIndex: 135, ObjectTypeCode: FamilySecurity})
	w.Carriers = []*CarrierRecord{testCarrier(50, 0, 10, 20, &CarrierCar{Active: true}, &CarrierCar{Active: true}, &CarrierCar{})}
	w.SpecialLinks[0] = &SpecialLinkSegment{Active: true, Flags: 2 << 1} // escalator, 2 units
	w.SpecialLinks[1] = &SpecialLinkSegment{Active: true, Flags: 1}      // stairs: rate 0
	l := CreateLedgerState(1_000_000)
	DoExpenseSweep(l, w)
	want := 1_000_000 - 30_000 - 20_000 - 2*20_000 - 2*5_000
	if l.CashBalance != want {
		t.Fatalf("cash %d want %d", l.CashBalance, want)
	}
	if l.ExpenseLedger[0x18] != 30_000 || l.ExpenseLedger[42] != 40_000 || l.ExpenseLedger[27] != 10_000 {
		t.Fatalf("expense ledger %v %v %v", l.ExpenseLedger[0x18], l.ExpenseLedger[42], l.ExpenseLedger[27])
	}
	DoLedgerRollover(l, w, 4)
	if l.ExpenseLedger[42] == 0 {
		t.Fatal("rollover on non-boundary day")
	}
	DoLedgerRollover(l, w, 6)
	if l.ExpenseLedger[42] != 0 || l.CashBalanceCycleBase != l.CashBalance {
		t.Fatal("rollover did not reset")
	}
}

func TestRebuildFacilityLedgerAndThreeDayCashflow(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	office := &PlacedObjectRecord{ObjectTypeCode: FamilyOffice, AuxValueOrTimer: 1, OccupiedFlag: 1, RentLevel: 1}
	unrented := &PlacedObjectRecord{ObjectTypeCode: FamilyOffice, OccupiedFlag: 1, RentLevel: 1}
	w.Sidecars = SidecarList{&CommercialVenueRecord{AvailabilityState: VenueDormant}, &CommercialVenueRecord{AvailabilityState: VenuePartial}}
	dormantRetail := &PlacedObjectRecord{ObjectTypeCode: FamilyRetail, OccupiedFlag: 1, RentLevel: 0, LinkedRecordIndex: 0}
	retail := &PlacedObjectRecord{ObjectTypeCode: FamilyRetail, OccupiedFlag: 1, RentLevel: 0, LinkedRecordIndex: 1}
	w.PlacedObjects.Set("1,100", office)
	w.PlacedObjects.Set("20,100", unrented)
	w.PlacedObjects.Set("40,100", dormantRetail)
	w.PlacedObjects.Set("60,100", retail)
	l := CreateLedgerState(0)
	RebuildFacilityLedger(l, w)
	if l.PopulationLedger[ledgerOffice] != 12 || l.PopulationLedger[ledgerRetail] != 96 {
		t.Fatalf("population ledger %v", l.PopulationLedger)
	}
	ActivateThreeDayCashflow(w, l, 5)
	if l.CashBalance != 10_000+20_000 || office.AuxValueOrTimer != 6 || retail.AuxValueOrTimer != 6 || dormantRetail.AuxValueOrTimer != 0 {
		t.Fatalf("cash %d office aux %d", l.CashBalance, office.AuxValueOrTimer)
	}
	ActivateThreeDayCashflow(w, l, 5) // guarded
	if l.CashBalance != 30_000 {
		t.Fatalf("guard failed: %d", l.CashBalance)
	}
}
