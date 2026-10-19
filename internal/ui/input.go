package ui

import (
	"simtower/internal/term"
)

// handle routes one input event.
func (a *App) handle(ev term.Event) {
	switch {
	case ev.Resize:
		if ev.Cols < 10 || ev.Rows < 5 {
			return // not a plausible size report
		}
		if ev.Cols != a.cols || ev.Rows != a.rows {
			a.layout(ev.Cols, ev.Rows)
			a.scr.Invalidate()
			a.render()
		}
		return
	case ev.Mouse:
		a.mouse(ev)
		return
	}
	if a.tooSmall {
		if ev.Kind == term.KeyChar && (ev.Ch == 'q' || ev.Ch == 'Q') {
			a.quit = true
		}
		return
	}
	if v := a.top(); v != nil {
		if v.key(a, ev) {
			return
		}
		a.popupKey(ev)
		return
	}
	a.key(ev)
}

// popupKey: generic focus handling inside the top popup.
func (a *App) popupKey(ev term.Event) {
	switch ev.Kind {
	case term.KeyTab:
		if ev.Shift {
			a.focusIdx--
		} else {
			a.focusIdx++
		}
	case term.KeyRight, term.KeyDown:
		a.focusIdx++
	case term.KeyLeft, term.KeyUp:
		a.focusIdx--
	case term.KeyEnter:
		a.activateFocus()
	case term.KeyEsc, term.KeyBackspace:
		if !a.topModal() {
			a.closeTop()
		}
	case term.KeyChar:
		switch ev.Ch {
		case ' ':
			a.activateFocus()
		case 'q', 'Q':
			a.quit = true
		case 'b', 'B', 'i', 'I', 'v', 'V', 'm', 'M', 'f', 'F', 'z', 'Z', 'l', 'L':
			// main hotkeys still work: close the popup and act (not over a
			// prompt that must be answered)
			if a.topModal() {
				return
			}
			a.closeAll()
			a.key(ev)
		}
	}
}

func (a *App) activateFocus() {
	n := 0
	for i := a.popupHot0; i < len(a.hots); i++ {
		h := a.hots[i]
		if !h.focusable {
			continue
		}
		if n == a.focusIdx {
			a.do(&h, h.x, h.y, false)
			return
		}
		n++
	}
}

// focusedHot returns the focused hotspot of the top popup.
func (a *App) focusedHot() *hotspot {
	n := 0
	for i := a.popupHot0; i < len(a.hots); i++ {
		if !a.hots[i].focusable {
			continue
		}
		if n == a.focusIdx {
			return &a.hots[i]
		}
		n++
	}
	return nil
}

