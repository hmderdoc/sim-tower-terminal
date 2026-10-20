package ui

import (
	"strings"

	"simtower/internal/screen"
	"simtower/internal/term"
)

// ================= build palette =================

type buildView struct {
	sel  int
	page int
	// layout of the last draw
	perPage int
}

func newBuildView(a *App) *buildView {
	v := &buildView{}
	cat := a.m.Catalog()
	for i, d := range cat {
		if d.Kind == a.tool {
			v.sel = i
		}
	}
	return v
}

const palColW = 33

func (v *buildView) geom(a *App) (px, py, pw, ph, ncol, rpc int) {
	n := len(a.m.Catalog())
	ncol = clampInt((a.cols-12)/palColW, 2, 4)
	maxRows := (a.vrows - 2 - 3) / 2
	rpc = clampInt((n+ncol-1)/ncol, 1, maxRows)
	pw = ncol*palColW + 2
	ph = rpc*2 + 3
	if pw < 68 {
		pw = 68
	}
	px, py, pw, ph = a.popupRect(pw, ph)
	return
}

func (v *buildView) draw(a *App) {
	cat := a.m.Catalog()
	px, py, pw, ph, ncol, rpc := v.geom(a)
	v.perPage = ncol * rpc
	v.sel = clampInt(v.sel, 0, len(cat)-1)
	v.page = v.sel / v.perPage
	pages := (len(cat) + v.perPage - 1) / v.perPage
	a.box(px, py, pw, ph, "BUILD", 15)
	stars := a.m.Stars()
	for i := v.page * v.perPage; i < len(cat) && i < (v.page+1)*v.perPage; i++ {
		d := cat[i]
		k := i - v.page*v.perPage
		col, row := k/rpc, k%rpc
		x, y := px+2+col*palColW, py+2+row*2
		lk := d.MinStars > stars
		hk, nm, st := uint8(14), uint8(7), uint8(14)
		if lk {
			hk, nm, st = 8, 8, 8
		}
		a.put(x, y, string(rune(d.Hotkey)), hk, 0)
		a.put(x+2, y, padRight(clip(d.Name, 13), 13), nm, 0)
		a.put(x+15, y, padLeft(clip(costLabel(d), 12), 12), nm, 0)
		if d.MinStars >= 2 {
			a.put(x+28, y, strings.Repeat("*", d.MinStars), st, 0)
		}
		v.preview(a, d, x+2, y+1, lk)
		if i == v.sel {
			for xx := x - 1; xx < x+palColW-1 && xx < px+pw-1; xx++ {
				c := a.get(xx, y)
				c.Bg = 1
				if c.Fg == 7 || c.Fg == 14 {
					c.Fg = 15
				}
				a.set(xx, y, c)
			}
		}
		a.hot(x-1, y, palColW-1, 2, actPalette, i)
	}
	if pages > 1 {
		s := " PgUp/PgDn " + itoa(v.page+1) + "/" + itoa(pages) + " "
		a.put(px+pw-2-len(s), py, s, 0, 7)
		a.hot(px+pw-2-len(s), py, len(s), 1, actPalettePg, 1)
	}
	d := cat[v.sel]
	size := itoa(d.W) + "x" + itoa(maxInt(d.H, 1))
	if d.Kind.IsShaft() {
		size = itoa(d.W) + " wide"
	}
	foot := " " + d.Name + "  " + costLabel(d) + "  " + size + "  " + d.Desc
	if d.MinStars > stars {
		foot += "  Requires " + itoa(d.MinStars) + " stars."
	}
	foot += " Click to place, right-click/Esc cancel "
	a.put(px+2, py+ph-1, clip(foot, pw-4), 0, 7)
}

func (v *buildView) preview(a *App, d BuildDef, x, y int, gray bool) {
	switch d.Kind {
	case KindFloor:
		fg := uint8(7)
		if gray {
			fg = 8
		}
		a.put(x, y, strings.Repeat(string(gUpHalf), 12), fg, 0)
	case KindBulldozer:
		fg := uint8(14)
		if gray {
			fg = 8
		}
		a.put(x, y, CP("▒▒▒ demolish"), fg, 0)
	default:
		previewRow(a, x, y, d.Kind, 1, 20, gray, a.tframe)
	}
}

func (v *buildView) pick(a *App, i int) {
	cat := a.m.Catalog()
	if i < 0 || i >= len(cat) {
		return
	}
	v.sel = i
	d := cat[i]
	if d.MinStars > a.m.Stars() {
		a.say(d.Name+" requires "+itoa(d.MinStars)+" stars.", true)
		return
	}
	a.closeTop()
	a.arm(d)
}

