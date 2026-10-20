// Package ui is the door's presentation layer: the tower viewport, chrome,
// popups and input handling, on top of the Model interface (model.go). See
// docs/graphics.md (spec), docs/graphics_plan.html (reference mock) and
// docs/ui_notes.md (how this package is put together).
package ui

import (
	"log"
	"time"

	"simtower/internal/screen"
	"simtower/internal/term"
)

// Config is everything main hands the app.
type Config struct {
	Term     term.Term
	Events   <-chan term.Event
	Cols     int
	Rows     int
	Player   string
	SavePath string
	Logger   *log.Logger
	Trace    bool // log every input event (Logger must be set)
	Version  string
	Model    Model // nil -> DemoModel
	// Launcher, when set, opens the start menu over the splash first; the
	// chosen model replaces Model (which is only the backdrop until then).
	Launcher *Launcher
	// ProbeEvery re-sends the parked-cursor size probe so a resized client
	// window is noticed (the CPR reply arrives as a Resize event). 0 = off.
	ProbeEvery time.Duration
}

const (
	frameRate = 8 // render ticks per second
	gutter    = 4 // floor label columns
	minCols   = 80
	minRows   = 24
)

type zone uint8

const (
	zoneView zone = iota
	zoneStatus
	zoneHotbar
)

// action is what a hotspot does when clicked or activated from the keyboard.
type action uint8

const (
	actNone action = iota
	actFinance
	actRating
	actSpeed
	actBuild
	actMap
	actInspect
	actEval
	actZoom
	actPause
	actScrollHelp
	actQuit
	actMsgLog
	actClose
	actPalette   // arg = catalog index
	actPalettePg // arg = +1 / -1
	actRent      // arg = tier
	actMovie     // arg = 1 classic, 0 new
	actTenant    // arg = person ID
	actRename    //
	actServe     // arg = floor
	actHome      // arg = floor
	actMode      // arg = period
	actDayType   //
	actResponse  // arg = +1/-1
	actDeparture // arg = +1/-1
	actAddCar    //
	actRemoveCar //
	actCarSel    //
	actEventPick // arg = choice
	actMapArea   // map grid (uses click coords)
	actMapMode   // arg = eval mode
	actLogScroll // arg = +1/-1
	actTooSmall  //
	actStart     // start menu, arg = choice
)

type hotspot struct {
	x, y, w, h int
	act        action
	arg        int
	focusable  bool
}

func (h *hotspot) hit(x, y int) bool {
	return x >= h.x && x < h.x+h.w && y >= h.y && y < h.y+h.h
}

type rect struct{ x, y, w, h int }

