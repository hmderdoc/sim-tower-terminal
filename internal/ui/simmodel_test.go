package ui

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"simtower/internal/screen"
	"simtower/internal/sim"
)

// buildTestTower lays out, through the adapter, a lobby, four floors of
// offices with a condo, and a standard elevator 1F..5F.
func buildTestTower(t *testing.T) *SimModel {
	t.Helper()
	m := NewSimModel("test", "Test Tower", "")
	must := func(what string, r Result) {
		t.Helper()
		if !r.OK {
			t.Fatalf("%s: %s", what, r.Reason)
		}
	}
	must("lobby", m.Place(KindLobby, 100, 0, 64, 1))
	for f := 1; f <= 4; f++ {
		must("floor", m.Place(KindFloor, 100, f, 64, 1))
	}
	for f := 1; f <= 4; f++ {
		for x := 100; x+9 <= 136; x += 9 {
			must("office", m.Place(KindOffice, x, f, 9, 1))
		}
	}
	must("condo", m.Place(KindCondo, 146, 1, 16, 1))
	must("elevator", m.Place(KindElevator, 138, 0, 4, 5))
	return m
}

func TestClockOfTick(t *testing.T) {
	for _, c := range []struct{ tick, h, m int }{
		{0, 7, 0}, {399, 11, 59}, {400, 12, 0}, {800, 12, 30}, {1200, 13, 0}, {1600, 17, 0},
		{2000, 21, 0}, {2300, 0, 0}, {2400, 1, 0}, {sim.NewGameDayTick, 4, 59}, {2599, 6, 58},
	} {
		got := ClockOfTick(c.tick)
		if got != c.h*60+c.m {
			t.Errorf("tick %d: %02d:%02d, want %02d:%02d", c.tick, got/60, got%60, c.h, c.m)
		}
	}
}

