package ui

import (
	"math"
	"strconv"
)

// DemoModel implements Model with the tower from the mock's TOWER object
// (docs/graphics_plan.html) and the mock's scripted motion: cars cycling
// their shafts, lobby walkers, rooms changing state with the clock. It
// accepts build commands so the sysop can try the tools, and raises a fire
// and a bomb prompt so the event popups can be seen. All report data that
// only the real sim can provide is marked DEMO.
type DemoModel struct {
	t        float64 // the mock's animation clock (4 per second at speed 1)
	minute   float64 // minutes since midnight
	day      int
	speed    int
	funds    int64
	pop      int
	stars    int
	nextID   int
	lastHour int

	spans   map[int][2]int
	items   []demoItem
	shafts  []*demoShaft
	walkers int

	itemsOut  []Item
	shaftsOut []Shaft
	floorsOut []FloorSpan
	peopleOut []Person
	msgs      []Message
	event     *Event
	fire      struct {
		active   bool
		rescued  bool
		ticks    float64
		itemIdx  []int
		spreadTo int
	}
	names     map[int]string
	saveCount int
}

type demoItem struct {
	Item
	st func(d *DemoModel) State // nil: fixed State
}

type demoShaft struct {
	id, x, bottom, top int
	kind               Kind
	phase, load        int
	queues             []Queue
	served             []bool
	modes              [2][7]PeriodMode
	response, depart   int
	extra              []Car // added cars (idle at home)
	home0              int
}

// tod is the mock's time-of-day string.
func (d *DemoModel) tod() string {
	c := d.Clock()
	switch {
	case c.Rain:
		return "rain"
	case c.Light == Night:
		return "night"
	case c.Light == Dusk:
		return "dusk"
	}
	return "day"
}

func (d *DemoModel) ti() int { return int(d.t) }