func (r rect) has(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

// App is the running door. New builds it; Run blocks until the player quits
// or the connection drops.
type App struct {
	cfg Config
	m   Model
	scr *screen.Screen

	cols, rows int
	tooSmall   bool
	vy0, vrows int // viewport rows [vy0, vy0+vrows)
	nfloors    int // whole floors visible
	units      int // visible units
	topFloor   int // floor whose top row is vy0
	left       int // unit at screen column `gutter`
	bounds     Bounds

	frame  int // render tick counter
	tframe int // animation clock (the mock's t: 4 per second, frozen while paused)
	sub    int

	clip rect

	// keyboard cursor (unit, floor)
	curX, curF int
	kbd        bool // cursor visible (keyboard in use)
	focus      zone
	zoneIdx    int

	// build tool
	tool     Kind // KindNone = inspect
	toolDef  BuildDef
	dragging bool
	dragKbd  bool
	dx0, df0 int // drag start (unit, floor)
	dx1, df1 int // drag end / hover
	hoverOK  bool
	ghost    struct {
		k          Kind
		x, f, w, h int
		car        int // >0: the elevator tool is over shaft ID `car`; click adds a car at f
		res        Result
		valid      bool
	}
	anyMotion bool // 1003 mode on (hover ghost)

	// drag pan: right button anywhere, or left button with no tool armed
	// (a left press that never moves inspects on release)
	panning     bool
	panBtn      int
	panMoved    bool
	panX, panY  int
	panU, panF  int // cell under the left press, inspected if it stays a click
	evalMode    int // 0 off, 1 eval, 2 pricing, 3 hotel
	zoom        bool
	alert       string
	alertBad    bool
	alertUntil  int
	views       []view
	focusIdx    int // focus within the top popup
	nfocus      int // focusables registered this frame
	hots        []hotspot
	popupHot0   int // first hotspot index belonging to the top popup
	statusSpots []hotspot
	hotbarSpots []hotspot
	lastEvent   *Event
	quit        bool
	lastProbe   time.Time
	mapScratch  []uint8
	passive     bool // drawing an inactive (covered) popup
}

func New(cfg Config) *App {
	if cfg.Cols < 1 {
		cfg.Cols = 80
	}
	if cfg.Rows < 1 {
		cfg.Rows = 24
	}
	m := cfg.Model
	if m == nil {
		m = NewDemo()
	}
	a := &App{cfg: cfg, m: m, scr: screen.New(cfg.Cols, cfg.Rows), hots: make([]hotspot, 0, 256)}
	a.bounds = m.Bounds()
	a.layout(cfg.Cols, cfg.Rows)
	a.topFloor = a.nfloors - 2 // 1F second from the bottom, B1 visible (mock default)
	a.clampScroll()
	a.curX, a.curF = a.units/2, 0
	if cfg.Launcher != nil {
		a.open(&startView{})
	}
	return a
}

// layout recomputes the geometry for a terminal size.
func (a *App) layout(cols, rows int) {
	a.cols, a.rows = cols, rows
	a.tooSmall = cols < minCols || rows < minRows
	if a.scr.W != cols || a.scr.H != rows {
		a.scr.Resize(cols, rows)
	}
	a.vy0 = 1
	a.vrows = rows - 3
	if a.vrows < 3 {
		a.vrows = 3
	}
	a.nfloors = a.vrows / 3
	a.units = cols - gutter
	a.clampScroll()
}

func (a *App) clampScroll() {
	b := a.bounds
	a.topFloor = clampInt(a.topFloor, b.MinFloor+a.nfloors-1, b.MaxFloor)
	a.left = clampInt(a.left, 0, b.Width-a.units)
	a.curX = clampInt(a.curX, 0, b.Width-1)
	a.curF = clampInt(a.curF, b.MinFloor, b.MaxFloor)
}

// fy is the screen row of the top row of floor f.
func (a *App) fy(f int) int { return a.vy0 + (a.topFloor-f)*3 }

// sx is the screen column of unit u.
func (a *App) sx(u int) int { return gutter + u - a.left }

// cellAt maps a screen cell inside the viewport to (unit, floor).
func (a *App) cellAt(x, y int) (u, f int, ok bool) {
	if x < gutter || y < a.vy0 || y >= a.vy0+a.vrows || x >= a.cols {
		return 0, 0, false
	}
	u = a.left + x - gutter
	f = a.topFloor - (y-a.vy0)/3
	return u, f, u >= 0 && u < a.bounds.Width && f >= a.bounds.MinFloor && f <= a.bounds.MaxFloor
}

func (a *App) viewRect() rect { return rect{gutter, a.vy0, a.cols - gutter, a.vrows} }

// ---- drawing primitives (clipped to a.clip) ----

func (a *App) set(x, y int, c screen.Cell) {
	if !a.clip.has(x, y) {
		return
	}
	a.scr.Set(x, y, c)
}

func (a *App) get(x, y int) screen.Cell { return a.scr.Get(x, y) }

// put writes CP437 bytes.
func (a *App) put(x, y int, s string, fg, bg uint8) {
	for i := 0; i < len(s); i++ {
		a.set(x+i, y, screen.Cell{Ch: s[i], Fg: fg, Bg: bg})
	}
}

func (a *App) fill(x, y, w, h int, ch byte, fg, bg uint8) {
	c := screen.Cell{Ch: ch, Fg: fg, Bg: bg}
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			a.set(xx, yy, c)
		}
	}
}

func (a *App) fullClip() { a.clip = rect{0, 0, a.cols, a.rows} }