// key handles keys with no popup open.
func (a *App) key(ev term.Event) {
	if ev.Kind == term.KeyChar {
		switch ev.Ch {
		case 'b', 'B':
			a.do(&hotspot{act: actBuild}, 0, 0, false)
		case 'i', 'I':
			a.do(&hotspot{act: actInspect}, 0, 0, false)
		case 'v', 'V':
			a.do(&hotspot{act: actEval}, 0, 0, false)
		case 'm', 'M':
			a.do(&hotspot{act: actMap}, 0, 0, false)
		case 'f', 'F':
			a.do(&hotspot{act: actFinance}, 0, 0, false)
		case 'z', 'Z':
			a.do(&hotspot{act: actZoom}, 0, 0, false)
		case ' ':
			if a.focus == zoneView {
				a.do(&hotspot{act: actPause}, 0, 0, false)
			} else {
				a.activateZone()
			}
		case 'q', 'Q':
			a.quit = true
		case 'l', 'L':
			a.do(&hotspot{act: actMsgLog}, 0, 0, false)
		}
		return
	}
	switch ev.Kind {
	case term.KeyTab:
		d := 1
		if ev.Shift {
			d = 2
		}
		a.focus = zone((int(a.focus) + d) % 3)
		a.zoneIdx = 0
		return
	}
	if a.focus != zoneView {
		spots := a.statusSpots
		if a.focus == zoneHotbar {
			spots = a.hotbarSpots
		}
		switch ev.Kind {
		case term.KeyLeft:
			a.zoneIdx = (a.zoneIdx + len(spots) - 1) % maxInt(len(spots), 1)
		case term.KeyRight:
			a.zoneIdx = (a.zoneIdx + 1) % maxInt(len(spots), 1)
		case term.KeyEnter:
			a.activateZone()
		case term.KeyEsc, term.KeyBackspace, term.KeyUp, term.KeyDown:
			a.focus = zoneView
		}
		return
	}
	if a.zoom {
		switch ev.Kind {
		case term.KeyUp:
			a.scrollFloors(1)
		case term.KeyDown:
			a.scrollFloors(-1)
		case term.KeyLeft:
			a.scrollUnits(-8)
		case term.KeyRight:
			a.scrollUnits(8)
		case term.KeyPgUp:
			a.scrollFloors(10)
		case term.KeyPgDn:
			a.scrollFloors(-10)
		case term.KeyHome:
			a.centerOn(a.curX, 0)
		case term.KeyEnter, term.KeyEsc, term.KeyBackspace:
			a.zoom = false
		}
		return
	}
	// viewport keyboard cursor
	switch ev.Kind {
	case term.KeyUp, term.KeyDown, term.KeyLeft, term.KeyRight:
		a.kbd = true
		if ev.Shift && a.tool != KindNone && !a.dragging {
			a.beginDrag(a.curX, a.curF, true)
		}
		switch ev.Kind {
		case term.KeyUp:
			a.moveCursor(0, 1)
		case term.KeyDown:
			a.moveCursor(0, -1)
		case term.KeyLeft:
			a.moveCursor(-1, 0)
		case term.KeyRight:
			a.moveCursor(1, 0)
		}
		if a.dragging && a.dragKbd && !ev.Shift {
			a.dragging = false // plain arrow cancels a keyboard drag
		}
	case term.KeyPgUp:
		a.kbd = true
		a.moveCursor(0, 10)
	case term.KeyPgDn:
		a.kbd = true
		a.moveCursor(0, -10)
	case term.KeyHome:
		a.kbd = true
		a.curF = 0
		a.centerOn(a.curX, 0)
		a.syncHover()
	case term.KeyEnter:
		a.kbd = true
		a.leftPress(a.curX, a.curF)
		a.leftRelease()
	case term.KeyEsc, term.KeyBackspace:
		a.rightClick()
	}
}

func (a *App) activateZone() {
	spots := a.statusSpots
	if a.focus == zoneHotbar {
		spots = a.hotbarSpots
	}
	if a.zoneIdx >= 0 && a.zoneIdx < len(spots) {
		h := spots[a.zoneIdx]
		a.do(&h, h.x, h.y, false)
	}
}

// moveCursor moves the keyboard cursor and scrolls at the viewport edges.
func (a *App) moveCursor(dx, df int) {
	b := a.bounds
	a.curX = clampInt(a.curX+dx, 0, b.Width-1)
	a.curF = clampInt(a.curF+df, b.MinFloor, b.MaxFloor)
	if a.curX < a.left {
		a.left = a.curX
	}
	if a.curX >= a.left+a.units {
		a.left = a.curX - a.units + 1
	}
	if a.curF > a.topFloor {
		a.topFloor = a.curF
	}
	if a.curF < a.topFloor-a.nfloors+1 {
		a.topFloor = a.curF + a.nfloors - 1
	}
	a.clampScroll()
	a.syncHover()
}

func (a *App) syncHover() {
	if a.kbd {
		a.dx1, a.df1, a.hoverOK = a.curX, a.curF, true
	}
}

func (a *App) scrollFloors(n int) {
	a.topFloor += n
	a.clampScroll()
}

func (a *App) scrollUnits(n int) {
	a.left += n
	a.clampScroll()
}

// ---- mouse ----

func (a *App) mouse(ev term.Event) {
	x, y := ev.MX-1, ev.MY-1
	if a.tooSmall {
		return
	}
	switch ev.MKind {
	case term.MouseWheelUp, term.MouseWheelDown:
		dir := 1
		if ev.MKind == term.MouseWheelDown {
			dir = -1
		}
		if v := a.top(); v != nil {
			if w, ok := v.(wheeler); ok {
				w.wheel(a, dir, ev.Shift)
			}
			return
		}
		if ev.Shift {
			a.scrollUnits(-dir * 8)
		} else {
			a.scrollFloors(dir)
		}
		a.mouseHover(x, y)
		return
	case term.MousePress:
		a.kbd = false
		switch ev.Button {
		case 0:
			a.leftClickAt(x, y)
		case 2:
			a.panning, a.panMoved, a.panX, a.panY = true, false, x, y
		}
	case term.MouseMotion:
		switch {
		case ev.Button == 2 && a.panning:
			dx, dy := x-a.panX, y-a.panY
			if dx != 0 || dy/3 != 0 {
				a.panMoved = true
				a.left -= dx
				a.panX = x
				if dy/3 != 0 {
					a.topFloor += dy / 3
					a.panY += (dy / 3) * 3
				}
				a.clampScroll()
			}
		case ev.Button == 0 && a.dragging:
			if u, f, ok := a.cellAt(x, y); ok {
				a.dx1, a.df1 = u, f
			}
		default:
			a.mouseHover(x, y)
		}
	case term.MouseRelease:
		if a.panning && (ev.Button == 2 || ev.Button == 3) {
			a.panning = false
			if !a.panMoved {
				a.rightClick()
			}
			return
		}
		if a.dragging && !a.dragKbd {
			if u, f, ok := a.cellAt(x, y); ok {
				a.dx1, a.df1 = u, f
			}
			a.leftRelease()
		}
	}
}

