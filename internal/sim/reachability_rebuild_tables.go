package sim

import "sort"

// Port of reference/tower-together/sim/reachability/rebuild-tables.ts.
//
// 11b8:00f2 rebuildRouteReachabilityTables
// 11b8:049f rebuildTransferGroupCache
// 11b8:0000 clearRouteReachabilityTables
// 11b8:006d clearTransferGroupCache

// ClearRouteReachabilityTables TS: reachability/rebuild-tables.ts clearRouteReachabilityTables (binary 11b8:0000)
func ClearRouteReachabilityTables(world *WorldState) {
	world.FloorWalkabilityFlags = make([]int, GridHeight)
}

// ClearTransferGroupCache TS: reachability/rebuild-tables.ts clearTransferGroupCache (binary 11b8:006d)
func ClearTransferGroupCache(world *WorldState) {
	world.TransferGroupCache = make([]int, GridHeight)
	world.TransferGroupEntries = make([]*TransferGroupEntry, MaxTransferGroups)
	for i := range world.TransferGroupEntries {
		world.TransferGroupEntries[i] = &TransferGroupEntry{Active: false, TaggedFloor: 0xff, CarrierMask: 0}
	}
	for _, record := range world.SpecialLinkRecords {
		trFill(record.ReachabilityMasksByFloor, 0)
	}
}

// markSpecialLinkWalkability ORs each active segment's walkability bit
// (escalator = 1, stairs = 2) into floorWalkabilityFlags over
// [entryFloor, entryFloor + (flags >> 1) + 1]. Shared body of
// rebuildRouteReachabilityTables and the inline refresh in
// rebuildSpecialLinkRouteRecords (identical loops in the TS).
func markSpecialLinkWalkability(world *WorldState) {
	for _, segment := range world.SpecialLinks {
		if !segment.Active {
			continue
		}
		bit := 1
		if segment.Flags&1 != 0 {
			bit = 2
		}
		extentMinusOne := segment.Flags >> 1
		// Binary encoding: top_floor = entry_floor + (flags >> 1) + 1.
		topFloor := segment.EntryFloor + extentMinusOne + 1
		for floor := segment.EntryFloor; floor <= topFloor; floor++ {
			if floor >= 0 && floor < GridHeight {
				world.FloorWalkabilityFlags[floor] |= bit
			}
		}
	}
}

// RebuildRouteReachabilityTables populates per-floor walkability bits from
// the active special-link segments. Bit 0 = escalator, bit 1 = stairs.
// TS: reachability/rebuild-tables.ts rebuildRouteReachabilityTables (binary 11b8:00f2)
func RebuildRouteReachabilityTables(world *WorldState) {
	ClearRouteReachabilityTables(world)
	markSpecialLinkWalkability(world)
}