// NewDemo builds the mock tower at 10:42 AM, WD1 Q2 Y1.
func NewDemo() *DemoModel {
	d := &DemoModel{minute: 10*60 + 42, day: 3, speed: 1, funds: 1732500, pop: 412, stars: 2,
		lastHour: 10, names: map[int]string{}}
	d.spans = map[int][2]int{0: {0, 128}, 1: {0, 128}, 2: {0, 128}, 3: {0, 128}, 4: {0, 96}, 5: {0, 96},
		6: {0, 96}, 7: {0, 80}, 8: {0, 80}, 9: {0, 80}, 10: {0, 64}, 11: {0, 64}, -1: {0, 128}, -2: {0, 128}, -3: {0, 128}, -4: {0, 64}}
	night := func(d *DemoModel) bool { return d.tod() == "night" }
	lit := func(base State) func(d *DemoModel) State {
		return func(d *DemoModel) State { return base }
	}
	_ = lit
	add := func(k Kind, x, f int, st State, fn func(d *DemoModel) State) {
		w, h := TileSize(k)
		d.nextID++
		it := demoItem{Item: Item{ID: d.nextID, Kind: k, X: x, Floor: f, W: w, H: h / 3, State: st, Rent: -1}, st: fn}
		d.items = append(d.items, it)
	}
	officeLit := func(d *DemoModel) State { return StateOccupied } // Lit set in refresh
	add(KindLobby, 0, 0, StateIdle, nil)
	d.items[0].W, d.items[0].H = 128, 1
	add(KindOffice, 0, 1, StateOccupied, officeLit)
	add(KindOffice, 9, 1, StateVacant, nil)
	add(KindFastFood, 28, 1, StateOpen, func(d *DemoModel) State {
		if night(d) {
			return StateClosed
		}
		if d.ti()%40 < 20 {
			return StateBusy
		}
		return StateOpen
	})
	retail := func(d *DemoModel) State {
		if night(d) {
			return StateClosed
		}
		return StateOpen
	}
	add(KindRetail, 44, 1, StateOpen, retail)
	add(KindStairs, 56, 1, StateActive, nil)
	add(KindHotelSingle, 64, 1, StateVacant, nil)
	add(KindHotelTwin, 68, 1, StateDirty, nil)
	add(KindCondo, 0, 2, StateOccupied, officeLit)
	add(KindHotelSingle, 16, 2, StateAsleep, nil)
	add(KindHotelSingle, 20, 2, StateOccupied, nil)
	add(KindHotelSuite, 28, 2, StateVIP, nil)
	add(KindHotelTwin, 38, 2, StateAsleep, nil)
	add(KindHousekeeping, 44, 2, StateIdle, nil)
	add(KindSecurity, 59, 2, StateIdle, nil)
	add(KindOffice, 0, 3, StateOccupied, officeLit)
	add(KindOffice, 9, 3, StateOccupied, nil)
	add(KindRestaurant, 28, 3, StateClosed, func(d *DemoModel) State {
		if t := d.tod(); t == "day" || t == "rain" {
			return StateClosed
		}
		return StateOpen
	})
	add(KindOffice, 52, 3, StateOccupied, nil)
	add(KindOffice, 61, 3, StateOccupied, nil)
	add(KindCondo, 0, 4, StateForSale, nil)
	add(KindCinema, 28, 4, StateActive, func(d *DemoModel) State {
		if d.ti()%60 < 40 {
			return StateActive
		}
		return StateIdle
	})
	add(KindEscalator, 60, 4, StateActive, nil)
	add(KindCondo, 0, 5, StateOccupied, officeLit)
	add(KindParkingRamp, 0, -1, StateIdle, nil)
	for i, x := range []int{28, 32, 36, 40, 44, 48} {
		st := StateOccupied
		if i == 2 || i == 5 {
			st = StateVacant
		}
		add(KindParking, x, -1, st, nil)
	}
	// wide-only content
	add(KindOffice, 82, 1, StateOccupied, nil)
	add(KindOffice, 91, 1, StateOccupied, nil)
	add(KindRetail, 104, 1, StateOpen, nil)
	add(KindRetail, 116, 1, StateOpen, nil)
	add(KindHotelSuite, 82, 2, StateOccupied, nil)
	add(KindHotelTwin, 92, 2, StateVacant, nil)
	add(KindHotelSingle, 104, 2, StateAsleep, nil)
	add(KindHotelSingle, 108, 2, StateVacant, nil)
	add(KindMedical, 102, 3, StateIdle, nil)
	add(KindOffice, 82, 3, StateOccupied, nil)
	add(KindPartyHall, 60, 5, StateActive, func(d *DemoModel) State {
		if d.ti()%50 < 30 {
			return StateActive
		}
		return StateIdle
	})
	add(KindCondo, 0, 6, StateOccupied, nil)
	add(KindCondo, 0, 7, StateOccupied, nil)
	add(KindOffice, 28, 7, StateOccupied, nil)
	add(KindOffice, 37, 7, StateOccupied, nil)
	add(KindOffice, 46, 7, StateVacant, nil)
	add(KindOffice, 28, 8, StateOccupied, nil)
	add(KindOffice, 37, 8, StateOccupied, nil)
	add(KindCondo, 0, 8, StateForSale, nil)
	add(KindStairs, 60, 7, StateIdle, nil)
	add(KindCondo, 0, 9, StateOccupied, nil)
	add(KindOffice, 28, 9, StateOccupied, nil)
	add(KindOffice, 37, 9, StateOccupied, nil)
	add(KindOffice, 28, 10, StateOccupied, nil)
	add(KindOffice, 0, 10, StateOccupied, nil)
	add(KindOffice, 9, 10, StateOccupied, nil)
	add(KindMetro, 28, -4, StateActive, func(d *DemoModel) State {
		if d.ti()%30 < 16 {
			return StateActive
		}
		return StateIdle
	})
	add(KindRecycling, 82, -2, StateIdle, nil)
	for i, x := range []int{60, 64, 68} {
		st := StateOccupied
		if i == 2 {
			st = StateVacant
		}
		add(KindParking, x, -1, st, nil)
	}
	// plausible evals, rents, occupants
	for i := range d.items {
		it := &d.items[i].Item
		switch it.Kind {
		case KindOffice, KindCondo, KindHotelSingle, KindHotelTwin, KindHotelSuite, KindFastFood, KindRestaurant, KindRetail:
			it.Eval = []Eval{EvalBlue, EvalBlue, EvalYellow, EvalBlue, EvalRed, EvalYellow}[i%6]
			it.Rent = []int{1, 1, 0, 1, 2, 3, 1}[i%7]
			if it.State == StateVacant || it.State == StateForSale {
				it.Eval = EvalNone
			}
		case KindCinema, KindPartyHall:
			it.Eval = EvalYellow
			it.Rent = 1
		}
		switch it.Kind {
		case KindOffice:
			it.Capacity = 6
		case KindCondo:
			it.Capacity = 3
		case KindHotelSingle:
			it.Capacity = 1
		case KindHotelTwin, KindHotelSuite:
			it.Capacity = 2
		}
		if it.State != StateVacant && it.State != StateForSale {
			it.Occupants = it.Capacity
		}
		if it.Kind == KindFastFood {
			it.Name, it.Customers = "Hamburger", 37
		}
		if it.Kind == KindRestaurant {
			it.Name, it.Customers = "Steak House", 22
		}
		if it.Kind == KindRetail {
			it.Name, it.Customers = "Book Store", 41
		}
	}
	mk := func(x, bottom, top int, k Kind, phase, load int, qs []Queue) {
		s := &demoShaft{id: len(d.shafts) + 1, x: x, bottom: bottom, top: top, kind: k, phase: phase, load: load,
			queues: qs, response: 5, depart: 0, home0: bottom}
		if bottom <= 0 && top >= 0 {
			s.home0 = 0
		}
		s.served = make([]bool, top-bottom+1)
		for i := range s.served {
			s.served[i] = true
		}
		d.shafts = append(d.shafts, s)
	}
	R, P, N := StressRed, StressPink, StressNormal
	mk(24, -1, 6, KindElevator, 0, 9, []Queue{{Floor: 1, Up: []Stress{R, P, N}, Down: []Stress{N}}, {Floor: 3, Up: []Stress{N}, Down: []Stress{N, N}}})
	mk(76, -3, 11, KindExpress, 9, 31, []Queue{{Floor: 0, Up: []Stress{N, N, N, N}}})
	mk(100, -1, 11, KindElevator, 20, 4, []Queue{{Floor: 5, Up: []Stress{N, N}}})
	d.shafts[0].served[5-(-1)] = false // 6F skipped (elevator window mock)
	d.shafts[0].modes[0][0] = ModeExpressUp
	d.shafts[0].modes[0][4] = ModeExpressDown
	d.walkers = 14
	d.msgs = []Message{
		{Text: "Fast food on 2F: Business is average.", When: "Y1 Q2 WD1 10:31 AM"},
		{Text: "Office workers demand Parking.", When: "Y1 Q2 WD1 10:40 AM"},
	}
	d.refresh()
	return d
}

