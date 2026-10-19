package ui

import (
	"strconv"

	"simtower/internal/sim"
)

// simCatalog is the build palette over the verified gameplay catalogue
// (sim.CatalogCosts) with the sim's widths; unlock stars follow
// docs/mechanics.md section 2 (twin/suite at 3 stars, not the TS's 2).
var simCatalog = func() []BuildDef {
	type row struct {
		k      Kind
		name   string
		key    byte
		w, h   int
		stars  int
		desc   string
		perUni bool
	}
	rows := []row{
		{KindLobby, "Lobby", 'L', 4, 1, 1, "1F, then 15F, 30F, 45F... Drag to extend.", true},
		{KindOffice, "Office", 'O', 0, 1, 1, "6 workers; rent paid each quarter.", false},
		{KindCondo, "Condo", 'C', 0, 1, 1, "Sold once; 3 residents.", false},
		{KindFastFood, "Fast food", 'F', 0, 1, 1, "Open 10 AM - 9 PM; paid daily.", false},
		{KindElevator, "Elevator", 'E', 4, 1, 1, "Drag to set the height (max 31 floors).", false},
		{KindStairs, "Stairs", 'A', 0, 2, 1, "Links two floors.", false},
		{KindHotelSingle, "Hotel single", 'H', 0, 1, 2, "Needs housekeeping.", false},
		{KindHousekeeping, "Housekeeping", 'K', 0, 1, 2, "Cleans hotel rooms.", false},
		{KindSecurity, "Security", 'S', 0, 1, 2, "Needed for 3 stars.", false},
		{KindService, "Service elev", 'V', 4, 1, 2, "Housekeepers only.", false},
		{KindHotelTwin, "Hotel twin", 'T', 0, 1, 3, "Two guests.", false},
		{KindHotelSuite, "Hotel suite", 'U', 0, 1, 3, "VIPs stay here.", false},
		{KindRestaurant, "Restaurant", 'R', 0, 1, 3, "Open 5 PM - 11 PM; paid daily.", false},
		{KindRetail, "Retail shop", 'P', 0, 1, 3, "Rent paid each quarter.", false},
		{KindCinema, "Cinema", 'N', 0, 2, 3, "Two showings a day.", false},
		{KindPartyHall, "Party hall", 'Y', 0, 2, 3, "$20,000 a day when booked.", false},
		{KindExpress, "Express elev", 'X', 6, 1, 3, "Stops at lobbies and basements.", false},
		{KindEscalator, "Escalator", 'G', 0, 2, 3, "Moves crowds between shops.", false},
		{KindMedical, "Medical", 'M', 0, 1, 3, "Demanded near the lobby.", false},
		{KindParking, "Parking", 'J', 0, 1, 3, "Underground; needs a ramp.", false},
		{KindParkingRamp, "Parking ramp", 'Q', 0, 1, 3, "Underground, hanging from B1.", false},
		{KindRecycling, "Recycling", 'Z', 0, 2, 3, "Underground; one per 2,500 people.", false},
		{KindMetro, "Metro", 'I', 0, 3, 4, "Underground; one per tower.", false},
		{KindCathedral, "Cathedral", 'D', 0, 5, 5, "Built on 100F; weddings.", false},
		{KindFloor, "Floor", 'W', 1, 1, 1, "Empty floor area; drag to extend.", true},
		{KindBulldozer, "Bulldozer", 'B', 1, 1, 1, "Demolish; no refund.", false},
	}
	out := make([]BuildDef, 0, len(rows))
	for _, r := range rows {
		d := BuildDef{Kind: r.k, Name: r.name, Hotkey: r.key, W: r.w, H: r.h, MinStars: r.stars, Desc: r.desc, PerUnit: r.perUni}
		if t, ok := kindTile[r.k]; ok {
			d.Cost = int64(sim.CatalogCost(t))
			if d.W == 0 {
				d.W = sim.TileWidths[t]
				if t == "recyclingCenter" {
					d.W = sim.TileWidths["recyclingCenterUpper"]
				}
			}
		}
		out = append(out, d)
	}
	return out
}()