func (a *App) mouseHover(x, y int) {
	if a.dragging {
		return
	}
	if len(a.views) > 0 || a.zoom {
		a.hoverOK = false
		return
	}
	u, f, ok := a.cellAt(x, y)
	a.dx1, a.df1, a.hoverOK = u, f, ok
}

func (a *App) leftClickAt(x, y int) {
	// hotspots: only the top popup's when one is open
	start, end := 0, len(a.hots)
	if len(a.views) > 0 {
		start = a.popupHot0
	}
	for i := end - 1; i >= start; i-- {
		h := a.hots[i]
		if h.hit(x, y) {
			if h.focusable {
				// clicking moves the focus ring there
				n := 0
				for j := a.popupHot0; j < i; j++ {
					if a.hots[j].focusable {
						n++
					}
				}
				if len(a.views) > 0 {
					a.focusIdx = n
				}
			}
			a.do(&h, x, y, true)
			return
		}
	}
	if len(a.views) > 0 && (y == 0 || y >= a.rows-2) && !a.topModal() {
		// status row and footer stay live under a popup
		for i := a.popupHot0 - 1; i >= 0; i-- {
			if h := a.hots[i]; h.hit(x, y) {
				a.do(&h, x, y, true)
				return
			}
		}
	}
	if len(a.views) > 0 || a.zoom {
		return
	}
	if u, f, ok := a.cellAt(x, y); ok {
		a.focus = zoneView
		a.curX, a.curF = u, f
		a.leftPress(u, f)
	}
}

// leftPress = left click on the tower at (u, f): place with a tool armed,
// otherwise inspect.
func (a *App) leftPress(u, f int) {
	if a.tool == KindNone {
		a.inspectAt(u, f)
		return
	}
	if a.dragging && a.dragKbd {
		return // Enter finishes a keyboard drag in leftRelease
	}
	a.beginDrag(u, f, false)
}

func (a *App) beginDrag(u, f int, kbd bool) {
	a.dragging, a.dragKbd = true, kbd
	a.dx0, a.df0, a.dx1, a.df1 = u, f, u, f
	a.hoverOK = true
}

func (a *App) leftRelease() {
	if !a.dragging {
		return
	}
	x, f, w, h := a.footprint() // while still dragging: the drag's extent
	a.dragging = false
	a.dragKbd = false
	if a.tool == KindBulldozer {
		var res Result
		for u := x; u < x+w; u++ {
			if r := a.m.Remove(u, f); r.OK {
				res.OK = true
				res.Cost += r.Cost
			} else if !res.OK && r.Reason != "" {
				res.Reason = r.Reason
			}
		}
		if !res.OK && res.Reason != "" {
			a.say(res.Reason, true)
		}
		a.ghost.valid = false
		return
	}
	if a.tool.IsShaft() {
		// The elevator tool on an existing shaft adds a car at the clicked
		// floor, which becomes that car's home floor (as in the original).
		if s, ok := a.shaftAt(a.dx0, a.df0); ok {
			if s.Kind != a.tool {
				a.say("That is a "+kindName(s.Kind)+" shaft.", true)
			} else if res := a.m.AddCar(s.ID, a.df0); !res.OK {
				a.say(res.Reason, true)
			} else {
				a.say("Car added: "+money(res.Cost), false)
			}
			a.ghost.valid = false
			return
		}
	}
	res := a.m.Place(a.tool, x, f, w, h)
	if !res.OK {
		a.say(res.Reason, true)
	} else {
		a.say(a.toolDef.Name+" built: "+money(res.Cost), false)
	}
	a.ghost.valid = false
}

