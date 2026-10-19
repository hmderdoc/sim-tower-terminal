package ui

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"simtower/internal/screen"
	"simtower/internal/term"
)

var update = flag.Bool("update", false, "rewrite testdata golden files")

func newTestApp(cols, rows int) (*App, *DemoModel) {
	d := NewDemo()
	a := New(Config{Cols: cols, Rows: rows, Model: d})
	return a, d
}

// dump renders the screen as text: the glyph grid (CP437 shown as Unicode),
// then the fg grid (hex digit per cell) and the bg grid (digit per cell).
func dump(s *screen.Screen) string {
	var b strings.Builder
	b.WriteString("# chars\n")
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			b.WriteRune(cpToUni[s.Get(x, y).Ch])
		}
		b.WriteByte('\n')
	}
	b.WriteString("# fg (0-f)\n")
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			b.WriteByte("0123456789abcdef"[s.Get(x, y).Fg&15])
		}
		b.WriteByte('\n')
	}
	b.WriteString("# bg (0-7)\n")
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			b.WriteByte('0' + s.Get(x, y).Bg&7)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func golden(t *testing.T, name string, a *App) {
	t.Helper()
	a.render()
	got := dump(a.scr)
	path := filepath.Join("testdata", name+".txt")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/ui -update)", err)
	}
	if string(want) != got {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range gl {
			if i >= len(wl) || gl[i] != wl[i] {
				w := ""
				if i < len(wl) {
					w = wl[i]
				}
				t.Fatalf("%s differs at line %d:\n got %q\nwant %q\n(run go test ./internal/ui -update to accept)", name, i+1, gl[i], w)
			}
		}
		t.Fatalf("%s differs", name)
	}
}

func shaftByX(d *DemoModel, x int) int {
	for _, s := range d.shafts {
		if s.x == x {
			return s.id
		}
	}
	return 0
}

// scenes is every view the goldens and the byte-safety sweep cover.
var scenes = []struct {
	name  string
	setup func(a *App, d *DemoModel)
}{
	{"main", func(a *App, d *DemoModel) {}},
	{"night", func(a *App, d *DemoModel) { d.minute = 23*60 + 20; d.refresh() }},
	{"dusk", func(a *App, d *DemoModel) { d.minute = 18*60 + 10; d.refresh() }},
	{"rain", func(a *App, d *DemoModel) { d.day = 5; d.minute = 14 * 60; d.refresh() }},
	{"build", func(a *App, d *DemoModel) { a.open(newBuildView(a)) }},
	{"elevator", func(a *App, d *DemoModel) { a.open(&elevatorView{id: shaftByX(d, 24)}) }},
	{"facility", func(a *App, d *DemoModel) { a.open(&facilityView{id: d.items[3].ID}) }},
	{"cinema", func(a *App, d *DemoModel) {
		for _, it := range d.items {
			if it.Kind == KindCinema {
				a.open(&facilityView{id: it.ID})
			}
		}
	}},
	{"tenant", func(a *App, d *DemoModel) { a.open(&tenantView{id: 1003}) }},
	{"eval", func(a *App, d *DemoModel) { a.evalMode = 1 }},
	{"map", func(a *App, d *DemoModel) { a.open(&mapView{}) }},
	{"zoom", func(a *App, d *DemoModel) { a.zoom = true }},
	{"finance", func(a *App, d *DemoModel) { a.open(&financeView{}) }},
	{"rating", func(a *App, d *DemoModel) { a.open(&ratingView{}) }},
	{"log", func(a *App, d *DemoModel) { a.open(&logView{}) }},
	{"fire", func(a *App, d *DemoModel) {
		d.startFire()
		d.refresh()
		a.tick(0)
	}},
	{"ghost", func(a *App, d *DemoModel) {
		a.arm(demoCatalog[1]) // office
		a.kbd = true
		a.curX, a.curF = 46, 4
		a.syncHover()
	}},
}

func TestGolden80x24(t *testing.T) {
	for _, sc := range scenes {
		a, d := newTestApp(80, 24)
		sc.setup(a, d)
		golden(t, "80x24_"+sc.name, a)
	}
}