func simDef(k Kind) (BuildDef, bool) {
	for _, d := range simCatalog {
		if d.Kind == k {
			return d, true
		}
	}
	return BuildDef{}, false
}

const floorPremium = 500 // $ per new floor segment under a room (mechanics.md section 1)

// Shaft rules (commands.go constants).
const (
	shaftStdMinGap  = 4
	shaftExpMinGap  = 8
	shaftMaxFloors  = 31
	shaftMaxCount   = 24
	cathedralFloor  = 103
	metroStackRows  = 3
	cathedralSlices = 5
)

// planStep is one sim command of a placement.
type planStep struct {
	tile string
	x, y int
	cost int
}

// virtual holds cells/overlays a multi-step placement adds before the sim
// has seen them, so later steps validate against the earlier ones.
type virtual struct {
	cells    map[string]string
	overlays map[string]string
}

func (v *virtual) cell(w *sim.WorldState, k string) bool {
	if v != nil && v.cells[k] != "" {
		return true
	}
	return w.Cells.At(k) != "" || w.CellToAnchor.At(k) != ""
}

func (v *virtual) overlayAt(w *sim.WorldState, k string) string {
	if v != nil {
		if t := v.overlays[k]; t != "" {
			return t
		}
	}
	return w.Overlays.At(k)
}

// CanPlace validates a placement with the rules commands.go applies,
// without touching the world, and prices it with the gameplay catalogue.
func (m *SimModel) CanPlace(k Kind, x, f, w, h int) Result {
	_, res := m.plan(k, x, f, w, h)
	return res
}

func (m *SimModel) plan(k Kind, x, f, w, h int) ([]planStep, Result) {
	def, ok := simDef(k)
	if !ok || k == KindBulldozer {
		return nil, Result{Reason: "Unknown item."}
	}
	if def.MinStars > m.Stars() {
		return nil, Result{Reason: def.Name + " requires " + itoa(def.MinStars) + " stars."}
	}
	b := m.Bounds()
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if x < 0 || x+w > b.Width || f < b.MinFloor || f+h-1 > b.MaxFloor {
		return nil, Result{Reason: "Outside the building lot."}
	}
	tile := kindTile[k]
	wd := m.world()
	var steps []planStep
	reason := ""
	switch {
	case k == KindFloor || k == KindLobby:
		y := yOfUI(f)
		for u := x; u < x+w; u++ {
			key := ckey(u, y)
			cur := wd.Cells.At(key)
			if wd.CellToAnchor.Has(key) || (cur != "" && (k == KindFloor || cur == "lobby")) {
				continue // already built: a drag fills only what is missing
			}
			if r := m.checkStandard(tile, u, y); r != "" {
				if reason == "" {
					reason = r
				}
				continue
			}
			cost := sim.CatalogCost(tile)
			if k == KindLobby && cur == "" {
				cost += floorPremium
			}
			steps = append(steps, planStep{tile, u, y, cost})
		}
		if len(steps) == 0 && reason == "" {
			reason = "Already built."
		}
		if len(steps) > 0 && k == KindLobby {
			reason = "" // partial drags build what they can
		}
	case k.IsShaft():
		v := &virtual{cells: map[string]string{}, overlays: map[string]string{}}
		for ff := f; ff < f+h; ff++ {
			y := yOfUI(ff)
			if v.overlayAt(wd, ckey(x, y)) == tile {
				continue // this floor of the shaft exists already
			}
			r, cost, floors := m.checkShaft(tile, x, y, v)
			if r != "" {
				reason = r
				steps = nil
				break
			}
			steps = append(steps, planStep{tile, x, y, cost + floors*floorPremium})
			v.overlays[ckey(x, y)] = tile
			for dx := 1; dx < sim.TileWidths[tile]; dx++ {
				v.overlays[ckey(x+dx, y)] = "+"
			}
			for dx := 0; dx < sim.TileWidths[tile]; dx++ {
				if !v.cell(wd, ckey(x+dx, y)) {
					v.cells[ckey(x+dx, y)] = "floor"
				}
			}
		}
		if len(steps) == 0 && reason == "" {
			reason = "That elevator is already there."
		}
	case k == KindStairs || k == KindEscalator:
		y := yOfUI(f)
		if reason = m.checkBridge(tile, x, y); reason == "" {
			steps = append(steps, planStep{tile, x, y, sim.CatalogCost(tile)})
		}
	default:
		y, rows := yOfUI(f), 1
		switch k {
		case KindCinema, KindPartyHall, KindRecycling:
			rows = 2 // y is the lower row
		case KindMetro:
			rows = metroStackRows
			y = yOfUI(f + rows - 1) // anchor is the top row
		case KindCathedral:
			rows = cathedralSlices
			y = yOfUI(f + rows - 1)
		}
		reason = m.checkRoom(tile, x, y)
		if reason == "" {
			cost := sim.CatalogCost(tile)
			top := y
			if k == KindCinema || k == KindPartyHall || k == KindRecycling {
				top = y - 1
			}
			wdt := def.W
			for r := 0; r < rows; r++ {
				for dx := 0; dx < wdt; dx++ {
					if wd.Cells.At(ckey(x+dx, top+r)) == "" {
						cost += floorPremium
					}
				}
			}
			steps = append(steps, planStep{tile, x, y, cost})
		}
	}
	if reason != "" && len(steps) == 0 {
		return nil, Result{Reason: reason}
	}
	total := 0
	for _, s := range steps {
		total += s.cost
	}
	if total > m.ts.Cash() {
		return nil, Result{Reason: "Not enough money for construction (" + money(int64(total)) + ").", Cost: int64(total)}
	}
	return steps, Result{OK: true, Cost: int64(total)}
}

