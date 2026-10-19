package sim

// Port of reference/tower-together/sim/ledger.ts: three-ledger money model.
//
// cashBalance          - live balance, capped at 99,999,999
// populationLedger[f]  - live per-family active-unit counts
// incomeLedger[f]      - income accumulated since last 3-day rollover
// expenseLedger[f]     - expenses accumulated since last 3-day rollover
// cashBalanceCycleBase - balance saved at each rollover
//
// YEN #1001 / #1002 values are in units of $1,000.

const (
	yenUnit = 1_000
	cashCap = 99_999_999
)

const (
	ledgerOffice      = 0
	ledgerHotelSingle = 1
	ledgerHotelTwin   = 2
	ledgerHotelSuite  = 3
	ledgerRetail      = 4
	ledgerFastFood    = 5
	ledgerRestaurant  = 6
	ledgerPartyHall   = 7
	ledgerCinema      = 8
	ledgerCondo       = 9
)

// LedgerState TS: ledger.ts LedgerState
type LedgerState struct {
	CashBalance int `json:"cashBalance"`
	// PopulationLedger: live per-family active-unit counts (10 ledger slots).
	PopulationLedger []int `json:"populationLedger"`
	// IncomeLedger: income since last rollover (10 ledger slots).
	IncomeLedger []int `json:"incomeLedger"`
	// ExpenseLedger: expenses since last rollover, by objectTypeCode (256).
	ExpenseLedger        []int `json:"expenseLedger"`
	CashBalanceCycleBase int   `json:"cashBalanceCycleBase"`
}

// CreateLedgerState TS: ledger.ts createLedgerState
func CreateLedgerState(startingCash int) *LedgerState {
	return &LedgerState{
		CashBalance:          startingCash,
		PopulationLedger:     make([]int, 10),
		IncomeLedger:         make([]int, 10),
		ExpenseLedger:        make([]int, 256),
		CashBalanceCycleBase: startingCash,
	}
}

// FamilyToLedgerIndex TS: ledger.ts familyToLedgerIndex
func FamilyToLedgerIndex(familyCode int) int {
	switch familyCode {
	case FamilyOffice:
		return ledgerOffice
	case FamilyHotelSingle:
		return ledgerHotelSingle
	case FamilyHotelTwin:
		return ledgerHotelTwin
	case FamilyHotelSuite:
		return ledgerHotelSuite
	case FamilyRetail:
		return ledgerRetail
	case FamilyFastFood:
		return ledgerFastFood
	case FamilyRestaurant:
		return ledgerRestaurant
	case FamilyPartyHall, FamilyPartyHallLower:
		return ledgerPartyHall
	case FamilyCinema, FamilyCinemaLower, FamilyCinemaStairsUpper, FamilyCinemaStairsLower:
		return ledgerCinema
	case FamilyCondo:
		return ledgerCondo
	default:
		return -1
	}
}

// yen1001Amount returns YEN_1001[tileName][min(rentLevel,3)] * YEN_UNIT.
// PORT-NOTE: the TS multiplies a JS float; every table entry * 1000 is an
// exact integer, so the int conversion is exact.
func yen1001Amount(payouts []float64, rentLevel int) int {
	return int(payouts[min(rentLevel, 3)] * yenUnit)
}

// ─── Income ───────────────────────────────────────────────────────────────────

// AddCashflowFromFamilyResource credits checkout/activation income using
// YEN #1001. TS: ledger.ts addCashflowFromFamilyResource
func AddCashflowFromFamilyResource(ledger *LedgerState, tileName string, rentLevel int, familyCode int) {
	payouts, ok := Yen1001[tileName]
	if !ok {
		return
	}
	amount := yen1001Amount(payouts, rentLevel)
	ledger.CashBalance = min(cashCap, ledger.CashBalance+amount)
	if familyCode >= 0 && familyCode < 256 {
		index := FamilyToLedgerIndex(familyCode)
		if index == -1 {
			return
		}
		ledger.IncomeLedger[index] += amount
	}
}

// RemoveCashflowFromFamilyResource TS: ledger.ts removeCashflowFromFamilyResource
func RemoveCashflowFromFamilyResource(ledger *LedgerState, tileName string, rentLevel int, familyCode int) {
	payouts, ok := Yen1001[tileName]
	if !ok {
		return
	}
	amount := yen1001Amount(payouts, rentLevel)
	ledger.CashBalance = max(0, ledger.CashBalance-amount)
	if familyCode >= 0 && familyCode < 256 {
		index := FamilyToLedgerIndex(familyCode)
		if index == -1 {
			return
		}
		ledger.IncomeLedger[index] = max(0, ledger.IncomeLedger[index]-amount)
	}
}

// ─── Expense sweep ────────────────────────────────────────────────────────────