func TestSimModelMapping(t *testing.T) {
	m := buildTestTower(t)
	if c := m.Clock(); c.DayLabel != "WD1" || c.Quarter != 1 || c.Year != 1 || c.TimeString() != "4:59 AM" {
		t.Fatalf("new game clock %+v %s", c, c.TimeString())
	}
	// structure
	var lobbies, offices, condos int
	for _, it := range m.Items() {
		switch it.Kind {
		case KindLobby:
			lobbies++
			if it.X != 100 || it.W != 64 || it.Floor != 0 || it.H != 1 {
				t.Errorf("lobby run %+v, want one 64-unit run at 100 on 1F", it)
			}
		case KindOffice:
			offices++
			if it.W != 9 || it.H != 1 || it.Rent != 1 || it.Capacity != 6 {
				t.Errorf("office %+v", it)
			}
		case KindCondo:
			condos++
		}
	}
	if lobbies != 1 || offices != 16 || condos != 1 {
		t.Fatalf("items: %d lobby runs, %d offices, %d condos", lobbies, offices, condos)
	}
	spans := map[int]FloorSpan{}
	for _, sp := range m.Floors() {
		spans[sp.Floor] = sp
	}
	for f := 0; f <= 4; f++ {
		if sp, ok := spans[f]; !ok || sp.Left != 100 || sp.Right < 164 {
			t.Errorf("floor span %d: %+v", f, sp)
		}
	}
	sh := m.Shafts()
	if len(sh) != 1 || sh[0].Kind != KindElevator || sh[0].X != 138 || sh[0].Bottom != 0 || sh[0].Top != 4 || len(sh[0].Cars) != 1 {
		t.Fatalf("shafts %+v", sh)
	}
	if sc := m.Schedule(sh[0].ID); len(sc.Served) != 5 || sc.Capacity != 21 || sc.Response != 5 || sc.CarCost != 80000 {
		t.Fatalf("schedule %+v", sc)
	}

	// one day at normal speed, 1/8 s frames like the App
	m.SetSpeed(1)
	t0 := m.ts.SimTime()
	sawQueue, sawMoving, sawPeople, sawOccupied := false, false, false, false
	for i := 0; i < 8*130; i++ {
		m.Step(0.125)
		if m.Event() != nil {
			m.Respond(1)
		}
		for _, s := range m.Shafts() {
			for _, c := range s.Cars {
				if c.Pos < s.Bottom*3 || c.Pos > s.Top*3 {
					t.Fatalf("car outside its shaft: %+v in %d..%d", c, s.Bottom, s.Top)
				}
				if c.Dir != 0 {
					sawMoving = true
				}
				if c.Passengers > c.Capacity {
					t.Fatalf("overfull car %+v", c)
				}
			}
			for _, q := range s.Queues {
				if q.Floor < s.Bottom || q.Floor > s.Top {
					t.Fatalf("queue off the shaft: %+v", q)
				}
				if len(q.Up)+len(q.Down) > 0 {
					sawQueue = true
				}
			}
		}
		if len(m.People()) > 0 {
			sawPeople = true
		}
		for _, p := range m.People() {
			if p.Floor < -10 || p.Floor > 109 || p.X < 0 || p.X >= 375 {
				t.Fatalf("walker off the world: %+v", p)
			}
		}
		for _, it := range m.Items() {
			if it.Kind == KindOffice && it.State == StateOccupied {
				sawOccupied = true
			}
		}
	}
	if got := m.ts.SimTime() - t0; got != sim.DayTickMax {
		t.Fatalf("one day at speed 1 advanced %d ticks, want %d", got, sim.DayTickMax)
	}
	if !sawQueue || !sawMoving || !sawOccupied || !sawPeople {
		t.Fatalf("day looked dead: queue=%v moving=%v occupied=%v people=%v", sawQueue, sawMoving, sawOccupied, sawPeople)
	}
	if c := m.Clock(); c.DayLabel != "WD2" || c.TimeString() != "4:59 AM" {
		t.Fatalf("after one day: %+v %s", c, c.TimeString())
	}
	if m.Population() <= 0 {
		t.Fatal("no population after a working day")
	}
	fin := m.Finance()
	if len(fin.Income) != 10 || len(fin.Maintenance) != 10 || fin.Balance != m.Funds() || fin.Quarter != "Y1 Q1" {
		t.Fatalf("finance shape %+v", fin)
	}
	var income int64
	for _, l := range fin.Income {
		income += l.Amount
	}
	if fin.Income[0].Amount <= 0 || income <= 0 {
		t.Fatalf("offices paid no rent: %+v", fin.Income)
	}
	// every number the sim moved is accounted for: start cash - construction + income - maintenance
	var maint int64
	for _, l := range fin.Maintenance {
		maint += l.Amount
	}
	if want := fin.PrevBalance - fin.Construction + income - maint; want != fin.Balance {
		t.Fatalf("finance does not add up: prev %d - constr %d + income %d - maint %d = %d, balance %d",
			fin.PrevBalance, fin.Construction, income, maint, want, fin.Balance)
	}
	// inspect an occupied office
	for _, it := range m.Items() {
		if it.Kind == KindOffice && it.State == StateOccupied {
			in := m.Inspect(it.ID)
			if in.Status != "Occupied" || len(in.Tiers) != 4 || in.Rent != 1 || len(in.Occupants) == 0 {
				t.Fatalf("inspect office %+v", in)
			}
			ti := m.Tenant(in.Occupants[0])
			if ti.Name != "Office worker" || !strings.HasPrefix(ti.From, "Office, Floor") {
				t.Fatalf("tenant %+v", ti)
			}
			if r := m.RenameTenant(in.Occupants[0], "Bob"); !r.OK || m.Tenant(in.Occupants[0]).Name != "Bob" {
				t.Fatalf("rename: %+v", r)
			}
			break
		}
	}
	r := m.Rating()
	if r.Stars != 1 || !strings.Contains(r.Next, "300") {
		t.Fatalf("rating %+v", r)
	}
}