// Place runs the planned sim commands and charges the catalogue cost.
func (m *SimModel) Place(k Kind, x, f, w, h int) Result {
	m.refresh() // attribute earlier sim-side changes (fire) before ours
	steps, res := m.plan(k, x, f, w, h)
	if !res.OK {
		return res
	}
	led := m.ts.Ledger()
	var spent int64
	reason := ""
	for _, s := range steps {
		cost := s.cost
		if k.IsShaft() {
			// recompute: extension vs new shaft is decided against the live carriers
			_, c, floors := m.checkShaft(s.tile, s.x, s.y, nil)
			cost = c + floors*floorPremium
		}
		r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdPlaceTile, X: s.x, Y: s.y, TileType: s.tile})
		if !r.Accepted {
			if reason == "" {
				reason = r.Reason
			}
			if k.IsShaft() {
				break
			}
			continue
		}
		led.CashBalance -= cost
		spent += int64(cost)
	}
	m.construction += spent
	m.after()
	if spent == 0 && reason != "" {
		return Result{Reason: reason}
	}
	return Result{OK: true, Cost: spent}
}

// ---- validation (read-only ports of commands.go HandlePlaceTile) ----

func (m *SimModel) commonChecks(tile string, x, y int) string {
	w := m.world()
	if !sim.ValidTileTypes[tile] && tile != "recyclingCenter" {
		return "Invalid tile type"
	}
	if x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return "Out of bounds"
	}
	if y >= sim.UndergroundY && !sim.UndergroundAllowedTiles[tile] {
		return "This tile cannot be placed underground"
	}
	return ""
}

// checkStandard: the "standard tile placement" branch (floor, lobby and
// every one-row room).
func (m *SimModel) checkStandard(tile string, x, y int) string {
	if r := m.commonChecks(tile, x, y); r != "" {
		return r
	}
	w := m.world()
	width := sim.TileWidths[tile]
	if x+width-1 >= w.Width {
		return "Out of bounds"
	}
	if tile == "lobby" && !sim.IsValidLobbyY(y, w.LobbyMode) {
		return "Lobbies go on 1F and every 15th floor (15F, 30F, ...)."
	}
	for dx := 0; dx < width; dx++ {
		k := ckey(x+dx, y)
		if w.CellToAnchor.Has(k) {
			return "Cell already occupied"
		}
		if cur := w.Cells.At(k); cur != "" && (tile == "floor" || cur != "floor") {
			return "Cell already occupied"
		}
	}
	if !(tile == "lobby" && y == sim.GroundY) {
		sy := y + 1
		if y >= sim.UndergroundY {
			sy = y - 1
		}
		for dx := 0; dx < width; dx++ {
			if sy < 0 || sy >= w.Height || w.Cells.At(ckey(x+dx, sy)) == "" {
				return "No support: build on top of a floor (or below one underground)."
			}
		}
	}
	if tile == "parkingRamp" {
		if y < sim.UndergroundY {
			return "Parking ramps may only be placed underground"
		}
		if y != sim.UndergroundY && w.Cells.At(ckey(x, y-1)) != "parkingRamp" {
			return "Parking ramp must hang from the ramp above (top of the stack on B1)"
		}
	}
	if tile == "parking" && y < sim.UndergroundY {
		return "Parking must be built underground."
	}
	return ""
}

