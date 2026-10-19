package sim

import (
	"math"
	"strconv"
)

// Port of reference/tower-together/sim/commands.ts: place/remove tile with
// every validation rule, rent level, cinema movie pool and elevator config
// commands.

// ─── Patch type ───────────────────────────────────────────────────────────────

// CellPatch TS: commands.ts CellPatch. Optional TS fields are pointers
// (nil == absent) so the JSON matches the TS spread-or-omit shape.
type CellPatch struct {
	X              int    `json:"x"`
	Y              int    `json:"y"`
	TileType       string `json:"tileType"`
	IsAnchor       bool   `json:"isAnchor"`
	IsOverlay      bool   `json:"isOverlay,omitempty"`
	EvalActiveFlag *int   `json:"evalActiveFlag,omitempty"`
	UnitStatus     *int   `json:"unitStatus,omitempty"`
	EvalLevel      *int   `json:"evalLevel,omitempty"`
	EvalScore      *int   `json:"evalScore,omitempty"`
	CoverageFlag   *int   `json:"coverageFlag,omitempty"`
}

// CommandResult TS: commands.ts CommandResult
type CommandResult struct {
	Accepted       bool        `json:"accepted"`
	Patch          []CellPatch `json:"patch,omitempty"`
	Reason         string      `json:"reason,omitempty"`
	EconomyChanged bool        `json:"economyChanged,omitempty"`
}

// Command type strings (SimCommand.Type).
const (
	CmdPlaceTile                     = "place_tile"
	CmdRemoveTile                    = "remove_tile"
	CmdPromptResponse                = "prompt_response"
	CmdSetRentLevel                  = "set_rent_level"
	CmdAddElevatorCar                = "add_elevator_car"
	CmdRemoveElevatorCar             = "remove_elevator_car"
	CmdSetElevatorDwellDelay         = "set_elevator_dwell_delay"
	CmdSetElevatorWaitingCarResponse = "set_elevator_waiting_car_response"
	CmdSetElevatorHomeFloor          = "set_elevator_home_floor"
	CmdToggleElevatorFloorStop       = "toggle_elevator_floor_stop"
	CmdSetCinemaMoviePool            = "set_cinema_movie_pool"
)

// SimCommand is the TS discriminated union flattened: Type selects which
// fields are meaningful (see the TS SimCommand type).
type SimCommand struct {
	Type      string `json:"type"`
	X         int    `json:"x,omitempty"`
	Y         int    `json:"y,omitempty"`
	TileType  string `json:"tileType,omitempty"`
	PromptID  string `json:"promptId,omitempty"`
	Accepted  bool   `json:"accepted,omitempty"`
	RentLevel int    `json:"rentLevel,omitempty"`
	Value     int    `json:"value,omitempty"`
	CarIndex  int    `json:"carIndex,omitempty"`
	Floor     int    `json:"floor,omitempty"`
	Pool      string `json:"pool,omitempty"` // "classic" | "new"
}

func rejected(reason string) CommandResult {
	return CommandResult{Accepted: false, Reason: reason}
}

// daypartOf reads time.daypartIndex; nil stands for the TS default
// `{ daypartIndex: 0 }`.
func daypartOf(time *TimeState) int {
	if time == nil {
		return 0
	}
	return time.DaypartIndex
}

func tileWidthOr(tileType string, def int) int {
	if w, ok := TileWidths[tileType]; ok {
		return w
	}
	return def
}

func tileCostOr(tileType string, def int) int {
	if c, ok := TileCosts[tileType]; ok {
		return c
	}
	return def
}

// truthy string lookup: JS `world.cells[key]` in a boolean context.
func hasStr(m *OrderedMap[string, string], key string) bool {
	return m.At(key) != ""
}

// ─── Infrastructure tiles (no PlacedObjectRecord) ─────────────────────────────

var infrastructureTiles = map[string]bool{"floor": true, "stairs": true}

const (
	cathedralAnchorLogicalFloor = 103
	cathedralSliceCount         = 5
	metroStackTileType          = "metro"
	metroStackHeight            = 3
)

var metroFamilies = [3]int{FamilyMetroTop, FamilyMetroMiddle, FamilyMetroBottom}

// Families whose rentLevel initialises to 1; all others initialise to 4.
var variantInitOneFamilies = map[int]bool{3: true, 4: true, 5: true, 7: true, 9: true, FamilyRetail: true}

var hotelInitFamilies = map[int]bool{FamilyHotelSingle: true, FamilyHotelTwin: true, FamilyHotelSuite: true}

// TS: commands.ts makePlacedObject
func makePlacedObject(x, y int, tileType string, world *WorldState, time *TimeState, vipFlag bool) *PlacedObjectRecord {
	width := tileWidthOr(tileType, 1)
	familyCode := TileToFamilyCode[tileType] // ?? 0
	sidecarIndex := allocSidecar(tileType, x, y, world)
	unitStatus := 0
	if familyCode == FamilyOffice {
		unitStatus = 0x10
	} else if hotelInitFamilies[familyCode] || familyCode == FamilyCondo {
		if daypartOf(time) < 4 {
			unitStatus = 0x18
		} else {
			unitStatus = 0x20
		}
	}
	rentLevel := 4
	if variantInitOneFamilies[familyCode] {
		rentLevel = 1
	}
	return &PlacedObjectRecord{
		LeftTileIndex:     x,
		RightTileIndex:    x + width - 1,
		ObjectTypeCode:    familyCode,
		UnitStatus:        unitStatus,
		LinkedRecordIndex: sidecarIndex,
		AuxValueOrTimer:   0,
		EvalLevel:         0xff,
		EvalScore:         -1,
		// Binary placement writes both +0x13 and +0x14 = 1 on fresh placement.
		DirtyFlag:               1,
		OccupiedFlag:            1,
		ActivationTickCount:     0,
		RentLevel:               rentLevel,
		HousekeepingClaimedFlag: 0,
		VipFlag:                 vipFlag,
	}
}

func newStackObject(left, width, familyCode, unitStatus, linked int) *PlacedObjectRecord {
	return &PlacedObjectRecord{
		LeftTileIndex:     left,
		RightTileIndex:    left + width - 1,
		ObjectTypeCode:    familyCode,
		UnitStatus:        unitStatus,
		LinkedRecordIndex: linked,
		EvalLevel:         0xff,
		EvalScore:         -1,
		DirtyFlag:         1,
		OccupiedFlag:      1,
		RentLevel:         4,
	}
}

// TS: commands.ts placeCathedralStack
func placeCathedralStack(x, y int, world *WorldState, ledger *LedgerState, freeBuild bool) CommandResult {
	tileType := "cathedral"
	tileWidth := tileWidthOr(tileType, 28)
	cost := tileCostOr(tileType, 0)
	logicalFloor := GroundY - y

	if logicalFloor != cathedralAnchorLogicalFloor {
		return rejected("Cathedral must be placed on floor 103")
	}
	if world.GateFlags.EvalSimIndex >= 0 && world.GateFlags.EvalSimIndex != 0xffff {
		return rejected("Cathedral already placed")
	}
	if x+tileWidth-1 >= world.Width {
		return rejected("Out of bounds")
	}
	if !freeBuild && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}

	rows := make([]int, cathedralSliceCount)
	for i := range rows {
		rows[i] = y + i
	}
	for _, rowY := range rows {
		if rowY < 0 || rowY >= world.Height {
			return rejected("Out of bounds")
		}
		for dx := 0; dx < tileWidth; dx++ {
			key := cellKey(x+dx, rowY)
			if hasStr(&world.CellToAnchor, key) {
				return rejected("Cell already occupied")
			}
			existing := world.Cells.At(key)
			if existing != "" && existing != "floor" {
				return rejected("Cell already occupied")
			}
		}
	}

	bottomY := y + cathedralSliceCount - 1
	supportY := bottomY + 1
	for dx := 0; dx < tileWidth; dx++ {
		if supportY < 0 || supportY >= world.Height || !hasStr(&world.Cells, cellKey(x+dx, supportY)) {
			return rejected("No support")
		}
	}

	for _, rowY := range rows {
		for dx := 0; dx < tileWidth; dx++ {
			world.Cells.Delete(cellKey(x+dx, rowY))
		}
	}

	visualAnchorKey := cellKey(x, y)
	var patch []CellPatch
	for slice := 0; slice < cathedralSliceCount; slice++ {
		rowY := bottomY - slice
		familyCode := FamilyCathedralBase + slice
		objKey := cellKey(x, rowY)
		for dx := 0; dx < tileWidth; dx++ {
			key := cellKey(x+dx, rowY)
			world.Cells.Set(key, tileType)
			if key != visualAnchorKey {
				world.CellToAnchor.Set(key, visualAnchorKey)
			}
			patch = append(patch, CellPatch{X: x + dx, Y: rowY, TileType: tileType, IsAnchor: key == visualAnchorKey})
		}
		world.PlacedObjects.Set(objKey, newStackObject(x, tileWidth, familyCode, 0, -1))
	}

	world.GateFlags.EvalSimIndex = YToFloor(bottomY)
	if !freeBuild {
		ledger.CashBalance -= cost
	}

	for _, rowY := range rows {
		FillRowGaps(rowY, world, &patch)
	}
	RunGlobalRebuilds(world, ledger)

	return CommandResult{Accepted: true, Patch: patch, EconomyChanged: cost > 0}
}