func TestSimModelCanPlace(t *testing.T) {
	m := buildTestTower(t)
	cells, cash := m.world().Cells.Len(), m.Funds()
	cases := []struct {
		k          Kind
		x, f, w, h int
		want       string
	}{
		{KindOffice, 100, 6, 9, 1, "No support"},
		{KindOffice, 100, 1, 9, 1, "occupied"},
		{KindLobby, 100, 2, 4, 1, "Lobbies go on 1F"},
		{KindLobby, 100, 14, 4, 1, "No support"},
		{KindHotelSingle, 140, 5, 4, 1, "requires 2 stars"},
		{KindElevator, 144, 0, 4, 2, "too close"},
		{KindStairs, 200, 0, 8, 2, "require a base tile"},
		{KindOffice, -3, 1, 9, 1, "Outside"},
		{KindRestaurant, 100, 1, 24, 1, "requires 3 stars"},
	}
	for _, c := range cases {
		r := m.CanPlace(c.k, c.x, c.f, c.w, c.h)
		if r.OK || !strings.Contains(r.Reason, c.want) {
			t.Errorf("%s at %d,%d: %+v, want %q", kindName(c.k), c.x, c.f, r, c.want)
		}
	}
	if r := m.CanPlace(KindFloor, 100, 5, 30, 1); !r.OK || r.Cost != 30*500 {
		t.Errorf("floor drag: %+v", r)
	}
	if r := m.CanPlace(KindElevator, 138, 4, 4, 3); !r.OK || r.Cost != 2*5000+2*4*floorPremium {
		// 4F exists; 5F and 6F are extensions on bare air (auto floor)
		t.Errorf("shaft extension: %+v", r)
	}
	m.ts.Ledger().CashBalance = 1000
	if r := m.CanPlace(KindOffice, 146, 2, 9, 1); r.OK || !strings.Contains(r.Reason, "Not enough money") {
		t.Errorf("broke: %+v", r)
	}
	m.ts.Ledger().CashBalance = int(cash)
	if m.world().Cells.Len() != cells || m.Funds() != cash {
		t.Fatal("CanPlace changed the world")
	}
}

func TestSimModelPlaceCosts(t *testing.T) {
	m := buildTestTower(t)
	before := m.Funds()
	if r := m.Place(KindOffice, 146, 2, 9, 1); !r.OK || r.Cost != 40000 || m.Funds() != before-40000 {
		t.Fatalf("office on a floor: %+v funds %d -> %d", r, before, m.Funds())
	}
	before = m.Funds()
	if r := m.Place(KindLobby, 164, 0, 2, 1); !r.OK || r.Cost != 2*(5000+floorPremium) || m.Funds() != before-r.Cost {
		t.Fatalf("lobby extension on bare ground: %+v", r)
	}
	before = m.Funds()
	if r := m.Place(KindElevator, 138, 5, 4, 1); !r.OK || r.Cost != 5000+4*floorPremium || m.Funds() != before-r.Cost {
		t.Fatalf("shaft extension: %+v", r)
	}
	if s := m.Shafts(); len(s) != 1 || s[0].Top != 5 {
		t.Fatalf("extension did not grow the shaft: %+v", s)
	}
	before = m.Funds()
	if r := m.AddCar(m.Shafts()[0].ID, 0); !r.OK || m.Funds() != before-80000 || len(m.Shafts()[0].Cars) != 2 {
		t.Fatalf("add car %+v", r)
	}
	if fin := m.Finance(); fin.Construction != 2000000-m.Funds() {
		t.Fatalf("construction %d, spent %d", fin.Construction, 2000000-m.Funds())
	}
	// removal
	if r := m.Remove(101, 1); !r.OK {
		t.Fatalf("bulldoze office: %+v", r)
	}
	if r := m.Remove(101, 1); r.OK || r.Reason != "" {
		t.Fatalf("bulldozing the floor left behind must be quiet: %+v", r)
	}
	if r := m.Remove(120, 0); r.OK || !strings.Contains(r.Reason, "lobby") {
		t.Fatalf("lobby removal: %+v", r)
	}
	if r := m.Remove(139, 3); !r.OK || len(m.Shafts()) != 0 {
		t.Fatalf("bulldozing a shaft floor removes the whole shaft: %+v %+v", r, m.Shafts())
	}
}

