package ui

import (
	"simtower/internal/screen"
)

// drawScene composes the main viewport exactly like scene() in the mock:
// sky, dirt, built floor area, grass, rooms, eval overlay, lobbies, walkers,
// shafts with cars and queues, then the build ghost and keyboard cursor.
// skyColor picks the sky background for the time of day. The building
// interior is black, so no sky state may be black: day is cyan, dusk magenta,
// night dark blue (stars and moon on it), rain light gray.
func skyColor(clk Clock) uint8 {
	switch {
	case clk.Light == Night:
		return 1
	case clk.Light == Dusk:
		return 5
	case clk.Rain:
		return 7
	}
	return 3
}

func (a *App) drawScene() {
	clk := a.m.Clock()
	t := a.tframe
	y0, vrows, cols := a.vy0, a.vrows, a.cols
	units := a.units
	night, dusk, rain := clk.Light == Night, clk.Light == Dusk, clk.Rain
	skyBg := skyColor(clk)
	// gutter + sky
	a.clip = rect{0, y0, cols, vrows}
	a.fill(0, y0, gutter, vrows, ' ', 7, 0)
	a.fill(gutter, y0, cols-gutter, vrows, ' ', 7, skyBg)
	a.clip = a.viewRect()
	groundTop := a.fy(0) // first row of 1F
	skyRows := minInt(groundTop-y0, vrows)
	if night {
		if skyRows > 0 {
			for i := 0; i < 40; i++ {
				ch := gCable
				if i%3 == 0 {
					ch = gDot
				}
				a.set(gutter+(i*37)%units, y0+(i*11)%skyRows, screen.Cell{Ch: ch, Fg: 15, Bg: skyBg})
			}
		}
		a.put(cols-9, y0+1, moonTop, 15, skyBg)
		a.put(cols-9, y0+2, moonBot, 15, skyBg)
	}
	if !night && !rain {
		cx := gutter + int(float64(t)*0.3)%(units-12)
		a.put(cx, y0+1, cloudA, 15, skyBg)
		a.put(cx+1, y0+2, cloudB, 15, skyBg)
		a.put(gutter+(int(float64(t)*0.2)+40)%(units-8), y0+4, cloudC, 15, skyBg)
		if !dusk {
			a.set(cols-6, y0+1, screen.Cell{Ch: gSun, Fg: 14, Bg: skyBg})
		}
	}
	if rain {
		for y := y0; y < y0+skyRows; y++ {
			for x := gutter; x < cols; x++ {
				if (x*7+y*3+t)%11 == 0 {
					a.set(x, y, screen.Cell{Ch: '/', Fg: 1, Bg: skyBg})
				}
			}
		}
	}
	// dirt below ground
	bottomFloor := a.topFloor - a.nfloors + 1
	for f := minInt(-1, a.topFloor); f >= bottomFloor; f-- {
		yy := a.fy(f)
		for r := 0; r < 3; r++ {
			for x := gutter; x < cols; x++ {
				ch := byte(' ')
				if (x+r)%3 == 0 {
					ch = gLight
				}
				a.set(x, yy+r, screen.Cell{Ch: ch, Fg: 6, Bg: 0})
			}
		}
	}
	if bottomFloor-1 < 0 {
		for y := a.fy(bottomFloor) + 3; y < y0+vrows; y++ {
			for x := gutter; x < cols; x++ {
				ch := byte(' ')
				if (x+y)%3 == 0 {
					ch = gLight
				}
				a.set(x, y, screen.Cell{Ch: ch, Fg: 6, Bg: 0})
			}
		}
	}
	// built floor area
	g0l, g0r := 0, 0
	for _, sp := range a.m.Floors() {
		if sp.Floor == 0 {
			g0l, g0r = sp.Left, sp.Right
		}
		if sp.Floor > a.topFloor+1 || sp.Floor < bottomFloor-1 {
			continue
		}
		yy := a.fy(sp.Floor)
		x0, x1 := maxInt(a.sx(sp.Left), gutter), minInt(a.sx(sp.Right), cols)
		if x1 > x0 {
			a.fill(x0, yy, x1-x0, 3, ' ', 7, 0)
		}
	}
	// grass at ground level outside the tower
	if gy := a.fy(0) + 2; gy >= y0 && gy < y0+vrows {
		for i := 0; i < units; i++ {
			if u := a.left + i; u < g0l || u >= g0r {
				a.set(gutter+i, gy, screen.Cell{Ch: gDark, Fg: 2, Bg: 0})
			}
		}
	}
	// rooms
	items := a.m.Items()
	for i := range items {
		it := &items[i]
		if it.Kind == KindLobby || it.Kind == KindFloor || !a.itemVisible(it) {
			continue
		}
		fr := t + it.X%3
		if it.Kind == KindParking {
			fr = it.X / 4 // per-space car color, not animated
		}
		Blit(a, a.sx(it.X), a.fy(it.Floor+it.H-1), it.Kind, tileStateFor(it), fr)
	}
	if a.evalMode != 0 {
		a.drawEvalOverlay(items)
	}
	// lobbies: amber band, pillars every 16 units, lamps, desk, plants
	for i := range items {
		it := &items[i]
		if it.Kind != KindLobby || !a.itemVisible(it) {
			continue
		}
		la, lb := it.X, it.X+it.W
		for f := it.Floor; f < it.Floor+maxInt(it.H, 1); f++ {
			yy := a.fy(f)
			x0, x1 := a.sx(la), a.sx(lb)
			a.fill(x0, yy, x1-x0, 3, ' ', 0, 6)
			for u := la; u < lb; u += 16 {
				for r := 0; r < 3; r++ {
					a.set(a.sx(u), yy+r, screen.Cell{Ch: gWallD, Fg: 15, Bg: 6})
				}
				if u+5 < lb {
					a.set(a.sx(u+5), yy, screen.Cell{Ch: gSun, Fg: 14, Bg: 6})
				}
				if u+12 < lb {
					a.set(a.sx(u+12), yy, screen.Cell{Ch: gSun, Fg: 14, Bg: 6})
				}
			}
			if f == it.Floor {
				mid := (la + lb) / 2
				a.put(a.sx(mid-1), yy+1, deskTop, 0, 6)
				a.put(a.sx(mid-1), yy+2, deskBot, 0, 6)
				a.set(a.sx(mid+4), yy+1, screen.Cell{Ch: gPlant, Fg: 2, Bg: 6})
				a.set(a.sx(mid-5), yy+1, screen.Cell{Ch: gPlant, Fg: 2, Bg: 6})
			}
		}
	}
	// walkers (two cells tall)
	for _, p := range a.m.People() {
		x := a.sx(p.X)
		yy := a.fy(p.Floor)
		if yy+2 < y0 || yy >= y0+vrows || x < gutter || x >= cols {
			continue
		}
		bg := a.get(x, yy+1).Bg
		var fg uint8
		if bg == 6 {
			fg = [3]uint8{0, 13, 4}[p.Stress]
		} else {
			fg = [3]uint8{15, 13, 12}[p.Stress]
		}
		if p.Named && p.Stress == StressNormal {
			fg = 9
		}
		legs := gLegsB
		if (t+p.ID)%2 == 1 {
			legs = gLegsA
		}
		a.set(x, yy+1, screen.Cell{Ch: gFace, Fg: fg, Bg: bg})
		a.set(x, yy+2, screen.Cell{Ch: legs, Fg: fg, Bg: bg})
	}
	a.drawShafts()
	a.drawGhost()
	a.drawCursor()
}

