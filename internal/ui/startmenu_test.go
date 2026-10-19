package ui

import (
	"path/filepath"
	"testing"

	"simtower/internal/screen"
	"simtower/internal/term"
)

func newLauncherApp(t *testing.T, withSave bool) (*App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.json")
	if withSave {
		m := NewSimModel("p", "P", path)
		if r := m.Place(KindLobby, 150, 0, 20, 1); !r.OK {
			t.Fatal(r.Reason)
		}
		if err := m.Save(); err != nil {
			t.Fatal(err)
		}
	}
	l := &Launcher{
		SaveInfo: SaveSummary(path),
		Continue: func() (Model, error) { return LoadSimModel(path) },
		NewGame:  func() (Model, error) { m := NewSimModel("p", "P", path); return m, m.Save() },
		Demo:     func() Model { return NewDemo() },
	}
	a := New(Config{Cols: 80, Rows: 24, Model: NewDemo(), Launcher: l})
	a.render()
	return a, path
}

func TestStartMenuKeyboard(t *testing.T) {
	a, _ := newLauncherApp(t, true)
	if _, ok := a.top().(*startView); !ok {
		t.Fatalf("no start menu: %T", a.top())
	}
	for y := 0; y < a.rows; y++ {
		for x := 0; x < a.cols; x++ {
			if !screen.SafeByte(a.scr.Get(x, y).Ch) {
				t.Fatalf("unsafe byte at %d,%d", x, y)
			}
		}
	}
	// hotkeys and right click must not dismiss it; the demo backdrop must not run
	keyCh(a, 'b')
	click(a, 40, 12, 2)
	key(a, term.KeyEsc)
	a.tick(1)
	if _, ok := a.top().(*startView); !ok {
		t.Fatalf("start menu dismissed: %T", a.top())
	}
	key(a, term.KeyEnter) // focus 0 = CONTINUE
	m, ok := a.m.(*SimModel)
	if !ok || len(a.views) != 0 {
		t.Fatalf("continue: model %T views %d", a.m, len(a.views))
	}
	if items := m.Items(); len(items) != 1 || items[0].Kind != KindLobby || items[0].W != 20 {
		t.Fatalf("loaded tower %+v", items)
	}
}

func TestStartMenuNewConfirm(t *testing.T) {
	a, _ := newLauncherApp(t, true)
	keyCh(a, 'n')
	v := a.top().(*startView)
	if !v.confirm {
		t.Fatal("new tower over a save must ask first")
	}
	keyCh(a, 'q') // = no
	if v.confirm || a.quit {
		t.Fatal("q in the confirm step should step back, not quit")
	}
	keyCh(a, 'n')
	keyCh(a, 'y')
	m, ok := a.m.(*SimModel)
	if !ok || len(m.Items()) != 0 {
		t.Fatalf("new tower: %T", a.m)
	}
}

func TestStartMenuClicks(t *testing.T) {
	a, _ := newLauncherApp(t, false)
	var demo *hotspot
	for i := a.popupHot0; i < len(a.hots); i++ {
		if h := a.hots[i]; h.act == actStart && h.arg == startContinue {
			t.Fatal("CONTINUE offered without a save")
		} else if h.act == actStart && h.arg == startDemo {
			demo = &h
		}
	}
	if demo == nil {
		t.Fatal("no DEMO button")
	}
	click(a, demo.x+1, demo.y, 0)
	if _, ok := a.m.(*DemoModel); !ok || len(a.views) != 0 {
		t.Fatalf("demo: %T views %d", a.m, len(a.views))
	}
}