// RebuildTransferGroupCache flood-fills transfer-group membership across
// the carrier + special-link graph. Each entry pairs a tagged floor with a
// carrier-mask bit field (bits 0..23 = carriers, 24..31 = derived records).
// TS: reachability/rebuild-tables.ts rebuildTransferGroupCache (binary 11b8:049f)
//
// Iteration order: tile candidates come from Object.entries(placedObjects)
// (OrderedMap snapshot) and are then stable-sorted by (floor, left), so ties
// keep insertion order like Array.prototype.sort.
func RebuildTransferGroupCache(world *WorldState) {
	ClearTransferGroupCache(world)

	// Tile-level parking candidates (one per placed-object cell),
	// coalesced into contiguous same-floor spans.
	type tileCandidate struct{ floor, left, right, x int }
	tileCandidates := []tileCandidate{}
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if object.ObjectTypeCode != FamilyParking {
			continue
		}
		x, y := parseKeyXY(e.Key)
		tileCandidates = append(tileCandidates, tileCandidate{
			floor: YToFloor(y),
			left:  object.LeftTileIndex,
			right: object.RightTileIndex,
			x:     x,
		})
	}
	sort.SliceStable(tileCandidates, func(i, j int) bool {
		a, b := tileCandidates[i], tileCandidates[j]
		if a.floor == b.floor {
			return a.left < b.left
		}
		return a.floor < b.floor
	})

	type span struct{ floor, left, right int }
	spans := []*span{}
	for _, tile := range tileCandidates {
		var prev *span
		if len(spans) > 0 {
			prev = spans[len(spans)-1]
		}
		if prev != nil && prev.floor == tile.floor && tile.left <= prev.right+1 {
			if tile.right > prev.right {
				prev.right = tile.right
			}
		} else {
			spans = append(spans, &span{floor: tile.floor, left: tile.left, right: tile.right})
		}
	}

	type candidate struct{ floor, x, membershipMask int }
	candidates := []candidate{}
	for _, sp := range spans {
		membershipMask := 0
		for _, carrier := range world.Carriers {
			if carrier.CarrierID >= 24 {
				continue
			}
			if carrier.Column < sp.left || carrier.Column > sp.right {
				continue
			}
			// Binary 11b8:049f gates membership on served_floor_flags
			// [tagged_floor] != 0: the carrier must span the tagged floor.
			if !CarrierSpansFloor(carrier, sp.floor) {
				continue
			}
			// Player toggle (FUN_10a8_0085) drops a disabled floor.
			slot := sp.floor - carrier.BottomServedFloor
			if trAt(carrier.StopFloorEnabled, slot, 1) == 0 {
				continue
			}
			membershipMask |= trBit(carrier.CarrierID)
		}
		if membershipMask != 0 {
			candidates = append(candidates, candidate{floor: sp.floor, x: sp.left, membershipMask: membershipMask})
		}
	}

	// Append+collapse: collapse into the immediately preceding entry when it
	// has the same tagged floor and an overlapping carrier mask.
	entryCount := 0
	for _, c := range candidates {
		if entryCount > 0 {
			prev := world.TransferGroupEntries[entryCount-1]
			if prev != nil && prev.Active && prev.TaggedFloor == c.floor && prev.CarrierMask&c.membershipMask != 0 {
				prev.CarrierMask |= c.membershipMask
				continue
			}
		}
		if entryCount >= MaxTransferGroups {
			return
		}
		world.TransferGroupEntries[entryCount] = &TransferGroupEntry{
			Active:      true,
			TaggedFloor: c.floor,
			CarrierMask: c.membershipMask,
		}
		entryCount++
	}

	for recordIndex, record := range world.SpecialLinkRecords {
		if !record.Active {
			continue
		}
		for _, entry := range world.TransferGroupEntries {
			if !entry.Active {
				continue
			}
			if entry.TaggedFloor >= record.LowerFloor && entry.TaggedFloor <= record.UpperFloor {
				entry.CarrierMask |= trBit(24 + recordIndex)
			}
		}
	}

	for _, entry := range world.TransferGroupEntries {
		if entry == nil || !entry.Active {
			continue
		}
		trSet(&world.TransferGroupCache, entry.TaggedFloor, trAt(world.TransferGroupCache, entry.TaggedFloor, 0)|entry.CarrierMask)
	}

	for recordIndex, record := range world.SpecialLinkRecords {
		if !record.Active {
			continue
		}
		rebuildSpecialLinkRecordReachability(world, recordIndex, record)
	}
}

// TS: reachability/rebuild-tables.ts rebuildSpecialLinkRecordReachability
func rebuildSpecialLinkRecordReachability(world *WorldState, recordIndex int, record *SpecialLinkRecord) {
	recordBit := trBit(24 + recordIndex)
	aggregateMask := 0
	for _, entry := range world.TransferGroupEntries {
		if !entry.Active {
			continue
		}
		if entry.CarrierMask&recordBit == 0 {
			continue
		}
		aggregateMask |= entry.CarrierMask
	}
	aggregateMask &^= recordBit

	for floor := 0; floor < GridHeight; floor++ {
		insideSpan := floor >= record.LowerFloor && floor <= record.UpperFloor
		localEntryIndex := -1
		for i, entry := range world.TransferGroupEntries {
			if entry.Active && entry.TaggedFloor == floor && entry.CarrierMask&recordBit != 0 {
				localEntryIndex = i
				break
			}
		}
		if insideSpan && localEntryIndex >= 0 {
			trSet(&record.ReachabilityMasksByFloor, floor, localEntryIndex+1)
			continue
		}

		projectedMask := 0
		for carrierIndex := 0; carrierIndex < 24; carrierIndex++ {
			if aggregateMask&trBit(carrierIndex) == 0 {
				continue
			}
			carrier := trFindCarrier(world, carrierIndex)
			if carrier == nil {
				continue
			}
			if !CarrierServesFloor(carrier, floor) {
				continue
			}
			// Binary served_floor_flags[floor] gate.
			slot := floor - carrier.BottomServedFloor
			if trAt(carrier.StopFloorEnabled, slot, 1) == 0 {
				continue
			}
			projectedMask |= trBit(carrierIndex)
		}
		for peerIndex := 0; peerIndex < MaxSpecialLinkRecords; peerIndex++ {
			if aggregateMask&trBit(24+peerIndex) == 0 {
				continue
			}
			if peerIndex >= len(world.SpecialLinkRecords) {
				continue
			}
			peer := world.SpecialLinkRecords[peerIndex]
			if peer == nil || !peer.Active {
				continue
			}
			if floor >= peer.LowerFloor && floor <= peer.UpperFloor {
				projectedMask |= trBit(24 + peerIndex)
			}
		}
		trSet(&record.ReachabilityMasksByFloor, floor, projectedMask)
	}
}