// TS: commands.ts placeMetroStationStack
func placeMetroStationStack(x, y int, world *WorldState, ledger *LedgerState, freeBuild bool, time *TimeState) CommandResult {
	tileWidth := tileWidthOr("metro", 30)
	cost := tileCostOr("metro", 0)
	if world.GateFlags.MetroStationFloorIndex >= 0 {
		return rejected("Metro station already placed")
	}
	if y < UndergroundY {
		return rejected("Metro station may only be placed underground")
	}
	if y+metroStackHeight-1 >= world.Height || x+tileWidth-1 >= world.Width {
		return rejected("Out of bounds")
	}

	// stackCells: a JS Set iterated in insertion order (row-major).
	var stackOrder []string
	stackCells := map[string]bool{}
	for rowOffset := 0; rowOffset < metroStackHeight; rowOffset++ {
		rowY := y + rowOffset
		for dx := 0; dx < tileWidth; dx++ {
			k := cellKey(x+dx, rowY)
			if !stackCells[k] {
				stackCells[k] = true
				stackOrder = append(stackOrder, k)
			}
		}
	}

	var floorToRemove []string
	for _, key := range stackOrder {
		if hasStr(&world.CellToAnchor, key) {
			return rejected("Cell already occupied")
		}
		existing := world.Cells.At(key)
		if existing == "" {
			continue
		}
		if existing == "floor" {
			floorToRemove = append(floorToRemove, key)
		} else {
			return rejected("Cell already occupied")
		}
	}

	for rowOffset := 0; rowOffset < metroStackHeight; rowOffset++ {
		rowY := y + rowOffset
		for dx := 0; dx < tileWidth; dx++ {
			supportY := rowY - 1
			supportKey := cellKey(x+dx, supportY)
			if supportY < 0 || (!hasStr(&world.Cells, supportKey) && !stackCells[supportKey]) {
				return rejected("No support")
			}
		}
	}

	initialStatus := 1
	if daypartOf(time) < 4 {
		initialStatus = 0
	}
	shouldCharge := !freeBuild
	if shouldCharge && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}

	for _, key := range floorToRemove {
		world.Cells.Delete(key)
	}

	visualAnchorKey := cellKey(x, y)
	var patch []CellPatch
	for rowOffset := 0; rowOffset < metroStackHeight; rowOffset++ {
		rowY := y + rowOffset
		objKey := cellKey(x, rowY)
		for dx := 0; dx < tileWidth; dx++ {
			key := cellKey(x+dx, rowY)
			world.Cells.Set(key, metroStackTileType)
			if key != visualAnchorKey {
				world.CellToAnchor.Set(key, visualAnchorKey)
			}
			patch = append(patch, CellPatch{X: x + dx, Y: rowY, TileType: metroStackTileType, IsAnchor: key == visualAnchorKey})
		}
		world.PlacedObjects.Set(objKey, newStackObject(x, tileWidth, metroFamilies[rowOffset], initialStatus, -1))
	}

	if shouldCharge {
		ledger.CashBalance -= cost
	}
	world.GateFlags.MetroPlaced = 1
	world.GateFlags.MetroStationFloorIndex = YToFloor(y)

	for rowOffset := 0; rowOffset < metroStackHeight; rowOffset++ {
		FillRowGaps(y+rowOffset, world, &patch)
	}
	RunGlobalRebuilds(world, ledger)

	return CommandResult{Accepted: true, Patch: patch, EconomyChanged: shouldCharge && cost > 0}
}

// allocSidecar allocates a sidecar for tiles that need one; returns the
// index or -1. TS: commands.ts allocSidecar
func allocSidecar(tileType string, x, y int, world *WorldState) int {
	var record SidecarRecord

	switch tileType {
	case "restaurant", "fastFood", "retail":
		availability := VenuePartial
		if tileType == "retail" {
			// Retail starts dormant (unrented); restaurant/fast-food start active
			availability = VenueDormant
		}
		record = &CommercialVenueRecord{
			OwnerSubtypeIndex: x,
			Capacity:          48,
			VisitCount:        0,
			// Binary allocate_facility_record (11b0:07d3) link-disabled branch
			// seeds record[+7] = 10.
			TodayVisitCount:      10,
			YesterdayVisitCount:  0,
			AcquireCount:         0,
			AvailabilityState:    availability,
			CurrentPopulation:    0,
			LastAcquireTick:      0,
			EligibilityThreshold: 0,
			RemainingCapacity:    0,
			PhaseASeed:           10,
			PhaseBSeed:           10,
			OverrideSeed:         10,
		}
	case "recyclingCenterUpper", "recyclingCenterLower", "parking":
		r := &ServiceRequestEntry{OwnerSubtypeIndex: x, CoverageFlag: intPtr(0)}
		if tileType == "parking" {
			r.FloorIndex = intPtr(YToFloor(y))
		}
		record = r
	case "medical":
		record = &MedicalCenterRecord{OwnerSubtypeIndex: x, PendingVisitorsCount: 0}
	}
	// Cinema and party hall allocate their sidecar in placeEntertainmentVenue.

	if record == nil {
		return -1
	}
	world.Sidecars = append(world.Sidecars, record)
	return len(world.Sidecars) - 1
}

// freeSidecar marks a sidecar invalid (demolished). TS: commands.ts freeSidecar
func freeSidecar(index int, world *WorldState) {
	if rec := GetSidecar(world, index); rec != nil {
		rec.SetOwnerSubtypeIndex(0xff)
	}
}

// TS: commands.ts getOverlayAnchorKey ("" == null)
func getOverlayAnchorKey(world *WorldState, x, y int) string {
	key := cellKey(x, y)
	if a := world.OverlayToAnchor.At(key); a != "" {
		return a
	}
	if hasStr(&world.Overlays, key) {
		return key
	}
	return ""
}

// TS: commands.ts elevatorModeForOverlay ("" == null)
func elevatorModeForOverlay(t string) string {
	switch t {
	case "elevator":
		return "standard"
	case "elevatorExpress":
		return "express"
	case "elevatorService":
		return "service"
	}
	return ""
}

// hasOverlappingMisalignedStairs: stairs may not horizontally overlap unless
// they share the exact anchor column. TS: commands.ts hasOverlappingMisalignedStairs
func hasOverlappingMisalignedStairs(world *WorldState, x, y, width int) bool {
	newRight := x + width - 1
	otherWidth := tileWidthOr("stairs", width)
	for _, e := range world.Overlays.Entries() {
		if e.Val != "stairs" {
			continue
		}
		ox, oy := parseKeyXY(e.Key)
		if ox == x && oy == y {
			continue
		}
		if ox == x {
			continue
		}
		otherRight := ox + otherWidth - 1
		if otherRight < x || ox > newRight {
			continue
		}
		return true
	}
	return false
}