// checkRoom validates rooms, including the multi-row stacks. y is the row
// HandlePlaceTile expects (lower row for 2-floor rooms, top row for metro
// and cathedral).
func (m *SimModel) checkRoom(tile string, x, y int) string {
	if r := m.commonChecks(tile, x, y); r != "" {
		return r
	}
	w := m.world()
	metroFloor := w.GateFlags.MetroStationFloorIndex
	switch tile {
	case "metro":
		return m.checkMetro(x, y)
	case "cinema", "partyHall", "recyclingCenter":
		if metroFloor >= 0 && sim.YToFloor(y) < metroFloor {
			return "This facility cannot be placed below the metro station"
		}
	}
	switch tile {
	case "cinema", "partyHall":
		return m.checkTwoRow(tile, x, y, false)
	case "recyclingCenter":
		if y < sim.UndergroundY {
			return "Recycling centers must be built underground."
		}
		return m.checkTwoRow("recyclingCenterUpper", x, y, true)
	case "cathedral":
		return m.checkCathedral(x, y)
	}
	return m.checkStandard(tile, x, y)
}

func (m *SimModel) rowsFree(x, width int, rows []int) string {
	w := m.world()
	for _, ry := range rows {
		if ry < 0 || ry >= w.Height {
			return "Out of bounds"
		}
		for dx := 0; dx < width; dx++ {
			k := ckey(x+dx, ry)
			if w.CellToAnchor.Has(k) {
				return "Cell already occupied"
			}
			if cur := w.Cells.At(k); cur != "" && cur != "floor" {
				return "Cell already occupied"
			}
		}
	}
	return ""
}

func (m *SimModel) checkTwoRow(tile string, x, y int, recycling bool) string {
	w := m.world()
	width := sim.TileWidths[tile]
	upper, lower := y-1, y
	if upper < 0 || lower >= w.Height || x+width-1 >= w.Width {
		return "Out of bounds"
	}
	if recycling && !m.recyclingNear(sim.YToFloor(upper)) {
		return "Recycling center must be placed near an existing recycling-center stack"
	}
	if r := m.rowsFree(x, width, []int{upper, lower}); r != "" {
		return r
	}
	inStack := func(k string) bool {
		xx, yy := parseXY(k)
		return xx >= x && xx < x+width && (yy == upper || yy == lower)
	}
	check := func(ry int) string {
		sy := ry + 1
		if ry >= sim.UndergroundY {
			sy = ry - 1
		}
		if sy < 0 || sy >= w.Height {
			return "No support"
		}
		for dx := 0; dx < width; dx++ {
			k := ckey(x+dx, sy)
			if w.Cells.At(k) == "" && !inStack(k) {
				return "No support: build on top of a floor (or below one underground)."
			}
		}
		return ""
	}
	if recycling {
		if r := check(upper); r != "" {
			return r
		}
		return check(lower)
	}
	return check(lower)
}

func (m *SimModel) recyclingNear(proposed int) bool {
	has, near := false, false
	m.world().PlacedObjects.Range(func(k string, o *sim.PlacedObjectRecord) bool {
		if o.ObjectTypeCode != sim.FamilyRecyclingCenterUpper && o.ObjectTypeCode != sim.FamilyRecyclingCenterLower {
			return true
		}
		has = true
		_, oy := parseXY(k)
		ef := sim.YToFloor(oy)
		if proposed >= ef-2 && proposed <= ef+1 {
			near = true
			return false
		}
		return true
	})
	return !has || near
}