func (v *buildView) act(a *App, h *hotspot) bool {
	switch h.act {
	case actPalette:
		v.pick(a, h.arg)
		return true
	case actPalettePg:
		v.sel = (v.page + 1) * v.perPage
		if v.sel >= len(a.m.Catalog()) {
			v.sel = 0
		}
		return true
	}
	return false
}

func (v *buildView) key(a *App, ev term.Event) bool {
	cat := a.m.Catalog()
	_, _, _, _, _, rpc := v.geom(a)
	switch ev.Kind {
	case term.KeyUp:
		v.sel = (v.sel + len(cat) - 1) % len(cat)
	case term.KeyDown, term.KeyTab:
		v.sel = (v.sel + 1) % len(cat)
	case term.KeyLeft:
		v.sel = clampInt(v.sel-rpc, 0, len(cat)-1)
	case term.KeyRight:
		v.sel = clampInt(v.sel+rpc, 0, len(cat)-1)
	case term.KeyPgDn:
		v.sel = clampInt(v.sel+v.perPage, 0, len(cat)-1)
	case term.KeyPgUp:
		v.sel = clampInt(v.sel-v.perPage, 0, len(cat)-1)
	case term.KeyEnter:
		v.pick(a, v.sel)
	case term.KeyChar:
		if ev.Ch == ' ' {
			v.pick(a, v.sel)
			return true
		}
		c := upper(ev.Ch)
		for i, d := range cat {
			if d.Hotkey == c {
				v.pick(a, i)
				return true
			}
		}
		return false
	default:
		return false
	}
	return true
}