// TS: commands.ts hasMisalignedAdjacentOverlay
// PORT-NOTE: the TS compares `world.overlays[anchorKey] !== type` with type
// fixed to "elevator", so express/service neighbours are never counted here
// (kept verbatim).
func hasMisalignedAdjacentOverlay(world *WorldState, x, y int, typ string, width int) bool {
	for _, adjacentY := range [2]int{y - 1, y + 1} {
		if adjacentY < 0 || adjacentY >= world.Height {
			continue
		}
		var adjacentAnchors []string // JS Set, insertion order
		seen := map[string]bool{}
		for dx := 0; dx < width; dx++ {
			anchorKey := getOverlayAnchorKey(world, x+dx, adjacentY)
			if anchorKey == "" {
				continue
			}
			if world.Overlays.At(anchorKey) != typ {
				continue
			}
			if !seen[anchorKey] {
				seen[anchorKey] = true
				adjacentAnchors = append(adjacentAnchors, anchorKey)
			}
		}
		if len(adjacentAnchors) == 0 {
			continue
		}
		if len(adjacentAnchors) > 1 {
			return true
		}
		anchorKey := adjacentAnchors[0]
		if anchorKey == "" {
			continue
		}
		anchorX, _ := parseKeyXY(anchorKey)
		if anchorX != x {
			return true
		}
	}
	return false
}

// Minimum empty-tile gap a shaft requires on each side, edge-to-edge.
const (
	elevatorStandardMinGap      = 4
	elevatorExpressMinGap       = 8
	elevatorNonexpressMaxFloors = 31
	elevatorMaxShafts           = 24
)

// TS: commands.ts shaftMinGap
func shaftMinGap(mode int) int {
	if mode == 0 {
		return elevatorExpressMinGap
	}
	return elevatorStandardMinGap
}

// findCarrierByColumnAndFloor resolves a carrier at column x whose served
// range covers floor. TS: commands.ts findCarrierByColumnAndFloor
func findCarrierByColumnAndFloor(world *WorldState, x, floor int) *CarrierRecord {
	for _, c := range world.Carriers {
		if c.Column == x && floor >= c.BottomServedFloor && floor <= c.TopServedFloor {
			return c
		}
	}
	return nil
}

// findCarrierAtClick resolves a carrier from a click, falling back to the
// closest segment at the same column. TS: commands.ts findCarrierAtClick
func findCarrierAtClick(world *WorldState, x, y int) *CarrierRecord {
	floor := YToFloor(y)
	if direct := findCarrierByColumnAndFloor(world, x, floor); direct != nil {
		return direct
	}
	var best *CarrierRecord
	bestDistance := math.MaxInt
	for _, c := range world.Carriers {
		if c.Column != x {
			continue
		}
		var distance int
		if floor < c.BottomServedFloor {
			distance = c.BottomServedFloor - floor
		} else {
			distance = floor - c.TopServedFloor
		}
		if distance < bestDistance {
			best = c
			bestDistance = distance
		}
	}
	return best
}

// TS: commands.ts overlayTypeToMode
func overlayTypeToMode(t string) int {
	if t == "elevatorExpress" {
		return 0
	}
	if t == "elevatorService" {
		return 2
	}
	return 1
}

// TS: commands.ts carrierOverlayWidth
func carrierOverlayWidth(mode int) int {
	if mode == 0 {
		return tileWidthOr("elevatorExpress", 6)
	}
	if mode == 2 {
		return tileWidthOr("elevatorService", 4)
	}
	return tileWidthOr("elevator", 4)
}

// TS: commands.ts tooCloseToExistingShaft
func tooCloseToExistingShaft(world *WorldState, x, width, newMode int) bool {
	newRight := x + width - 1
	for _, carrier := range world.Carriers {
		if carrier.Column == x {
			continue
		}
		otherWidth := carrierOverlayWidth(carrier.CarrierMode)
		otherRight := carrier.Column + otherWidth - 1
		gap := -1
		if carrier.Column > newRight {
			gap = carrier.Column - newRight - 1
		} else if x > otherRight {
			gap = x - otherRight - 1
		}
		requiredGap := max(shaftMinGap(newMode), shaftMinGap(carrier.CarrierMode))
		if gap < requiredGap {
			return true
		}
	}
	return false
}

// TS: commands.ts hasAdjacentElevatorModeConflict
func hasAdjacentElevatorModeConflict(world *WorldState, x, y int, typ string, width int) bool {
	mode := elevatorModeForOverlay(typ)
	if mode == "" {
		return false
	}
	for _, adjacentY := range [2]int{y - 1, y + 1} {
		if adjacentY < 0 || adjacentY >= world.Height {
			continue
		}
		for dx := 0; dx < width; dx++ {
			anchorKey := getOverlayAnchorKey(world, x+dx, adjacentY)
			if anchorKey == "" {
				continue
			}
			adjacentMode := elevatorModeForOverlay(world.Overlays.At(anchorKey))
			if adjacentMode == "" {
				continue
			}
			if adjacentMode != mode {
				return true
			}
		}
	}
	return false
}

// ─── Global rebuilds ──────────────────────────────────────────────────────────

// appendParkingCoveragePatches appends a patch for every parking anchor with
// its current coverageFlag. TS: commands.ts appendParkingCoveragePatches
func appendParkingCoveragePatches(world *WorldState, patch *[]CellPatch) {
	parkingCode := TileToFamilyCode["parking"]
	for _, e := range world.PlacedObjects.Entries() {
		obj := e.Val
		if obj.ObjectTypeCode != parkingCode {
			continue
		}
		x, y := parseKeyXY(e.Key)
		sr, ok := GetSidecar(world, obj.LinkedRecordIndex).(*ServiceRequestEntry)
		if !ok {
			continue
		}
		cov := 0
		if sr.CoverageFlag != nil {
			cov = *sr.CoverageFlag
		}
		*patch = append(*patch, CellPatch{X: x, Y: y, TileType: "parking", IsAnchor: true, CoverageFlag: intPtr(cov)})
	}
}

// RunGlobalRebuilds runs all post-build / post-demolish global rebuilds.
// Order matters: carriers -> special_links -> walkability -> transfer_cache.
// TS: commands.ts runGlobalRebuilds
func RunGlobalRebuilds(world *WorldState, ledger *LedgerState) {
	world.GateFlags.OfficePlaced = 0
	world.GateFlags.MetroPlaced = 0
	world.GateFlags.MetroStationFloorIndex = -1
	world.GateFlags.SecurityPlaced = 0
	world.GateFlags.VipSuiteFloor = 0xffff
	world.GateFlags.RecyclingCenterCount = 0
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if object.ObjectTypeCode == FamilyOffice {
			world.GateFlags.OfficePlaced = 1
		}
		if object.ObjectTypeCode == FamilyMetroTop {
			world.GateFlags.MetroPlaced = 1
			_, y := parseKeyXY(e.Key)
			world.GateFlags.MetroStationFloorIndex = YToFloor(y)
		}
		if object.ObjectTypeCode == FamilySecurity {
			world.GateFlags.SecurityPlaced = 1
		}
		if object.ObjectTypeCode == FamilyRecyclingCenterUpper {
			world.GateFlags.RecyclingCenterCount++
		}
	}

	RebuildFacilityLedger(ledger, world)
	RebuildRuntimeSims(world)
	RebuildParkingCoverage(world)
	RebuildParkingDemandLog(world)
	RebuildCarrierList(world)
	RebuildSpecialLinkRouteRecords(world)
	RebuildRouteReachabilityTables(world)
	RebuildTransferGroupCache(world)
	RecomputeCommercialVenueBuckets(world)
}

// recyclingStackOverlapOK: true when there is no existing stack or the
// proposed floor overlaps one (anchor-2 .. anchor+1).
// TS: commands.ts hasRecyclingStackOverlap
func hasRecyclingStackOverlap(world *WorldState, proposedFloor int) bool {
	hasExisting := false
	overlaps := false
	for _, e := range world.PlacedObjects.Entries() {
		obj := e.Val
		if obj.ObjectTypeCode != FamilyRecyclingCenterUpper && obj.ObjectTypeCode != FamilyRecyclingCenterLower {
			continue
		}
		hasExisting = true
		_, oy := parseKeyXY(e.Key)
		existingFloor := YToFloor(oy)
		if proposedFloor >= existingFloor-2 && proposedFloor <= existingFloor+1 {
			overlaps = true
			break
		}
	}
	return !hasExisting || overlaps
}