// DoExpenseSweep charges operating expenses for all placed tiles (YEN #1002).
// Called at checkpoint 0x09e5 every 3 days. Iterates placedObjects in
// insertion order (order matters: each charge clamps cash at 0).
// TS: ledger.ts doExpenseSweep
func DoExpenseSweep(ledger *LedgerState, world *WorldState) {
	parkingRate := ParkingExpenseRateByStar[min(world.StarCount, 5)]

	lobbyFloor := UndergroundFloors // internal floor index of ground lobby
	lobbyHeight := max(1, world.LobbyHeight)

	for _, e := range world.PlacedObjects.Entries() {
		key, obj := e.Key, e.Val
		code := obj.ObjectTypeCode

		// Parking (= lobby family 0x18): star-dependent rate x width / 10
		if code == 0x18 {
			if parkingRate > 0 {
				// Skip upper floors of multi-floor lobby
				_, y := parseKeyXY(key)
				floor := YToFloor(y)
				if floor >= lobbyFloor+1 && floor < lobbyFloor+lobbyHeight {
					continue
				}
				width := obj.RightTileIndex - obj.LeftTileIndex + 1
				amount := ((width * parkingRate) / 10) * yenUnit // operands non-negative: trunc == /
				ledger.CashBalance = max(0, ledger.CashBalance-amount)
				ledger.ExpenseLedger[code] += amount
			}
			continue
		}

		rate := QuarterlyExpenses[code]
		if rate == 0 {
			continue
		}
		amount := rate * yenUnit
		ledger.CashBalance = max(0, ledger.CashBalance-amount)
		if code >= 0 && code < 256 {
			ledger.ExpenseLedger[code] += amount
		}
	}

	for _, carrier := range world.Carriers {
		activeCarCount := 0
		for _, car := range carrier.Cars {
			if car.Active {
				activeCarCount++
			}
		}
		if activeCarCount == 0 {
			continue
		}
		familyCode := 1
		if carrier.CarrierMode == 0 {
			familyCode = 42
		} else if carrier.CarrierMode == 2 {
			familyCode = 43
		}
		rate := QuarterlyExpenses[familyCode]
		if rate == 0 {
			continue
		}
		amount := rate * yenUnit * activeCarCount
		ledger.CashBalance = max(0, ledger.CashBalance-amount)
		ledger.ExpenseLedger[familyCode] += amount
	}

	for _, segment := range world.SpecialLinks {
		if !segment.Active {
			continue
		}
		units := max(1, segment.Flags>>1)
		// Binary: bit 0 clear -> escalator (family 27); bit 0 set -> stairs (family 22, rate 0).
		familyCode := 27
		if segment.Flags&1 == 1 {
			familyCode = 22
		}
		rate := QuarterlyExpenses[familyCode]
		if rate == 0 {
			continue
		}
		amount := rate * yenUnit * units
		ledger.CashBalance = max(0, ledger.CashBalance-amount)
		ledger.ExpenseLedger[familyCode] += amount
	}
}

// ─── Facility ledger rebuild ──────────────────────────────────────────────────

// RebuildFacilityLedger rebuilds populationLedger by sweeping placedObjects
// (binary rebuild_facility_ledger). TS: ledger.ts rebuildFacilityLedger
func RebuildFacilityLedger(ledger *LedgerState, world *WorldState) {
	for i := range ledger.PopulationLedger {
		ledger.PopulationLedger[i] = 0
	}
	for _, obj := range world.PlacedObjects.Values() {
		code := obj.ObjectTypeCode
		if code >= 0 && code < 256 {
			index := FamilyToLedgerIndex(code)
			if index == -1 {
				continue
			}
			ledger.PopulationLedger[index] += EntityPopulationByType[code]
		}
	}
}

// ─── 3-day rollover ───────────────────────────────────────────────────────────

// DoLedgerRollover: on a 3-day boundary save the cycle base and reset the
// rolling ledgers. TS: ledger.ts doLedgerRollover
func DoLedgerRollover(ledger *LedgerState, _ *WorldState, dayCounter int) {
	if dayCounter%3 != 0 {
		return
	}
	ledger.CashBalanceCycleBase = ledger.CashBalance
	for i := range ledger.IncomeLedger {
		ledger.IncomeLedger[i] = 0
	}
	for i := range ledger.ExpenseLedger {
		ledger.ExpenseLedger[i] = 0
	}
}

// ActivateThreeDayCashflow mirrors binary
// recompute_all_operational_status_and_cashflow at the 3-day checkpoint.
// TS: ledger.ts activateThreeDayCashflow
func ActivateThreeDayCashflow(world *WorldState, ledger *LedgerState, dayCounter int) {
	guard := dayCounter + 1
	for _, obj := range world.PlacedObjects.Values() {
		if obj.ObjectTypeCode == FamilyOffice {
			if obj.AuxValueOrTimer == guard {
				continue
			}
			if obj.AuxValueOrTimer == 0 {
				continue
			}
			if obj.OccupiedFlag != 1 {
				continue
			}
			obj.AuxValueOrTimer = guard
			AddCashflowFromFamilyResource(ledger, "office", obj.RentLevel, FamilyOffice)
			continue
		}
		if obj.ObjectTypeCode == FamilyRetail {
			if obj.AuxValueOrTimer == guard {
				continue
			}
			if obj.OccupiedFlag != 1 {
				continue
			}
			// Binary activate_family_cashflow_if_operational (1138:0bad) gates
			// family 10 on availability_state >= 0 (signed); VENUE_DORMANT excluded.
			var cv *CommercialVenueRecord
			if obj.LinkedRecordIndex >= 0 {
				cv, _ = GetSidecar(world, obj.LinkedRecordIndex).(*CommercialVenueRecord)
			}
			if cv == nil || cv.AvailabilityState == VenueDormant {
				continue
			}
			obj.AuxValueOrTimer = guard
			AddCashflowFromFamilyResource(ledger, "retail", obj.RentLevel, FamilyRetail)
		}
	}
}
