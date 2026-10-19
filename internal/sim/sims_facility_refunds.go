package sim

// Port of reference/tower-together/sim/sims/facility-refunds.ts: commercial
// venue daily runtime rebuild, closure payouts, retail activation, unhappy
// facility refunds and venue seed increments.

// SelectFacilityProgressSlot TS: sims/facility-refunds.ts selectFacilityProgressSlot
// (binary select_facility_progress_slot 11b0:1870). Returns 3, 4 or 5.
func SelectFacilityProgressSlot(world *WorldState, time *TimeState) int {
	if world.GateFlags.FacilityProgressOverride != 0 {
		return 5
	}
	if time.WeekendFlag != 0 {
		return 4
	}
	return 3
}

// readSeedForSlot TS: sims/facility-refunds.ts readSeedForSlot
func readSeedForSlot(record *CommercialVenueRecord, slot int) int {
	if slot == 5 {
		return record.OverrideSeed
	}
	if slot == 4 {
		return record.PhaseBSeed
	}
	return record.PhaseASeed
}

// writeSeedForSlot TS: sims/facility-refunds.ts writeSeedForSlot
func writeSeedForSlot(record *CommercialVenueRecord, slot, value int) {
	switch slot {
	case 5:
		record.OverrideSeed = value
	case 4:
		record.PhaseBSeed = value
	default:
		record.PhaseASeed = value
	}
}

// rollCommercialVenueRecord is the shared body of the two rebuild loops
// (binary recompute_facility_runtime_state 11b0:02f2): capacity from
// phaseASeed capped at caps[0] and floored at 10, visit counters rolled, the
// active slot cleared.
func rollCommercialVenueRecord(world *WorldState, record *CommercialVenueRecord, code, slot int) {
	if record.AvailabilityState != VenueDormant {
		record.AvailabilityState = VenueAvailable
	}
	cap := record.PhaseASeed
	if caps, ok := CommercialCapacityCaps[code]; ok && cap > caps[0] {
		cap = caps[0]
	}
	if cap < 10 {
		cap = 10
	}
	record.RemainingCapacity = cap
	record.EligibilityThreshold = -(cap + 1)

	// Binary 11b0:0461: add_to_primary_family_ledger_bucket after the roll.
	record.YesterdayVisitCount = record.TodayVisitCount
	AddToPopulationBucket(world, code, record.YesterdayVisitCount)
	record.TodayVisitCount = 0
	record.AcquireCount = 0
	record.CurrentPopulation = 0
	record.VisitCount = 0

	writeSeedForSlot(record, slot, 0)
}

// RebuildCommercialVenueRuntime TS: sims/facility-refunds.ts
// rebuildCommercialVenueRuntime (binary rebuild_linked_facility_records
// 11b0:0184, checkpoint 0x0f0). Fast food and retail only.
func RebuildCommercialVenueRuntime(world *WorldState, time *TimeState) {
	slot := SelectFacilityProgressSlot(world, time)
	ClearPopulationBucket(world, FamilyFastFood)
	ClearPopulationBucket(world, FamilyRetail)
	for _, obj := range world.PlacedObjects.Values() {
		code := obj.ObjectTypeCode
		if code != FamilyFastFood && code != FamilyRetail {
			continue
		}
		if obj.LinkedRecordIndex < 0 {
			continue
		}
		record := commercialVenueAt(world, obj.LinkedRecordIndex)
		if record == nil {
			continue
		}
		rollCommercialVenueRecord(world, record, code, slot)
	}
	RecomputeCommercialVenueBuckets(world)
}

// RebuildRestaurantFacilityRecords TS: sims/facility-refunds.ts
// rebuildRestaurantFacilityRecords (binary rebuild_type6_facility_records
// 11b0:0250, checkpoint 0x640).
func RebuildRestaurantFacilityRecords(world *WorldState, time *TimeState) {
	slot := SelectFacilityProgressSlot(world, time)
	ClearPopulationBucket(world, FamilyRestaurant)
	for _, obj := range world.PlacedObjects.Values() {
		if obj.ObjectTypeCode != FamilyRestaurant {
			continue
		}
		if obj.LinkedRecordIndex < 0 {
			continue
		}
		record := commercialVenueAt(world, obj.LinkedRecordIndex)
		if record == nil {
			continue
		}
		rollCommercialVenueRecord(world, record, FamilyRestaurant, slot)
	}
	RecomputeRestaurantVenueBuckets(world)
}

// CloseCommercialVenues TS: sims/facility-refunds.ts closeCommercialVenues
func CloseCommercialVenues(world *WorldState) {
	for _, record := range world.Sidecars {
		if venue, ok := record.(*CommercialVenueRecord); ok {
			venue.AvailabilityState = VenueClosed
		}
	}
}

// CloseCommercialVenuesByFamily TS: sims/facility-refunds.ts
// closeCommercialVenuesByFamily (seed_facility_runtime_link_state sweeps at
// tick 2000 / 2200). Bands read acquireCount (record+0x10), as the binary's
// derive_commercial_venue_state_code (11b0:1731) does.
func CloseCommercialVenuesByFamily(world *WorldState, ledger *LedgerState, familyCode int) {
	tileName, hasTile := FamilyCodeToTile[familyCode]
	var payouts []int
	hasPayouts := false
	if hasTile && tileName != "" {
		payouts, hasPayouts = CommercialClosurePayouts[tileName]
	}

	for _, object := range world.PlacedObjects.Values() {
		if object.ObjectTypeCode != familyCode {
			continue
		}
		if object.LinkedRecordIndex < 0 {
			continue
		}
		record := commercialVenueAt(world, object.LinkedRecordIndex)
		if record == nil {
			continue
		}
		if record.AvailabilityState == VenueDormant {
			continue
		}

		if hasPayouts {
			visits := record.AcquireCount
			band := 0
			for _, threshold := range CommercialClosureBands {
				if visits >= threshold {
					band++
				}
			}
			yenK := 0
			if band < len(payouts) {
				yenK = payouts[band]
			}
			if yenK != 0 {
				applyClosureCash(ledger, familyCode, yenK)
			}
		}

		record.AvailabilityState = VenueClosed
	}
}