var (
	moonTop = CP("██")
	moonBot = CP("▀▀")
	cloudA  = CP("░▒▒░")
	cloudB  = CP("░░")
	cloudC  = CP("▒▒▒")
	deskTop = CP("▬▬▬")
	deskBot = CP("▀▀▀")
)

func (a *App) itemVisible(it *Item) bool {
	bottom := a.topFloor - a.nfloors
	if it.Floor > a.topFloor+1 || it.Floor+it.H-1 < bottom {
		return false
	}
	return it.X+it.W > a.left && it.X < a.left+a.units
}

var stressFg = [3]uint8{15, 13, 12}

func (a *App) drawShafts() {
	y0, vrows := a.vy0, a.vrows
	for si, s := range a.m.Shafts() {
		_ = si
		w := s.Kind.ShaftWidth()
		x := a.sx(s.X)
		if x+w <= gutter || x >= a.cols {
			continue
		}
		wall := gWallS
		if s.Kind == KindExpress {
			wall = gWallD
		}
		for f := s.Top; f >= s.Bottom; f-- {
			yy := a.fy(f)
			if yy+2 < y0 || yy >= y0+vrows {
				continue
			}
			for r := 0; r < 3; r++ {
				a.set(x, yy+r, screen.Cell{Ch: wall, Fg: 8})
				a.set(x+w-1, yy+r, screen.Cell{Ch: wall, Fg: 8})
				for i := 1; i < w-1; i++ {
					ch := byte(' ')
					if r == 1 {
						ch = gCable
					}
					a.set(x+i, yy+r, screen.Cell{Ch: ch, Fg: 8})
				}
			}
		}
		// home floors in red where no car sits
		for ci, c := range s.Cars {
			if c.Pos != c.Home*3 {
				a.set(x+1, a.fy(c.Home)+1, screen.Cell{Ch: byte('1' + ci%9), Fg: 12})
			}
		}
		for _, c := range s.Cars {
			cy := y0 + a.topFloor*3 - c.Pos
			if cy+2 < y0 || cy >= y0+vrows {
				continue
			}
			switch {
			case c.DoorsOpen:
				Blit(a, x+1, cy, s.Kind, TOpen, 0)
				for i := c.Passengers; i < w-2; i++ {
					a.set(x+1+i, cy+1, screen.Cell{Ch: ' ', Fg: 7})
				}
			default:
				Blit(a, x+1, cy, s.Kind, TMoving, 0)
				if c.Dir != 0 && c.Passengers > 0 {
					n := c.Passengers
					if lim := pow10(w - 2); n >= lim {
						n = lim - 1
					}
					a.put(x+1, cy+1, padLeft(itoa(n), w-2), 0, 7)
				}
			}
		}
		// queues: up on the left, down on the right (manual p.47)
		for _, q := range s.Queues {
			yy := a.fy(q.Floor)
			if yy+2 < y0 || yy >= y0+vrows {
				continue
			}
			for i, st := range q.Up {
				if i >= 12 {
					break
				}
				fg := stressFg[st]
				a.set(x-1-i, yy+1, screen.Cell{Ch: gFace, Fg: fg})
				a.set(x-1-i, yy+2, screen.Cell{Ch: gLegsA, Fg: fg})
			}
			for i, st := range q.Down {
				if i >= 12 {
					break
				}
				fg := stressFg[st]
				a.set(x+w+i, yy+1, screen.Cell{Ch: gFace, Fg: fg})
				a.set(x+w+i, yy+2, screen.Cell{Ch: gLegsA, Fg: fg})
			}
		}
	}
}