// ---- reads ----

func (d *DemoModel) Bounds() Bounds { return Bounds{MinFloor: -10, MaxFloor: 99, Width: 375} }

var demoCatalog = []BuildDef{
	{KindLobby, "Lobby", 'L', 5000, true, 4, 1, 1, "Lobbies go on 1F and every 15th floor."},
	{KindOffice, "Office", 'O', 40000, false, 9, 1, 1, "Workers need a lobby route."},
	{KindCondo, "Condo", 'C', 80000, false, 16, 1, 1, "Sold once; quiet neighbors."},
	{KindFastFood, "Fast food", 'F', 100000, false, 16, 1, 1, "Busy at lunch."},
	{KindElevator, "Elevator", 'E', 200000, false, 4, 1, 1, "Drag to set the height (max 30 floors)."},
	{KindStairs, "Stairs", 'A', 5000, false, 8, 2, 1, "Links two floors."},
	{KindHotelSingle, "Hotel single", 'H', 20000, false, 4, 1, 2, "Needs housekeeping."},
	{KindHousekeeping, "Housekeeping", 'K', 50000, false, 15, 1, 2, "Cleans hotel rooms."},
	{KindSecurity, "Security", 'S', 100000, false, 16, 1, 2, "Needed for 3 stars."},
	{KindHotelTwin, "Hotel twin", 'T', 50000, false, 6, 1, 3, "Two guests."},
	{KindHotelSuite, "Hotel suite", 'U', 100000, false, 10, 1, 3, "VIPs stay here."},
	{KindRestaurant, "Restaurant", 'R', 200000, false, 24, 1, 3, "Busy in the evening."},
	{KindRetail, "Retail shop", 'P', 100000, false, 12, 1, 3, "Pays rent quarterly."},
	{KindCinema, "Cinema", 'N', 500000, false, 31, 2, 3, "Shows a movie daily."},
	{KindPartyHall, "Party hall", 'Y', 100000, false, 24, 2, 3, "Pays when booked."},
	{KindExpress, "Express elev", 'X', 400000, false, 6, 1, 3, "Stops at lobbies and basements."},
	{KindFloor, "Floor", 'W', 500, true, 1, 1, 1, "Empty floor area; drag to extend."},
	{KindService, "Service elev", 'V', 100000, false, 4, 1, 2, "Housekeepers only."},
	{KindEscalator, "Escalator", 'G', 20000, false, 8, 2, 3, "Moves crowds between shops."},
	{KindMedical, "Medical", 'M', 500000, false, 26, 1, 3, "Demanded near the lobby."},
	{KindParking, "Parking", 'J', 3000, false, 4, 1, 3, "Underground; needs a ramp."},
	{KindParkingRamp, "Parking ramp", 'Q', 50000, false, 15, 1, 3, "Underground; one per tower."},
	{KindRecycling, "Recycling", 'Z', 500000, false, 25, 2, 3, "Underground; 4 stars need it."},
	{KindMetro, "Metro", 'I', 1000000, false, 30, 3, 4, "Lowest basement floors."},
	{KindCathedral, "Cathedral", 'D', 3000000, false, 28, 4, 5, "Top floor; weddings."},
	{KindBulldozer, "Bulldozer", 'B', 0, false, 1, 1, 1, "Demolish; no refund."},
}

func (d *DemoModel) Catalog() []BuildDef { return demoCatalog }

func (d *DemoModel) Floors() []FloorSpan {
	d.floorsOut = d.floorsOut[:0]
	for f := -10; f <= 99; f++ {
		if s, ok := d.spans[f]; ok {
			d.floorsOut = append(d.floorsOut, FloorSpan{Floor: f, Left: s[0], Right: s[1]})
		}
	}
	return d.floorsOut
}

func (d *DemoModel) Items() []Item { return d.itemsOut }