// hot registers a click target.
func (a *App) hot(x, y, w, h int, act action, arg int) {
	if a.passive {
		return
	}
	a.hots = append(a.hots, hotspot{x: x, y: y, w: w, h: h, act: act, arg: arg})
}

// button draws "[LABEL]" black on light gray (blue when focused) and
// registers it as a focusable click target. Returns its width.
func (a *App) button(x, y int, label string, act action, arg int) int {
	s := "[" + label + "]"
	fg, bg := uint8(0), uint8(7)
	if a.passive {
		a.put(x, y, s, fg, bg)
		return len(s)
	}
	if a.nfocus == a.focusIdx {
		fg, bg = 15, 1
	}
	a.put(x, y, s, fg, bg)
	a.hots = append(a.hots, hotspot{x: x, y: y, w: len(s), h: 1, act: act, arg: arg, focusable: true})
	a.nfocus++
	return len(s)
}

// focusable registers a non-button control (grid cell, dial) in the focus
// order; it reports whether it currently has focus so the caller can draw
// the blue focus ring.
func (a *App) focusable(x, y, w int, act action, arg int) bool {
	if a.passive {
		return false
	}
	f := a.nfocus == a.focusIdx
	a.hots = append(a.hots, hotspot{x: x, y: y, w: w, h: 1, act: act, arg: arg, focusable: true})
	a.nfocus++
	return f
}

// ring recolors a span as focused (bg blue, bright white).
func (a *App) ring(x, y, w int) {
	for i := 0; i < w; i++ {
		c := a.get(x+i, y)
		c.Bg, c.Fg = 1, 15
		a.set(x+i, y, c)
	}
}

// box draws a double-line popup frame with a title tab (mock box()).
func (a *App) box(px, py, pw, ph int, title string, fg uint8) {
	a.fill(px, py, pw, ph, ' ', 7, 0)
	for x := px; x < px+pw; x++ {
		a.set(x, py, screen.Cell{Ch: gHLineD, Fg: fg})
		a.set(x, py+ph-1, screen.Cell{Ch: gHLineD, Fg: fg})
	}
	for y := py; y < py+ph; y++ {
		a.set(px, y, screen.Cell{Ch: gWallD, Fg: fg})
		a.set(px+pw-1, y, screen.Cell{Ch: gWallD, Fg: fg})
	}
	a.set(px, py, screen.Cell{Ch: boxTL, Fg: fg})
	a.set(px+pw-1, py, screen.Cell{Ch: boxTR, Fg: fg})
	a.set(px, py+ph-1, screen.Cell{Ch: boxBL, Fg: fg})
	a.set(px+pw-1, py+ph-1, screen.Cell{Ch: boxBR, Fg: fg})
	a.put(px+2, py, clip(" "+title+" ", pw-4), 0, 7)
}

var (
	boxTL = CP("╔")[0]
	boxTR = CP("╗")[0]
	boxBL = CP("╚")[0]
	boxBR = CP("╝")[0]
)

// popupRect centers a pw x ph box in the viewport.
func (a *App) popupRect(pw, ph int) (int, int, int, int) {
	pw = minInt(pw, a.cols)
	ph = minInt(ph, a.vrows)
	px := (a.cols - pw) / 2
	py := a.vy0 + (a.vrows-ph)/2
	return px, py, pw, ph
}

// say shows a transient line on the message bar (build errors, hints).
func (a *App) say(s string, bad bool) {
	a.alert, a.alertBad, a.alertUntil = s, bad, a.frame+frameRate*4
}

// ---- views (popups) ----

type view interface {
	draw(a *App)
	// key handles a key event first; false lets the App apply the generic
	// focus keys (Tab/arrows/Enter/Space/Esc).
	key(a *App, ev term.Event) bool
}

// wheeler is implemented by views that scroll.
type wheeler interface {
	wheel(a *App, dir int, shift bool)
}

// clicker is implemented by views that handle coordinate clicks (map).
type clicker interface {
	click(a *App, h *hotspot, x, y int)
}

func (a *App) open(v view) {
	a.views = append(a.views, v)
	a.focusIdx = 0
}

func (a *App) closeTop() {
	if n := len(a.views); n > 0 {
		a.views = a.views[:n-1]
		a.focusIdx = 0
	}
}

