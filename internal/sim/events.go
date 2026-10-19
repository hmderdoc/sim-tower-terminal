package sim

// Port of reference/tower-together/sim/events.ts: bomb, fire (with
// firefighter helpers), metro-driven VIP visitor, random news gate, the daily
// event checkpoint dispatcher and player prompt responses.

import (
	"strconv"
	"strings"
)

// ─── Tuning constants ────────────────────────────────────────────────────────
// From startup tuning resource (type 0xff05, id 1000) at DS:0xe6xx.

const bombDeadlineTicks = 0x4b0 // 1200

// bombRansom: ransom amounts indexed by star count (2, 3, 4). Stars 1 and 5
// cannot trigger. TS: BOMB_RANSOM (lookup-only Record).
var bombRansom = map[int]int{
	2: 200_000,
	3: 300_000,
	4: 1_000_000,
}

// Blast rectangle dimensions.
const blastHalfTiles = 20

const (
	fireSpreadRate              = 7       // DS:0xe644 — ticks per tile advance
	fireVerticalDelay           = 80      // DS:0xe646 — ticks per floor
	helicopterExtinguishRate    = 1       // DS:0xe648 — ticks per tile
	helicopterPromptDelay       = 2       // DS:0xe64a — ticks after fire start / bomb extension
	rescueCountdownWithSecurity = 80      // DS:0xe64c
	helicopterRescueCost        = 500_000 // DS:0xe688
)

