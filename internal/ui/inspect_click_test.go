package ui

import (
	"strings"
	"testing"
)

func TestClickInspectsAndSetsRent(t *testing.T) {
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
	x, y := a.sx(office.X+2), a.fy(office.Floor)+1
	t.Logf("office %+v click at %d,%d cell=%v", office, x, y, func() any { u, f, ok := a.cellAt(x, y); return []any{u, f, ok} }())
	click(a, x, y, 0)
	fv, ok := a.top().(*facilityView)
	if !ok {
		t.Fatalf("click did not open the facility window: %T alert=%q", a.top(), a.alert)
	}
	if fv.id != office.ID {
		t.Fatalf("opened item %d, want %d", fv.id, office.ID)
	}
	// find the rent buttons and click tier 2
	var hit *hotspot
	for i := a.popupHot0; i < len(a.hots); i++ {
		if a.hots[i].act == actRent && a.hots[i].arg == 2 {
			h := a.hots[i]
			hit = &h
		}
	}
	if hit == nil {
		t.Fatalf("no rent button hotspots")
	}
	click(a, hit.x+1, hit.y, 0)
	if _, ok := a.top().(*facilityView); !ok {
		t.Fatalf("window closed after rent click: %T", a.top())
	}
	for _, it := range m.Items() {
		if it.ID == office.ID && it.Rent != 2 {
			t.Fatalf("rent not applied: %d", it.Rent)
		}
	}
	in := m.Inspect(office.ID)
	if in.Rent != 2 {
		t.Fatalf("inspect rent %d", in.Rent)
	}
	// step and re-check persistence
	for i := 0; i < 8*20; i++ {
		m.Step(0.125)
	}
	if in := m.Inspect(office.ID); in.Rent != 2 {
		t.Fatalf("rent reverted to %d after stepping", in.Rent)
	}
}

// The for-sale condo tile shows the asking price, not the art's sample.
func TestCondoTileShowsAskingPrice(t *testing.T) {
	m := buildTestTower(t)
	a := New(Config{Cols: 80, Rows: 24, Model: m})
	var condo Item
	for _, it := range m.Items() {
		if it.Kind == KindCondo {
			condo = it
		}
	}
	a.centerOn(condo.X, condo.Floor)
	row := func() string {
		a.render()
		y := a.fy(condo.Floor) + 1
		var b []byte
		for x := a.sx(condo.X); x < a.sx(condo.X)+condo.W; x++ {
			b = append(b, a.scr.Get(x, y).Ch)
		}
		return string(b)
	}
	if r := row(); !strings.Contains(r, "FOR SALE $150K") {
		t.Fatalf("default tile row %q", r)
	}
	if r := m.SetRent(condo.ID, 3); !r.OK {
		t.Fatal(r.Reason)
	}
	if r := row(); !strings.Contains(r, "FOR SALE $40K") {
		t.Fatalf("after repricing: %q", r)
	}
}