func pow10(n int) int {
	p := 1
	for ; n > 0; n-- {
		p *= 10
	}
	return p
}

// eval overlay colors
func evalFg(e Eval) uint8 { return [4]uint8{8, 9, 14, 12}[e] }

func priceFg(tier int) uint8 {
	switch tier {
	case 0:
		return 12
	case 1:
		return 14
	case 2:
		return 10
	case 3:
		return 11
	}
	return 8
}

func isHotel(k Kind) bool { return k == KindHotelSingle || k == KindHotelTwin || k == KindHotelSuite }

func (a *App) overlayColor(it *Item) (uint8, bool) {
	switch a.evalMode {
	case 1:
		return evalFg(it.Eval), true
	case 2:
		return priceFg(it.Rent), true
	case 3:
		if !isHotel(it.Kind) {
			return 0, false
		}
		if it.State == StateDirty || it.State == StateInfested {
			return 12, true
		}
		return 8, true
	}
	return 0, false
}

// drawEvalOverlay recolors room fills: blanks become a light shade in the
// overlay color, glyphs take the overlay color as foreground.
func (a *App) drawEvalOverlay(items []Item) {
	for i := range items {
		it := &items[i]
		if it.Kind == KindLobby || it.Kind == KindFloor || it.Kind == KindStairs || it.Kind == KindEscalator || !a.itemVisible(it) {
			continue
		}
		fg, ok := a.overlayColor(it)
		if !ok {
			continue
		}
		top := a.fy(it.Floor + it.H - 1)
		for r := 0; r < it.H*3; r++ {
			for u := 0; u < it.W-1; u++ {
				x, y := a.sx(it.X+u), top+r
				c := a.get(x, y)
				if c.Ch == ' ' {
					c.Ch = gLight
				}
				c.Fg, c.Bg = fg, 0
				a.set(x, y, c)
			}
		}
	}
}