func TestSimModelNoStepDuringPrompt(t *testing.T) {
	m := buildTestTower(t)
	m.infoQueue = append(m.infoQueue, &Event{Kind: EventInfo, Title: "T", Choices: []string{"OK"}, Floor: -100})
	t0 := m.ts.SimTime()
	m.Step(1)
	if m.Event() == nil || m.ts.SimTime() != t0 {
		t.Fatalf("stepped under a prompt: event=%v ticks %d", m.Event(), m.ts.SimTime()-t0)
	}
	m.Respond(0)
	m.Step(1)
	if m.ts.SimTime()-t0 != SimTicksPerSecond {
		t.Fatalf("1 s at speed 1 = %d ticks", m.ts.SimTime()-t0)
	}
	m.SetSpeed(0)
	m.Step(1)
	if m.ts.SimTime()-t0 != SimTicksPerSecond {
		t.Fatal("paused model stepped")
	}
	m.SetSpeed(2)
	m.Step(0.5)
	if got := m.ts.SimTime() - t0 - SimTicksPerSecond; got != SimTicksPerSecond*SimFastMultiplier/2 {
		t.Fatalf("fast: %d ticks in 0.5 s", got)
	}
}

func TestSimModelSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "player.json")
	m := buildTestTower(t)
	m.path = path
	for i := 0; i < 40; i++ {
		m.Step(0.5)
	}
	id := 0
	for _, it := range m.Items() {
		if it.Kind == KindOffice {
			id = it.ID
		}
	}
	m.SetRent(id, 0)
	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(SaveSummary(path), "Year 1 Q1") {
		t.Fatalf("summary %q", SaveSummary(path))
	}
	l, err := LoadSimModel(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(m.ts.SaveState())
	b, _ := json.Marshal(l.ts.SaveState())
	if !bytes.Equal(a, b) {
		t.Fatal("snapshot changed across save/load")
	}
	if l.Funds() != m.Funds() || l.Clock() != m.Clock() || len(l.Items()) != len(m.Items()) || len(l.Shafts()) != 1 {
		t.Fatal("loaded model differs")
	}
	for _, it := range l.Items() {
		if it.ID == id && it.Rent != 0 {
			t.Fatalf("rent tier lost: %+v", it)
		}
	}
	// both continue identically (determinism through the save)
	for i := 0; i < 20; i++ {
		m.Step(0.5)
		l.Step(0.5)
	}
	a, _ = json.Marshal(m.ts.SaveState())
	b, _ = json.Marshal(l.ts.SaveState())
	if !bytes.Equal(a, b) {
		t.Fatal("loaded tower diverged from the original")
	}
}

// TestSimModelRenders draws the real tower in every main view and checks
// byte safety.
func TestSimModelRenders(t *testing.T) {
	m := buildTestTower(t)
	for i := 0; i < 8*20; i++ {
		m.Step(0.125)
	}
	a := New(Config{Cols: 80, Rows: 24, Model: m})
	a.centerOn(130, 2)
	views := []view{nil, newBuildView(a), &financeView{}, &ratingView{}, &mapView{}, &logView{}}
	for _, it := range m.Items() {
		if it.Kind == KindOffice {
			views = append(views, &facilityView{id: it.ID})
			break
		}
	}
	views = append(views, &elevatorView{id: m.Shafts()[0].ID})
	for _, v := range views {
		a.closeAll()
		if v != nil {
			a.open(v)
		}
		for f := 0; f < 8; f++ {
			a.tick(0.125)
			a.render()
			for y := 0; y < a.rows; y++ {
				for x := 0; x < a.cols; x++ {
					if c := a.scr.Get(x, y).Ch; !screen.SafeByte(c) {
						t.Fatalf("%T: unsafe byte 0x%02X at %d,%d", v, c, x, y)
					}
				}
			}
		}
	}
}