func (d *DemoModel) Shafts() []Shaft { return d.shaftsOut }

func (d *DemoModel) People() []Person { return d.peopleOut }

func (d *DemoModel) Clock() Clock {
	m := int(d.minute)
	c := Clock{Day: d.day, Tick: m, Hour: m / 60, Minute: m % 60}
	c.DayLabel = [3]string{"WD1", "WD2", "WE"}[d.day%3]
	c.Weekend = d.day%3 == 2
	c.Quarter = (d.day/3)%4 + 1
	c.Year = d.day/12 + 1
	switch {
	case c.Hour >= 6 && c.Hour < 17:
		c.Light = Day
	case c.Hour >= 17 && c.Hour < 19:
		c.Light = Dusk
	default:
		c.Light = Night
	}
	c.Rain = d.day%4 == 1 && c.Hour >= 13 && c.Hour < 16
	return c
}

func (d *DemoModel) Funds() int64        { return d.funds }
func (d *DemoModel) Population() int     { return d.pop }
func (d *DemoModel) Stars() int          { return d.stars }
func (d *DemoModel) Speed() int          { return d.speed }
func (d *DemoModel) Messages() []Message { return d.msgs }
func (d *DemoModel) Event() *Event       { return d.event }

func (d *DemoModel) Finance() Finance {
	return Finance{
		Quarter: "Y1 Q2 (DEMO)",
		Income: []Line{{"Office", 180000}, {"Single room", 24000}, {"Twin room", 18000}, {"Suite", 36000},
			{"Shop", 45000}, {"Fast food", 9000}, {"Restaurant", 0}, {"Party hall", 60000}, {"Cinema", 30000}, {"Condominium", 300000}},
		Maintenance: []Line{{"Lobby", 0}, {"Elevator", 20000}, {"Express elevator", 20000}, {"Service elevator", 0},
			{"Escalator", 5000}, {"Parking ramp", 10000}, {"Recycling center", 50000}, {"Metro station", 100000},
			{"Housekeeping", 10000}, {"Security", 20000}},
		OtherIncome:  0,
		Construction: 435000,
		PrevBalance:  1525000,
		Balance:      d.funds,
	}
}

func (d *DemoModel) Rating() Rating {
	return Rating{Stars: d.stars, Population: d.pop, Next: "3 stars at population 1,000 (DEMO)",
		Needs: []string{"Population 1,000", "A security office (built)", "DEMO: requirements come from the sim"}}
}

func (d *DemoModel) find(id int) *demoItem {
	for i := range d.items {
		if d.items[i].ID == id {
			return &d.items[i]
		}
	}
	return nil
}

var tierTables = map[Kind][4]int64{
	KindOffice: {15000, 10000, 5000, 2000}, KindRetail: {20000, 15000, 10000, 4000},
	KindHotelSingle: {3000, 2000, 1500, 500}, KindHotelTwin: {4500, 3000, 2000, 800},
	KindHotelSuite: {9000, 6000, 4000, 1500}, KindCondo: {200000, 150000, 100000, 40000},
	KindFastFood: {5000, 3000, 2000, -3000}, KindRestaurant: {10000, 6000, 4000, -6000},
	KindCinema: {15000, 10000, 2000, 0},
}

func kindName(k Kind) string {
	if td := tiles[k]; td != nil {
		return td.name
	}
	for _, c := range demoCatalog {
		if c.Kind == k {
			return c.Name
		}
	}
	return "Facility"
}

func (d *DemoModel) Inspect(id int) InspectInfo {
	di := d.find(id)
	if di == nil {
		return InspectInfo{ID: id, Title: "Gone", Rent: -1, Demo: true}
	}
	it := &di.Item
	in := InspectInfo{ID: id, Kind: it.Kind, Title: kindName(it.Kind) + ", Floor " + floorLabel(it.Floor),
		Eval: it.Eval, Rent: it.Rent, Demo: true, Length: strconv.Itoa(2+id%9) + " days"}
	switch it.State {
	case StateVacant:
		in.Status = "For Rent"
	case StateForSale:
		in.Status = "For Sale"
	case StateClosed:
		in.Status = "Closed"
	case StateOpen, StateBusy:
		in.Status = "Open"
	case StateDirty:
		in.Status = "Needs cleaning"
	case StateAsleep:
		in.Status = "Guest asleep"
	case StateBurning:
		in.Status = "ON FIRE"
	case StateBurned:
		in.Status = "Burned out"
	default:
		in.Status = "Occupied"
	}
	in.EvalText = [4]string{"Not rated", "Excellent", "Good", "Terrible"}[it.Eval]
	in.Message = [4]string{"", "Business is good.", "Business is average.", "Stairs/Elevators are too far away."}[it.Eval]
	if it.Kind == KindFastFood || it.Kind == KindRestaurant {
		in.ShowCount, in.Customers = true, it.Customers
		in.Income = "Daily sales " + money(int64(it.Customers)*100) + " (DEMO)"
	}
	for i := 0; i < it.Occupants; i++ {
		in.Occupants = append(in.Occupants, 5000+id*10+i)
	}
	if tt, ok := tierTables[it.Kind]; ok && it.Rent >= 0 {
		for _, n := range tt {
			in.Tiers = append(in.Tiers, RentTier{Label: money(n), Amount: n})
		}
	} else {
		in.Rent = -1
	}
	if it.Kind == KindCinema {
		in.Movie = "\"Tower of Terror\" (classic), day 3 of its run (DEMO)"
	}
	if in.Message == "" {
		in.Message = "DEMO: facility messages come from the sim."
	}
	return in
}

