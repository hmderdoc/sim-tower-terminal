package sim

// Port of reference/tower-together/sim/reachability/mask-tests.ts.
//
// 11b8:0f33 testCarrierTransferReachability
// 11b8:0fe6 testSpecialLinkTransferReachability
// 11b8:0e41 chooseTransferFloorFromCarrierReachability
//
// Bit-mask tests against transferGroupEntries + specialLinkRecords, plus
// the transfer-floor picker used when a carrier doesn't directly serve the
// target floor. Masks: bits 0..23 = carriers, bits 24..31 = derived
// special-link records (JS int32 semantics via trBit).

// TestCarrierTransferReachability walks the transfer-group entries for any
// entry whose carrier mask includes carrierID AND can reach toFloor.
// TS: reachability/mask-tests.ts testCarrierTransferReachability (binary 11b8:0f33)
func TestCarrierTransferReachability(world *WorldState, carrierID, toFloor int, preferLocalMode bool) bool {
	for _, entry := range world.TransferGroupEntries {
		if !entry.Active {
			continue
		}
		if entry.CarrierMask&trBit(carrierID) == 0 {
			continue
		}
		if entryReachesDestinationFloor(world, entry, toFloor, preferLocalMode) {
			return true
		}
	}
	return false
}

// TestSpecialLinkTransferReachability probes whether any peer special-link
// record in the entry covers toFloor.
// TS: reachability/mask-tests.ts testSpecialLinkTransferReachability (binary 11b8:0fe6)
func TestSpecialLinkTransferReachability(world *WorldState, entry *TransferGroupEntry, toFloor int) bool {
	for recordIndex := 0; recordIndex < MaxSpecialLinkRecords; recordIndex++ {
		if entry.CarrierMask&trBit(24+recordIndex) == 0 {
			continue
		}
		if recordIndex >= len(world.SpecialLinkRecords) {
			continue
		}
		record := world.SpecialLinkRecords[recordIndex]
		if record == nil || !record.Active {
			continue
		}
		if DerivedRecordReachesFloor(record, toFloor) {
			return true
		}
	}
	return false
}

// ChooseTransferFloorFromCarrierReachability resolves the floor a carrier
// should carry a rider to: the target itself when the carrier stops there,
// otherwise the first transfer-group tagged floor in the travel direction
// whose peers reach the target. -1 when no valid transfer exists.
// TS: reachability/mask-tests.ts chooseTransferFloorFromCarrierReachability (binary 11b8:0e41)
func ChooseTransferFloorFromCarrierReachability(world *WorldState, carrierID, currentFloor, targetFloor int) int {
	carrier := trFindCarrier(world, carrierID)
	if carrier == nil {
		return -1
	}

	// Binary 11b8:0e41 opens with carrier.served_floor_flags[target] != 0
	// (span AND stop-enabled AND express-stop for express carriers).
	if CarrierSpansFloor(carrier, targetFloor) {
		targetSlot := targetFloor - carrier.BottomServedFloor
		stopEnabled := trAt(carrier.StopFloorEnabled, targetSlot, 1) != 0
		expressOk := carrier.CarrierMode != 0 || IsExpressStopFloor(targetFloor, world.LobbyMode)
		if stopEnabled && expressOk {
			return targetFloor
		}
	}

	// Scan the 16-slot transfer_group_cache in order; first entry whose
	// tagged floor lies in the travel direction and whose peers reach the
	// target wins.
	directionUp := targetFloor > currentFloor
	carrierBit := trBit(carrierID)

	for _, entry := range world.TransferGroupEntries {
		if !entry.Active {
			continue
		}
		if entry.CarrierMask&carrierBit == 0 {
			continue
		}
		if entry.TaggedFloor == currentFloor {
			continue
		}
		if directionUp && entry.TaggedFloor <= currentFloor {
			continue
		}
		if !directionUp && entry.TaggedFloor >= currentFloor {
			continue
		}

		peersMask := entry.CarrierMask &^ carrierBit
		if !peersMaskReachesFloor(world, peersMask, targetFloor) {
			continue
		}
		return entry.TaggedFloor
	}

	return -1
}

// TS: reachability/mask-tests.ts carrierStopsAtFloor
func carrierStopsAtFloor(carrier *CarrierRecord, floor int, lobbyMode LobbyMode) bool {
	if !CarrierSpansFloor(carrier, floor) {
		return false
	}
	// Binary served_floor_flags[floor] != 0: the stop-floor dialog toggle
	// (FUN_10a8_0085) clears this bit.
	slot := floor - carrier.BottomServedFloor
	if trAt(carrier.StopFloorEnabled, slot, 1) == 0 {
		return false
	}
	if carrier.CarrierMode == 0 {
		return IsExpressStopFloor(floor, lobbyMode)
	}
	return true
}

// TS: reachability/mask-tests.ts peersMaskReachesFloor
func peersMaskReachesFloor(world *WorldState, mask, targetFloor int) bool {
	for carrierIndex := 0; carrierIndex < 24; carrierIndex++ {
		if mask&trBit(carrierIndex) == 0 {
			continue
		}
		peer := trFindCarrier(world, carrierIndex)
		if peer != nil && carrierStopsAtFloor(peer, targetFloor, world.LobbyMode) {
			return true
		}
	}
	for recordIndex := 0; recordIndex < MaxSpecialLinkRecords; recordIndex++ {
		if mask&trBit(24+recordIndex) == 0 {
			continue
		}
		if recordIndex >= len(world.SpecialLinkRecords) {
			continue
		}
		record := world.SpecialLinkRecords[recordIndex]
		if record != nil && record.Active && DerivedRecordReachesFloor(record, targetFloor) {
			return true
		}
	}
	return false
}

// DerivedRecordReachesFloor is shared by TestCarrierTransferReachability
// and the derived-record scanners.
// TS: reachability/mask-tests.ts derivedRecordReachesFloor
func DerivedRecordReachesFloor(record *SpecialLinkRecord, targetFloor int) bool {
	if targetFloor >= record.LowerFloor && targetFloor <= record.UpperFloor {
		return true
	}
	return trAt(record.ReachabilityMasksByFloor, targetFloor, 0) != 0
}

// TS: reachability/mask-tests.ts entryReachesDestinationFloor
func entryReachesDestinationFloor(world *WorldState, entry *TransferGroupEntry, toFloor int, preferLocalMode bool) bool {
	for carrierIndex := 0; carrierIndex < 24; carrierIndex++ {
		if entry.CarrierMask&trBit(carrierIndex) == 0 {
			continue
		}
		carrier := trFindCarrier(world, carrierIndex)
		if carrier == nil {
			continue
		}
		if preferLocalMode {
			if carrier.CarrierMode == 2 {
				continue
			}
		} else if carrier.CarrierMode != 2 {
			continue
		}
		if carrierStopsAtFloor(carrier, toFloor, world.LobbyMode) {
			return true
		}
	}
	return TestSpecialLinkTransferReachability(world, entry, toFloor)
}