// bigTower: 1F lobby + 14 office floors 200 units wide, three shafts.
func bigTower(b testing.TB) *SimModel {
	m := NewSimModel("big", "Big", "")
	m.ts.Ledger().CashBalance = 90_000_000
	m.Place(KindLobby, 80, 0, 200, 1)
	for f := 1; f <= 14; f++ {
		m.Place(KindFloor, 80, f, 200, 1)
		for x := 80; x+9 <= 280; x += 9 {
			m.Place(KindOffice, x, f, 9, 1)
		}
	}
	for _, x := range []int{120, 180, 240} {
		for f := 0; f <= 14; f++ {
			m.Remove(x, f)
			m.Remove(x+3, f)
		}
		if r := m.Place(KindElevator, x, 0, 4, 15); !r.OK {
			b.Fatalf("shaft at %d: %s", x, r.Reason)
		}
	}
	m.SetSpeed(1)
	for i := 0; i < 8*60; i++ { // to mid-morning of day 1
		m.Step(0.125)
		if m.Event() != nil {
			m.Respond(1)
		}
	}
	return m
}

// BenchmarkSimModelFastFrame is one 1/8 s frame in fast mode (25 ticks)
// plus the reads the renderer makes.
func BenchmarkSimModelFastFrame(b *testing.B) {
	m := bigTower(b)
	m.SetSpeed(2)
	m.stepBudget = time.Hour
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Step(0.125)
		if m.Event() != nil {
			m.Respond(1)
		}
		m.Items()
		m.Shafts()
		m.People()
	}
	b.ReportMetric(float64(len(m.world().Sims)), "sims")
}

func TestSimModelFire(t *testing.T) {
	m := buildTestTower(t)
	for i := 0; i < 8*20 && m.Event() == nil; i++ {
		m.Step(0.125) // into the morning: offices fill
	}
	w := m.world()
	w.StarCount = 3
	sim.TryTriggerFireEvent(w, m.ts.Ledger(), m.ts.Time())
	if w.EventState.GameStateFlags&evFlagFire == 0 {
		t.Skip("fire did not start (no eligible floor)")
	}
	fireFloor := uiFloorOfSim(w.EventState.FireFloor)
	sawBurning, sawPrompt := false, false
	m.SetSpeed(2)
	for i := 0; i < 8*120 && w.EventState.GameStateFlags&evFlagFire != 0; i++ {
		m.Step(0.125)
		for _, it := range m.Items() {
			if it.State == StateBurning {
				sawBurning = true
			}
		}
		if ev := m.Event(); ev != nil {
			if ev.Kind == EventFire {
				sawPrompt = true
				if ev.Floor != fireFloor || len(ev.Choices) != 2 {
					t.Fatalf("fire prompt %+v, fire on %d", ev, fireFloor)
				}
			}
			m.Respond(1) // let security fight it
		}
	}
	if !sawBurning || !sawPrompt {
		t.Fatalf("burning=%v prompt=%v", sawBurning, sawPrompt)
	}
	burned := 0
	for _, it := range m.Items() {
		if it.State == StateBurned {
			burned++
			if r := m.Remove(it.X, it.Floor); !r.OK {
				t.Fatalf("bulldozing a burned area: %+v", r)
			}
			break
		}
	}
	t.Logf("burned areas: %d (fire flags now %#x)", burned, w.EventState.GameStateFlags)
}