// applyClosureCash TS: sims/facility-refunds.ts applyClosureCash.
// PORT-NOTE: the TS indexes incomeLedger by the raw familyCode (< 256)
// although the array has 10 slots: restaurant (6) lands in slot 6, while
// fast food (12) grows the JS array with a NaN entry (undefined + n) that the
// next rollover's fill(0) zeroes. Go cannot hold NaN in an int, so the
// out-of-range write is dropped. incomeLedger is display-only; cash is
// unaffected either way.
func applyClosureCash(ledger *LedgerState, familyCode, yenK int) {
	amount := yenK * 1000
	if amount > 0 {
		ledger.CashBalance = min(99999999, ledger.CashBalance+amount)
		if familyCode >= 0 && familyCode < 256 && familyCode < len(ledger.IncomeLedger) {
			ledger.IncomeLedger[familyCode] += amount
		}
	} else {
		debit := -amount
		ledger.CashBalance = max(0, ledger.CashBalance-debit)
		if familyCode >= 0 && familyCode < 256 && familyCode < len(ledger.ExpenseLedger) {
			ledger.ExpenseLedger[familyCode] += debit
		}
	}
}

// ActivateRetailShop TS: sims/facility-refunds.ts activateRetailShop
func ActivateRetailShop(world *WorldState, object *PlacedObjectRecord, record *CommercialVenueRecord, ledger *LedgerState) {
	record.AvailabilityState = VenuePartial
	AddCashflowFromFamilyResource(ledger, "retail", object.RentLevel, object.ObjectTypeCode)
	AddToPopulationBucket(world, FamilyRetail, 10)
}

// deactivateRetailShop TS: sims/facility-refunds.ts deactivateRetailShop
// (binary deactivate_retail_shop_cashflow 1180:1255).
func deactivateRetailShop(world *WorldState, object *PlacedObjectRecord, record *CommercialVenueRecord, ledger *LedgerState) {
	record.AvailabilityState = VenueDormant
	// 1180:1296 dirty, 1180:12a6 occupied cleared.
	object.DirtyFlag = 1
	object.OccupiedFlag = 0
	object.ActivationTickCount = 0
	RemoveCashflowFromFamilyResource(ledger, "retail", object.RentLevel, object.ObjectTypeCode)
	AddToPopulationBucket(world, FamilyRetail, -10)
}

// RefundUnhappyFacilities TS: sims/facility-refunds.ts refundUnhappyFacilities
func RefundUnhappyFacilities(world *WorldState, ledger *LedgerState, time *TimeState) {
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if object.ObjectTypeCode == FamilyCondo {
			if object.EvalLevel != 0 {
				continue
			}
			if object.UnitStatus >= UnitStatusCondoVacant {
				continue
			}
			RemoveCashflowFromFamilyResource(ledger, "condo", object.RentLevel, object.ObjectTypeCode)
			AddToPopulationBucket(world, FamilyCondo, -3)
			if time.DaypartIndex < 4 {
				object.UnitStatus = UnitStatusCondoVacant
			} else {
				object.UnitStatus = UnitStatusCondoVacantEvening
			}
			// Binary revert_condo_to_unsold (1180:1102): 115b dirty, 116c occupied cleared.
			object.DirtyFlag = 1
			object.OccupiedFlag = 0
			object.ActivationTickCount = 0

			x, y := parseKeyXY(e.Key)
			for _, sim := range world.Sims {
				if sim.HomeColumn == x && sim.FloorAnchor == YToFloor(y) {
					sim.StateCode = StateParked
					sim.SelectedFloor = sim.FloorAnchor
					sim.DestinationFloor = -1
					sim.VenueReturnState = 0
					ClearSimRoute(sim)
				}
			}
			continue
		}

		if object.ObjectTypeCode == FamilyRetail {
			if object.EvalLevel != 0 {
				continue
			}
			if object.LinkedRecordIndex < 0 {
				continue
			}
			record := commercialVenueAt(world, object.LinkedRecordIndex)
			if record == nil {
				continue
			}
			if record.AvailabilityState == VenueDormant {
				continue
			}
			deactivateRetailShop(world, object, record, ledger)
		}
	}
}

// IncrementVenueSeed TS: sims/facility-refunds.ts incrementVenueSeed (binary
// clamp_object_type_limit 11b0:121c). Stress is
// compute_runtime_tile_stress_average (1138:037b).
func IncrementVenueSeed(record *CommercialVenueRecord, familyCode int, sim *SimRecord, world *WorldState, time *TimeState) {
	caps, ok := CommercialCapacityCaps[familyCode]
	if !ok {
		return
	}
	slot := SelectFacilityProgressSlot(world, time)
	current := readSeedForSlot(record, slot)

	stress := 0
	if sim.TripCount > 0 {
		stress = sim.AccumulatedTicks / sim.TripCount // Math.trunc
	}
	lower, upper := 80, 150
	if th, ok := OpScoreThresholds[min(world.StarCount, 5)]; ok {
		lower, upper = th[0], th[1]
	}
	increment := 0
	if stress < lower {
		increment = 2
	} else if stress < upper {
		increment = 1
	}

	writeSeedForSlot(record, slot, min(current+increment, caps[0]))
}