func (m *SimModel) checkMetro(x, y int) string {
	w := m.world()
	width := sim.TileWidths["metro"]
	if w.GateFlags.MetroStationFloorIndex >= 0 {
		return "Metro station already placed"
	}
	if y < sim.UndergroundY {
		return "Metro station may only be placed underground"
	}
	if y+metroStackRows-1 >= w.Height || x+width-1 >= w.Width {
		return "Out of bounds"
	}
	rows := []int{y, y + 1, y + 2}
	if r := m.rowsFree(x, width, rows); r != "" {
		return r
	}
	for _, ry := range rows {
		for dx := 0; dx < width; dx++ {
			sy := ry - 1
			if sy < 0 || (w.Cells.At(ckey(x+dx, sy)) == "" && !(sy >= y && sy < y+metroStackRows)) {
				return "No support: the metro hangs from the basement floor above."
			}
		}
	}
	return ""
}

func (m *SimModel) checkCathedral(x, y int) string {
	w := m.world()
	width := sim.TileWidths["cathedral"]
	if sim.GroundY-y != cathedralFloor {
		return "The cathedral is built on 100F (it rises to 104F)."
	}
	if w.GateFlags.EvalSimIndex >= 0 && w.GateFlags.EvalSimIndex != 0xffff {
		return "Cathedral already placed"
	}
	if x+width-1 >= w.Width {
		return "Out of bounds"
	}
	rows := make([]int, cathedralSlices)
	for i := range rows {
		rows[i] = y + i
	}
	if r := m.rowsFree(x, width, rows); r != "" {
		return r
	}
	sy := y + cathedralSlices
	for dx := 0; dx < width; dx++ {
		if sy >= w.Height || w.Cells.At(ckey(x+dx, sy)) == "" {
			return "No support"
		}
	}
	return ""
}

func (m *SimModel) checkBridge(tile string, x, y int) string {
	if r := m.commonChecks(tile, x, y); r != "" {
		return r
	}
	w := m.world()
	width := sim.TileWidths[tile]
	if x+width-1 >= sim.GridWidth {
		return "Out of bounds"
	}
	label := "Escalators"
	if tile == "stairs" {
		label = "Stairs"
	}
	for dx := 0; dx < width; dx++ {
		k := ckey(x+dx, y)
		if w.Cells.At(k) == "" && w.CellToAnchor.At(k) == "" {
			return label + " require a base tile"
		}
		if w.Overlays.At(k) != "" || w.OverlayToAnchor.At(k) != "" {
			return "Cell already has an overlay"
		}
	}
	if y == sim.GroundY {
		for dx := 0; dx < width; dx++ {
			k := ckey(x+dx, y-1)
			if w.Cells.At(k) == "" && w.CellToAnchor.At(k) == "" {
				return label + " require a base tile on the floor above"
			}
		}
	}
	if tile == "stairs" {
		right := x + width - 1
		bad := false
		w.Overlays.Range(func(k, t string) bool {
			if t != "stairs" {
				return true
			}
			ox, _ := parseXY(k)
			if ox == x {
				return true
			}
			if ox+width-1 < x || ox > right {
				return true
			}
			bad = true
			return false
		})
		if bad {
			return "Stairs must align with existing stairs in the same column or not overlap them"
		}
	}
	return ""
}

func shaftMode(tile string) int {
	switch tile {
	case "elevatorExpress":
		return 0
	case "elevatorService":
		return 2
	}
	return 1
}

func shaftWidthMode(mode int) int {
	if mode == 0 {
		return sim.TileWidths["elevatorExpress"]
	}
	return sim.TileWidths["elevator"]
}

func gapFor(mode int) int {
	if mode == 0 {
		return shaftExpMinGap
	}
	return shaftStdMinGap
}

