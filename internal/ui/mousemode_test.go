package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"simtower/internal/term"
)

type sinkTerm struct{ bytes.Buffer }

func (s *sinkTerm) Read(p []byte) (int, error)                         { select {} }
func (s *sinkTerm) ReadTimeout(p []byte, d time.Duration) (int, error) { return 0, nil }
func (s *sinkTerm) Close() error                                       { return nil }

// Dropping a build tool must leave click tracking on: SyncTERM has a single
// tracking mode, and a bare "?1003l" switches its mouse off completely.
func TestDisarmKeepsClickTracking(t *testing.T) {
	d := NewDemo()
	sink := &sinkTerm{}
	a := New(Config{Cols: 80, Rows: 24, Model: d, Term: sink})
	a.render()
	a.arm(d.Catalog()[0])
	if !strings.Contains(sink.String(), "\x1b[?1003h") {
		t.Fatalf("arm did not enable any-motion tracking: %q", sink.String())
	}
	sink.Reset()
	a.disarm()
	out := sink.String()
	off := strings.Index(out, "\x1b[?1003l")
	on := strings.Index(out, term.MouseOn)
	if off < 0 || on < 0 || on < off {
		t.Fatalf("disarm must reset 1003 then re-select click tracking, got %q", out)
	}
}

// With no tool armed a left drag pans the view; a left press released in
// place still inspects.
func TestLeftDragPans(t *testing.T) {
	a, _ := newTestApp(80, 24)
	a.topFloor = 8
	a.left = 10
	a.render()
	x, y := a.sx(30), a.fy(5)+1
	left0, top0 := a.left, a.topFloor
	press := func(x, y int) {
		a.handle(term.Event{Mouse: true, MKind: term.MousePress, MX: x + 1, MY: y + 1, Button: 0})
	}
	move := func(x, y int) {
		a.handle(term.Event{Mouse: true, MKind: term.MouseMotion, MX: x + 1, MY: y + 1, Button: 0})
	}
	release := func(x, y int) {
		a.handle(term.Event{Mouse: true, MKind: term.MouseRelease, MX: x + 1, MY: y + 1, Button: 0})
		a.render()
	}
	press(x, y)
	if len(a.views) != 0 {
		t.Fatal("press alone must not open a window")
	}
	move(x-5, y+3)
	release(x-5, y+3)
	if a.left != left0+5 || a.topFloor != top0+1 {
		t.Fatalf("drag did not pan: left %d->%d top %d->%d", left0, a.left, top0, a.topFloor)
	}
	if len(a.views) != 0 {
		t.Fatalf("drag must not inspect: %T", a.top())
	}
	// a plain click inspects the room under it
	a.topFloor, a.left = 8, 0
	a.render()
	click(a, a.sx(30), a.fy(5)+1, 0)
	if _, ok := a.top().(*facilityView); !ok {
		t.Fatalf("click did not inspect: %T alert=%q", a.top(), a.alert)
	}
}
