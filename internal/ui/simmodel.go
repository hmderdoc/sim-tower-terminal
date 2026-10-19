package ui

import (
	"math"
	"strconv"
	"time"

	"simtower/internal/sim"
)

// SimModel implements Model over the real simulation (*sim.TowerSim). It is
// the only place the UI meets internal/sim: reads go straight to the live
// world through the facade's accessors, commands go through SubmitCommand.
// See docs/ui_notes.md "Sim adapter" for the mapping rules.
//
// Coordinates: the sim uses grid rows y (0 = top) and internal floors
// 0..119 with 10 = 1F; the UI uses floor index 0 = 1F, -1 = B1:
//
//	uiFloor = simFloor - 10,  y = 109 - uiFloor
type SimModel struct {
	ts   *sim.TowerSim
	path string // save file ("" = never written)

	speed int
	acc   float64 // fractional ticks owed to the sim
	frac  float64 // acc after the last Step, for car interpolation
	// carPrev is each car's effective floor before the last sim tick.
	carPrev map[*sim.CarrierCar]float64

	// identity maps (adapter-owned, stable across sim rebuilds)
	shaftIDs  map[*sim.CarrierRecord]int
	nextShaft int
	personIDs map[string]int // SimKey -> person ID
	personKey map[int]string
	nextPers  int

	// per-frame caches, rebuilt by refresh()
	dirty       bool
	lastEvFlags int
	noBurn      bool // rebuilding after our own command: removals are not fire damage
	items       []Item
	itemRec     map[int]itemRef // item ID -> anchor
	floors      []FloorSpan
	shafts      []Shaft
	shaftRec    map[int]*sim.CarrierRecord
	people      []Person
	simByKey    map[string]*sim.SimRecord
	simsByObj   map[int][]*sim.SimRecord // item ID -> sims
	burned      map[int]Item             // destroyed by fire/bomb, until rebuilt or bulldozed
	lastItems   map[int]Item
	refreshed   int // sim tick of the last refresh
	refreshSet  bool

	// UI-side state that the sim does not keep (persisted in the extras file)
	names        map[string]string // SimKey -> custom name
	construction int64             // construction spending since the last ledger rollover
	msgs         []Message
	event        *Event
	eventPrompt  *sim.SimPrompt // the sim prompt behind event (nil: local info popup)
	prompts      []sim.SimPrompt
	infoQueue    []*Event
	lastQuarter  int
	lastMsgText  string
	lastMsgDay   int
	saveErr      error
	stepBudget   time.Duration
}

type itemRef struct {
	key  string // placed-object anchor key ("x,y"), "" for overlays/lobby runs
	tile string
	x, y int // anchor cell
}

// Tick cadence (docs/ui_notes.md "Sim adapter / time"): the upstream
// tower-together server and client both run the sim at 50 ms per tick at
// 1x (TowerRoom.startTick, lockstepSession BASE_TICK_INTERVAL_MS), with 3x
// and 10x speed multipliers. One game day (2600 ticks) is then 2 min 10 s.
// The non-linear tick -> clock table makes the clock crawl through the
// lunch rush and race through the night, as the manual describes.
const (
	SimTicksPerSecond = 20
	SimFastMultiplier = 10
	// maxTicksPerStep bounds the catch-up after a stall (one second of
	// fast mode); anything beyond is dropped rather than queued.
	maxTicksPerStep = SimTicksPerSecond * SimFastMultiplier
)

// SimLobbyMode is the lobby cadence new towers use: the binary's
// 1F + 15F/30F/... sky lobbies (internal floors 24, 39, ...).
const SimLobbyMode = sim.LobbyModePerfectParity

// NewSimModel starts a new tower. path is where Save writes ("" = nowhere).
func NewSimModel(towerID, name, path string) *SimModel {
	ts := sim.Create(towerID, name, SimLobbyMode)
	return newSimModel(ts, path)
}