// TS: commands.ts placeRecyclingCenterStack
func placeRecyclingCenterStack(x, y int, normalizedTileType string, world *WorldState, ledger *LedgerState, freeBuild bool, time *TimeState) CommandResult {
	tileWidth := tileWidthOr("recyclingCenterUpper", 2)
	upperY := y - 1
	if normalizedTileType == "recyclingCenterUpper" {
		upperY = y
	}
	lowerY := upperY + 1
	cost := TileCosts["recyclingCenter"]

	if upperY < 0 || lowerY >= world.Height || x+tileWidth-1 >= world.Width {
		return rejected("Out of bounds")
	}
	if !freeBuild && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}
	if !hasRecyclingStackOverlap(world, YToFloor(upperY)) {
		return rejected("Recycling center must be placed near an existing recycling-center stack")
	}

	var stackOrder []string
	stackCells := map[string]bool{}
	for _, rowY := range [2]int{upperY, lowerY} {
		for dx := 0; dx < tileWidth; dx++ {
			k := cellKey(x+dx, rowY)
			if !stackCells[k] {
				stackCells[k] = true
				stackOrder = append(stackOrder, k)
			}
		}
	}

	var floorToRemove []string
	for _, key := range stackOrder {
		if hasStr(&world.CellToAnchor, key) {
			return rejected("Cell already occupied")
		}
		existing := world.Cells.At(key)
		if existing != "" {
			if existing == "floor" {
				floorToRemove = append(floorToRemove, key)
			} else {
				return rejected("Cell already occupied")
			}
		}
	}

	for _, rowY := range [2]int{upperY, lowerY} {
		supportY := rowY + 1
		if rowY >= UndergroundY {
			supportY = rowY - 1
		}
		for dx := 0; dx < tileWidth; dx++ {
			supportKey := cellKey(x+dx, supportY)
			if supportY < 0 || supportY >= world.Height || (!hasStr(&world.Cells, supportKey) && !stackCells[supportKey]) {
				return rejected("No support")
			}
		}
	}

	for _, key := range floorToRemove {
		world.Cells.Delete(key)
	}
	rowsTypes := [2]struct {
		rowY     int
		tileType string
	}{{upperY, "recyclingCenterUpper"}, {lowerY, "recyclingCenterLower"}}
	for _, rt := range rowsTypes {
		world.Cells.Set(cellKey(x, rt.rowY), rt.tileType)
		for dx := 1; dx < tileWidth; dx++ {
			world.Cells.Set(cellKey(x+dx, rt.rowY), rt.tileType)
			world.CellToAnchor.Set(cellKey(x+dx, rt.rowY), cellKey(x, rt.rowY))
		}
		world.PlacedObjects.Set(cellKey(x, rt.rowY), makePlacedObject(x, rt.rowY, rt.tileType, world, time, false))
	}
	if !freeBuild {
		ledger.CashBalance -= cost
	}

	var patch []CellPatch
	for _, rt := range rowsTypes {
		record := world.PlacedObjects.At(cellKey(x, rt.rowY))
		for dx := 0; dx < tileWidth; dx++ {
			p := CellPatch{X: x + dx, Y: rt.rowY, TileType: rt.tileType, IsAnchor: dx == 0}
			if dx == 0 && record != nil {
				// evalActiveFlag is the user-facing scored bit: binary +0x14.
				p.EvalActiveFlag = intPtr(record.OccupiedFlag)
				p.UnitStatus = intPtr(record.UnitStatus)
			}
			patch = append(patch, p)
		}
	}

	FillRowGaps(upperY, world, &patch)
	FillRowGaps(lowerY, world, &patch)
	RunGlobalRebuilds(world, ledger)

	return CommandResult{Accepted: true, Patch: patch, EconomyChanged: cost > 0}
}

// entertainmentSubRecord: sub-record layout for a cinema or party hall.
// TS: commands.ts EntertainmentSubRecord
type entertainmentSubRecord struct {
	offsetX, rowY, width, familyCode int
}

// TS: commands.ts cinemaSubRecords
func cinemaSubRecords(x, upperY, lowerY int) []entertainmentSubRecord {
	return []entertainmentSubRecord{
		{x + 0, upperY, 7, FamilyCinemaStairsUpper},
		{x + 7, upperY, 24, FamilyCinemaLower},
		{x + 0, lowerY, 7, FamilyCinemaStairsLower},
		{x + 7, lowerY, 24, FamilyCinema},
	}
}

// TS: commands.ts partyHallSubRecords
func partyHallSubRecords(x, upperY, lowerY int) []entertainmentSubRecord {
	return []entertainmentSubRecord{
		{x, upperY, 27, FamilyPartyHall},
		{x, lowerY, 27, FamilyPartyHallLower},
	}
}

// sampleCinemaInitialSelector: the binary consumes 13 placement-time RNG
// samples before allocate_entertainment_link_record draws rand()%14.
// TS: commands.ts sampleCinemaInitialSelector
func sampleCinemaInitialSelector(world *WorldState) int {
	for i := 0; i < 13; i++ {
		SampleRng(world)
	}
	return SampleRng(world) % 14
}

// TS: commands.ts placeEntertainmentVenue
func placeEntertainmentVenue(x, y int, tileType string, world *WorldState, ledger *LedgerState, freeBuild bool, time *TimeState) CommandResult {
	tileWidth := tileWidthOr(tileType, 1)
	cost := tileCostOr(tileType, 0)
	upperY := y - 1
	lowerY := y

	if upperY < 0 || lowerY >= world.Height || x+tileWidth-1 >= world.Width {
		return rejected("Out of bounds")
	}
	if !freeBuild && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}

	var structOrder []string
	structureCells := map[string]bool{}
	for _, rowY := range [2]int{upperY, lowerY} {
		for dx := 0; dx < tileWidth; dx++ {
			k := cellKey(x+dx, rowY)
			if !structureCells[k] {
				structureCells[k] = true
				structOrder = append(structOrder, k)
			}
		}
	}

	var floorToRemove []string
	for _, key := range structOrder {
		if hasStr(&world.CellToAnchor, key) {
			return rejected("Cell already occupied")
		}
		existing := world.Cells.At(key)
		if existing != "" {
			if existing == "floor" {
				floorToRemove = append(floorToRemove, key)
			} else {
				return rejected("Cell already occupied")
			}
		}
	}

	supportY := lowerY + 1
	if lowerY >= UndergroundY {
		supportY = lowerY - 1
	}
	if supportY < 0 || supportY >= world.Height {
		return rejected("No support")
	}
	for dx := 0; dx < tileWidth; dx++ {
		supportKey := cellKey(x+dx, supportY)
		if !hasStr(&world.Cells, supportKey) && !structureCells[supportKey] {
			return rejected("No support")
		}
	}

	// Binary allocate_entertainment_link_record @ 1188:0073: venue_selector
	// is rand()%14 for the cinema stairway; party hall gets 0xff.
	upperHalfFloor := YToFloor(lowerY)
	lowerHalfFloor := upperHalfFloor - 1
	selector := 0xff
	if tileType == "cinema" {
		selector = sampleCinemaInitialSelector(world)
	}
	sidecar := &EntertainmentLinkRecord{
		OwnerSubtypeIndex:              x,
		PairedSubtypeIndex:             0xff,
		FamilySelectorOrSingleLinkFlag: selector,
		LowerHalfFloor:                 lowerHalfFloor,
		UpperHalfFloor:                 upperHalfFloor,
	}
	world.Sidecars = append(world.Sidecars, sidecar)
	sidecarIndex := len(world.Sidecars) - 1

	for _, key := range floorToRemove {
		world.Cells.Delete(key)
	}

	visualAnchorKey := cellKey(x, upperY)
	for _, rowY := range [2]int{upperY, lowerY} {
		for dx := 0; dx < tileWidth; dx++ {
			key := cellKey(x+dx, rowY)
			world.Cells.Set(key, tileType)
			if key != visualAnchorKey {
				world.CellToAnchor.Set(key, visualAnchorKey)
			}
		}
	}

	var subs []entertainmentSubRecord
	if tileType == "cinema" {
		subs = cinemaSubRecords(x, upperY, lowerY)
	} else {
		subs = partyHallSubRecords(x, upperY, lowerY)
	}
	for _, sub := range subs {
		// split_entertainment_object_into_stairway_pair sets both flags on placement.
		world.PlacedObjects.Set(cellKey(sub.offsetX, sub.rowY), newStackObject(sub.offsetX, sub.width, sub.familyCode, 0, sidecarIndex))
	}
	_ = time

	if !freeBuild {
		ledger.CashBalance -= cost
	}

	anchorRecord := world.PlacedObjects.At(visualAnchorKey)
	var patch []CellPatch
	for _, rowY := range [2]int{upperY, lowerY} {
		for dx := 0; dx < tileWidth; dx++ {
			isAnchor := dx == 0 && rowY == upperY
			p := CellPatch{X: x + dx, Y: rowY, TileType: tileType, IsAnchor: isAnchor}
			if isAnchor && anchorRecord != nil {
				p.EvalActiveFlag = intPtr(anchorRecord.OccupiedFlag)
				p.UnitStatus = intPtr(anchorRecord.UnitStatus)
			}
			patch = append(patch, p)
		}
	}

	FillRowGaps(upperY, world, &patch)
	FillRowGaps(lowerY, world, &patch)
	RunGlobalRebuilds(world, ledger)

	return CommandResult{Accepted: true, Patch: patch, EconomyChanged: cost > 0}
}