func (d *DemoModel) shaft(id int) *demoShaft {
	for _, s := range d.shafts {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (d *DemoModel) Schedule(id int) Schedule {
	s := d.shaft(id)
	if s == nil {
		return Schedule{ShaftID: id}
	}
	n := 0
	for _, o := range d.shafts {
		if o.kind == s.kind {
			n++
		}
		if o == s {
			break
		}
	}
	title, capy, cost := "STANDARD ELEVATOR #", 21, int64(80000)
	switch s.kind {
	case KindExpress:
		title, capy, cost = "EXPRESS ELEVATOR #", 42, 150000
	case KindService:
		title, capy, cost = "SERVICE ELEVATOR #", 21, 50000
	}
	return Schedule{ShaftID: id, Title: title + strconv.Itoa(n), Served: s.served, Modes: s.modes,
		Response: s.response, Departure: s.depart, Capacity: capy, CarCost: cost}
}

var demoNames = []string{"Office worker", "Hotel guest", "Shopper", "Condo resident", "Diner", "Visitor", "Movie goer"}

func (d *DemoModel) Tenant(id int) TenantInfo {
	ti := TenantInfo{ID: id, Name: demoNames[id%len(demoNames)] + " #" + strconv.Itoa(id), Demo: true,
		From: "Office, Floor 2F", GoingTo: "Lobby, Floor 1F", Stress: Stress(id % 3), Eval: Eval(1 + id%3)}
	if n, ok := d.names[id]; ok {
		ti.Name = n
	}
	if id >= 5000 {
		if it := d.find((id - 5000) / 10); it != nil {
			ti.From = kindName(it.Kind) + ", Floor " + floorLabel(it.Floor)
			ti.Eval = it.Eval
		}
	}
	return ti
}

// ---- commands ----

func (d *DemoModel) SetSpeed(n int) { d.speed = clampInt(n, 0, 2) }

func (d *DemoModel) Step(dt float64) {
	if d.speed == 0 || d.event != nil {
		return
	}
	mult := 1.0
	if d.speed == 2 {
		mult = 3
	}
	d.t += dt * 4 * mult
	d.minute += dt * 4 * mult // 4 game minutes per second: a day in 6 minutes
	if d.minute >= 24*60 {
		d.minute -= 24 * 60
		d.day++
	}
	h := int(d.minute) / 60
	if h != d.lastHour {
		d.lastHour = h
		d.hourly(h)
	}
	if d.fire.active {
		d.fire.ticks += dt * 4 * mult
		lim := 80.0
		if d.fire.rescued {
			lim = 16
		}
		if !d.fire.rescued && d.fire.spreadTo >= 0 && d.fire.ticks > 24 {
			d.fire.itemIdx = append(d.fire.itemIdx, d.fire.spreadTo)
			d.fire.spreadTo = -1
		}
		if d.fire.ticks > lim {
			d.fire.active = false
			for _, i := range d.fire.itemIdx {
				if d.fire.rescued {
					d.items[i].State = StateOccupied
				} else {
					d.items[i].State = StateBurned
					d.items[i].st = nil
				}
			}
			if d.fire.rescued {
				d.msg("The fire has been put out by the rescue team.", false)
			} else {
				d.msg("The fire is out. Burned areas must be rebuilt.", true)
			}
		}
	}
	d.refresh()
}

var demoNews = []string{
	"Office workers demand Parking.",
	"Fast food on 2F: Business is average.",
	"Sims on 8F need a path to 1F.",
	"Hotel guest checked in at 3F suite (VIP).",
	"Medical Center demanded near Lobby.",
	"Income from Offices: $180,000.",
	"Condo on 5F sold for $150,000.",
}

func (d *DemoModel) hourly(h int) {
	d.msg(demoNews[(d.day*24+h)%len(demoNews)], false)
	if h == 13 && d.day == 3 && !d.fire.active && d.event == nil {
		d.startFire()
	}
	if h == 10 && d.day == 4 && d.event == nil {
		d.event = &Event{Kind: EventBomb, Title: "BOMB THREAT", Floor: 6, X: 40,
			Lines: []string{"Terrorists have planted a bomb in your tower!", "They demand a ransom of $200,000.",
				"If you refuse, security will search the tower.", "(DEMO prompt)"},
			Choices: []string{"PAY $200,000", "REFUSE"}}
	}
}

func (d *DemoModel) startFire() {
	var idx []int
	spread := -1
	for i := range d.items {
		it := &d.items[i].Item
		if it.Floor == 7 && it.Kind == KindOffice && it.X == 28 {
			idx = append(idx, i)
		}
		if it.Floor == 7 && it.Kind == KindOffice && it.X == 37 {
			spread = i
		}
	}
	if len(idx) == 0 {
		return
	}
	for _, i := range idx {
		d.items[i].State = StateBurning
		d.items[i].st = nil
	}
	d.fire.active, d.fire.rescued, d.fire.ticks, d.fire.itemIdx, d.fire.spreadTo = true, false, 0, idx, spread
	d.event = &Event{Kind: EventFire, Title: "FIRE!", Floor: 7, X: 32,
		Lines: []string{"A fire has broken out on 8F!", "Call the rescue helicopter for $500,000,",
			"or let security fight it from the emergency stairs?", "(DEMO prompt)"},
		Choices: []string{"CALL RESCUE $500,000", "WAIT"}}
}

func (d *DemoModel) Respond(choice int) {
	ev := d.event
	if ev == nil {
		return
	}
	d.event = nil
	switch ev.Kind {
	case EventFire:
		if choice == 0 {
			d.funds -= 500000
			d.fire.rescued = true
			d.fire.ticks = 0
			d.msg("Rescue helicopter dispatched: -$500,000.", false)
		} else {
			d.msg("Security is fighting the fire on 8F.", true)
		}
		for _, i := range d.fire.itemIdx {
			d.items[i].State = StateBurning
		}
	case EventBomb:
		if choice == 0 {
			d.funds -= 200000
			d.msg("Ransom paid: -$200,000.", false)
		} else {
			d.msg("Security found and defused the bomb (DEMO).", false)
		}
	}
	d.refresh()
}

func (d *DemoModel) msg(s string, alert bool) {
	c := d.Clock()
	d.msgs = append(d.msgs, Message{Text: s, Alert: alert,
		When: "Y" + strconv.Itoa(c.Year) + " Q" + strconv.Itoa(c.Quarter) + " " + c.DayLabel + " " + c.TimeString()})
	if len(d.msgs) > 200 {
		d.msgs = append(d.msgs[:0], d.msgs[len(d.msgs)-200:]...)
	}
}

func (d *DemoModel) def(k Kind) (BuildDef, bool) {
	for _, c := range demoCatalog {
		if c.Kind == k {
			return c, true
		}
	}
	return BuildDef{}, false
}

func overlap(a0, a1, b0, b1 int) bool { return a0 < b1 && b0 < a1 }

func (d *DemoModel) CanPlace(k Kind, x, f, w, h int) Result {
	def, ok := d.def(k)
	if !ok {
		return Result{Reason: "Unknown item."}
	}
	if def.MinStars > d.stars {
		return Result{Reason: def.Name + " requires " + strconv.Itoa(def.MinStars) + " stars."}
	}
	b := d.Bounds()
	if x < 0 || x+w > b.Width || f < b.MinFloor || f+h-1 > b.MaxFloor {
		return Result{Reason: "Outside the building lot."}
	}
	cost := def.Cost
	if def.PerUnit {
		cost *= int64(w)
	}
	if k.IsShaft() {
		if h > 30 && k != KindExpress {
			return Result{Reason: "Elevators can span at most 30 floors."}
		}
		for _, s := range d.shafts {
			if overlap(x, x+w, s.x, s.x+s.kind.ShaftWidth()) && overlap(f, f+h, s.bottom, s.top+1) {
				return Result{Reason: "Too close to another elevator."}
			}
		}
		// Shafts are an overlay in front of rooms (as in the original), so rooms
		// never block them.
		cost += int64(h-1) * 5000
	} else if k == KindLobby {
		if f%15 != 0 || f < 0 {
			return Result{Reason: "Lobbies only on 1F and every 15th floor."}
		}
	} else if k != KindFloor {
		if (k == KindParking || k == KindParkingRamp || k == KindMetro || k == KindRecycling) && f >= 0 {
			return Result{Reason: def.Name + " must be built underground."}
		}
		for i := range d.items {
			it := &d.items[i].Item
			if it.Kind == KindLobby {
				if overlap(x, x+w, it.X, it.X+it.W) && overlap(f, f+h, it.Floor, it.Floor+it.H) {
					return Result{Reason: "Cannot build on the lobby."}
				}
				continue
			}
			if overlap(x, x+w, it.X, it.X+it.W) && overlap(f, f+h, it.Floor, it.Floor+it.H) {
				return Result{Reason: "Overlaps " + kindName(it.Kind) + "."}
			}
		}
		// Rooms may be built behind a shaft (the shaft is an overlay), as in the
		// original.
	}
	// support: above ground needs the floor below, basements the floor above
	if f > 0 {
		s, ok := d.spans[f-1]
		if !ok || x < s[0] || x+w > s[1] {
			return Result{Reason: "Must be built above an existing floor."}
		}
	} else if f < 0 {
		s, ok := d.spans[f+1]
		if !ok || x < s[0] || x+w > s[1] {
			if f != -1 {
				return Result{Reason: "Dig down from an existing basement floor."}
			}
		}
	}
	if cost > d.funds {
		return Result{Reason: "Not enough funds (" + money(cost) + ").", Cost: cost}
	}
	return Result{OK: true, Cost: cost}
}

func (d *DemoModel) extendSpan(f, x0, x1 int) {
	if s, ok := d.spans[f]; ok {
		d.spans[f] = [2]int{minInt(s[0], x0), maxInt(s[1], x1)}
	} else {
		d.spans[f] = [2]int{x0, x1}
	}
}

func (d *DemoModel) Place(k Kind, x, f, w, h int) Result {
	r := d.CanPlace(k, x, f, w, h)
	if !r.OK {
		return r
	}
	d.funds -= r.Cost
	switch {
	case k.IsShaft():
		s := &demoShaft{id: len(d.shafts) + 100, x: x, bottom: f, top: f + h - 1, kind: k, response: 5, home0: f}
		s.served = make([]bool, h)
		for i := range s.served {
			s.served[i] = true
		}
		s.phase = -1 // idle car
		d.shafts = append(d.shafts, s)
		for ff := f; ff < f+h; ff++ {
			d.extendSpan(ff, x, x+w)
		}
	case k == KindFloor:
		d.extendSpan(f, x, x+w)
	case k == KindLobby:
		d.extendSpan(f, x, x+w)
		for i := range d.items {
			it := &d.items[i].Item
			if it.Kind == KindLobby && it.Floor == f && x <= it.X+it.W && it.X <= x+w {
				nx0, nx1 := minInt(x, it.X), maxInt(x+w, it.X+it.W)
				it.X, it.W = nx0, nx1-nx0
				d.refresh()
				return r
			}
		}
		d.nextID++
		d.items = append(d.items, demoItem{Item: Item{ID: d.nextID, Kind: KindLobby, X: x, Floor: f, W: w, H: 1, Rent: -1}})
	default:
		for ff := f; ff < f+h; ff++ {
			d.extendSpan(ff, x, x+w)
		}
		st := StateVacant
		switch k {
		case KindCondo:
			st = StateForSale
		case KindFastFood, KindRestaurant, KindRetail:
			st = StateClosed
		case KindStairs, KindEscalator, KindMedical, KindSecurity, KindHousekeeping, KindRecycling, KindParkingRamp, KindMetro, KindCathedral, KindCinema, KindPartyHall:
			st = StateIdle
		}
		d.nextID++
		rent := -1
		if _, ok := tierTables[k]; ok {
			rent = 1
		}
		d.items = append(d.items, demoItem{Item: Item{ID: d.nextID, Kind: k, X: x, Floor: f, W: w, H: h, State: st, Rent: rent}})
	}
	d.refresh()
	return r
}

func (d *DemoModel) Remove(x, f int) Result {
	for i := range d.items {
		it := &d.items[i].Item
		if x >= it.X && x < it.X+it.W && f >= it.Floor && f < it.Floor+it.H {
			if it.Kind == KindLobby {
				return Result{Reason: "Lobbies cannot be removed."}
			}
			d.items = append(d.items[:i], d.items[i+1:]...)
			d.fire.active = false
			d.refresh()
			return Result{OK: true}
		}
	}
	for i, s := range d.shafts {
		if x >= s.x && x < s.x+s.kind.ShaftWidth() && f >= s.bottom && f <= s.top {
			d.shafts = append(d.shafts[:i], d.shafts[i+1:]...)
			d.refresh()
			return Result{OK: true}
		}
	}
	return Result{}
}

func (d *DemoModel) AddCar(id, home int) Result {
	s := d.shaft(id)
	if s == nil {
		return Result{Reason: "No such shaft."}
	}
	if 1+len(s.extra) >= 8 {
		return Result{Reason: "A shaft holds at most 8 cars."}
	}
	cost := d.Schedule(id).CarCost
	if cost > d.funds {
		return Result{Reason: "Not enough funds."}
	}
	d.funds -= cost
	s.extra = append(s.extra, Car{Pos: home * 3, Home: home, Capacity: d.Schedule(id).Capacity})
	d.refresh()
	return Result{OK: true, Cost: cost}
}

func (d *DemoModel) RemoveCar(id int) Result {
	s := d.shaft(id)
	if s == nil || len(s.extra) == 0 {
		return Result{Reason: "The last car cannot be removed."}
	}
	s.extra = s.extra[:len(s.extra)-1]
	d.refresh()
	return Result{OK: true}
}

func (d *DemoModel) SetCarHome(id, car, f int) Result {
	s := d.shaft(id)
	if s == nil || f < s.bottom || f > s.top {
		return Result{Reason: "Floor not in this shaft."}
	}
	if car == 0 {
		s.home0 = f
	} else if car-1 < len(s.extra) {
		s.extra[car-1].Home = f
		s.extra[car-1].Pos = f * 3
	}
	d.refresh()
	return Result{OK: true}
}

func (d *DemoModel) ToggleServe(id, f int) Result {
	s := d.shaft(id)
	if s == nil || f < s.bottom || f > s.top {
		return Result{Reason: "Floor not in this shaft."}
	}
	if f == s.home0 {
		return Result{Reason: "Cannot stop serving a car's home floor."}
	}
	if s.kind == KindExpress {
		return Result{Reason: "Express floors cannot be toggled."}
	}
	s.served[f-s.bottom] = !s.served[f-s.bottom]
	return Result{OK: true}
}

func (d *DemoModel) SetPeriodMode(id int, we bool, p int, m PeriodMode) {
	if s := d.shaft(id); s != nil && p >= 0 && p < 7 {
		i := 0
		if we {
			i = 1
		}
		s.modes[i][p] = m
	}
}

func (d *DemoModel) SetResponse(id, n int) {
	if s := d.shaft(id); s != nil {
		s.response = n
	}
}

func (d *DemoModel) SetDeparture(id, n int) {
	if s := d.shaft(id); s != nil {
		s.depart = n
	}
}

func (d *DemoModel) SetRent(id, tier int) Result {
	if it := d.find(id); it != nil && it.Rent >= 0 {
		it.Rent = clampInt(tier, 0, 3)
		d.refresh()
		return Result{OK: true}
	}
	return Result{Reason: "No price to set."}
}

func (d *DemoModel) ChangeMovie(id int, classic bool) Result {
	cost := int64(300000)
	if classic {
		cost = 150000
	}
	if cost > d.funds {
		return Result{Reason: "Not enough funds."}
	}
	d.funds -= cost
	d.msg("New movie booked (DEMO): -"+money(cost)[1:], false)
	return Result{OK: true, Cost: cost}
}

func (d *DemoModel) RenameTenant(id int, name string) Result {
	if name == "" {
		delete(d.names, id)
	} else {
		d.names[id] = name
	}
	return Result{OK: true}
}

func (d *DemoModel) Save() error { d.saveCount++; return nil }

// refresh recomputes the scripted state and the output slices.
func (d *DemoModel) refresh() {
	t := d.ti()
	night := d.tod() == "night"
	d.itemsOut = d.itemsOut[:0]
	for i := range d.items {
		di := &d.items[i]
		if di.st != nil {
			di.State = di.st(d)
		}
		it := di.Item
		switch it.Kind {
		case KindOffice, KindCondo:
			it.Lit = night && it.State == StateOccupied && (it.X == 0 && it.Floor <= 5)
		}
		d.itemsOut = append(d.itemsOut, it)
	}
	d.shaftsOut = d.shaftsOut[:0]
	for _, s := range d.shafts {
		out := Shaft{ID: s.id, Kind: s.kind, X: s.x, Bottom: s.bottom, Top: s.top, Queues: s.queues}
		c := Car{Pos: s.home0 * 3, Home: s.home0, Capacity: 21}
		if s.phase >= 0 && s.top > s.bottom {
			span3 := (s.top - s.bottom) * 3
			period := span3*2 + 12
			ph := (int(math.Floor(d.t*0.8)) + s.phase) % period
			sub, dir, open := 0, 0, false
			switch {
			case ph < span3:
				sub, dir = ph, 1
			case ph < span3+6:
				sub, open = span3, true
			case ph < span3*2+6:
				sub, dir = span3*2+6-ph, -1
			default:
				sub, open = 0, true
			}
			c.Pos, c.Dir, c.DoorsOpen, c.Passengers = s.bottom*3+sub, dir, open, s.load
		}
		out.Cars = append(out.Cars, c)
		out.Cars = append(out.Cars, s.extra...)
		d.shaftsOut = append(d.shaftsOut, out)
	}
	// lobby walkers (mock formula)
	d.peopleOut = d.peopleOut[:0]
	for i := range d.items {
		it := &d.items[i].Item
		if it.Kind != KindLobby || it.Floor != 0 || it.W < 8 {
			continue
		}
		span := it.W - 2
		for k := 0; k < d.walkers; k++ {
			dir := -1
			if k%2 == 1 {
				dir = 1
			}
			px := it.X + 1 + ((k*13+dir*int(math.Floor(float64(t)*0.9)))%span+span)%span
			st := StressNormal
			if k%7 == 3 {
				st = StressRed
			} else if k%11 == 5 {
				st = StressPink
			}
			id := 1000 + k
			_, named := d.names[id]
			d.peopleOut = append(d.peopleOut, Person{ID: id, X: px, Floor: 0, Stress: st, Named: named})
		}
		break
	}
}