func newSimModel(ts *sim.TowerSim, path string) *SimModel {
	// Costs are the verified gameplay catalogue (sim/catalog.go), charged by
	// the adapter; the sim's own TS cost tables stay out of the way.
	ts.FreeBuild = true
	m := &SimModel{ts: ts, path: path, speed: 1,
		carPrev:    map[*sim.CarrierCar]float64{},
		shaftIDs:   map[*sim.CarrierRecord]int{},
		personIDs:  map[string]int{},
		personKey:  map[int]string{},
		itemRec:    map[int]itemRef{},
		shaftRec:   map[int]*sim.CarrierRecord{},
		simByKey:   map[string]*sim.SimRecord{},
		simsByObj:  map[int][]*sim.SimRecord{},
		burned:     map[int]Item{},
		lastItems:  map[int]Item{},
		names:      map[string]string{},
		dirty:      true,
		stepBudget: 70 * time.Millisecond,
	}
	m.lastQuarter = m.quarterIndex()
	return m
}

// Sim exposes the wrapped simulation (tests, tools).
func (m *SimModel) Sim() *sim.TowerSim { return m.ts }

// ---- coordinate helpers ----

func uiFloorOfSim(simFloor int) int { return simFloor - 10 }
func simFloorOfUI(f int) int        { return f + 10 }
func yOfUI(f int) int               { return sim.GroundY - f }
func uiFloorOfY(y int) int          { return sim.GroundY - y }

func ckey(x, y int) string { return strconv.Itoa(x) + "," + strconv.Itoa(y) }

func parseXY(k string) (int, int) {
	x, y, i := 0, 0, 0
	neg := false
	for ; i < len(k) && k[i] != ','; i++ {
		if k[i] == '-' {
			neg = true
			continue
		}
		x = x*10 + int(k[i]-'0')
	}
	if neg {
		x = -x
	}
	neg = false
	for i++; i < len(k); i++ {
		if k[i] == '-' {
			neg = true
			continue
		}
		y = y*10 + int(k[i]-'0')
	}
	if neg {
		y = -y
	}
	return x, y
}

// itemID is stable for a given anchor cell.
func itemID(x, y int) int { return 1 + y*1024 + x }

// ---- reads ----

func (m *SimModel) Bounds() Bounds {
	return Bounds{MinFloor: -sim.UndergroundFloors, MaxFloor: sim.GridHeight - 1 - sim.UndergroundFloors, Width: sim.GridWidth}
}

func (m *SimModel) Catalog() []BuildDef { return simCatalog }

func (m *SimModel) Floors() []FloorSpan    { m.refresh(); return m.floors }
func (m *SimModel) Items() []Item          { m.refresh(); return m.items }
func (m *SimModel) Shafts() []Shaft        { m.refresh(); return m.shafts }
func (m *SimModel) People() []Person       { m.refresh(); return m.people }
func (m *SimModel) Funds() int64           { return int64(m.ts.Cash()) }
func (m *SimModel) Population() int        { return m.ts.CurrentPopulation() }
func (m *SimModel) Stars() int             { return clampInt(m.ts.StarCount(), 1, 6) }
func (m *SimModel) Speed() int             { return m.speed }
func (m *SimModel) Messages() []Message    { return m.msgs }
func (m *SimModel) Event() *Event          { return m.event }
func (m *SimModel) SetSpeed(n int)         { m.speed = clampInt(n, 0, 2) }
func (m *SimModel) SaveError() error       { return m.saveErr }
func (m *SimModel) touch()                 { m.dirty = true }
func (m *SimModel) quarterIndex() int      { return m.ts.Time().DayCounter / 3 }
func (m *SimModel) world() *sim.WorldState { return m.ts.World() }

// ---- stepping ----

// Step advances the sim by dt real seconds at the current speed. It never
// steps while a prompt is open (the original blocks on its dialogs).
func (m *SimModel) Step(dt float64) {
	if m.event == nil {
		m.popEvent()
	}
	if m.speed == 0 || m.event != nil || dt <= 0 {
		m.frac = 0
		return
	}
	rate := float64(SimTicksPerSecond)
	if m.speed == 2 {
		rate *= SimFastMultiplier
	}
	m.acc += dt * rate
	n := int(m.acc)
	if n > maxTicksPerStep {
		n = maxTicksPerStep
		m.acc = float64(n)
	}
	start := time.Now()
	done := 0
	for done < n {
		m.snapCars()
		res := m.ts.Step()
		done++
		m.afterTick(res)
		if m.event != nil {
			m.acc = 0
			break
		}
		if done%8 == 0 && time.Since(start) > m.stepBudget {
			m.acc = 0 // machine cannot keep up: drop the backlog
			break
		}
	}
	m.acc -= float64(done)
	if m.acc < 0 {
		m.acc = 0
	}
	m.frac = m.acc
	if m.frac > 1 {
		m.frac = 1
	}
	if done > 0 {
		m.dirty = true
	}
}