// ─── Place tile ───────────────────────────────────────────────────────────────

// HandlePlaceTile validates and applies a tile placement. time may be nil
// (TS default `{ daypartIndex: 0 }`).
// TS: commands.ts handlePlaceTile
func HandlePlaceTile(x, y int, tileType string, world *WorldState, ledger *LedgerState, freeBuild bool, time *TimeState) CommandResult {
	// LEGACY_TILE_ALIASES[LEGACY_VIP[t] ?? t] ?? LEGACY_VIP[t] ?? t
	vipStd, isVip := LegacyVipTileToStandard[tileType]
	inner := tileType
	if isVip {
		inner = vipStd
	}
	normalizedTileType := tileType
	if alias, ok := LegacyTileAliases[inner]; ok {
		normalizedTileType = alias
	} else if isVip {
		normalizedTileType = vipStd
	}
	vipFlag := isVip

	if !ValidTileTypes[normalizedTileType] {
		return rejected("Invalid tile type")
	}
	if x < 0 || x >= world.Width || y < 0 || y >= world.Height {
		return rejected("Out of bounds")
	}
	if y >= UndergroundY && !UndergroundAllowedTiles[normalizedTileType] {
		return rejected("This tile cannot be placed underground")
	}

	if normalizedTileType == metroStackTileType {
		return placeMetroStationStack(x, y, world, ledger, freeBuild, time)
	}

	metroFloor := world.GateFlags.MetroStationFloorIndex
	targetFloor := YToFloor(y)
	if metroFloor >= 0 &&
		(normalizedTileType == "cinema" || normalizedTileType == "partyHall" ||
			normalizedTileType == "recyclingCenter" || normalizedTileType == "recyclingCenterUpper") &&
		targetFloor < metroFloor {
		return rejected("This facility cannot be placed below the metro station")
	}

	if normalizedTileType == "cinema" || normalizedTileType == "partyHall" {
		return placeEntertainmentVenue(x, y, normalizedTileType, world, ledger, freeBuild, time)
	}

	if normalizedTileType == "cathedral" {
		return placeCathedralStack(x, y, world, ledger, freeBuild)
	}

	if normalizedTileType == "recyclingCenter" || normalizedTileType == "recyclingCenterUpper" || normalizedTileType == "recyclingCenterLower" {
		return placeRecyclingCenterStack(x, y, normalizedTileType, world, ledger, freeBuild, time)
	}

	// ── Elevator: overlay on a floor/lobby tile ─────────────────────────────
	if normalizedTileType == "elevator" || normalizedTileType == "elevatorExpress" || normalizedTileType == "elevatorService" {
		var patch []CellPatch
		overlayWidth := tileWidthOr(normalizedTileType, 1)
		if world.GateFlags.MetroStationFloorIndex >= 0 && YToFloor(y) < world.GateFlags.MetroStationFloorIndex-1 {
			return rejected("Elevator cannot extend below the metro station")
		}
		if x+overlayWidth-1 >= world.Width {
			return rejected("Out of bounds")
		}
		for dx := 0; dx < overlayWidth; dx++ {
			key := cellKey(x+dx, y)
			if hasStr(&world.Overlays, key) || hasStr(&world.OverlayToAnchor, key) {
				return rejected("Cell already has an overlay")
			}
		}
		// New shaft charges the base tile cost; extending an adjacent
		// same-column shaft charges the per-floor extension cost.
		placedFloor := YToFloor(y)
		isNewShaft := true
		for _, c := range world.Carriers {
			if c.Column == x && placedFloor >= c.BottomServedFloor-1 && placedFloor <= c.TopServedFloor+1 {
				isNewShaft = false
				break
			}
		}
		shaftMode := overlayTypeToMode(normalizedTileType)
		shaftCost := CarrierExtendFloorCost[shaftMode]
		if isNewShaft {
			shaftCost = tileCostOr(normalizedTileType, 0)
		}
		if !freeBuild && shaftCost > ledger.CashBalance {
			return rejected("Insufficient funds")
		}
		if isNewShaft && len(world.Carriers) >= elevatorMaxShafts {
			return rejected("Maximum " + strconv.Itoa(elevatorMaxShafts) + " elevator shafts per tower")
		}
		if normalizedTileType != "elevatorExpress" {
			topY := y
			bottomY := y
			for topY > 0 && world.Overlays.At(cellKey(x, topY-1)) == normalizedTileType {
				topY--
			}
			for bottomY < world.Height-1 && world.Overlays.At(cellKey(x, bottomY+1)) == normalizedTileType {
				bottomY++
			}
			if bottomY-topY+1 > elevatorNonexpressMaxFloors {
				label := "Standard"
				if normalizedTileType == "elevatorService" {
					label = "Service"
				}
				return rejected(label + " shaft may span at most " + strconv.Itoa(elevatorNonexpressMaxFloors) + " floors")
			}
		}
		if isNewShaft && tooCloseToExistingShaft(world, x, overlayWidth, shaftMode) {
			return rejected("Elevator too close to an adjacent shaft")
		}
		if hasMisalignedAdjacentOverlay(world, x, y, "elevator", overlayWidth) {
			return rejected("Elevator must align with adjacent shaft segments")
		}
		if hasAdjacentElevatorModeConflict(world, x, y, normalizedTileType, overlayWidth) {
			return rejected("Elevator shaft mode must match adjacent segments")
		}
		// Auto-place floor tiles where empty but supported.
		// PORT-NOTE: like the TS, a "requires a base tile" rejection here can
		// leave floor cells already written for earlier dx (no rollback).
		for dx := 0; dx < overlayWidth; dx++ {
			key := cellKey(x+dx, y)
			if hasStr(&world.Cells, key) || hasStr(&world.CellToAnchor, key) {
				continue
			}
			supportY := y + 1
			if y >= UndergroundY {
				supportY = y - 1
			}
			supportKey := cellKey(x+dx, supportY)
			if supportY < 0 || supportY >= world.Height || !hasStr(&world.Cells, supportKey) {
				return rejected("Elevator requires a base tile or support")
			}
			world.Cells.Set(key, "floor")
			patch = append(patch, CellPatch{X: x + dx, Y: y, TileType: "floor", IsAnchor: true})
		}
		world.Overlays.Set(cellKey(x, y), normalizedTileType)
		for dx := 1; dx < overlayWidth; dx++ {
			world.OverlayToAnchor.Set(cellKey(x+dx, y), cellKey(x, y))
		}
		patch = append(patch, CellPatch{X: x, Y: y, TileType: normalizedTileType, IsAnchor: true, IsOverlay: true})
		if !freeBuild {
			ledger.CashBalance -= shaftCost
		}
		RunGlobalRebuilds(world, ledger)
		return CommandResult{Accepted: true, Patch: patch}
	}

	// ── Stairs / Escalator: bridge overlay on existing base tiles ─────────────
	if normalizedTileType == "stairs" || normalizedTileType == "escalator" {
		overlayWidth := tileWidthOr(normalizedTileType, 1)
		overlayCost := tileCostOr(normalizedTileType, 0)
		if x+overlayWidth-1 >= GridWidth {
			return rejected("Out of bounds")
		}
		if !freeBuild && overlayCost > ledger.CashBalance {
			return rejected("Insufficient funds")
		}
		baseRequiredLabel := "Escalators"
		if normalizedTileType == "stairs" {
			baseRequiredLabel = "Stairs"
		}
		for dx := 0; dx < overlayWidth; dx++ {
			key := cellKey(x+dx, y)
			if !hasStr(&world.Cells, key) && !hasStr(&world.CellToAnchor, key) {
				return rejected(baseRequiredLabel + " require a base tile")
			}
			if hasStr(&world.Overlays, key) || hasStr(&world.OverlayToAnchor, key) {
				return rejected("Cell already has an overlay")
			}
		}
		if y == GroundY {
			for dx := 0; dx < overlayWidth; dx++ {
				upperLandingKey := cellKey(x+dx, y-1)
				if !hasStr(&world.Cells, upperLandingKey) && !hasStr(&world.CellToAnchor, upperLandingKey) {
					return rejected(baseRequiredLabel + " require a base tile on the floor above")
				}
			}
		}
		if normalizedTileType == "stairs" && hasOverlappingMisalignedStairs(world, x, y, overlayWidth) {
			return rejected("Stairs must align with existing stairs in the same column or not overlap them")
		}
		world.Overlays.Set(cellKey(x, y), normalizedTileType)
		for dx := 1; dx < overlayWidth; dx++ {
			world.OverlayToAnchor.Set(cellKey(x+dx, y), cellKey(x, y))
		}
		if !freeBuild {
			ledger.CashBalance -= overlayCost
		}
		RunGlobalRebuilds(world, ledger)
		return CommandResult{Accepted: true, Patch: []CellPatch{{X: x, Y: y, TileType: normalizedTileType, IsAnchor: true, IsOverlay: true}}}
	}

	// ── Standard tile placement ───────────────────────────────────────────────
	tileWidth := tileWidthOr(normalizedTileType, 1)
	cost := tileCostOr(normalizedTileType, 0)

	if x+tileWidth-1 >= world.Width {
		return rejected("Out of bounds")
	}
	if normalizedTileType == "lobby" && !IsValidLobbyY(y, world.LobbyMode) {
		cadence := "14, then 15"
		if world.LobbyMode == LobbyModeModern {
			cadence = "15"
		}
		return rejected("Lobby only allowed on ground floor or every 15 floors above (offsets +" + cadence + ")")
	}
	if !freeBuild && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}

	canReplaceFloor := normalizedTileType != "floor"
	var floorToRemove []string
	for dx := 0; dx < tileWidth; dx++ {
		key := cellKey(x+dx, y)
		if hasStr(&world.CellToAnchor, key) {
			return rejected("Cell already occupied")
		}
		existing := world.Cells.At(key)
		if existing != "" {
			if canReplaceFloor && existing == "floor" {
				floorToRemove = append(floorToRemove, key)
			} else {
				return rejected("Cell already occupied")
			}
		}
	}

	// Support from the adjacent row (below above-ground, above underground);
	// only the ground-floor lobby is exempt.
	if !(normalizedTileType == "lobby" && y == GroundY) {
		supportY := y + 1
		if y >= UndergroundY {
			supportY = y - 1
		}
		for dx := 0; dx < tileWidth; dx++ {
			supportKey := cellKey(x+dx, supportY)
			if supportY < 0 || supportY >= world.Height || !hasStr(&world.Cells, supportKey) {
				return rejected("No support")
			}
		}
	}

	// Parking ramps stack downward from the row just below ground.
	if normalizedTileType == "parkingRamp" {
		if y < UndergroundY {
			return rejected("Parking ramps may only be placed underground")
		}
		isTopOfStack := y == UndergroundY
		hasRampAbove := world.Cells.At(cellKey(x, y-1)) == "parkingRamp"
		if !isTopOfStack && !hasRampAbove {
			return rejected("Parking ramp must hang from the row above (top of stack at floor B1)")
		}
	}

	// Recycling-center stacks must overlap an existing 0x14/0x15 stack.
	// PORT-NOTE: unreachable in practice (recycling tiles return early via
	// placeRecyclingCenterStack above); kept verbatim.
	familyCode := TileToFamilyCode[normalizedTileType]
	if familyCode == FamilyRecyclingCenterUpper || familyCode == FamilyRecyclingCenterLower {
		proposedFloor := YToFloor(y)
		hasExisting := false
		overlaps := false
		for _, e := range world.PlacedObjects.Entries() {
			obj := e.Val
			if obj.ObjectTypeCode != FamilyRecyclingCenterUpper && obj.ObjectTypeCode != FamilyRecyclingCenterLower {
				continue
			}
			hasExisting = true
			_, oy := parseKeyXY(e.Key)
			existingFloor := YToFloor(oy)
			if proposedFloor >= existingFloor-2 && proposedFloor <= existingFloor+1 {
				overlaps = true
				break
			}
		}
		if hasExisting && !overlaps {
			return rejected("Recycling center must be placed near an existing recycling-center stack")
		}
	}

	// Apply placement
	for _, key := range floorToRemove {
		world.Cells.Delete(key)
	}
	world.Cells.Set(cellKey(x, y), normalizedTileType)
	for dx := 1; dx < tileWidth; dx++ {
		world.Cells.Set(cellKey(x+dx, y), normalizedTileType)
		world.CellToAnchor.Set(cellKey(x+dx, y), cellKey(x, y))
	}
	if !freeBuild {
		ledger.CashBalance -= cost
	}

	if !infrastructureTiles[normalizedTileType] {
		world.PlacedObjects.Set(cellKey(x, y), makePlacedObject(x, y, normalizedTileType, world, time, vipFlag))
	}

	record := world.PlacedObjects.At(cellKey(x, y))
	patch := make([]CellPatch, 0, tileWidth)
	for dx := 0; dx < tileWidth; dx++ {
		p := CellPatch{X: x + dx, Y: y, TileType: normalizedTileType, IsAnchor: dx == 0}
		if dx == 0 && record != nil {
			p.EvalActiveFlag = intPtr(record.OccupiedFlag)
			p.UnitStatus = intPtr(record.UnitStatus)
		}
		patch = append(patch, p)
	}

	FillRowGaps(y, world, &patch)

	RunGlobalRebuilds(world, ledger)

	if normalizedTileType == "parking" || normalizedTileType == "parkingRamp" {
		appendParkingCoveragePatches(world, &patch)
	}

	return CommandResult{Accepted: true, Patch: patch, EconomyChanged: cost > 0}
}