// ---- ghost + cursor ----

type ghostCanvas struct {
	a  *App
	fg uint8
}

func (g ghostCanvas) set(x, y int, c screen.Cell) {
	if c.Ch == ' ' {
		return
	}
	c.Fg, c.Bg = g.fg, 0
	g.a.set(x, y, c)
}

// footprint returns the item rectangle the armed tool would place now.
func (a *App) footprint() (x, f, w, h int) {
	d := a.toolDef
	k := a.tool
	switch {
	case k.IsShaft():
		w = k.ShaftWidth()
		if a.dragging {
			return a.dx0, minInt(a.df0, a.df1), w, absInt(a.df1-a.df0) + 1
		}
		return a.dx1, a.df1, w, 1
	case k == KindLobby || k == KindFloor || k == KindBulldozer:
		if a.dragging {
			x0, x1 := minInt(a.dx0, a.dx1), maxInt(a.dx0, a.dx1)
			w = x1 - x0 + 1
			if w < d.W {
				w = d.W
			}
			return x0, a.df0, w, 1
		}
		return a.dx1, a.df1, maxInt(d.W, 1), 1
	}
	return a.dx1, a.df1, maxInt(d.W, 1), maxInt(d.H, 1)
}

func (a *App) updateGhost() {
	if a.tool == KindNone || !a.hoverOK {
		a.ghost.valid = false
		return
	}
	x, f, w, h := a.footprint()
	g := &a.ghost
	if g.valid && g.k == a.tool && g.x == x && g.f == f && g.w == w && g.h == h {
		return
	}
	g.k, g.x, g.f, g.w, g.h, g.valid = a.tool, x, f, w, h, true
	if a.tool == KindBulldozer {
		g.res = Result{OK: true}
		return
	}
	g.res = a.m.CanPlace(a.tool, x, f, w, h)
	if !g.res.OK && g.res.Reason != "" {
		a.say(g.res.Reason, true)
	} else if a.alertBad {
		a.alertUntil = 0
	}
}

func (a *App) drawGhost() {
	a.updateGhost()
	g := &a.ghost
	if !g.valid {
		return
	}
	var fg uint8 = 15
	if !g.res.OK {
		fg = 12
	}
	if g.k == KindBulldozer {
		fg = 14
	}
	top := a.fy(g.f + g.h - 1)
	if tw, _ := TileSize(g.k); tw > 0 && !g.k.IsShaft() && g.k != KindLobby {
		a.fill(a.sx(g.x), top, g.w, g.h*3, gLight, fg, 0)
		Blit(ghostCanvas{a, fg}, a.sx(g.x), top, g.k, tiles[g.k].states[0], a.tframe)
		return
	}
	a.fill(a.sx(g.x), top, g.w, g.h*3, gLight, fg, 0)
	if g.k.IsShaft() {
		wall := gWallS
		if g.k == KindExpress {
			wall = gWallD
		}
		for r := 0; r < g.h*3; r++ {
			a.set(a.sx(g.x), top+r, screen.Cell{Ch: wall, Fg: fg})
			a.set(a.sx(g.x+g.w-1), top+r, screen.Cell{Ch: wall, Fg: fg})
		}
	}
}

func (a *App) drawCursor() {
	if !a.kbd || a.focus != zoneView || len(a.views) > 0 {
		return
	}
	if (a.frame/3)%2 == 1 {
		return
	}
	x, y := a.sx(a.curX), a.fy(a.curF)+1
	c := a.get(x, y)
	if c.Bg == 7 {
		c.Fg, c.Bg = 15, 0
	} else {
		c.Fg, c.Bg = 0, 7
	}
	if c.Ch == ' ' {
		c.Ch = gFull
		c.Fg = 15
	}
	a.set(x, y, c)
}