// snapCars records every car's effective floor before a tick.
func (m *SimModel) snapCars() {
	for _, c := range m.ts.LiveCarriers() {
		for _, car := range c.Cars {
			m.carPrev[car] = carEffective(car)
		}
	}
}

// carEffective is the car's position in floors, counting the settle
// countdown as the last sixth-steps into the floor it just reached (the
// upstream client's interpolatedFloor rule).
func carEffective(car *sim.CarrierCar) float64 {
	sign := -1.0
	if car.PrevFloor > car.CurrentFloor {
		sign = 1
	}
	return float64(car.CurrentFloor) + sign*float64(car.SettleCounter)/6
}

func (m *SimModel) afterTick(res sim.StepResult) {
	for _, n := range res.Notifications {
		m.notify(n)
	}
	for _, p := range res.Prompts {
		m.prompts = append(m.prompts, p)
	}
	if m.event == nil {
		m.popEvent()
	}
	t := m.ts.Time()
	if t.DayTick == sim.NewGameDayTick && t.DayCounter%3 == 0 {
		// ledger rollover just ran: a new finance quarter begins
		m.construction = 0
	}
	if q := m.quarterIndex(); q != m.lastQuarter {
		m.lastQuarter = q
		if m.path != "" {
			if err := m.Save(); err != nil {
				m.msg("Autosave failed: "+err.Error(), true)
			}
		}
	}
}

// ---- messages and prompts ----

func (m *SimModel) whenString() string {
	c := m.Clock()
	return "Y" + itoa(c.Year) + " Q" + itoa(c.Quarter) + " " + c.DayLabel + " " + c.TimeString()
}

func (m *SimModel) msg(s string, alert bool) {
	s = asciiOnly(s)
	day := m.ts.Time().DayCounter
	if s == m.lastMsgText && day == m.lastMsgDay {
		return // the sim repeats demand notices every few ticks
	}
	m.lastMsgText, m.lastMsgDay = s, day
	m.msgs = append(m.msgs, Message{Text: s, When: m.whenString(), Alert: alert})
	if len(m.msgs) > 200 {
		m.msgs = append(m.msgs[:0], m.msgs[len(m.msgs)-200:]...)
	}
}

func (m *SimModel) notify(n sim.StepNotification) {
	switch n.Kind {
	case sim.NotificationRouteFailure:
		m.msg(n.Message, true)
	case sim.NotificationStarAdvanced:
		st, _ := strconv.Atoi(n.Message)
		label := itoa(st) + " stars"
		if st >= 6 {
			label = "TOWER status"
		}
		m.msg("Your tower has been rated "+label+"!", false)
		m.infoQueue = append(m.infoQueue, &Event{Kind: EventInfo, Title: "CONGRATULATIONS", Floor: -100,
			Lines:   []string{"Your tower has been rated " + label + "!", "New facilities are available in the build menu."},
			Choices: []string{"OK"}})
	default:
		if n.Message == "" || n.Message == "metro_train_arrival" {
			return // cosmetic (train sprite toggle)
		}
		m.msg(n.Message, false)
	}
}

// popEvent turns the next queued sim prompt (or info popup) into Event.
func (m *SimModel) popEvent() {
	if len(m.prompts) > 0 {
		p := m.prompts[0]
		m.prompts = m.prompts[1:]
		m.eventPrompt = &p
		m.event = m.promptEvent(&p)
		m.touch()
		return
	}
	if len(m.infoQueue) > 0 {
		m.event = m.infoQueue[0]
		m.infoQueue = m.infoQueue[1:]
		m.eventPrompt = nil
	}
}

