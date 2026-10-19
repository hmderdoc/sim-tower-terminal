package ui

import "simtower/internal/term"

// Launcher configures the start menu (Config.Launcher). With a Launcher the
// App opens on the splash with Continue / New tower / Demo / Quit and swaps
// the chosen model in; without one it runs Config.Model directly (tests).
type Launcher struct {
	SaveInfo string                // one-line summary of the existing save; "" = none
	Continue func() (Model, error) // load the save
	NewGame  func() (Model, error) // start a fresh tower (overwrites the save on the next autosave)
	Demo     func() Model          // the scripted demo tower
}

// modalView is implemented by popups that must be answered: right click,
// Esc and the main hotkeys do not close them, and the status row and footer
// under them are inert.
type modalView interface{ modal() }

func (a *App) topModal() bool {
	_, ok := a.top().(modalView)
	return ok
}

const (
	startContinue = iota
	startNew
	startDemo
	startQuit
	startYes
	startNo
)

type startView struct {
	confirm bool   // "overwrite your tower?" step of New tower
	err     string // last load error
}

func (v *startView) modal() {}

func (v *startView) draw(a *App) {
	a.drawSplash(a.frame)
	l := a.cfg.Launcher
	r := a.splashMenuRect(46, 7)
	a.box(r.x, r.y, r.w, r.h, "START", 15)
	x, y := r.x+2, r.y+1
	if v.confirm {
		a.put(x, y, clip("Start over? Your saved tower will be", r.w-4), 15, 0)
		a.put(x, y+1, clip("replaced at the next save.", r.w-4), 15, 0)
		bx := x
		bx += a.button(bx, y+3, "YES, NEW TOWER", actStart, startYes) + 2
		a.button(bx, y+3, "NO", actStart, startNo)
		return
	}
	info := "No saved tower yet."
	if l != nil && l.SaveInfo != "" {
		info = l.SaveInfo
	}
	a.put(x, y, clip(info, r.w-4), 11, 0)
	type entry struct {
		label string
		arg   int
	}
	var list []entry
	if l != nil && l.SaveInfo != "" && l.Continue != nil {
		list = append(list, entry{"C  CONTINUE  ", startContinue})
	}
	list = append(list, entry{"N  NEW TOWER ", startNew}, entry{"D  DEMO TOWER", startDemo}, entry{"Q  QUIT      ", startQuit})
	for i, e := range list {
		a.button(x+(i%2)*21, y+1+i/2, e.label, actStart, e.arg)
	}
	if v.err != "" {
		a.put(x, r.y+r.h-2, clip(v.err, r.w-4), 12, 0)
	} else {
		a.put(x, r.y+r.h-2, clip("Arrows/Tab + Enter, or click.", r.w-4), 8, 0)
	}
}

func (v *startView) act(a *App, h *hotspot) bool {
	if h.act != actStart {
		return false
	}
	v.pick(a, h.arg)
	return true
}

func (v *startView) pick(a *App, choice int) {
	l := a.cfg.Launcher
	if l == nil {
		a.closeTop()
		return
	}
	switch choice {
	case startContinue:
		if l.Continue == nil || l.SaveInfo == "" {
			return
		}
		m, err := l.Continue()
		if err != nil {
			v.err = "Cannot load: " + err.Error()
			if a.cfg.Logger != nil {
				a.cfg.Logger.Printf("continue: %v", err)
			}
			return
		}
		a.setModel(m)
	case startNew:
		if l.SaveInfo != "" && !v.confirm {
			v.confirm = true
			a.focusIdx = 1 // default to NO
			return
		}
		fallthrough
	case startYes:
		if l.NewGame == nil {
			return
		}
		m, err := l.NewGame()
		if err != nil {
			v.err = err.Error()
			v.confirm = false
			return
		}
		a.setModel(m)
	case startNo:
		v.confirm = false
		a.focusIdx = 0
	case startDemo:
		if l.Demo != nil {
			a.setModel(l.Demo())
		}
	case startQuit:
		a.quit = true
	}
}

func (v *startView) key(a *App, ev term.Event) bool {
	switch ev.Kind {
	case term.KeyChar:
		switch upper(ev.Ch) {
		case 'C':
			if !v.confirm {
				v.pick(a, startContinue)
			}
		case 'N':
			if !v.confirm {
				v.pick(a, startNew)
			}
		case 'D':
			if !v.confirm {
				v.pick(a, startDemo)
			}
		case 'Y':
			if v.confirm {
				v.pick(a, startYes)
			}
		case 'Q':
			if v.confirm {
				v.pick(a, startNo)
			} else {
				v.pick(a, startQuit)
			}
		case ' ':
			a.activateFocus()
		}
		return true
	case term.KeyEsc, term.KeyBackspace:
		if v.confirm {
			v.pick(a, startNo)
		}
		return true
	}
	return false // Tab / arrows / Enter: generic focus handling
}

// setModel swaps the running model (start menu choice) and resets the view
// to the ground floor of the new tower.
func (a *App) setModel(m Model) {
	a.m = m
	a.bounds = m.Bounds()
	a.closeAll()
	a.disarm()
	a.lastEvent = nil
	a.evalMode = 0
	a.zoom = false
	a.layout(a.cols, a.rows)
	a.topFloor = a.nfloors - 2
	a.left = (a.bounds.Width - a.units) / 2
	for _, it := range m.Items() {
		if it.Kind == KindLobby && it.Floor == 0 {
			a.left = it.X + it.W/2 - a.units/2
			break
		}
	}
	a.clampScroll()
	a.curX, a.curF = a.left+a.units/2, 0
	a.scr.Invalidate()
}