// ---- gutter, status row, footer ----

func (a *App) drawGutter() {
	if a.zoom {
		return // zoom draws its own labels
	}
	for f := a.topFloor; f > a.topFloor-a.nfloors; f-- {
		var fg uint8 = 8
		if f == 0 {
			fg = 14
		}
		a.put(0, a.fy(f)+1, padLeft(floorLabel(f), 3), fg, 0)
	}
}

func (a *App) drawStatus() {
	a.fill(0, 0, a.cols, 1, ' ', 15, 1)
	a.statusSpots = a.statusSpots[:0]
	spot := func(x int, s string, fg, bg uint8, act action) {
		a.put(x, 0, s, fg, bg)
		h := hotspot{x: x, y: 0, w: len(s), h: 1, act: act}
		a.hots = append(a.hots, h)
		a.statusSpots = append(a.statusSpots, h)
	}
	funds := a.m.Funds()
	var ffg uint8 = 15
	if funds < 0 {
		ffg = 12
	}
	spot(1, money(funds), ffg, 1, actFinance)
	spot(16, "Pop "+commas(a.m.Population()), 15, 1, actRating)
	stars := a.m.Stars()
	st := "TOWER"
	if stars < 6 {
		st = ""
		for i := 0; i < stars; i++ {
			st += "*"
		}
	}
	spot(26, st, 14, 1, actRating)
	clk := a.m.Clock()
	spot(32, "Y"+itoa(clk.Year)+" Q"+itoa(clk.Quarter)+" "+clk.DayLabel+" "+clk.TimeString(), 15, 1, actMsgLog)
	sp := "||"
	switch a.m.Speed() {
	case 1:
		sp = speedNormal
	case 2:
		sp = speedFast
	}
	spot(a.cols-19, sp, 14, 1, actSpeed)
	spot(a.cols-14, "[BUILD]", 0, 7, actBuild)
	spot(a.cols-6, "[MAP]", 0, 7, actMap)
	if a.focus == zoneStatus && len(a.views) == 0 && len(a.statusSpots) > 0 {
		a.zoneIdx = clampInt(a.zoneIdx, 0, len(a.statusSpots)-1)
		h := a.statusSpots[a.zoneIdx]
		for i := 0; i < h.w; i++ {
			c := a.get(h.x+i, 0)
			c.Fg, c.Bg = 15, 3
			a.set(h.x+i, 0, c)
		}
	}
}

var (
	speedNormal = CP("►")
	speedFast   = CP("►►")
)

type barEntry struct {
	label string
	act   action
	gap   int // spaces before
}

var hotbarFull = []barEntry{
	{"B)uild", actBuild, 0}, {"I)nspect", actInspect, 1}, {"V)eval", actEval, 1}, {"M)ap", actMap, 1},
	{"F)inance", actFinance, 1}, {"Z)oom", actZoom, 1}, {"Space pause", actPause, 2},
	{"Arrows/Wheel scroll", actScrollHelp, 2}, {"Q)uit", actQuit, 2},
}