// ─── Remove tile ──────────────────────────────────────────────────────────────

// HandleRemoveTile TS: commands.ts handleRemoveTile
func HandleRemoveTile(x, y int, world *WorldState, ledger *LedgerState) CommandResult {
	if x < 0 || x >= world.Width || y < 0 || y >= world.Height {
		return rejected("Out of bounds")
	}
	clickedKey := cellKey(x, y)

	// Remove overlay first if present
	overlayAnchorKey := world.OverlayToAnchor.At(clickedKey)
	if overlayAnchorKey == "" && hasStr(&world.Overlays, clickedKey) {
		overlayAnchorKey = clickedKey
	}
	if overlayAnchorKey != "" {
		overlayType := world.Overlays.At(overlayAnchorKey)
		ow := tileWidthOr(overlayType, 1)
		ax, _ := parseKeyXY(overlayAnchorKey)
		world.Overlays.Delete(overlayAnchorKey)
		for dx := 1; dx < ow; dx++ {
			world.OverlayToAnchor.Delete(cellKey(ax+dx, y))
		}
		oax, oay := parseKeyXY(overlayAnchorKey)
		// Carrier overlays require a routing rebuild on removal
		switch overlayType {
		case "elevator", "elevatorExpress", "elevatorService", "escalator", "stairs":
			RunGlobalRebuilds(world, ledger)
		}
		return CommandResult{Accepted: true, Patch: []CellPatch{{X: oax, Y: oay, TileType: "empty", IsAnchor: true, IsOverlay: true}}}
	}

	anchorKey := world.CellToAnchor.At(clickedKey)
	if anchorKey == "" {
		anchorKey = clickedKey
	}
	tileType := world.Cells.At(anchorKey)
	if tileType == "" {
		return rejected("Cell is empty")
	}

	ax, ay := parseKeyXY(anchorKey)
	tileWidth := tileWidthOr(tileType, 1)
	isTwoFloor := tileType == "cinema" || tileType == "partyHall"
	isCathedral := tileType == "cathedral"
	isMetro := tileType == metroStackTileType
	var occupiedRows []int
	switch {
	case isCathedral:
		for i := 0; i < cathedralSliceCount; i++ {
			occupiedRows = append(occupiedRows, ay+i)
		}
	case isMetro:
		for i := 0; i < metroStackHeight; i++ {
			occupiedRows = append(occupiedRows, ay+i)
		}
	case isTwoFloor:
		occupiedRows = []int{ay, ay + 1}
	default:
		occupiedRows = []int{ay}
	}
	topRow := occupiedRows[0]
	baseRow := occupiedRows[len(occupiedRows)-1]

	// Replacement: floor if anything sits above or tile is between neighbours.
	hasAbove := false
	for dx := 0; dx < tileWidth && !hasAbove; dx++ {
		if hasStr(&world.Cells, cellKey(ax+dx, topRow-1)) {
			hasAbove = true
		}
	}
	hasLeft := false
	for lx := ax - 1; lx >= 0 && !hasLeft; lx-- {
		if hasStr(&world.Cells, cellKey(lx, baseRow)) {
			hasLeft = true
		}
	}
	hasRight := false
	for rx := ax + tileWidth; rx < world.Width && !hasRight; rx++ {
		if hasStr(&world.Cells, cellKey(rx, baseRow)) {
			hasRight = true
		}
	}
	turnToFloor := hasAbove || (hasLeft && hasRight)

	for _, rowY := range occupiedRows {
		for dx := 0; dx < tileWidth; dx++ {
			key := cellKey(ax+dx, rowY)
			world.Cells.Delete(key)
			if key != anchorKey {
				world.CellToAnchor.Delete(key)
			}
		}
	}

	// Remove PlacedObjectRecord(s) and free sidecars (entertainment venues
	// share one sidecar across 4 / 2 sub-records).
	var subRecordKeys []string
	if isTwoFloor || isCathedral || isMetro {
		for _, key := range world.PlacedObjects.Keys() {
			kx, ky := parseKeyXY(key)
			if ky >= ay && ky <= occupiedRows[len(occupiedRows)-1] && kx >= ax && kx <= ax+tileWidth-1 {
				subRecordKeys = append(subRecordKeys, key)
			}
		}
	} else {
		subRecordKeys = []string{anchorKey}
	}

	freedSidecars := map[int]bool{}
	for _, key := range subRecordKeys {
		rec := world.PlacedObjects.At(key)
		if rec == nil {
			continue
		}
		if rec.LinkedRecordIndex >= 0 && !freedSidecars[rec.LinkedRecordIndex] {
			if _, ok := GetSidecar(world, rec.LinkedRecordIndex).(*MedicalCenterRecord); ok {
				InvalidateMedicalSlotsForSidecar(world, rec.LinkedRecordIndex)
			}
			freeSidecar(rec.LinkedRecordIndex, world)
			freedSidecars[rec.LinkedRecordIndex] = true
		}
		world.PlacedObjects.Delete(key)
	}
	if isCathedral {
		world.GateFlags.EvalSimIndex = 0xffff
	}
	if isMetro {
		world.GateFlags.MetroPlaced = 0
		world.GateFlags.MetroStationFloorIndex = -1
	}

	CleanupSimsForRemovedTile(world, ax, ay)

	var patch []CellPatch
	for _, rowY := range occupiedRows {
		emitFloor := turnToFloor && rowY == baseRow
		for dx := 0; dx < tileWidth; dx++ {
			resultType := "empty"
			if emitFloor {
				resultType = "floor"
				world.Cells.Set(cellKey(ax+dx, rowY), "floor")
			}
			patch = append(patch, CellPatch{X: ax + dx, Y: rowY, TileType: resultType, IsAnchor: true})
		}
	}

	RunGlobalRebuilds(world, ledger)

	if tileType == "parking" || tileType == "parkingRamp" {
		appendParkingCoveragePatches(world, &patch)
	}

	return CommandResult{Accepted: true, Patch: patch}
}