// jsToLocaleString mirrors Number.prototype.toLocaleString() in an en-US
// locale for a non-negative integer ("1000000" -> "1,000,000").
// PORT-NOTE: the TS output depends on the host locale; en-US is assumed.
func jsToLocaleString(v int) string {
	s := strconv.Itoa(v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// highestPopulatedFloor finds the highest floor with at least one placed
// object. TS: events.ts highestPopulatedFloor
func highestPopulatedFloor(world *WorldState) int {
	maxFloor := -1
	for _, key := range world.PlacedObjects.Keys() {
		_, y := parseKeyXY(key)
		floor := YToFloor(y)
		if floor > maxFloor {
			maxFloor = floor
		}
	}
	return maxFloor
}

// floorTileBoundsForFloor TS: events.ts floorTileBoundsForFloor. Returns
// ok=false where the TS returns null.
//
// Match the binary's per-floor header bounds (field0_0x0[+2]/[+4],
// initialized by `setup_floor_support` and read by the fire spread loop at
// 10f0:03f2 / 10f0:0379). Approximated by scanning world.cells; the
// cell-deleting deleteObjectCoveringFloorTile shrinks the extent as fire
// eats structure. Order of the scan does not matter (min/max).
func floorTileBoundsForFloor(world *WorldState, floor int) (left, right int, ok bool) {
	first := true
	objY := GridHeight - 1 - floor
	for _, key := range world.Cells.Keys() {
		x, y := parseKeyXY(key)
		if y != objY {
			continue
		}
		if first || x < left {
			left = x
		}
		if first || x > right {
			right = x
		}
		first = false
	}
	if first {
		return 0, 0, false
	}
	return left, right, true
}

// seedBombSearchCursor TS: events.ts seedBombSearchCursor
func seedBombSearchCursor(world *WorldState, floor int) {
	_, right, ok := floorTileBoundsForFloor(world, floor)
	world.EventState.BombSearchCurrentFloor = floor
	if ok {
		world.EventState.BombSearchScanTile = right - 2
	} else {
		world.EventState.BombSearchScanTile = -1
	}
}

// hasEmergencyResponseCoverage mirrors the binary's
// `g_security_office_count > 0` guard at trigger_fire_event (10f0:003a) and
// trigger_bomb_event (10d0:0086), recomputed from anchors.
// TS: events.ts hasEmergencyResponseCoverage
func hasEmergencyResponseCoverage(world *WorldState) bool {
	for _, record := range world.PlacedObjects.Values() {
		if record.ObjectTypeCode == 0x0e { // FAMILY_SECURITY
			return true
		}
	}
	return false
}

// fireIndestructibleFamilies: families the binary's FUN_1200_3474
// (1200:3474) validity gate blocks from
// delete_placed_object_and_release_sidecars, so fire cannot destroy them.
// TS: FIRE_INDESTRUCTIBLE_FAMILIES (lookup-only Set).
var fireIndestructibleFamilies = map[int]bool{
	0x0e: true, 0x0f: true, 0x18: true, 0x19: true, 0x1a: true, 0x1f: true,
	0x20: true, 0x21: true, 0x24: true, 0x25: true, 0x26: true, 0x27: true,
	0x28: true, 0x2d: true,
}

// TS: FIRE_COMMERCIAL_FAMILIES / FIRE_ENTERTAINMENT_FAMILIES.
var fireCommercialFamilies = map[int]bool{0x06: true, 0x0a: true, 0x0c: true}
var fireEntertainmentFamilies = map[int]bool{
	0x12: true, 0x13: true, 0x1d: true, 0x1e: true, 0x22: true, 0x23: true,
}

// applyTeardownSideEffects mirrors the per-family side effects of
// delete_placed_object_and_release_sidecars (1200:369d) that fire teardown
// depends on. TS: events.ts applyTeardownSideEffects
func applyTeardownSideEffects(world *WorldState, ledger *LedgerState, record *PlacedObjectRecord, objY int) {
	family := record.ObjectTypeCode
	cond := record.UnitStatus
	// Cash refund: condo only. `cond < 0x18` matches binary 10f0:026e.
	if family == 0x09 && cond < 0x18 {
		RemoveCashflowFromFamilyResource(ledger, "condo", record.EvalLevel, 9)
	}
	// Commercial-venue freelist push: mark the sidecar dormant.
	if fireCommercialFamilies[family] && record.LinkedRecordIndex >= 0 {
		if sidecar, ok := GetSidecar(world, record.LinkedRecordIndex).(*CommercialVenueRecord); ok {
			sidecar.AvailabilityState = 0xff // VENUE_DORMANT
		}
	}
	// Entertainment-link freelist push (free_entertainment_link_record
	// 1188:0ee5 writes 0xfe to the halves).
	if fireEntertainmentFamilies[family] && record.LinkedRecordIndex >= 0 {
		if sidecar, ok := GetSidecar(world, record.LinkedRecordIndex).(*EntertainmentLinkRecord); ok {
			sidecar.UpperHalfFloor = 0xfe
			sidecar.LowerHalfFloor = 0xfe
			sidecar.LinkPhaseState = 0
			sidecar.ActiveRuntimeCount = 0
		}
	}
	// Sim eviction sweep (update_sim_tile_span 1228:1018 second pass writes
	// sim[+4]=0; sim[+6]=0). Office and hotel families also release their
	// service request (release_service_request_entry 11a0:0489).
	left := record.LeftTileIndex
	right := record.RightTileIndex
	releasesService := family == 0x07 || family == 0x03 || family == 0x04 || family == 0x05
	for _, sim := range world.Sims {
		if sim.FamilyCode != family {
			continue
		}
		// PORT-NOTE: compares a floor index (floorAnchor) with a grid y
		// (objY), as the TS does. Kept verbatim.
		if sim.FloorAnchor != objY {
			continue
		}
		if sim.HomeColumn < left || sim.HomeColumn > right {
			continue
		}
		if releasesService {
			ReleaseServiceRequest(world, sim)
		}
		sim.FamilyCode = 0
		sim.StateCode = 0
	}
	// Parking ramp coverage rebuild (1200:36ec ->
	// rebuild_parking_ramp_coverage_and_demand_history 11a0:07b5).
	if family == 0x2c {
		RebuildParkingCoverage(world)
		RebuildParkingDemandLog(world)
	}
}

// deleteObjectCoveringFloorTile mirrors delete_object_covering_floor_tile
// (1200:3619) -> delete_placed_object_and_release_sidecars (1200:369d),
// including the indestructible-family validity gate. On non-blocked
// deletions also removes the covered cells so fire bounds shrink.
// TS: events.ts deleteObjectCoveringFloorTile
func deleteObjectCoveringFloorTile(world *WorldState, ledger *LedgerState, floor, tile int) {
	objY := GridHeight - 1 - floor
	for _, e := range world.PlacedObjects.Entries() {
		_, y := parseKeyXY(e.Key)
		if y != objY {
			continue
		}
		record := e.Val
		if tile < record.LeftTileIndex || tile > record.RightTileIndex {
			continue
		}
		// Validity gate: indestructible families survive fire.
		if fireIndestructibleFamilies[record.ObjectTypeCode] {
			return
		}
		applyTeardownSideEffects(world, ledger, record, objY)
		// Remove every cell the record covered (per-floor blob compaction
		// FUN_1200_3b78).
		for x := record.LeftTileIndex; x <= record.RightTileIndex; x++ {
			k := cellKey(x, objY)
			if world.Cells.Has(k) {
				world.Cells.Delete(k)
			}
			if world.CellToAnchor.Has(k) {
				world.CellToAnchor.Delete(k)
			}
		}
		world.PlacedObjects.Delete(e.Key)
		return
	}
	// No facility record matched: still remove the cell at this position.
	k := cellKey(tile, objY)
	if world.Cells.Has(k) {
		world.Cells.Delete(k)
	}
	if world.CellToAnchor.Has(k) {
		world.CellToAnchor.Delete(k)
	}
}

// selectRandomLiveFloor: contiguous-live-floor scan; returns a uniformly
// chosen floor from [lowerBound, topLiveFloor], or -1. Draws one RNG value
// only when a floor is found. TS: events.ts selectRandomLiveFloor
func selectRandomLiveFloor(world *WorldState, lowerBound int) int {
	firstOccupied := -1
	for f := lowerBound; f < GridHeight; f++ {
		if _, _, ok := floorTileBoundsForFloor(world, f); ok {
			firstOccupied = f
			break
		}
	}
	if firstOccupied < 0 {
		return -1
	}
	topLiveFloor := firstOccupied
	for f := firstOccupied + 1; f < GridHeight; f++ {
		if _, _, ok := floorTileBoundsForFloor(world, f); !ok {
			break
		}
		topLiveFloor = f
	}
	rng := topLiveFloor - lowerBound + 1
	if rng <= 0 {
		return -1
	}
	return lowerBound + (SampleRng(world) % rng)
}

// ─── Bomb Event ──────────────────────────────────────────────────────────────

// TryTriggerBombEvent is called at the daily event checkpoint when
// dayCounter % 60 == 59. TS: events.ts tryTriggerBombEvent
func TryTriggerBombEvent(world *WorldState, _ *LedgerState, time *TimeState) {
	es := &world.EventState
	if es.GameStateFlags&9 != 0 {
		return
	}
	if highestPopulatedFloor(world) < 0 {
		return
	}
	if time.DayTick >= 0x4b1 {
		return
	}
	if world.StarCount < 2 || world.StarCount > 4 {
		return
	}
	// Binary: `g_lobby_height + 10` (DS:0xbc5a + 10), no underground term.
	lobbyHeight := max(1, world.LobbyHeight)
	minFloor := lobbyHeight + 10

	selectedFloor := selectRandomLiveFloor(world, minFloor)
	if selectedFloor < 0 {
		return
	}
	left, right, ok := floorTileBoundsForFloor(world, selectedFloor)
	if !ok || right-left < 4 {
		return
	}
	tileRange := right - left - 4
	selectedTile := left
	if tileRange > 0 {
		selectedTile = left + SampleRng(world)%(tileRange+1)
	}

	es.BombFloor = selectedFloor
	es.BombTile = selectedTile
	es.GameStateFlags |= 1 // bomb active
	es.BombDeadline = bombDeadlineTicks
	es.BombSearchLowerBound = selectedFloor - 1
	es.BombSearchUpperBound = selectedFloor
	if hasEmergencyResponseCoverage(world) {
		seedBombSearchCursor(world, selectedFloor)
	} else {
		es.BombSearchCurrentFloor = -1
		es.BombSearchScanTile = -1
	}

	ransom := bombRansom[world.StarCount]
	world.PendingPrompts = append(world.PendingPrompts, SimPrompt{
		PromptID:   "bomb_" + strconv.Itoa(time.DayCounter),
		PromptKind: PromptBombRansom,
		Message:    "A bomb threat has been received! Pay $" + jsToLocaleString(ransom) + " ransom to avoid detonation?",
		Cost:       intPtr(ransom),
	})
}

// TickBombEvent is the per-tick bomb handler. TS: events.ts tickBombEvent
func TickBombEvent(world *WorldState, ledger *LedgerState, time *TimeState) {
	es := &world.EventState
	if es.GameStateFlags&0x60 != 0 && time.DayTick >= es.BombDeadline {
		bombCleanup(world, time)
		return
	}
	if es.GameStateFlags&1 != 0 && hasEmergencyResponseCoverage(world) {
		advanceBombSearch(world, ledger, time)
	}
	if es.GameStateFlags&1 != 0 && time.DayTick >= es.BombDeadline {
		resolveBombSearch(world, ledger, time, false)
	}
}

// advanceBombSearch TS: events.ts advanceBombSearch
func advanceBombSearch(world *WorldState, ledger *LedgerState, time *TimeState) {
	es := &world.EventState
	if es.BombSearchCurrentFloor < 0 {
		return
	}
	floor := es.BombSearchCurrentFloor
	left, right, ok := floorTileBoundsForFloor(world, floor)
	if !ok {
		if !advanceBombSearchFloor(world) {
			es.BombSearchCurrentFloor = -1
		}
		return
	}
	if es.BombSearchScanTile < 0 {
		es.BombSearchScanTile = right - 2
	}
	if left < es.BombSearchScanTile {
		es.BombSearchScanTile--
		CheckSecurityPatrolBomb(world, ledger, time, floor, es.BombSearchScanTile)
		return
	}
	if !advanceBombSearchFloor(world) {
		es.BombSearchCurrentFloor = -1
	}
}

// advanceBombSearchFloor TS: events.ts advanceBombSearchFloor
func advanceBombSearchFloor(world *WorldState) bool {
	es := &world.EventState
	current := es.BombSearchCurrentFloor
	if current < 0 {
		return false
	}
	tryAbove, tryBelow := es.BombSearchLowerBound, es.BombSearchUpperBound
	if current <= es.BombFloor {
		tryAbove, tryBelow = es.BombSearchUpperBound, es.BombSearchLowerBound
	}
	for _, candidate := range [2]int{tryAbove, tryBelow} {
		if candidate < 0 || candidate >= GridHeight {
			continue
		}
		if candidate == current {
			continue
		}
		if _, _, ok := floorTileBoundsForFloor(world, candidate); !ok {
			continue
		}
		if candidate >= es.BombFloor {
			es.BombSearchUpperBound = candidate + 1
		} else {
			es.BombSearchLowerBound = candidate - 1
		}
		seedBombSearchCursor(world, candidate)
		return true
	}
	return false
}

// CheckSecurityPatrolBomb is called when a security guard visits a
// floor/tile; finds the bomb when it matches. TS: events.ts checkSecurityPatrolBomb
func CheckSecurityPatrolBomb(world *WorldState, ledger *LedgerState, time *TimeState, floor, tile int) {
	es := &world.EventState
	if es.GameStateFlags&1 == 0 {
		return
	}
	if floor == es.BombFloor && tile == es.BombTile {
		resolveBombSearch(world, ledger, time, true)
	}
}

// resolveBombSearch TS: events.ts resolveBombSearch
func resolveBombSearch(world *WorldState, ledger *LedgerState, time *TimeState, found bool) {
	es := &world.EventState
	es.GameStateFlags &^= 1
	if found {
		es.GameStateFlags |= 0x20
		es.BombDeadline = time.DayTick + helicopterPromptDelay // reuse tuning delay
	} else {
		es.GameStateFlags |= 0x40
		applyBlastDamage(world, ledger)
		es.BombDeadline = time.DayTick + 50
	}
}

// applyBlastDamage TS: events.ts applyBlastDamage
func applyBlastDamage(world *WorldState, ledger *LedgerState) {
	es := &world.EventState
	floorMin := es.BombFloor - 2
	floorMax := es.BombFloor + 3
	tileMin := es.BombTile - blastHalfTiles
	tileMax := es.BombTile + blastHalfTiles - 1
	for floor := floorMin; floor <= floorMax; floor++ {
		for tile := tileMin; tile <= tileMax; tile++ {
			deleteObjectCoveringFloorTile(world, ledger, floor, tile)
		}
	}
}

// bombCleanup TS: events.ts bombCleanup.
// PORT-NOTE: the TS casts away readonly and mutates the passed TimeState in
// place (dayTick fast-forward). The Go port mutates *time in place too; the
// day scheduler passes its freshly advanced nextTime, which it then returns.
func bombCleanup(world *WorldState, time *TimeState) {
	es := &world.EventState
	es.GameStateFlags &^= 0x60
	es.BombSearchCurrentFloor = -1
	es.BombSearchScanTile = -1
	es.BombSearchLowerBound = -1
	es.BombSearchUpperBound = -1
	if time.DayTick < 1500 {
		time.DayTick = 1500
		time.DaypartIndex = 1500 / 400
	}
}

// ─── Fire Event ──────────────────────────────────────────────────────────────

// TryTriggerFireEvent is called at the daily event checkpoint when
// dayCounter % 84 == 83. TS: events.ts tryTriggerFireEvent
func TryTriggerFireEvent(world *WorldState, _ *LedgerState, time *TimeState) {
	es := &world.EventState
	if es.GameStateFlags&9 != 0 {
		return
	}
	if highestPopulatedFloor(world) < 0 {
		return
	}
	if time.DaypartIndex >= 4 {
		return
	}
	if world.StarCount <= 2 {
		return
	}
	if world.GateFlags.EvalSimIndex >= 0 && world.GateFlags.EvalSimIndex != 0xffff {
		return
	}
	lobbyHeight := max(1, world.LobbyHeight)
	minFloor := lobbyHeight + 10

	selectedFloor := selectRandomLiveFloor(world, minFloor)
	if selectedFloor < 0 {
		return
	}
	left, right, ok := floorTileBoundsForFloor(world, selectedFloor)
	if !ok || right-left < 0x20 {
		return
	}

	es.FireFloor = selectedFloor
	es.FireTile = right - 0x20
	es.GameStateFlags |= 8
	es.FireStartTick = time.DayTick
	fillInts(es.FireLeftPos, 0xffff)
	fillInts(es.FireRightPos, 0xffff)
	// Eager-init the fireFloor entries (binary writes at fireFloor*2 +
	// 0xbd7e / 0xbc8e).
	es.FireLeftPos[selectedFloor] = es.FireTile
	es.FireRightPos[selectedFloor] = es.FireTile

	if hasEmergencyResponseCoverage(world) {
		es.RescueCountdown = rescueCountdownWithSecurity
	} else {
		es.RescueCountdown = 0
	}
	es.HelicopterExtinguishPos = 0
}

// fillInts is Array.prototype.fill(v) on an existing array.
func fillInts(a []int, v int) {
	for i := range a {
		a[i] = v
	}
}

// TickFireEvent is the per-tick fire spread and resolution.
// TS: events.ts tickFireEvent
func TickFireEvent(world *WorldState, ledger *LedgerState, time *TimeState) {
	es := &world.EventState
	if es.GameStateFlags&8 == 0 {
		return
	}
	if es.RescueCountdown > 0 {
		es.RescueCountdown--
		return
	}
	if es.HelicopterExtinguishPos > 0 {
		advanceHelicopterExtinguish(world, time)
	}
	advanceFireSpread(world, ledger, time)
	// Firefighter rescue helpers (firefighter_sim_per_tick_handler 1100:0bd3).
	tickFireRescueHelpers(world, time)

	if es.HelicopterExtinguishPos == 0 && time.DayTick == es.FireStartTick+helicopterPromptDelay {
		world.PendingPrompts = append(world.PendingPrompts, SimPrompt{
			PromptID:   "fire_" + strconv.Itoa(time.DayCounter),
			PromptKind: PromptFireRescue,
			Message:    "Fire is spreading! Call helicopter rescue for $" + jsToLocaleString(helicopterRescueCost) + "?",
			Cost:       intPtr(helicopterRescueCost),
		})
	}
	// Binary exact-equals at 10d0:0049.
	if time.DayTick == 2000 {
		resolveFireEvent(world, time)
	}
}

// advanceFireSpread mirrors advance_fire_frontier_damage (10f0:0306) over
// all 120 floors. TS: events.ts advanceFireSpread
func advanceFireSpread(world *WorldState, ledger *LedgerState, time *TimeState) {
	es := &world.EventState
	for floor := 0; floor < GridHeight; floor++ {
		if floor < es.FireFloor {
			continue
		}
		floorDelay := (floor - es.FireFloor) * fireVerticalDelay
		ignitionTick := es.FireStartTick + floorDelay
		if time.DayTick < ignitionTick {
			continue
		}

		if es.FireLeftPos[floor] == 0xffff && es.FireRightPos[floor] == 0xffff {
			if time.DayTick == ignitionTick {
				es.FireLeftPos[floor] = es.FireTile
				es.FireRightPos[floor] = es.FireTile
				if floor > es.FireFloor {
					es.FireLeftPos[floor-1] = 0xffff
					es.FireRightPos[floor-1] = 0xffff
					if _, right, ok := floorTileBoundsForFloor(world, floor); ok {
						for _, helper := range es.FireRescueHelpers {
							helper.Floor = floor
							helper.Column = right + 11
							helper.Status = 0
							helper.WindupRemaining = 0
						}
					}
				}
			}
			continue
		}

		// Absolute g_day_tick % spread_rate (10f0:039a, 10f0:0411).
		if time.DayTick%fireSpreadRate != 0 {
			continue
		}
		left, right, ok := floorTileBoundsForFloor(world, floor)
		if !ok {
			continue
		}

		// Left front (0xffff sentinel at 10f0:03a5).
		if es.FireLeftPos[floor] != 0xffff {
			deleteObjectCoveringFloorTile(world, ledger, floor, es.FireLeftPos[floor])
			es.FireLeftPos[floor]--
			if es.FireLeftPos[floor] < left {
				es.FireLeftPos[floor] = 0xffff
			}
		}

		// Right front: delete at position + 12 (10f0:0420).
		if es.FireRightPos[floor] != 0xffff {
			deletePos := es.FireRightPos[floor] + 12
			if deletePos <= right {
				deleteObjectCoveringFloorTile(world, ledger, floor, deletePos)
			}
			es.FireRightPos[floor]++
			if es.FireRightPos[floor]+12 > right {
				es.FireRightPos[floor] = 0xffff
			}
		}
	}
}

// advanceHelicopterExtinguish TS: events.ts advanceHelicopterExtinguish
func advanceHelicopterExtinguish(world *WorldState, time *TimeState) {
	es := &world.EventState
	if es.HelicopterExtinguishPos <= 0 {
		return
	}
	if time.DayTick%helicopterExtinguishRate != 0 {
		return
	}
	es.HelicopterExtinguishPos--

	// clear_fire_boundaries_past_spread_width (10f0:0858).
	for floor := 0; floor < GridHeight; floor++ {
		if es.FireLeftPos[floor] != 0xffff && es.FireLeftPos[floor] > es.HelicopterExtinguishPos {
			es.FireLeftPos[floor] = 0xffff
		}
		if es.FireRightPos[floor] != 0xffff && es.FireRightPos[floor] > es.HelicopterExtinguishPos {
			es.FireRightPos[floor] = 0xffff
		}
	}

	if es.HelicopterExtinguishPos <= 0 {
		resolveFireEvent(world, time)
	}
}

// resolveFireEvent TS: events.ts resolveFireEvent (mutates *time in place,
// see bombCleanup).
func resolveFireEvent(world *WorldState, time *TimeState) {
	es := &world.EventState
	es.GameStateFlags &^= 8
	fillInts(es.FireLeftPos, 0xffff)
	fillInts(es.FireRightPos, 0xffff)
	es.HelicopterExtinguishPos = 0
	es.RescueCountdown = 0
	// initialize_service_response_entities(0) via finalize_fire_event_cleanup 10f0:02a1.
	es.FireRescueHelpers = []*FireRescueHelper{}
	if time.DayTick < 1500 {
		time.DayTick = 1500
		time.DaypartIndex = 1500 / 400
	}
}

// ─── Firefighter rescue helpers ──────────────────────────────────────────────

const (
	fireWalkDelay       = 1 // DS:0xe640 (g_tuning_fire_walk_delay)
	fireExtinguishDelay = 5 // DS:0xe64e (g_tuning_fire_extinguish_delay)
)

// spawnFireRescueHelpers spawns one helper per placed FAMILY_SECURITY
// anchor (initialize_service_response_entities(8) 1100:033d), in
// placedObjects insertion order. TS: events.ts spawnFireRescueHelpers
func spawnFireRescueHelpers(world *WorldState) {
	helpers := []*FireRescueHelper{}
	for _, e := range world.PlacedObjects.Entries() {
		record := e.Val
		if record.ObjectTypeCode != 0x0e {
			continue
		}
		_, y := parseKeyXY(e.Key)
		officeFloor := GridHeight - 1 - y
		spawnColumn := record.RightTileIndex
		if _, right, ok := floorTileBoundsForFloor(world, officeFloor); ok {
			spawnColumn = right + 11
		}
		helpers = append(helpers, &FireRescueHelper{
			Floor:           officeFloor,
			Column:          spawnColumn,
			Status:          0,
			WindupRemaining: 0,
		})
	}
	world.EventState.FireRescueHelpers = helpers
}

// advanceFireRescueHelper mirrors firefighter_sim_per_tick_handler
// (1100:0bd3). TS: events.ts advanceFireRescueHelper
func advanceFireRescueHelper(world *WorldState, helper *FireRescueHelper, time *TimeState) {
	es := &world.EventState
	if helper.Status == 3 {
		return
	}
	if helper.Status == 1 {
		helper.WindupRemaining--
		if helper.WindupRemaining <= 0 {
			extinguishFireFrontAtTile(es, helper.Floor, helper.Column)
			helper.Status = 3
		}
		return
	}

	if time.DayTick%fireWalkDelay != 0 {
		return
	}

	left := es.FireLeftPos[helper.Floor]
	right := es.FireRightPos[helper.Floor]
	leftActive := left != 0xffff
	rightActive := right != 0xffff
	if !leftActive && !rightActive {
		// Relocate to the first floor with a live front
		// (move_bomb_search_response_sim_to_floor).
		for f := 0; f < GridHeight; f++ {
			if es.FireLeftPos[f] != 0xffff || es.FireRightPos[f] != 0xffff {
				if f != helper.Floor {
					helper.Floor = f
					if _, r, ok := floorTileBoundsForFloor(world, f); ok {
						helper.Column = r + 11
					}
				}
				return
			}
		}
		return
	}

	// Arrival predicate (FUN_10f0_076c).
	inLeftWindow := leftActive && helper.Column >= left && helper.Column < left+6
	inRightWindow := rightActive && helper.Column >= right+6 && helper.Column < right+12
	if inLeftWindow || inRightWindow {
		helper.Status = 1
		helper.WindupRemaining = fireExtinguishDelay
		return
	}

	if helper.Column > 0 {
		helper.Column--
	}
}

// extinguishFireFrontAtTile: asymmetric front clear
// (extinguish_fire_front_at_tile 10f0:07d8). TS: events.ts extinguishFireFrontAtTile
func extinguishFireFrontAtTile(es *EventState, floor, tile int) {
	left := es.FireLeftPos[floor]
	if left != 0xffff && tile >= left && tile < left+12 {
		es.FireLeftPos[floor] = 0xffff
	}
	right := es.FireRightPos[floor]
	if right != 0xffff && tile >= right && tile < right+12 {
		es.FireRightPos[floor] = 0xffff
	}
}

// tickFireRescueHelpers TS: events.ts tickFireRescueHelpers
func tickFireRescueHelpers(world *WorldState, time *TimeState) {
	for _, helper := range world.EventState.FireRescueHelpers {
		advanceFireRescueHelper(world, helper, time)
	}
}

// ─── VIP Special Visitor Event ───────────────────────────────────────────────

// TickVipSpecialVisitor is the per-tick VIP special visitor check
// (trigger_vip_special_visitor 11f0:0273). TS: events.ts tickVipSpecialVisitor
func TickVipSpecialVisitor(world *WorldState, time *TimeState) {
	es := &world.EventState
	if time.DayTick <= 0xf0 {
		return
	}
	if time.DaypartIndex >= 4 {
		return
	}
	if es.GameStateFlags&9 != 0 {
		return
	}
	if world.GateFlags.MetroStationFloorIndex < 0 {
		return
	}
	// 1% chance per tick.
	if SampleRng(world)%100 != 0 {
		return
	}
	anyArrival := false
	for _, object := range world.PlacedObjects.Values() {
		if object.ObjectTypeCode != FamilyMetroTop &&
			object.ObjectTypeCode != FamilyMetroMiddle &&
			object.ObjectTypeCode != FamilyMetroBottom {
			continue
		}
		if object.UnitStatus == 0 {
			object.UnitStatus = 2
			anyArrival = true
		} else {
			object.UnitStatus = 0
		}
		object.DirtyFlag = 1
	}
	if anyArrival {
		world.PendingNotifications = append(world.PendingNotifications, SimNotification{
			Kind:    "event",
			Message: "metro_train_arrival",
		})
	}
}

// TriggerRandomNewsEvent keeps the RNG-consuming gate of the audio-only
// random news event (play_classified_news_sound 11d0:042c); emits nothing.
// TS: events.ts triggerRandomNewsEvent
func TriggerRandomNewsEvent(world *WorldState, time *TimeState) {
	es := &world.EventState
	if es.DisableNewsEvents {
		return
	}
	if time.DayTick <= 0xf0 {
		return
	}
	if time.DaypartIndex >= 6 {
		return
	}
	if es.GameStateFlags&9 != 0 {
		return
	}
	if SampleRng(world)%16 != 0 {
		return
	}
	// TODO(client-audio) in the TS: no sound/notification emitted.
}

// ─── Daily event checkpoint dispatcher ───────────────────────────────────────

// CheckDailyEvents dispatches bomb/fire triggers on dayCounter modular
// conditions (fire first, then bomb). TS: events.ts checkDailyEvents
func CheckDailyEvents(world *WorldState, ledger *LedgerState, time *TimeState) {
	if time.DayCounter%84 == 83 {
		TryTriggerFireEvent(world, ledger, time)
	}
	if time.DayCounter%60 == 59 {
		TryTriggerBombEvent(world, ledger, time)
	}
}

// HandlePromptResponse handles a player's response to a prompt; returns true
// when the prompt was recognized. TS: events.ts handlePromptResponse
func HandlePromptResponse(world *WorldState, ledger *LedgerState, time *TimeState, promptID string, accepted bool) bool {
	if strings.HasPrefix(promptID, "bomb_") {
		if !accepted {
			return true // refused: bomb stays armed
		}
		ransom := bombRansom[world.StarCount]
		if ransom > 0 && ledger.CashBalance >= ransom {
			ledger.CashBalance -= ransom
			bombCleanup(world, time)
			world.PendingNotifications = append(world.PendingNotifications, SimNotification{
				Kind:    "event",
				Message: "Ransom paid. The bomb threat has been resolved.",
			})
		}
		return true
	}

	if strings.HasPrefix(promptID, "fire_") {
		es := &world.EventState
		if es.GameStateFlags&8 == 0 {
			return true // fire already resolved
		}
		if !accepted {
			// resolve_fire_rescue_followup (10f0:0149) IDOK branch.
			spawnFireRescueHelpers(world)
			return true
		}
		_, right, ok := floorTileBoundsForFloor(world, es.FireFloor)
		if ok && ledger.CashBalance >= helicopterRescueCost {
			ledger.CashBalance -= helicopterRescueCost
			es.HelicopterExtinguishPos = right - 12
			world.PendingNotifications = append(world.PendingNotifications, SimNotification{
				Kind:    "event",
				Message: "Helicopter dispatched to fight the fire!",
			})
		}
		return true
	}

	if strings.HasPrefix(promptID, "carrier_remove_") {
		column := world.EventState.PendingCarrierEditColumn
		y := world.EventState.PendingCarrierEditY
		world.EventState.PendingCarrierEditColumn = -1
		world.EventState.PendingCarrierEditY = -1
		if !accepted || column < 0 || y < 0 {
			return true
		}
		ApplyRemoveElevatorCar(world, column, y)
		return true
	}

	return false
}