// shaftAt returns the shaft covering unit u on floor f.
func (a *App) shaftAt(u, f int) (Shaft, bool) {
	for _, s := range a.m.Shafts() {
		if u >= s.X && u < s.X+s.Kind.ShaftWidth() && f >= s.Bottom && f <= s.Top {
			return s, true
		}
	}
	return Shaft{}, false
}

func (a *App) rightClick() {
	if a.dragging {
		a.dragging = false
		return
	}
	if a.tool != KindNone {
		a.disarm()
		return
	}
	if len(a.views) > 0 && !a.topModal() {
		a.closeTop()
	}
}

func (a *App) arm(d BuildDef) {
	a.tool, a.toolDef = d.Kind, d
	a.ghost.valid = false
	a.hoverOK = a.kbd
	a.syncHover()
	if !a.anyMotion && a.cfg.Term != nil {
		a.cfg.Term.Write([]byte("\x1b[?1003h")) // hover reports for the ghost
		a.anyMotion = true
	}
}

func (a *App) disarm() {
	a.tool = KindNone
	a.dragging = false
	a.ghost.valid = false
	if a.anyMotion && a.cfg.Term != nil {
		a.cfg.Term.Write([]byte("\x1b[?1003l"))
		a.anyMotion = false
	}
	a.alertUntil = 0
}

// inspectAt opens the window for whatever is at (u, f): person, shaft, room.
func (a *App) inspectAt(u, f int) {
	for _, p := range a.m.People() {
		if p.X == u && p.Floor == f {
			a.open(&tenantView{id: p.ID})
			return
		}
	}
	for _, s := range a.m.Shafts() {
		if u >= s.X && u < s.X+s.Kind.ShaftWidth() && f >= s.Bottom && f <= s.Top {
			a.open(&elevatorView{id: s.ID})
			return
		}
	}
	items := a.m.Items()
	for i := range items {
		it := &items[i]
		if it.Kind == KindFloor {
			continue
		}
		if u >= it.X && u < it.X+it.W && f >= it.Floor && f < it.Floor+maxInt(it.H, 1) {
			a.open(&facilityView{id: it.ID})
			return
		}
	}
	a.say("Nothing to inspect at "+floorLabel(f)+" unit "+itoa(u)+".", false)
}

// do performs a hotspot's action. click is true for mouse clicks (x, y are
// the click position), false for keyboard activation.
func (a *App) do(h *hotspot, x, y int, click bool) {
	if v := a.top(); v != nil {
		if c, ok := v.(clicker); ok && h.act == actMapArea {
			c.click(a, h, x, y)
			return
		}
		if pa, ok := v.(interface{ act(a *App, h *hotspot) bool }); ok && pa.act(a, h) {
			return
		}
	}
	switch h.act {
	case actFinance:
		a.closeAll()
		a.open(&financeView{})
	case actRating:
		a.closeAll()
		a.open(&ratingView{})
	case actSpeed:
		a.m.SetSpeed((a.m.Speed() + 1) % 3)
	case actPause:
		if a.m.Speed() == 0 {
			a.m.SetSpeed(1)
		} else {
			a.m.SetSpeed(0)
		}
	case actBuild:
		a.closeAll()
		a.zoom = false
		a.open(newBuildView(a))
	case actMap:
		a.closeAll()
		a.zoom = false
		a.open(&mapView{})
	case actInspect:
		a.closeAll()
		if a.tool != KindNone {
			a.disarm()
		}
		if a.kbd {
			a.inspectAt(a.curX, a.curF)
		} else {
			a.say("Inspect: click a room, an elevator shaft or a person.", false)
		}
	case actEval:
		a.evalMode = (a.evalMode + 1) % 4
	case actZoom:
		if click && h.arg == 1 && a.zoom {
			// click in the zoom view: jump there and return
			f := a.zoomTop() - (y - a.vy0)
			u := a.left + x - gutter
			a.zoom = false
			a.curX, a.curF = u, f
			a.topFloor = f + a.nfloors/2
			a.clampScroll()
			return
		}
		a.closeAll()
		a.zoom = !a.zoom
	case actScrollHelp:
		a.say("Arrows move the cursor, PgUp/PgDn 10 floors, Home = lobby. Wheel scrolls floors, Shift+wheel units, right-drag pans.", false)
	case actQuit:
		a.quit = true
	case actMsgLog:
		a.closeAll()
		a.open(&logView{})
	case actClose:
		a.closeTop()
	}
}

func (a *App) closeAll() { a.views = a.views[:0]; a.focusIdx = 0 }