// ─── Rent level adjustment ────────────────────────────────────────────────────

// rentAdjustableFamilies: families that support rent level 0-3.
var rentAdjustableFamilies = map[int]bool{3: true, 4: true, 5: true, 6: true, 7: true, 9: true, FamilyRetail: true, FamilyFastFood: true}

// HandleSetRentLevel TS: commands.ts handleSetRentLevel
func HandleSetRentLevel(x, y, rentLevel int, world *WorldState, time *TimeState) CommandResult {
	if rentLevel < 0 || rentLevel > 3 {
		return rejected("Rent level must be 0-3")
	}
	anchorKey := world.CellToAnchor.At(cellKey(x, y))
	if anchorKey == "" {
		anchorKey = cellKey(x, y)
	}
	record := world.PlacedObjects.At(anchorKey)
	if record == nil {
		return rejected("No facility here")
	}
	if !rentAdjustableFamilies[record.ObjectTypeCode] {
		return rejected("This facility does not have adjustable rent")
	}
	if record.ObjectTypeCode == FamilyCondo && record.UnitStatus < 0x18 {
		return rejected("Sold condos cannot change rent")
	}
	record.RentLevel = rentLevel
	_ = time
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// ─── Cinema movie pool ───────────────────────────────────────────────────────

var cinemaFamilyCodes = map[int]bool{
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
}

// HandleSetCinemaMoviePool cycles the selector within the chosen pool
// (classic = (cur+1)%7, new = ((cur+1)%7)+7), charges, and resets the age.
// TS: commands.ts handleSetCinemaMoviePool (binary MOVIETITLEDIALOGFILTER 0x1108:45C1)
func HandleSetCinemaMoviePool(x, y int, pool string, world *WorldState, ledger *LedgerState, freeBuild bool) CommandResult {
	anchorKey := world.CellToAnchor.At(cellKey(x, y))
	if anchorKey == "" {
		anchorKey = cellKey(x, y)
	}
	record := world.PlacedObjects.At(anchorKey)
	if record == nil || !cinemaFamilyCodes[record.ObjectTypeCode] {
		return rejected("Not a cinema")
	}
	sidecar, ok := GetSidecar(world, record.LinkedRecordIndex).(*EntertainmentLinkRecord)
	if !ok {
		return rejected("Cinema sidecar missing")
	}
	if sidecar.FamilySelectorOrSingleLinkFlag == 0xff {
		return rejected("Not a cinema")
	}

	cost := CinemaNewMovieCost
	if pool == "classic" {
		cost = CinemaClassicMovieCost
	}
	if !freeBuild && ledger.CashBalance < cost {
		return rejected("Insufficient funds")
	}

	cur := sidecar.FamilySelectorOrSingleLinkFlag
	next := ((cur + 1) % 7) + 7
	if pool == "classic" {
		next = (cur + 1) % 7
	}
	sidecar.FamilySelectorOrSingleLinkFlag = next
	sidecar.LinkAgeCounter = 0

	if !freeBuild {
		ledger.CashBalance -= cost
	}

	return CommandResult{Accepted: true, Patch: []CellPatch{}, EconomyChanged: !freeBuild}
}

// ─── Elevator car management ─────────────────────────────────────────────────

func activeCars(carrier *CarrierRecord) []*CarrierCar {
	var out []*CarrierCar
	for _, c := range carrier.Cars {
		if c.Active {
			out = append(out, c)
		}
	}
	return out
}

// HandleAddElevatorCar TS: commands.ts handleAddElevatorCar
func HandleAddElevatorCar(x, y int, world *WorldState, ledger *LedgerState, freeBuild bool) CommandResult {
	carrier := findCarrierAtClick(world, x, y)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	if len(activeCars(carrier)) >= 8 {
		return rejected("Maximum 8 cars per shaft")
	}
	cost := CarrierCarConstructionCost[carrier.CarrierMode]
	if !freeBuild && cost > ledger.CashBalance {
		return rejected("Insufficient funds")
	}
	if !freeBuild {
		ledger.CashBalance -= cost
	}
	// Binary add-car branch stores the click's floor as the car's home_floor.
	clicked := YToFloor(y)
	homeFloor := max(carrier.BottomServedFloor, min(carrier.TopServedFloor, clicked))
	// Activate first inactive car
	for _, car := range carrier.Cars {
		if !car.Active {
			car.Active = true
			car.CurrentFloor = homeFloor
			car.TargetFloor = homeFloor
			car.PrevFloor = homeFloor
			car.HomeFloor = homeFloor
			return CommandResult{Accepted: true, Patch: []CellPatch{}}
		}
	}
	// All existing cars are active — add a new one
	newCar := MakeCarrierCar(carrier.TopServedFloor-carrier.BottomServedFloor+1, homeFloor)
	carrier.Cars = append(carrier.Cars, newCar)
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// HandleRemoveElevatorCar prompts for confirmation when the carrier has
// live traffic, otherwise removes the last active car.
// TS: commands.ts handleRemoveElevatorCar
func HandleRemoveElevatorCar(x, y int, world *WorldState) CommandResult {
	carrier := findCarrierAtClick(world, x, y)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	if len(activeCars(carrier)) <= 1 {
		return rejected("Must keep at least 1 car")
	}
	hasActiveTraffic := len(carrier.PendingRoutes) > 0
	if !hasActiveTraffic {
		for _, car := range carrier.Cars {
			if car.AssignedCount > 0 || car.PendingAssignmentCount > 0 {
				hasActiveTraffic = true
				break
			}
			for _, slot := range car.ActiveRouteSlots {
				if slot.Active {
					hasActiveTraffic = true
					break
				}
			}
			if hasActiveTraffic {
				break
			}
		}
	}
	if hasActiveTraffic {
		world.EventState.PendingCarrierEditColumn = x
		world.EventState.PendingCarrierEditY = y
		world.PendingPrompts = append(world.PendingPrompts, SimPrompt{
			PromptID:   "carrier_remove_" + strconv.Itoa(x) + "_" + strconv.Itoa(y),
			PromptKind: PromptCarrierEditConfirmation,
			Message:    "Removing this elevator car will disrupt active traffic. Continue?",
		})
		return CommandResult{Accepted: true, Patch: []CellPatch{}}
	}
	return ApplyRemoveElevatorCar(world, x, y)
}

// ApplyRemoveElevatorCar deactivates the last active car.
// TS: commands.ts applyRemoveElevatorCar
func ApplyRemoveElevatorCar(world *WorldState, x, y int) CommandResult {
	carrier := findCarrierAtClick(world, x, y)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	cars := activeCars(carrier)
	if len(cars) <= 1 {
		return rejected("Must keep at least 1 car")
	}
	lastCar := cars[len(cars)-1]
	lastCar.Active = false
	lastCar.AssignedCount = 0
	lastCar.PendingAssignmentCount = 0
	lastCar.PendingRouteIDs = []string{}
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// HandleSetElevatorDwellDelay TS: commands.ts handleSetElevatorDwellDelay
func HandleSetElevatorDwellDelay(x, y, value int, world *WorldState) CommandResult {
	carrier := findCarrierAtClick(world, x, y)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	carrier.DwellDelay = filledInts(14, value)
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// HandleSetElevatorWaitingCarResponse TS: commands.ts handleSetElevatorWaitingCarResponse
func HandleSetElevatorWaitingCarResponse(x, y, value int, world *WorldState) CommandResult {
	carrier := findCarrierAtClick(world, x, y)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	carrier.WaitingCarResponseThreshold = value
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// HandleSetElevatorHomeFloor TS: commands.ts handleSetElevatorHomeFloor
func HandleSetElevatorHomeFloor(x, carIndex, floor int, world *WorldState) CommandResult {
	carrier := findCarrierByColumnAndFloor(world, x, floor)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	cars := activeCars(carrier)
	if carIndex < 0 || carIndex >= len(cars) {
		return rejected("Car not found")
	}
	car := cars[carIndex]
	if FloorToSlot(carrier, floor) < 0 {
		return rejected("Floor not served by this elevator")
	}
	car.HomeFloor = floor
	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// HandleToggleElevatorFloorStop toggles a floor stop; disabling clears the
// floor's route-status tags and evicts not-yet-boarded routes touching it.
// TS: commands.ts handleToggleElevatorFloorStop (binary FUN_10a8_0085)
func HandleToggleElevatorFloorStop(x, floor int, world *WorldState) CommandResult {
	carrier := findCarrierByColumnAndFloor(world, x, floor)
	if carrier == nil {
		return rejected("No elevator at this column")
	}
	slot := FloorToSlot(carrier, floor)
	if slot < 0 {
		return rejected("Floor not served by this elevator")
	}
	stop := 1
	if slot < len(carrier.StopFloorEnabled) {
		stop = carrier.StopFloorEnabled[slot]
	}
	wasEnabled := stop != 0
	// PORT-NOTE: JS assigns past the end of a short array (growing it with
	// holes); hydrate keeps stopFloorEnabled sized to the served span, so
	// out-of-range slots are grown here to stay equivalent.
	for len(carrier.StopFloorEnabled) <= slot {
		carrier.StopFloorEnabled = append(carrier.StopFloorEnabled, 1)
	}
	if wasEnabled {
		carrier.StopFloorEnabled[slot] = 0
	} else {
		carrier.StopFloorEnabled[slot] = 1
	}

	if wasEnabled {
		if slot < len(carrier.PrimaryRouteStatusByFloor) {
			if primaryTag := carrier.PrimaryRouteStatusByFloor[slot]; primaryTag != 0 {
				carrier.PrimaryRouteStatusByFloor[slot] = 0
				if i := primaryTag - 1; i >= 0 && i < len(carrier.Cars) {
					if car := carrier.Cars[i]; car != nil && car.PendingAssignmentCount > 0 {
						car.PendingAssignmentCount--
					}
				}
			}
		}
		if slot < len(carrier.SecondaryRouteStatusByFloor) {
			if secondaryTag := carrier.SecondaryRouteStatusByFloor[slot]; secondaryTag != 0 {
				carrier.SecondaryRouteStatusByFloor[slot] = 0
				if i := secondaryTag - 1; i >= 0 && i < len(carrier.Cars) {
					if car := carrier.Cars[i]; car != nil && car.PendingAssignmentCount > 0 {
						car.PendingAssignmentCount--
					}
				}
			}
		}

		var evictedIDs []string
		for _, route := range carrier.PendingRoutes {
			if !route.Boarded && (route.SourceFloor == floor || route.DestinationFloor == floor) {
				evictedIDs = append(evictedIDs, route.SimID)
			}
		}
		for _, evictedID := range evictedIDs {
			CancelRuntimeRouteRequest(carrier, evictedID)
		}
		if len(evictedIDs) > 0 {
			evictedSet := map[string]bool{}
			for _, id := range evictedIDs {
				evictedSet[id] = true
			}
			for _, sim := range world.Sims {
				if evictedSet[SimKey(sim)] {
					ClearSimRoute(sim)
				}
			}
		}
	}

	// Binary FUN_10a8_0085 unconditionally rebuilds the transfer-group cache
	// and route-reachability tables after any toggle.
	RebuildTransferGroupCache(world)
	RebuildRouteReachabilityTables(world)

	return CommandResult{Accepted: true, Patch: []CellPatch{}}
}

// ─── Gap-fill helper ──────────────────────────────────────────────────────────

// FillRowGaps fills supported horizontal gaps on row y with free floor tiles
// after a placement. TS: commands.ts fillRowGaps
func FillRowGaps(y int, world *WorldState, patch *[]CellPatch) {
	supportY := y + 1
	if y >= UndergroundY {
		supportY = y - 1
	}
	if supportY < 0 || supportY >= world.Height {
		return
	}

	leftmost := -1
	rightmost := -1
	for x := 0; x < world.Width; x++ {
		if hasStr(&world.Cells, cellKey(x, y)) {
			if leftmost == -1 {
				leftmost = x
			}
			rightmost = x
		}
	}
	if leftmost == -1 {
		return
	}

	for x := leftmost; x <= rightmost; x++ {
		key := cellKey(x, y)
		if hasStr(&world.Cells, key) {
			continue
		}
		if !hasStr(&world.Cells, cellKey(x, supportY)) {
			continue
		}
		world.Cells.Set(key, "floor")
		*patch = append(*patch, CellPatch{X: x, Y: y, TileType: "floor", IsAnchor: true})
	}
}