func (a *App) drawFooter() {
	// message line
	y := a.rows - 2
	a.fill(0, y, a.cols, 1, ' ', 7, 0)
	a.hot(0, y, a.cols, 1, actMsgLog, 0)
	switch {
	case a.alert != "" && a.frame < a.alertUntil:
		var fg uint8 = 15
		if a.alertBad {
			fg = 12
		}
		a.put(1, y, clip(a.alert, a.cols-2), fg, 0)
	case a.tool != KindNone:
		d := a.toolDef
		s := d.Name + "  " + costLabel(d) + "  Click to place, right-click/Esc cancel"
		if a.tool == KindBulldozer {
			s = "Bulldozer: click or drag to demolish, right-click/Esc cancel"
		}
		a.put(1, y, clip(s, a.cols-2), 15, 0)
	case a.evalMode != 0 && !a.zoom:
		a.put(1, y, clip(evalLegend[a.evalMode], a.cols-2), 11, 0)
	default:
		msgs := a.m.Messages()
		x := 1
		for i := len(msgs) - 1; i >= 0; i-- {
			s := msgs[i].Text
			if x > 1 {
				if x+3+len(s) > a.cols-1 {
					break
				}
				x += 3
			}
			var fg uint8 = 14
			if msgs[i].Alert {
				fg = 12
			}
			a.put(x, y, clip(s, a.cols-1-x), fg, 0)
			x += len(s)
		}
	}
	// hotkey bar
	y = a.rows - 1
	a.fill(0, y, a.cols, 1, ' ', 0, 7)
	total := 0
	for _, e := range hotbarFull {
		total += e.gap + len(e.label)
	}
	short := total > a.cols-2
	a.hotbarSpots = a.hotbarSpots[:0]
	x := 1
	for _, e := range hotbarFull {
		l := e.label
		if short && e.act == actScrollHelp {
			l = "Arrows scroll"
		}
		x += e.gap
		a.put(x, y, l, 0, 7)
		h := hotspot{x: x, y: y, w: len(l), h: 1, act: e.act}
		a.hots = append(a.hots, h)
		a.hotbarSpots = append(a.hotbarSpots, h)
		x += len(l)
	}
	if a.focus == zoneHotbar && len(a.views) == 0 && len(a.hotbarSpots) > 0 {
		a.zoneIdx = clampInt(a.zoneIdx, 0, len(a.hotbarSpots)-1)
		h := a.hotbarSpots[a.zoneIdx]
		a.ring(h.x, h.y, h.w)
	}
}

var evalLegend = [4]string{"",
	"EVAL: blue excellent, yellow good, red terrible, gray not rated   (V: pricing)",
	"PRICING: red high, yellow average, green low, cyan very low, gray n/a   (V: hotel)",
	"HOTEL: red rooms need housekeeping   (V: off)",
}

func costLabel(d BuildDef) string {
	s := money(d.Cost)
	if d.PerUnit {
		s += "/unit"
	}
	return s
}

// ---- zoom: one row per floor ----

type zstyle struct {
	ch  byte
	fg  uint8
	tag string
}

var zoomStyle = func() [numKinds]zstyle {
	var z [numKinds]zstyle
	sh := gMedium
	set := func(k Kind, fg uint8, tag string) { z[k] = zstyle{sh, fg, tag} }
	set(KindOffice, 11, "Of")
	set(KindCondo, 13, "Co")
	set(KindHotelSingle, 9, "H")
	set(KindHotelTwin, 9, "Tw")
	set(KindHotelSuite, 9, "Su")
	set(KindFastFood, 13, "FF")
	set(KindRestaurant, 12, "Rs")
	set(KindRetail, 14, "Sh")
	set(KindCinema, 12, "Cn")
	set(KindPartyHall, 13, "Pt")
	set(KindParkingRamp, 7, "Rp")
	set(KindParking, 7, "P")
	set(KindMetro, 7, "Mt")
	set(KindCathedral, 15, "Ca")
	set(KindMedical, 15, "Md")
	set(KindSecurity, 9, "Sc")
	set(KindHousekeeping, 7, "Hk")
	set(KindRecycling, 10, "Rc")
	z[KindStairs] = zstyle{'/', 7, ""}
	z[KindEscalator] = zstyle{CP("≡")[0], 11, ""}
	return z
}()

// zoomTop is the floor on the zoom view's first row: the main viewport band
// sits in the middle.
func (a *App) zoomTop() int {
	zt := a.topFloor + (a.vrows-a.nfloors)/2
	return clampInt(zt, a.bounds.MinFloor+a.vrows-1, a.bounds.MaxFloor)
}