func (v *buildView) wheel(a *App, dir int, shift bool) {
	n := len(a.m.Catalog())
	v.sel = clampInt(v.sel-dir, 0, n-1)
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// ================= facility window =================

type facilityView struct{ id int }

func (v *facilityView) draw(a *App) {
	info := a.m.Inspect(v.id)
	var item *Item
	items := a.m.Items()
	for i := range items {
		if items[i].ID == v.id {
			item = &items[i]
		}
	}
	tw, th := TileSize(info.Kind)
	pw := clampInt(maxInt(tw+6, 62), 62, a.cols-2)
	extra := 0
	if info.Movie != "" {
		extra = 2
	}
	snap := minInt(th, maxInt(a.vrows-14-extra, 3))
	if tw == 0 {
		snap = 0
	}
	px, py, pw, ph := a.popupRect(pw, 10+snap+extra)
	a.box(px, py, pw, ph, info.Title, 15)
	if info.Demo {
		a.put(px+pw-9, py, " DEMO ", 0, 6)
	}
	x, y := px+2, py+1
	label := func(s string) { a.put(x, y, padRight(s, 10), 11, 0) }
	label("Status")
	a.put(x+10, y, clip(info.Status, 28), 15, 0)
	if info.Length != "" {
		a.put(px+pw-24, y, "Length", 11, 0)
		a.put(px+pw-17, y, clip(info.Length, 15), 15, 0)
	}
	y++
	if info.ShowCount {
		label("Customers")
		a.put(x+10, y, itoa(info.Customers)+" today", 15, 0)
	} else {
		label("Eval")
		fg := evalFg(info.Eval)
		n := [4]int{0, 20, 13, 6}[info.Eval]
		for i := 0; i < 20; i++ {
			ch, c := gLight, uint8(8)
			if i < n {
				ch, c = gFull, fg
			}
			a.set(x+10+i, y, screen.Cell{Ch: ch, Fg: c})
		}
		a.put(x+32, y, clip(info.EvalText, pw-36), fg, 0)
	}
	y++
	label("Occupants")
	ox := x + 10
	if len(info.Occupants) == 0 {
		a.put(ox, y, "none", 8, 0)
	}
	for i, pid := range info.Occupants {
		if ox+2 > px+pw-18 {
			a.put(ox, y, "+"+itoa(len(info.Occupants)-i), 7, 0)
			break
		}
		focused := a.focusable(ox, y, 1, actTenant, pid)
		var bg uint8
		if focused {
			bg = 1
		}
		a.set(ox, y, screen.Cell{Ch: gFace, Fg: 15, Bg: bg})
		ox += 2
	}
	if len(info.Occupants) > 0 {
		a.put(px+pw-17, y, "(click a face)", 8, 0)
	}
	y++
	label("Rent")
	if info.Rent < 0 || len(info.Tiers) == 0 {
		a.put(x+10, y, "n/a", 8, 0)
	} else if info.RentFixed != "" {
		a.put(x+10, y, clip(info.Tiers[info.Rent].Label+" ("+info.RentFixed+")", pw-14), 7, 0)
	} else {
		bx := x + 10
		for i, t := range info.Tiers {
			l := t.Label
			if i == info.Rent {
				l = ">" + l + "<"
			}
			bx += a.button(bx, y, l, actRent, i) + 1
		}
	}
	y++
	if info.Income != "" {
		label("Income")
		a.put(x+10, y, clip(info.Income, pw-14), 15, 0)
	}
	y++
	if info.Movie != "" {
		label("Movie")
		a.put(x+10, y, clip(info.Movie, pw-14), 15, 0)
		y++
		bx := x + 10
		bx += a.button(bx, y, "NEW MOVIE $300,000", actMovie, 0) + 2
		a.button(bx, y, "CLASSIC $150,000", actMovie, 1)
		y++
	}
	// snapshot of the room itself
	if snap > 0 && item != nil {
		y++
		sx := px + (pw-tw)/2
		old := a.clip
		a.clip = rect{px + 1, y, pw - 2, snap}
		top := y - (th - snap)
		Blit(a, sx, top, info.Kind, tileStateFor(item), a.tframe+item.X%3)
		a.clip = old
		y += snap
	}
	y++
	if info.Message != "" {
		a.put(x, minInt(y, py+ph-3), clip(info.Message, pw-4), 14, 0)
	}
	a.button(px+pw-10, py+ph-2, "CLOSE", actClose, 0)
}

func (v *facilityView) act(a *App, h *hotspot) bool {
	switch h.act {
	case actRent:
		a.m.SetRent(v.id, h.arg)
		return true
	case actMovie:
		if r := a.m.ChangeMovie(v.id, h.arg == 1); !r.OK {
			a.say(r.Reason, true)
		}
		return true
	case actTenant:
		a.open(&tenantView{id: h.arg})
		return true
	}
	return false
}

func (v *facilityView) key(a *App, ev term.Event) bool { return false }

// ================= tenant window =================

type tenantView struct {
	id      int
	editing bool
	buf     []byte
}

func (v *tenantView) draw(a *App) {
	ti := a.m.Tenant(v.id)
	px, py, pw, ph := a.popupRect(52, 10)
	a.box(px, py, pw, ph, "TENANT", 15)
	if ti.Demo {
		a.put(px+pw-9, py, " DEMO ", 0, 6)
	}
	x, y := px+2, py+1
	row := func(l, val string, fg uint8) {
		a.put(x, y, padRight(l, 10), 11, 0)
		a.put(x+10, y, clip(val, pw-14), fg, 0)
		y++
	}
	if v.editing {
		a.put(x, y, padRight("Name", 10), 11, 0)
		a.put(x+10, y, padRight(string(v.buf)+"_", 21), 15, 1)
		y++
	} else {
		row("Name", ti.Name, 15)
	}
	row("From", ti.From, 7)
	row("Going to", ti.GoingTo, 7)
	a.put(x, y, padRight("Stress", 10), 11, 0)
	sfg := [3]uint8{9, 14, 12}[ti.Stress]
	n := [3]int{5, 12, 20}[ti.Stress]
	for i := 0; i < 20; i++ {
		ch, c := gLight, uint8(8)
		if i < n {
			ch, c = gFull, sfg
		}
		a.set(x+10+i, y, screen.Cell{Ch: ch, Fg: c})
	}
	a.put(x+32, y, [3]string{"low", "medium", "high"}[ti.Stress], sfg, 0)
	y++
	a.put(x, y, padRight("Eval", 10), 11, 0)
	a.put(x+10, y, [4]string{"not rated", "excellent", "good", "terrible"}[ti.Eval], evalFg(ti.Eval), 0)
	bx := x
	if v.editing {
		bx += a.button(bx, py+ph-2, "OK", actRename, 1) + 2
		a.button(bx, py+ph-2, "CANCEL", actRename, 0)
		return
	}
	bx += a.button(bx, py+ph-2, "RENAME", actRename, 2) + 2
	a.button(bx, py+ph-2, "CLOSE", actClose, 0)
}

func (v *tenantView) act(a *App, h *hotspot) bool {
	if h.act != actRename {
		return false
	}
	switch h.arg {
	case 2:
		v.editing = true
		v.buf = append(v.buf[:0], a.m.Tenant(v.id).Name...)
		if len(v.buf) > 20 {
			v.buf = v.buf[:20]
		}
		a.focusIdx = 0
	case 1:
		if r := a.m.RenameTenant(v.id, string(v.buf)); !r.OK {
			a.say(r.Reason, true)
		}
		v.editing = false
	case 0:
		v.editing = false
	}
	return true
}

func (v *tenantView) key(a *App, ev term.Event) bool {
	if !v.editing {
		return false
	}
	switch ev.Kind {
	case term.KeyChar:
		if ev.Ch >= 0x20 && ev.Ch < 0x7F && len(v.buf) < 20 {
			v.buf = append(v.buf, ev.Ch)
		}
	case term.KeyBackspace, term.KeyDelete:
		if len(v.buf) > 0 {
			v.buf = v.buf[:len(v.buf)-1]
		}
	case term.KeyEnter:
		a.do(&hotspot{act: actRename, arg: 1}, 0, 0, false)
	case term.KeyEsc:
		v.editing = false
	default:
		return false
	}
	return true
}

// ================= elevator window =================

type elevatorView struct {
	id      int
	weekend bool
	scroll  int // floor rows scrolled from the top
	car     int // selected car for home setting
	gridN   int // focusables in the floor grid this frame
	rows    int // visible floor rows
	nfloors int
}

const elevPerRow = 9 // serve, home, 7 periods

func (v *elevatorView) shaft(a *App) *Shaft {
	sh := a.m.Shafts()
	for i := range sh {
		if sh[i].ID == v.id {
			return &sh[i]
		}
	}
	return nil
}

func (v *elevatorView) draw(a *App) {
	s := v.shaft(a)
	if s == nil {
		a.closeTop()
		return
	}
	sc := a.m.Schedule(v.id)
	pw := minInt(72, a.cols-2)
	px, py, pw, ph := a.popupRect(pw, minInt(a.vrows, 19+maxInt(0, (s.Top-s.Bottom+1)-8)))
	title := sc.Title + "   " + floorLabel(s.Bottom) + " - " + floorLabel(s.Top) + "   " + itoa(len(s.Cars)) + " car"
	if len(s.Cars) != 1 {
		title += "s"
	}
	title += "   capacity " + itoa(sc.Capacity)
	a.box(px, py, pw, ph, title, 15)
	a.put(px+2, py+1, "Floor Serve Home", 11, 0)
	cx := px + 21
	for j, p := range periodLabels {
		a.put(cx+j*7, py+1, padRight(p, 6), 11, 0)
	}
	// WD / WE selector
	wdFg, wdBg, weFg, weBg := uint8(15), uint8(1), uint8(7), uint8(0)
	if v.weekend {
		wdFg, wdBg, weFg, weBg = 7, 0, 15, 1
	}
	a.put(px+18, py+1, "WD", wdFg, wdBg)
	a.put(px+18, py+2, "WE", weFg, weBg)
	a.hot(px+18, py+1, 2, 2, actDayType, 0)

	nf := s.Top - s.Bottom + 1
	v.nfloors = nf
	v.rows = ph - 11
	v.scroll = clampInt(v.scroll, 0, maxInt(nf-v.rows, 0))
	day := 0
	if v.weekend {
		day = 1
	}
	v.gridN = 0
	for i := 0; i < v.rows && v.scroll+i < nf; i++ {
		f := s.Top - v.scroll - i
		y := py + 3 + i
		var lfg uint8 = 7
		if f == 0 {
			lfg = 14
		}
		a.put(px+2, y, padLeft(floorLabel(f), 3), lfg, 0)
		served := true
		if idx := f - s.Bottom; idx >= 0 && idx < len(sc.Served) {
			served = sc.Served[idx]
		}
		if a.focusable(px+7, y, 3, actServe, f) {
			defer a.ring(px+7, y, 3)
		}
		if served {
			a.put(px+7, y, "yes", 10, 0)
		} else {
			a.put(px+7, y, " x ", 12, 0)
		}
		home := ""
		for ci, c := range s.Cars {
			if c.Home == f {
				home += "c" + itoa(ci+1)
			}
		}
		if a.focusable(px+12, y, 5, actHome, f) {
			defer a.ring(px+12, y, 5)
		}
		a.put(px+12, y, padRight(clip(home, 5), 5), 14, 0)
		for j := range Periods {
			x := cx + j*7
			mode := sc.Modes[day][j]
			txt, fg := "local", uint8(7)
			switch mode {
			case ModeExpressUp:
				txt, fg = "EXP"+string(gArrowUp), 14
			case ModeExpressDown:
				txt, fg = "EXP"+string(gArrowDn), 14
			}
			if !served {
				txt, fg = "  -", 8
			}
			if a.focusable(x, y, 6, actMode, j) {
				defer a.ring(x, y, 6)
			}
			a.put(x, y, padRight(txt, 6), fg, 0)
		}
		v.gridN += elevPerRow
	}
	if nf > v.rows {
		a.put(px+pw-12, py+2, padLeft(itoa(v.scroll+1)+"-"+itoa(minInt(v.scroll+v.rows, nf))+"/"+itoa(nf), 10), 8, 0)
	}
	y := py + ph - 7
	a.put(px+2, y, "Waiting Car Response ", 7, 0)
	bx := px + 23
	bx += a.button(bx, y, " "+itoa(sc.Response)+" ", actResponse, 1)
	a.put(bx, y, " floors    Standard Floor Departure ", 7, 0)
	bx += 36
	bx += a.button(bx, y, " "+itoa(sc.Departure)+" ", actDeparture, 1)
	a.put(bx, y, " s", 7, 0)
	a.put(px+2, y+1, clip("Modes: local, EXP"+string(gArrowUp)+" express to top, EXP"+string(gArrowDn)+" express to bottom. Click toggles.", pw-4), 8, 0)
	a.put(px+2, y+2, clip("WD/WE (W) picks the day type. Home cell: move the selected car home.", pw-4), 8, 0)
	by := py + ph - 2
	bx = px + 2
	bx += a.button(bx, by, "ADD CAR "+money(sc.CarCost), actAddCar, 0) + 2
	bx += a.button(bx, by, "REMOVE CAR", actRemoveCar, 0) + 2
	if len(s.Cars) > 0 {
		v.car = clampInt(v.car, 0, len(s.Cars)-1)
		bx += a.button(bx, by, "CAR c"+itoa(v.car+1)+" HOME "+floorLabel(s.Cars[v.car].Home), actCarSel, 0) + 2
	}
	a.button(px+pw-10, by, "CLOSE", actClose, 0)
}

func (v *elevatorView) act(a *App, h *hotspot) bool {
	s := v.shaft(a)
	if s == nil {
		return false
	}
	report := func(r Result) {
		if !r.OK && r.Reason != "" {
			a.say(r.Reason, true)
		}
	}
	sc := a.m.Schedule(v.id)
	day := 0
	if v.weekend {
		day = 1
	}
	switch h.act {
	case actServe:
		report(a.m.ToggleServe(v.id, h.arg))
	case actHome:
		if len(s.Cars) > 0 {
			report(a.m.SetCarHome(v.id, v.car, h.arg))
		}
	case actMode:
		a.m.SetPeriodMode(v.id, v.weekend, h.arg, (sc.Modes[day][h.arg]+1)%3)
	case actDayType:
		v.weekend = !v.weekend
	case actResponse:
		a.m.SetResponse(v.id, (sc.Response+h.arg+14)%15+1)
	case actDeparture:
		a.m.SetDeparture(v.id, (sc.Departure+h.arg*5+65)%65)
	case actAddCar:
		home := s.Bottom
		if s.Bottom <= 0 && s.Top >= 0 {
			home = 0
		}
		report(a.m.AddCar(v.id, home))
	case actRemoveCar:
		report(a.m.RemoveCar(v.id))
	case actCarSel:
		v.car = (v.car + 1) % maxInt(len(s.Cars), 1)
	default:
		return false
	}
	return true
}

func (v *elevatorView) key(a *App, ev term.Event) bool {
	switch ev.Kind {
	case term.KeyUp, term.KeyDown:
		if a.focusIdx < v.gridN {
			d := elevPerRow
			if ev.Kind == term.KeyUp {
				d = -d
			}
			n := a.focusIdx + d
			switch {
			case n < 0:
				v.scroll--
			case n >= v.gridN && v.scroll+v.rows < v.nfloors:
				v.scroll++
			case n >= 0 && n < v.gridN:
				a.focusIdx = n
			default:
				a.focusIdx = v.gridN // leave the grid downward
			}
			return true
		}
	case term.KeyLeft, term.KeyRight:
		// dials: arrows change the value when focused
		if h := a.focusedHot(); h != nil && (h.act == actResponse || h.act == actDeparture) {
			d := 1
			if ev.Kind == term.KeyLeft {
				d = -1
			}
			v.act(a, &hotspot{act: h.act, arg: d})
			return true
		}
	case term.KeyPgUp:
		v.scroll -= maxInt(v.rows, 1)
		return true
	case term.KeyPgDn:
		v.scroll += maxInt(v.rows, 1)
		return true
	case term.KeyChar:
		switch ev.Ch {
		case 'w', 'W':
			v.weekend = !v.weekend
			return true
		case '+', '-':
			if h := a.focusedHot(); h != nil && (h.act == actResponse || h.act == actDeparture) {
				d := 1
				if ev.Ch == '-' {
					d = -1
				}
				v.act(a, &hotspot{act: h.act, arg: d})
				return true
			}
		}
	}
	return false
}

func (v *elevatorView) wheel(a *App, dir int, shift bool) { v.scroll -= dir }

// ================= map =================

type mapView struct {
	gx, gy, sx, sy, gw, gh int // grid origin, scale, size in cells
	mt                     int // floor at the top of the first cell row
}

func (v *mapView) draw(a *App) {
	px, py, pw, ph := 1, a.vy0, a.cols-2, a.vrows
	a.box(px, py, pw, ph, "MAP", 15)
	b := a.bounds
	nfl := b.MaxFloor - b.MinFloor + 1
	aw, ah := pw-4, ph-4
	v.sx = maxInt(5, (b.Width+aw-1)/aw)
	v.sy = maxInt(5, (nfl+ah-1)/ah)
	// align buckets so floor 0 starts one: ground never shares a cell row
	v.mt = b.MaxFloor
	for (v.mt+1)%v.sy != 0 {
		v.mt++
	}
	nfl = v.mt - b.MinFloor + 1
	v.gw = (b.Width + v.sx - 1) / v.sx
	v.gh = (nfl + v.sy - 1) / v.sy
	v.gx = px + 2 + (aw-v.gw)/2
	v.gy = py + 1 + (ah-v.gh)/2
	// cell classes: 0 sky/dirt, 1 built, 2 lobby; eval color overrides
	n := v.gw * v.gh
	if cap(a.mapScratch) < n*2 {
		a.mapScratch = make([]uint8, n*2)
	}
	cls := a.mapScratch[:n]
	col := a.mapScratch[n : 2*n]
	for i := range cls {
		cls[i], col[i] = 0, 0
	}
	cell := func(u, f int) int {
		cx, cy := u/v.sx, floorDiv(v.mt-f, v.sy)
		if cx < 0 || cy < 0 || cx >= v.gw || cy >= v.gh {
			return -1
		}
		return cy*v.gw + cx
	}
	for _, sp := range a.m.Floors() {
		for u := sp.Left; u < sp.Right; u += maxInt(1, v.sx/2) {
			if i := cell(u, sp.Floor); i >= 0 && cls[i] == 0 {
				cls[i] = 1
			}
		}
	}
	items := a.m.Items()
	for i := range items {
		it := &items[i]
		if it.Kind == KindLobby {
			for u := it.X; u < it.X+it.W; u += maxInt(1, v.sx/2) {
				if c := cell(u, it.Floor); c >= 0 {
					cls[c] = 2
				}
			}
			continue
		}
		if fg, ok := a.overlayColor(it); ok && it.Kind != KindFloor {
			if c := cell(it.X+it.W/2, it.Floor); c >= 0 {
				// keep the worst (red beats yellow beats blue)
				if col[c] == 0 || rank(fg) > rank(col[c]) {
					col[c] = fg
				}
			}
		}
	}
	for cy := 0; cy < v.gh; cy++ {
		fTop := v.mt - cy*v.sy
		for cx := 0; cx < v.gw; cx++ {
			x, y := v.gx+cx, v.gy+cy
			c := screen.Cell{Ch: ' ', Fg: 7, Bg: 1}
			if fTop-v.sy+1 <= 0 && fTop >= 0 {
				c = screen.Cell{Ch: gDark, Fg: 2, Bg: 1}
			} else if fTop < 0 {
				c = screen.Cell{Ch: gLight, Fg: 6, Bg: 0}
			}
			switch cls[cy*v.gw+cx] {
			case 1:
				c = screen.Cell{Ch: gDark, Fg: 7, Bg: 0}
			case 2:
				c = screen.Cell{Ch: gDark, Fg: 6, Bg: 0}
			}
			if k := col[cy*v.gw+cx]; k != 0 {
				c = screen.Cell{Ch: gFull, Fg: k, Bg: 0}
			}
			a.set(x, y, c)
		}
	}
	for _, s := range a.m.Shafts() {
		var fg uint8 = 7
		ch := gWallS
		switch s.Kind {
		case KindExpress:
			fg, ch = 9, gWallD
		case KindService:
			fg = 12
		}
		for f := s.Bottom; f <= s.Top; f++ {
			if i := cell(s.X, f); i >= 0 {
				a.set(v.gx+i%v.gw, v.gy+i/v.gw, screen.Cell{Ch: ch, Fg: fg})
			}
		}
	}
	// viewport rectangle
	x0, x1 := a.left/v.sx, (a.left+a.units-1)/v.sx
	y0, y1 := floorDiv(v.mt-a.topFloor, v.sy), floorDiv(v.mt-(a.topFloor-a.nfloors+1), v.sy)
	for x := x0; x <= x1; x++ {
		a.recolor(v.gx+x, v.gy+y0, 14)
		a.recolor(v.gx+x, v.gy+y1, 14)
	}
	for y := y0; y <= y1; y++ {
		a.recolor(v.gx+x0, v.gy+y, 14)
		a.recolor(v.gx+x1, v.gy+y, 14)
	}
	a.set(v.gx+x0, v.gy+y0, screen.Cell{Ch: gCornerTL, Fg: 14})
	a.set(v.gx+x1, v.gy+y0, screen.Cell{Ch: gCornerTR, Fg: 14})
	a.set(v.gx+x0, v.gy+y1, screen.Cell{Ch: gCornerBL, Fg: 14})
	a.set(v.gx+x1, v.gy+y1, screen.Cell{Ch: gCornerBR, Fg: 14})
	a.hot(v.gx, v.gy, v.gw, v.gh, actMapArea, 0)
	a.put(px+2+len("MAP")+3, py, " 1 cell = "+itoa(v.sx)+" units x "+itoa(v.sy)+" floors ", 8, 0)
	bx := px + 2
	by := py + ph - 2
	for i, l := range [4]string{"EDIT", "EVAL", "PRICING", "HOTEL"} {
		if i == a.evalMode {
			l = ">" + l + "<"
		}
		bx += a.button(bx, by, l, actMapMode, i) + 1
	}
	a.put(bx+1, by, clip("Click to jump, arrows move the view", px+pw-12-bx-2), 8, 0)
	a.button(px+pw-10, by, "CLOSE", actClose, 0)
}

// recolor makes a map cell part of the viewport rectangle without hiding it.
func (a *App) recolor(x, y int, fg uint8) {
	c := a.get(x, y)
	c.Fg = fg
	if c.Ch == ' ' || c.Ch == gLight {
		c.Ch = gHLine
	}
	a.set(x, y, c)
}

func rank(fg uint8) int {
	switch fg {
	case 12:
		return 3
	case 14:
		return 2
	case 9:
		return 1
	}
	return 0
}

func (v *mapView) click(a *App, h *hotspot, x, y int) {
	u := (x-v.gx)*v.sx + v.sx/2
	f := v.mt - (y-v.gy)*v.sy - v.sy/2
	a.centerOn(u, f)
	a.curX, a.curF = clampInt(u, 0, a.bounds.Width-1), f
	a.closeTop()
}

func (v *mapView) act(a *App, h *hotspot) bool {
	if h.act == actMapMode {
		a.evalMode = h.arg
		return true
	}
	return false
}

func (v *mapView) key(a *App, ev term.Event) bool {
	switch ev.Kind {
	case term.KeyUp:
		a.scrollFloors(maxInt(v.sy, 1))
	case term.KeyDown:
		a.scrollFloors(-maxInt(v.sy, 1))
	case term.KeyLeft:
		a.scrollUnits(-maxInt(v.sx, 1))
	case term.KeyRight:
		a.scrollUnits(maxInt(v.sx, 1))
	case term.KeyChar:
		if ev.Ch == 'm' || ev.Ch == 'M' {
			a.closeTop()
			return true
		}
		return false
	default:
		return false
	}
	return true
}

func (v *mapView) wheel(a *App, dir int, shift bool) {
	if shift {
		a.scrollUnits(-dir * v.sx)
	} else {
		a.scrollFloors(dir * v.sy)
	}
}

// ================= finance =================

type financeView struct{}

func (v *financeView) draw(a *App) {
	fin := a.m.Finance()
	px, py, pw, ph := a.popupRect(minInt(74, a.cols-2), 20)
	a.box(px, py, pw, ph, "FINANCE  "+fin.Quarter, 15)
	cw := (pw - 4) / 2
	lx, rx := px+2, px+2+cw+1
	a.put(lx, py+1, "INCOME", 11, 0)
	a.put(rx, py+1, "MAINTENANCE", 11, 0)
	line := func(x, y int, l string, n int64, fg uint8) {
		a.put(x, y, padRight(clip(l, cw-14), cw-14), fg, 0)
		a.put(x+cw-14, y, padLeft(money(n), 13), fg, 0)
	}
	var ti, tm int64
	for i := 0; i < 10; i++ {
		if i < len(fin.Income) {
			line(lx, py+2+i, fin.Income[i].Label, fin.Income[i].Amount, 7)
			ti += fin.Income[i].Amount
		}
		if i < len(fin.Maintenance) {
			line(rx, py+2+i, fin.Maintenance[i].Label, fin.Maintenance[i].Amount, 7)
			tm += fin.Maintenance[i].Amount
		}
	}
	for x := lx; x < px+pw-2; x++ {
		a.set(x, py+12, screen.Cell{Ch: gHLine, Fg: 8})
	}
	line(lx, py+13, "Total income", ti, 15)
	line(rx, py+13, "Total maintenance", tm, 15)
	net := ti - tm
	line(lx, py+14, "Net revenues", net, 15)
	line(rx, py+14, "Other income", fin.OtherIncome, 7)
	line(lx, py+15, "Construction costs", fin.Construction, 7)
	qb := net + fin.OtherIncome - fin.Construction
	var qfg uint8 = 10
	if qb < 0 {
		qfg = 12
	}
	line(rx, py+15, "Quarter's balance", qb, qfg)
	line(lx, py+16, "Previous quarter", fin.PrevBalance, 7)
	line(rx, py+16, "Total balance", fin.Balance, 14)
	a.button(px+pw-10, py+ph-2, "CLOSE", actClose, 0)
}

func (v *financeView) key(a *App, ev term.Event) bool {
	if ev.Kind == term.KeyChar && (ev.Ch == 'f' || ev.Ch == 'F') {
		a.closeTop()
		return true
	}
	return false
}

// ================= rating =================

type ratingView struct{}

func (v *ratingView) draw(a *App) {
	r := a.m.Rating()
	px, py, pw, ph := a.popupRect(56, 8+len(r.Needs))
	a.box(px, py, pw, ph, "TOWER RATING", 15)
	st := "TOWER"
	if r.Stars < 6 {
		st = strings.Repeat("*", r.Stars) + strings.Repeat(".", 5-r.Stars)
	}
	a.put(px+2, py+1, "Rating", 11, 0)
	a.put(px+12, py+1, st, 14, 0)
	a.put(px+2, py+2, "Population", 11, 0)
	a.put(px+14, py+2, commas(r.Population), 15, 0)
	a.put(px+2, py+3, clip(r.Next, pw-4), 7, 0)
	for i, n := range r.Needs {
		a.put(px+2, py+5+i, clip("- "+n, pw-4), 14, 0)
	}
	a.button(px+pw-10, py+ph-2, "CLOSE", actClose, 0)
}

func (v *ratingView) key(a *App, ev term.Event) bool { return false }

// ================= event prompt =================

type eventView struct{}

func (v *eventView) modal() {}

func (v *eventView) draw(a *App) {
	ev := a.m.Event()
	if ev == nil {
		a.closeTop()
		return
	}
	pw := 58
	for _, l := range ev.Lines {
		pw = maxInt(pw, len(l)+4)
	}
	px, py, pw, ph := a.popupRect(minInt(pw, a.cols-2), 5+len(ev.Lines))
	var fg uint8 = 15
	switch ev.Kind {
	case EventFire, EventBomb:
		fg = 12
	case EventVIP, EventTreasure, EventWedding:
		fg = 14
	}
	a.box(px, py, pw, ph, ev.Title, fg)
	for i, l := range ev.Lines {
		a.put(px+2, py+2+i, clip(l, pw-4), 15, 0)
	}
	bx := px + 2
	for i, c := range ev.Choices {
		bx += a.button(bx, py+ph-2, c, actEventPick, i) + 2
	}
}

func (v *eventView) act(a *App, h *hotspot) bool {
	if h.act == actEventPick {
		a.m.Respond(h.arg)
		a.closeTop()
		return true
	}
	return false
}

func (v *eventView) key(a *App, ev term.Event) bool {
	// prompts must be answered: Esc does not dismiss them
	return ev.Kind == term.KeyEsc || ev.Kind == term.KeyBackspace
}

// ================= message log =================

type logView struct{ scroll int }

func (v *logView) draw(a *App) {
	msgs := a.m.Messages()
	px, py, pw, ph := a.popupRect(minInt(76, a.cols-2), a.vrows)
	a.box(px, py, pw, ph, "MESSAGES", 15)
	rows := ph - 4
	v.scroll = clampInt(v.scroll, 0, maxInt(len(msgs)-rows, 0))
	for i := 0; i < rows; i++ {
		k := len(msgs) - 1 - v.scroll - i
		if k < 0 {
			break
		}
		m := msgs[k]
		var fg uint8 = 14
		if m.Alert {
			fg = 12
		}
		a.put(px+2, py+1+i, padRight(clip(m.When, 18), 19), 8, 0)
		a.put(px+21, py+1+i, clip(m.Text, pw-23), fg, 0)
	}
	if len(msgs) == 0 {
		a.put(px+2, py+1, "No messages yet.", 8, 0)
	}
	bx := px + 2
	bx += a.button(bx, py+ph-2, "NEWER", actLogScroll, -1) + 1
	a.button(bx, py+ph-2, "OLDER", actLogScroll, 1)
	a.button(px+pw-10, py+ph-2, "CLOSE", actClose, 0)
}

func (v *logView) act(a *App, h *hotspot) bool {
	if h.act == actLogScroll {
		v.scroll += h.arg * 5
		return true
	}
	return false
}

func (v *logView) key(a *App, ev term.Event) bool {
	switch ev.Kind {
	case term.KeyUp:
		v.scroll--
	case term.KeyDown:
		v.scroll++
	case term.KeyPgUp:
		v.scroll -= 10
	case term.KeyPgDn:
		v.scroll += 10
	default:
		return false
	}
	return true
}

func (v *logView) wheel(a *App, dir int, shift bool) { v.scroll -= dir }