func TestGolden132x50(t *testing.T) {
	for _, name := range []string{"main", "build", "elevator", "map", "zoom", "finance"} {
		a, d := newTestApp(132, 50)
		a.topFloor = 11 // the wide mock's view
		a.clampScroll()
		for _, sc := range scenes {
			if sc.name == name {
				sc.setup(a, d)
			}
		}
		golden(t, "132x50_"+name, a)
	}
}

// TestNoUnsafeBytes renders every scene at several sizes and animation
// frames and checks no cell holds a byte the terminal would act on, and that
// the flushed stream carries none outside escape sequences.
func TestNoUnsafeBytes(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {100, 31}, {132, 50}, {200, 60}} {
		for _, sc := range scenes {
			a, d := newTestApp(sz[0], sz[1])
			sc.setup(a, d)
			for f := 0; f < 30; f++ {
				a.tframe, a.frame = f, f
				d.t = float64(f * 7)
				d.refresh()
				a.render()
				for y := 0; y < a.rows; y++ {
					for x := 0; x < a.cols; x++ {
						if c := a.scr.Get(x, y).Ch; !screen.SafeByte(c) {
							t.Fatalf("%dx%d %s frame %d: unsafe byte 0x%02X at %d,%d", sz[0], sz[1], sc.name, f, c, x, y)
						}
					}
				}
				var buf bytes.Buffer
				a.scr.Flush(&buf)
				for i, c := range buf.Bytes() {
					switch c {
					case 0x00, 0x07, 0x08, 0x09, 0x0A, 0x0D, 0x7F, 0xFF:
						t.Fatalf("%s: flushed unsafe byte 0x%02X at offset %d", sc.name, c, i)
					}
				}
			}
		}
	}
}

func TestTilesMatchMockWidths(t *testing.T) {
	want := map[Kind][2]int{KindOffice: {9, 3}, KindCondo: {16, 3}, KindHotelSingle: {4, 3}, KindHotelTwin: {6, 3},
		KindHotelSuite: {10, 3}, KindFastFood: {16, 3}, KindRestaurant: {24, 3}, KindRetail: {12, 3}, KindCinema: {31, 6},
		KindPartyHall: {24, 6}, KindHousekeeping: {15, 3}, KindSecurity: {16, 3}, KindMedical: {26, 3},
		KindParkingRamp: {15, 3}, KindParking: {4, 3}, KindMetro: {30, 9}, KindCathedral: {28, 12}, KindRecycling: {25, 6},
		KindStairs: {8, 6}, KindEscalator: {8, 6}, KindElevator: {4, 3}, KindExpress: {6, 3}, KindLobby: {16, 3}}
	for k, wh := range want {
		w, h := TileSize(k)
		if w != wh[0] || h != wh[1] {
			t.Errorf("%s: %dx%d, mock %dx%d", kindName(k), w, h, wh[0], wh[1])
		}
		for _, st := range tiles[k].states {
			if tileCache[k][st] == nil {
				t.Errorf("%s state %v not cached", kindName(k), st)
			}
		}
	}
}

// ---- input paths ----

func click(a *App, x, y, button int) {
	a.handle(term.Event{Mouse: true, MKind: term.MousePress, MX: x + 1, MY: y + 1, Button: button})
	a.render()
	a.handle(term.Event{Mouse: true, MKind: term.MouseRelease, MX: x + 1, MY: y + 1, Button: button})
	a.render()
}

func keyCh(a *App, c byte)       { a.handle(term.Event{Kind: term.KeyChar, Ch: c}); a.render() }
func key(a *App, k term.KeyKind) { a.handle(term.Event{Kind: k}); a.render() }

func TestClickBuildAndPlace(t *testing.T) {
	a, d := newTestApp(80, 24)
	a.render()
	click(a, a.cols-12, 0, 0) // [BUILD]
	if _, ok := a.top().(*buildView); !ok {
		t.Fatalf("[BUILD] did not open the palette: %T", a.top())
	}
	keyCh(a, 'o')
	if a.tool != KindOffice || len(a.views) != 0 {
		t.Fatalf("O did not arm the office tool: tool=%v views=%d", a.tool, len(a.views))
	}
	n := len(d.items)
	// 7F (floor 6) is empty from x=28; floor 5 below spans 0..96
	a.topFloor = 8
	a.render()
	click(a, a.sx(28), a.fy(6)+1, 0)
	if len(d.items) != n+1 {
		t.Fatalf("click did not place (alert %q)", a.alert)
	}
	if a.tool != KindOffice {
		t.Fatal("tool should stay armed after placing")
	}
	click(a, a.sx(28), a.fy(6)+1, 2) // right click disarms
	if a.tool != KindNone {
		t.Fatal("right click did not disarm")
	}
}