func (a *App) drawZoom() {
	clk := a.m.Clock()
	y0, vrows, cols := a.vy0, a.vrows, a.cols
	zt := a.zoomTop()
	zy := func(f int) int { return y0 + zt - f }
	skyBg := skyColor(clk)
	a.clip = rect{0, y0, cols, vrows}
	bandTop, bandBot := a.topFloor, a.topFloor-a.nfloors+1
	for r := 0; r < vrows; r++ {
		f := zt - r
		y := y0 + r
		a.fill(0, y, gutter, 1, ' ', 7, 0)
		var fg, bg uint8 = 8, 0
		if f == 0 {
			fg = 14
		}
		if f <= bandTop && f >= bandBot {
			fg, bg = 15, 1
			if f == 0 {
				fg = 14
			}
		}
		a.put(0, y, padLeft(floorLabel(f), 3), fg, bg)
		if f >= 0 {
			a.fill(gutter, y, cols-gutter, 1, ' ', 7, skyBg)
		} else {
			for x := gutter; x < cols; x++ {
				ch := byte(' ')
				if (x+r)%3 == 0 {
					ch = gLight
				}
				a.set(x, y, screen.Cell{Ch: ch, Fg: 6})
			}
		}
	}
	a.clip = a.viewRect()
	g0l, g0r := 0, 0
	for _, sp := range a.m.Floors() {
		if sp.Floor == 0 {
			g0l, g0r = sp.Left, sp.Right
		}
		x0, x1 := maxInt(a.sx(sp.Left), gutter), minInt(a.sx(sp.Right), cols)
		if x1 > x0 {
			a.fill(x0, zy(sp.Floor), x1-x0, 1, ' ', 7, 0)
		}
	}
	if gy := zy(0); gy >= y0 && gy < y0+vrows {
		for i := 0; i < a.units; i++ {
			if u := a.left + i; u < g0l || u >= g0r {
				a.set(gutter+i, gy, screen.Cell{Ch: gDark, Fg: 2})
			}
		}
	}
	items := a.m.Items()
	for i := range items {
		it := &items[i]
		if it.Kind == KindFloor {
			continue
		}
		for f := it.Floor; f < it.Floor+maxInt(it.H, 1); f++ {
			y := zy(f)
			if it.Kind == KindLobby {
				for u := it.X; u < it.X+it.W; u++ {
					ch := byte(' ')
					if (u-it.X)%16 == 0 {
						ch = gWallD
					}
					a.set(a.sx(u), y, screen.Cell{Ch: ch, Fg: 15, Bg: 6})
				}
				continue
			}
			z := zoomStyle[it.Kind]
			fg := z.fg
			if c, ok := a.overlayColor(it); ok {
				fg = c
			}
			if it.State == StateBurning {
				fg = [2]uint8{12, 14}[(a.frame/2+u8(f))%2]
			}
			for u := 0; u < it.W; u++ {
				a.set(a.sx(it.X+u), y, screen.Cell{Ch: z.ch, Fg: fg})
			}
			if it.Kind != KindStairs && it.Kind != KindEscalator {
				a.set(a.sx(it.X+it.W-1), y, screen.Cell{Ch: gWallS, Fg: 8})
				if f == it.Floor && it.W > len(z.tag)+1 {
					a.put(a.sx(it.X), y, z.tag, 15, 0)
				}
			}
		}
	}
	for _, s := range a.m.Shafts() {
		w := s.Kind.ShaftWidth()
		wall := gWallS
		if s.Kind == KindExpress {
			wall = gWallD
		}
		for f := s.Bottom; f <= s.Top; f++ {
			y := zy(f)
			a.set(a.sx(s.X), y, screen.Cell{Ch: wall, Fg: 8})
			a.set(a.sx(s.X+w-1), y, screen.Cell{Ch: wall, Fg: 8})
			for i := 1; i < w-1; i++ {
				a.set(a.sx(s.X+i), y, screen.Cell{Ch: gCable, Fg: 8})
			}
		}
		for _, c := range s.Cars {
			f := floorDiv(c.Pos+1, 3)
			var fg uint8 = 15
			if s.Kind == KindExpress {
				fg = 11
			}
			for i := 1; i < w-1; i++ {
				a.set(a.sx(s.X+i), zy(f), screen.Cell{Ch: gFull, Fg: fg})
			}
		}
	}
	a.hot(gutter, y0, cols-gutter, vrows, actZoom, 1)
}

func u8(n int) int { return n & 1 }

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