func (a *App) top() view {
	if n := len(a.views); n > 0 {
		return a.views[n-1]
	}
	return nil
}

// ---- frame loop ----

// Run blocks until the player quits or the connection drops.
func (a *App) Run() {
	tk := time.NewTicker(time.Second / frameRate)
	defer tk.Stop()
	last := time.Now()
	a.lastProbe = last
	a.render()
	a.flush()
	for !a.quit {
		select {
		case ev, ok := <-a.cfg.Events:
			if !ok || ev.Disconnect {
				a.saveQuiet()
				return
			}
			a.handle(ev)
			// drain whatever else is queued before the next tick
			for drained := false; !drained && !a.quit; {
				select {
				case ev, ok := <-a.cfg.Events:
					if !ok || ev.Disconnect {
						a.saveQuiet()
						return
					}
					a.handle(ev)
				default:
					drained = true
				}
			}
		case now := <-tk.C:
			dt := now.Sub(last).Seconds()
			last = now
			a.tick(dt)
			a.render()
			a.flush()
			if a.cfg.ProbeEvery > 0 && now.Sub(a.lastProbe) >= a.cfg.ProbeEvery {
				a.lastProbe = now
				a.cfg.Term.Write([]byte("\x1b7\x1b[999;999H\x1b[6n\x1b8"))
			}
		}
	}
	a.saveQuiet()
}

// logf writes to the door log when one is configured.
func (a *App) logf(format string, args ...any) {
	if a.cfg.Logger != nil {
		a.cfg.Logger.Printf(format, args...)
	}
}

func (a *App) saveQuiet() {
	if err := a.m.Save(); err != nil && a.cfg.Logger != nil {
		a.cfg.Logger.Printf("save: %v", err)
	}
}

func (a *App) flush() {
	if a.cfg.Term == nil {
		return
	}
	if err := a.scr.Flush(a.cfg.Term); err != nil && a.cfg.Logger != nil {
		a.cfg.Logger.Printf("flush: %v", err)
	}
}

// tick advances the model and the animation clocks by one render frame.
func (a *App) tick(dt float64) {
	if dt > 0.5 {
		dt = 0.5
	}
	a.frame++
	if _, menu := a.top().(*startView); menu {
		return // the backdrop model does not run under the start menu
	}
	a.m.Step(dt)
	if a.m.Speed() > 0 {
		a.sub++
		if a.sub >= frameRate/4 {
			a.sub = 0
			a.tframe++
		}
	}
	// a new event prompt pops up and scrolls the view to it
	if ev := a.m.Event(); ev != a.lastEvent {
		a.lastEvent = ev
		if ev != nil {
			if ev.Floor > -100 {
				a.centerOn(ev.X, ev.Floor)
			}
			a.zoom = false
			a.open(&eventView{})
		}
	}
}

func (a *App) centerOn(u, f int) {
	a.topFloor = f + a.nfloors/2
	a.left = u - a.units/2
	a.clampScroll()
}

// render draws one full frame into the screen buffer.
func (a *App) render() {
	a.hots = a.hots[:0]
	a.fullClip()
	if a.tooSmall {
		a.scr.Clear(0)
		a.put(1, 1, "SimTower needs at least 80x24.", 15, 0)
		a.put(1, 2, "This window is "+itoa(a.cols)+"x"+itoa(a.rows)+". Enlarge it or press Q to quit.", 7, 0)
		return
	}
	a.clip = a.viewRect()
	if a.zoom {
		a.drawZoom()
	} else {
		a.drawScene()
	}
	a.fullClip()
	a.drawGutter()
	a.drawStatus()
	a.drawFooter()
	a.popupHot0 = len(a.hots)
	for i, v := range a.views {
		// lower popups are drawn but inert; only the top one takes input
		a.passive = i < len(a.views)-1
		a.popupHot0 = len(a.hots)
		a.nfocus = 0
		v.draw(a)
	}
	a.passive = false
	if a.nfocus > 0 {
		a.focusIdx = ((a.focusIdx % a.nfocus) + a.nfocus) % a.nfocus
	}
}