func TestKeyboardOnly(t *testing.T) {
	a, d := newTestApp(80, 24)
	a.render()
	// Tab to the status row, Right to [BUILD], Enter
	key(a, term.KeyTab)
	if a.focus != zoneStatus {
		t.Fatal("Tab did not focus the status row")
	}
	for i := 0; i < 5; i++ {
		key(a, term.KeyRight)
	}
	key(a, term.KeyEnter)
	if _, ok := a.top().(*buildView); !ok {
		t.Fatalf("Enter on [BUILD] did not open the palette: %T", a.top())
	}
	key(a, term.KeyEsc)
	key(a, term.KeyTab)
	key(a, term.KeyTab) // back to the viewport
	if a.focus != zoneView {
		t.Fatal("Tab did not cycle back to the viewport")
	}
	// inspect the fast food at 2F x=28 with the cursor
	a.curX, a.curF = 30, 1
	key(a, term.KeyEnter)
	if v, ok := a.top().(*facilityView); !ok || d.find(v.id).Kind != KindFastFood {
		t.Fatalf("Enter did not inspect the fast food: %T", a.top())
	}
	key(a, term.KeyEsc)
	if len(a.views) != 0 {
		t.Fatal("Esc did not close")
	}
}

func TestElevatorWindowControls(t *testing.T) {
	a, d := newTestApp(80, 24)
	a.render()
	s := d.shafts[0]
	click(a, a.sx(s.x+1), a.fy(2)+1, 0)
	v, ok := a.top().(*elevatorView)
	if !ok {
		t.Fatalf("click on shaft did not open the elevator window: %T", a.top())
	}
	_ = v
	before := s.modes[0][3]
	// find the 1-5 cell hotspot on the first row and click it
	for i := a.popupHot0; i < len(a.hots); i++ {
		if h := a.hots[i]; h.act == actMode && h.arg == 3 {
			click(a, h.x, h.y, 0)
			break
		}
	}
	if s.modes[0][3] == before {
		t.Fatal("clicking a period cell did not change the mode")
	}
	cars := len(d.shaftsOut[0].Cars)
	for i := a.popupHot0; i < len(a.hots); i++ {
		if h := a.hots[i]; h.act == actAddCar {
			click(a, h.x, h.y, 0)
			break
		}
	}
	if len(d.shaftsOut[0].Cars) != cars+1 {
		t.Fatal("ADD CAR did not add a car")
	}
}

func TestEventPrompt(t *testing.T) {
	a, d := newTestApp(80, 24)
	d.startFire()
	a.tick(0)
	a.render()
	if _, ok := a.top().(*eventView); !ok {
		t.Fatalf("fire did not open a prompt: %T", a.top())
	}
	key(a, term.KeyEsc)
	if _, ok := a.top().(*eventView); !ok {
		t.Fatal("Esc must not dismiss an event prompt")
	}
	funds := d.funds
	key(a, term.KeyEnter) // focus starts on the first choice: rescue
	if d.event != nil || d.funds != funds-500000 {
		t.Fatalf("rescue not applied: event=%v funds=%d", d.event, d.funds)
	}
}

func TestResize(t *testing.T) {
	a, _ := newTestApp(80, 24)
	a.handle(term.Event{Resize: true, Cols: 132, Rows: 50})
	if a.cols != 132 || a.nfloors != 15 || a.units != 128 {
		t.Fatalf("relayout: cols=%d nfloors=%d units=%d", a.cols, a.nfloors, a.units)
	}
	a.handle(term.Event{Resize: true, Cols: 60, Rows: 20})
	a.render()
	if !a.tooSmall {
		t.Fatal("60x20 should be too small")
	}
}

func BenchmarkRender132x50(b *testing.B) {
	a, d := newTestApp(132, 50)
	var buf bytes.Buffer
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.tframe, a.frame = i, i
		d.Step(0.125)
		a.render()
		buf.Reset()
		a.scr.Flush(&buf)
	}
}