// checkShaft validates one floor of an elevator overlay and returns its
// catalogue cost (new shaft or per-floor extension) and how many empty
// cells it auto-floors.
func (m *SimModel) checkShaft(tile string, x, y int, v *virtual) (string, int, int) {
	if r := m.commonChecks(tile, x, y); r != "" {
		return r, 0, 0
	}
	w := m.world()
	width := sim.TileWidths[tile]
	mf := w.GateFlags.MetroStationFloorIndex
	if mf >= 0 && sim.YToFloor(y) < mf-1 {
		return "Elevator cannot extend below the metro station", 0, 0
	}
	if x+width-1 >= w.Width {
		return "Out of bounds", 0, 0
	}
	for dx := 0; dx < width; dx++ {
		k := ckey(x+dx, y)
		if v.overlayAt(w, k) != "" || w.OverlayToAnchor.At(k) != "" {
			return "Cell already has an overlay", 0, 0
		}
	}
	floor := sim.YToFloor(y)
	isNew := true
	for _, c := range w.Carriers {
		if c.Column == x && floor >= c.BottomServedFloor-1 && floor <= c.TopServedFloor+1 {
			isNew = false
			break
		}
	}
	if isNew && v != nil && (v.overlays[ckey(x, y+1)] == tile || v.overlays[ckey(x, y-1)] == tile) {
		isNew = false
	}
	mode := shaftMode(tile)
	cost := sim.CarrierExtendFloorCost[mode]
	if isNew {
		cost = sim.CatalogCost(tile)
		if len(w.Carriers) >= shaftMaxCount {
			return "Maximum " + strconv.Itoa(shaftMaxCount) + " elevator shafts per tower", 0, 0
		}
	}
	if tile != "elevatorExpress" {
		top, bot := y, y
		for top > 0 && v.overlayAt(w, ckey(x, top-1)) == tile {
			top--
		}
		for bot < w.Height-1 && v.overlayAt(w, ckey(x, bot+1)) == tile {
			bot++
		}
		if bot-top+1 > shaftMaxFloors {
			return "A standard or service shaft may span at most " + strconv.Itoa(shaftMaxFloors) + " floors", 0, 0
		}
	}
	if isNew {
		right := x + width - 1
		for _, c := range w.Carriers {
			if c.Column == x {
				continue
			}
			oright := c.Column + shaftWidthMode(c.CarrierMode) - 1
			gap := -1
			if c.Column > right {
				gap = c.Column - right - 1
			} else if x > oright {
				gap = x - oright - 1
			}
			if gap < maxInt(gapFor(mode), gapFor(c.CarrierMode)) {
				return "Elevator too close to an adjacent shaft", 0, 0
			}
		}
	}
	anchorOf := func(xx, yy int) string {
		k := ckey(xx, yy)
		if a := w.OverlayToAnchor.At(k); a != "" {
			return a
		}
		if w.Overlays.At(k) != "" {
			return k
		}
		return ""
	}
	for _, ay := range [2]int{y - 1, y + 1} {
		if ay < 0 || ay >= w.Height {
			continue
		}
		var anchors []string
		for dx := 0; dx < width; dx++ {
			a := anchorOf(x+dx, ay)
			if a == "" {
				continue
			}
			t := w.Overlays.At(a)
			if t == "elevator" {
				dup := false
				for _, e := range anchors {
					dup = dup || e == a
				}
				if !dup {
					anchors = append(anchors, a)
				}
			}
			if am := elevMode(t); am != "" && am != elevMode(tile) {
				return "Elevator shaft mode must match adjacent segments", 0, 0
			}
		}
		if len(anchors) > 1 {
			return "Elevator must align with adjacent shaft segments", 0, 0
		}
		if len(anchors) == 1 {
			if ax, _ := parseXY(anchors[0]); ax != x {
				return "Elevator must align with adjacent shaft segments", 0, 0
			}
		}
	}
	floors := 0
	for dx := 0; dx < width; dx++ {
		k := ckey(x+dx, y)
		if v.cell(w, k) {
			continue
		}
		sy := y + 1
		if y >= sim.UndergroundY {
			sy = y - 1
		}
		if sy < 0 || sy >= w.Height || !v.cell(w, ckey(x+dx, sy)) {
			return "Elevator requires a base tile or support", 0, 0
		}
		floors++
	}
	return "", cost, floors
}

func elevMode(t string) string {
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
