package ui

import (
	"testing"

	"simtower/internal/term"
)

// Keyboard-only: I, Enter opens the window, Tab + Enter sets a rent tier.
func TestKeyboardInspectAndRent(t *testing.T) {
	m := buildTestTower(t)
	for i := 0; i < 8*20; i++ {
		m.Step(0.125)
	}
	a := New(Config{Cols: 80, Rows: 24, Model: m})
	a.centerOn(130, 2)
	a.render()
	var office Item
	for _, it := range m.Items() {
		if it.Kind == KindOffice && it.Floor == 2 {
			office = it
			break
		}
	}
	a.curX, a.curF = office.X+1, office.Floor
	keyCh(a, 'i')
	enter := func() { key(a, term.KeyEnter) }
	enter()
	fv, ok := a.top().(*facilityView)
	if !ok || len(a.views) != 1 {
		t.Fatalf("after Enter: views=%d top=%T", len(a.views), a.top())
	}
	if fv.id != office.ID {
		t.Fatalf("opened %d want %d", fv.id, office.ID)
	}
	// tab to the tier-2 rent button
	for i := 0; i < 40; i++ {
		h := a.focusedHot()
		if h != nil && h.act == actRent && h.arg == 2 {
			break
		}
		key(a, term.KeyTab)
	}
	enter()
	if in := m.Inspect(office.ID); in.Rent != 2 {
		t.Fatalf("rent %d after keyboard set", in.Rent)
	}
	if len(a.views) != 1 {
		t.Fatalf("views after rent set: %d (%T)", len(a.views), a.top())
	}
}