func (m *SimModel) promptEvent(p *sim.SimPrompt) *Event {
	es := &m.world().EventState
	ev := &Event{Kind: EventInfo, Title: "NOTICE", Floor: -100, Lines: wrapText(asciiOnly(p.Message), 54)}
	cost := ""
	if p.Cost != nil {
		cost = " " + money(int64(*p.Cost))
	}
	switch p.PromptKind {
	case sim.PromptBombRansom:
		ev.Kind, ev.Title = EventBomb, "BOMB THREAT"
		ev.Floor, ev.X = uiFloorOfSim(es.BombFloor), es.BombTile
		ev.Lines = append(ev.Lines, "Refuse and security will search the tower.")
		ev.Choices = []string{"PAY" + cost, "REFUSE"}
	case sim.PromptFireRescue:
		ev.Kind, ev.Title = EventFire, "FIRE!"
		ev.Floor, ev.X = uiFloorOfSim(es.FireFloor), es.FireTile
		ev.Lines = append(ev.Lines, "Otherwise security fights it from the emergency stairs.")
		ev.Choices = []string{"CALL RESCUE" + cost, "WAIT"}
	case sim.PromptCarrierEditConfirmation:
		ev.Title = "ELEVATOR"
		ev.Choices = []string{"REMOVE CAR", "CANCEL"}
	default:
		ev.Choices = []string{"YES", "NO"}
	}
	return ev
}

// Respond answers the open prompt; choice 0 accepts.
func (m *SimModel) Respond(choice int) {
	if m.event == nil {
		return
	}
	if p := m.eventPrompt; p != nil {
		m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdPromptResponse, PromptID: p.PromptID, Accepted: choice == 0})
		m.drainAfterCommand()
	}
	m.event, m.eventPrompt = nil, nil
	m.touch()
	m.popEvent()
}

// drainAfterCommand collects notifications/prompts a command queued.
func (m *SimModel) drainAfterCommand() {
	for _, n := range m.ts.DrainNotifications() {
		m.notify(n)
	}
	m.prompts = append(m.prompts, m.ts.DrainPrompts()...)
	if m.event == nil {
		m.popEvent()
	}
}

// ---- clock ----

// ClockOfTick converts a day tick to the original's clock (EXE
// convert_simulation_tick_to_clock_time; docs/mechanics.md section 4).
// Returns minutes since midnight (0..1439).
func ClockOfTick(dayTick int) int {
	var base, mins float64
	t := float64(dayTick)
	switch {
	case dayTick < 400:
		base, mins = 7*60, t*0.75
	case dayTick < 800:
		base, mins = 12*60, (t-400)*0.075
	case dayTick < 1200:
		base, mins = 12*60+30, (t-800)*0.075
	case dayTick < 1600:
		base, mins = 13*60, (t-1200)*0.6
	case dayTick < 2000:
		base, mins = 17*60, (t-1600)*0.6
	case dayTick < 2400:
		base, mins = 21*60, (t-2000)*0.6
	default:
		base, mins = 1*60, (t-2400)*1.8
	}
	v := int(math.Floor(base + mins))
	return ((v % 1440) + 1440) % 1440
}

func (m *SimModel) Clock() Clock {
	t := m.ts.Time()
	mins := ClockOfTick(t.DayTick)
	c := Clock{Day: t.DayCounter, Tick: t.DayTick, Hour: mins / 60, Minute: mins % 60}
	c.DayLabel = [3]string{"WD1", "WD2", "WE"}[t.DayCounter%3]
	c.Weekend = t.WeekendFlag != 0
	c.Quarter = (t.DayCounter/3)%4 + 1
	c.Year = t.DayCounter/12 + 1
	switch {
	case c.Hour >= 6 && c.Hour < 17:
		c.Light = Day
	case c.Hour >= 17 && c.Hour < 19:
		c.Light = Dusk
	default:
		c.Light = Night
	}
	return c
}

// ---- text helpers ----

func asciiOnly(s string) string {
	ok := true
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			ok = false
			break
		}
	}
	if ok {
		return s
	}
	b := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7e:
			b = append(b, byte(r))
		case r == 0x2018 || r == 0x2019:
			b = append(b, '\'')
		case r == 0x201c || r == 0x201d:
			b = append(b, '"')
		case r == 0x2013 || r == 0x2014:
			b = append(b, '-')
		default:
			b = append(b, '?')
		}
	}
	return string(b)
}

func wrapText(s string, w int) []string {
	var out []string
	for len(s) > w {
		cut := w
		for cut > 0 && s[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = w
		}
		out = append(out, s[:cut])
		s = s[cut:]
		for len(s) > 0 && s[0] == ' ' {
			s = s[1:]
		}
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}
