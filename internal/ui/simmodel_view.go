package ui

import (
	"math"
	"sort"

	"simtower/internal/sim"
)

// tileKind maps sim tile names onto the UI vocabulary.
var tileKind = map[string]Kind{
	"floor":                KindFloor,
	"lobby":                KindLobby,
	"office":               KindOffice,
	"condo":                KindCondo,
	"hotelSingle":          KindHotelSingle,
	"hotelTwin":            KindHotelTwin,
	"hotelSuite":           KindHotelSuite,
	"fastFood":             KindFastFood,
	"restaurant":           KindRestaurant,
	"retail":               KindRetail,
	"cinema":               KindCinema,
	"partyHall":            KindPartyHall,
	"parkingRamp":          KindParkingRamp,
	"parking":              KindParking,
	"metro":                KindMetro,
	"cathedral":            KindCathedral,
	"medical":              KindMedical,
	"security":             KindSecurity,
	"housekeeping":         KindHousekeeping,
	"recyclingCenterUpper": KindRecycling,
	"stairs":               KindStairs,
	"escalator":            KindEscalator,
	"elevator":             KindElevator,
	"elevatorExpress":      KindExpress,
	"elevatorService":      KindService,
}

// kindTile is the sim tile name placed for a UI kind.
var kindTile = map[Kind]string{
	KindFloor: "floor", KindLobby: "lobby", KindOffice: "office", KindCondo: "condo",
	KindHotelSingle: "hotelSingle", KindHotelTwin: "hotelTwin", KindHotelSuite: "hotelSuite",
	KindFastFood: "fastFood", KindRestaurant: "restaurant", KindRetail: "retail",
	KindCinema: "cinema", KindPartyHall: "partyHall", KindParkingRamp: "parkingRamp",
	KindParking: "parking", KindMetro: "metro", KindCathedral: "cathedral",
	KindMedical: "medical", KindSecurity: "security", KindHousekeeping: "housekeeping",
	KindRecycling: "recyclingCenter", KindStairs: "stairs", KindEscalator: "escalator",
	KindElevator: "elevator", KindExpress: "elevatorExpress", KindService: "elevatorService",
}

// Names of the original's shops (Retail block of the save; docs/openskyscraper_notes.md).
var (
	restaurantNames = []string{"English Pub", "French", "Chinese", "Sushi Bar", "Steak House"}
	fastFoodNames   = []string{"Japanese Soba", "Chinese Cafe", "Hamburger Stand", "Ice Cream", "Coffee Shop"}
	shopNames       = []string{"Men's Clothing", "Pet Store", "Flower Shop", "Book Store", "Drug Store", "Boutique",
		"Electronics", "Bank", "Hair Salon", "Post Office", "Sports Gear"}
)

func venueName(k Kind, x, y int) string {
	h := (x*31 + y*17) & 0x7fffffff
	switch k {
	case KindRestaurant:
		return restaurantNames[h%len(restaurantNames)]
	case KindFastFood:
		return fastFoodNames[h%len(fastFoodNames)]
	case KindRetail:
		return shopNames[h%len(shopNames)]
	}
	return ""
}

// stressOf classifies a sim's current trip stress with the binary's fixed
// person-color thresholds: < 80 black, 80..119 pink, >= 120 red.
func (m *SimModel) stressOf(s *sim.SimRecord) Stress {
	t := sim.CurrentTripStressTicks(m.world(), m.ts.Time(), s)
	switch {
	case t >= 120:
		return StressRed
	case t >= 80:
		return StressPink
	}
	return StressNormal
}

func (m *SimModel) personID(key string) int {
	if id, ok := m.personIDs[key]; ok {
		return id
	}
	m.nextPers++
	id := 100000 + m.nextPers
	m.personIDs[key] = id
	m.personKey[id] = key
	return id
}

func (m *SimModel) shaftID(c *sim.CarrierRecord) int {
	if id, ok := m.shaftIDs[c]; ok {
		return id
	}
	m.nextShaft++
	m.shaftIDs[c] = m.nextShaft
	return m.nextShaft
}

// refresh rebuilds the per-frame read caches when the sim moved or a
// command changed the world.
func (m *SimModel) refresh() {
	tt := m.ts.SimTime()
	if !m.dirty && m.refreshSet && tt == m.refreshed {
		m.refreshCars() // cars still interpolate between ticks
		return
	}
	m.dirty = false
	m.refreshSet = true
	m.refreshed = tt
	m.rebuild()
}

const (
	evFlagBomb      = 1
	evFlagFire      = 8
	evFlagBombFound = 0x20
	evFlagDetonated = 0x40
)

func (m *SimModel) rebuild() {
	w := m.world()
	tm := m.ts.Time()
	clk := m.Clock()
	evFlags := w.EventState.GameStateFlags

	// sims by key / by owning object
	for k := range m.simByKey {
		delete(m.simByKey, k)
	}
	for k, v := range m.simsByObj {
		m.simsByObj[k] = v[:0]
	}
	for _, s := range w.Sims {
		key := sim.SimKey(s)
		m.simByKey[key] = s
		id := itemID(s.HomeColumn, sim.FloorToY(s.FloorAnchor))
		m.simsByObj[id] = append(m.simsByObj[id], s)
	}

	// floor spans and lobby cells
	type span struct{ l, r int }
	spans := map[int]span{}
	lobby := map[int][]int{} // y -> lobby xs
	m.items = m.items[:0]
	for k := range m.itemRec {
		delete(m.itemRec, k)
	}
	w.Cells.Range(func(k, tile string) bool {
		x, y := parseXY(k)
		if sp, ok := spans[y]; ok {
			if x < sp.l {
				sp.l = x
			}
			if x+1 > sp.r {
				sp.r = x + 1
			}
			spans[y] = sp
		} else {
			spans[y] = span{x, x + 1}
		}
		if w.CellToAnchor.Has(k) {
			return true
		}
		switch tile {
		case "floor", "recyclingCenterLower":
			return true
		case "lobby":
			lobby[y] = append(lobby[y], x)
			return true
		}
		kind, ok := tileKind[tile]
		if !ok {
			return true
		}
		m.addItem(kind, tile, k, x, y, clk, tm)
		return true
	})
	m.floors = m.floors[:0]
	for y, sp := range spans {
		m.floors = append(m.floors, FloorSpan{Floor: uiFloorOfY(y), Left: sp.l, Right: sp.r})
	}
	sort.Slice(m.floors, func(i, j int) bool { return m.floors[i].Floor < m.floors[j].Floor })

	// lobby runs: one item per contiguous run (pillars restart per item)
	for y, xs := range lobby {
		sort.Ints(xs)
		start := xs[0]
		for i := 1; i <= len(xs); i++ {
			if i < len(xs) && xs[i] == xs[i-1]+1 {
				continue
			}
			end := xs[i-1] + 1
			id := itemID(start, y)
			m.items = append(m.items, Item{ID: id, Kind: KindLobby, X: start, Floor: uiFloorOfY(y), W: end - start, H: 1, Rent: -1})
			m.itemRec[id] = itemRef{tile: "lobby", x: start, y: y}
			if i < len(xs) {
				start = xs[i]
			}
		}
	}

	// stairs and escalators (overlays); shafts come from the carriers
	w.Overlays.Range(func(k, tile string) bool {
		if tile != "stairs" && tile != "escalator" {
			return true
		}
		x, y := parseXY(k)
		id := itemID(x, y) + 1<<20
		it := Item{ID: id, Kind: tileKind[tile], X: x, Floor: uiFloorOfY(y), W: sim.TileWidths[tile], H: 2, Rent: -1}
		if tile == "escalator" {
			it.State = StateActive
		} else if m.stairsBusy(sim.YToFloor(y)) {
			it.State = StateActive
		}
		m.items = append(m.items, it)
		m.itemRec[id] = itemRef{tile: tile, x: x, y: y}
		return true
	})

	// fire: rooms under an active front burn
	if evFlags&evFlagFire != 0 {
		l, r := w.EventState.FireLeftPos, w.EventState.FireRightPos
		for i := range m.items {
			it := &m.items[i]
			if it.Kind == KindLobby || it.Kind == KindStairs || it.Kind == KindEscalator {
				continue
			}
			for f := it.Floor; f < it.Floor+it.H; f++ {
				sf := simFloorOfUI(f)
				if sf < 0 || sf >= len(l) {
					continue
				}
				lp, rp := l[sf], r[sf]
				if lp == 0xffff && rp == 0xffff {
					continue
				}
				a, b := lp, rp+12
				if lp == 0xffff {
					a = rp
				}
				if rp == 0xffff {
					b = lp + 1
				}
				if it.X < b && a < it.X+it.W {
					it.State = StateBurning
				}
			}
		}
	}

	// burned areas: rooms that vanished while a fire or bomb was active stay
	// on screen as burned until rebuilt over or bulldozed
	now := make(map[int]Item, len(m.items))
	for _, it := range m.items {
		now[it.ID] = it
	}
	if !m.noBurn && (evFlags != 0 || m.lastEvFlags != 0) {
		for id, it := range m.lastItems {
			if _, ok := now[id]; ok || it.Kind == KindLobby || it.Kind == KindStairs || it.Kind == KindEscalator {
				continue
			}
			it.State, it.Eval, it.Lit, it.Occupants, it.Customers = StateBurned, EvalNone, false, 0, 0
			m.burned[id] = it
		}
	}
	m.lastEvFlags = evFlags
	m.lastItems = now
	for id, b := range m.burned {
		gone := false
		for i := range m.items {
			it := &m.items[i]
			if it.Kind != KindLobby && overlap(b.X, b.X+b.W, it.X, it.X+it.W) && overlap(b.Floor, b.Floor+b.H, it.Floor, it.Floor+it.H) {
				gone = true
				break
			}
		}
		if gone {
			delete(m.burned, id)
			continue
		}
		m.items = append(m.items, b)
	}

	m.buildShafts()
	m.buildPeople()
}

// addItem maps one placed-object anchor to an Item.
func (m *SimModel) addItem(kind Kind, tile, key string, x, y int, clk Clock, tm *sim.TimeState) {
	w := m.world()
	f := uiFloorOfY(y)
	h := 1
	switch tile {
	case "cinema", "partyHall":
		h = 2 // anchor is the upper row
		f--
	case "metro":
		h = 3
		f -= 2
	case "cathedral":
		h = 5
		f -= 4
	case "recyclingCenterUpper":
		h = 2
		f--
	}
	id := itemID(x, y)
	it := Item{ID: id, Kind: kind, X: x, Floor: f, W: sim.TileWidths[tile], H: h, Rent: -1}
	rec := w.PlacedObjects.At(key)
	m.itemRec[id] = itemRef{key: key, tile: tile, x: x, y: y}
	if rec == nil {
		m.items = append(m.items, it)
		return
	}
	var venue *sim.CommercialVenueRecord
	var ent *sim.EntertainmentLinkRecord
	switch sc := sim.GetSidecar(w, rec.LinkedRecordIndex).(type) {
	case *sim.CommercialVenueRecord:
		venue = sc
	case *sim.EntertainmentLinkRecord:
		ent = sc
	}
	it.Eval = evalOf(rec.EvalLevel)
	if _, ok := sim.Yen1001[tile]; ok && rec.RentLevel <= 3 {
		it.Rent = rec.RentLevel
	}
	sims := m.simsByObj[id]
	switch kind {
	case KindOffice:
		it.Capacity = 6
		if rec.UnitStatus > 0x0f {
			it.State, it.Eval = StateVacant, EvalNone
		} else {
			it.State = StateOccupied
			it.Occupants = it.Capacity
			if clk.Hour >= 17 || clk.Hour < 7 {
				// lights stay on while any worker is still at the desk
				for _, s := range sims {
					if sim.StateBaseCode(s.StateCode) == sim.StateAtWork || s.StateCode == sim.StateAtWork {
						it.Lit = true
						break
					}
				}
			}
		}
	case KindCondo:
		it.Capacity = 3
		if rec.UnitStatus > 0x17 {
			it.State, it.Eval = StateForSale, EvalNone
		} else {
			it.State = StateOccupied
			it.Occupants = it.Capacity
			it.Lit = clk.Hour >= 18 && clk.Hour < 24
		}
	case KindHotelSingle, KindHotelTwin, KindHotelSuite:
		it.Capacity = 2
		if kind == KindHotelSingle {
			it.Capacity = 1
		}
		switch st := rec.UnitStatus; {
		case st < 0x18:
			it.State = StateOccupied
			it.Occupants = it.Capacity
			if clk.Hour >= 23 || clk.Hour < 6 {
				it.State = StateAsleep
			}
			if rec.VipFlag && kind == KindHotelSuite {
				it.State = StateVIP
			}
		case st < 0x28:
			it.State, it.Eval = StateVacant, EvalNone
		case st < 0x38:
			it.State = StateDirty
		default:
			it.State = StateInfested
		}
	case KindRetail, KindFastFood, KindRestaurant:
		it.Name = venueName(kind, x, y)
		if venue != nil {
			it.Customers = venue.TodayVisitCount
			switch venue.AvailabilityState {
			case sim.VenueDormant:
				if kind == KindRetail {
					it.State = StateVacant
				} else {
					it.State = StateClosed
				}
			case sim.VenueClosed:
				it.State = StateClosed
			case sim.VenueNearFull:
				it.State = StateOpen
				if kind == KindFastFood {
					it.State = StateBusy
				}
			default:
				it.State = StateOpen
			}
			if it.Eval == EvalNone && it.State != StateVacant {
				it.Eval = customerEval(venue.TodayVisitCount)
			}
		}
	case KindCinema:
		if ent != nil && ent.LinkPhaseState != 0 && tm.DayTick >= 1000 && tm.DayTick < 1900 {
			it.State = StateActive
		}
	case KindPartyHall:
		if ent != nil && ent.LinkPhaseState != 0 && tm.DayTick >= 1200 && tm.DayTick < 1600 {
			it.State = StateActive
		}
	case KindMetro:
		if rec.UnitStatus == 2 {
			it.State = StateActive
		}
	case KindParking:
		it.State = StateVacant
		if sr, ok := sim.GetSidecar(w, rec.LinkedRecordIndex).(*sim.ServiceRequestEntry); ok && sr.CoverageFlag != nil && *sr.CoverageFlag != 0 &&
			clk.Hour >= 7 && clk.Hour < 22 {
			it.State = StateOccupied
		}
	}
	m.items = append(m.items, it)
}

func evalOf(level int) Eval {
	switch level {
	case 2:
		return EvalBlue
	case 1:
		return EvalYellow
	case 0:
		return EvalRed
	}
	return EvalNone
}

// customerEval grades a shop by today's customers with the payout bands
// (docs/mechanics.md 2.2: < 25 loses money, >= 50 best band).
func customerEval(n int) Eval {
	switch {
	case n >= sim.CommercialClosureBands[2]:
		return EvalBlue
	case n >= sim.CommercialClosureBands[0]:
		return EvalYellow
	}
	return EvalRed
}

// stairsBusy reports whether a stairs flight starting at the internal floor
// carries anyone right now (the special-link load counters).
func (m *SimModel) stairsBusy(entryFloor int) bool {
	for _, seg := range m.world().SpecialLinks {
		if seg.Active && seg.Flags&1 == 1 && seg.EntryFloor == entryFloor &&
			(seg.AscendingLoadCounter > 0 || seg.DescendingLoadCounter > 0) {
			return true
		}
	}
	return false
}

// ---- shafts ----

func (m *SimModel) buildShafts() {
	w := m.world()
	m.shafts = m.shafts[:0]
	for k := range m.shaftRec {
		delete(m.shaftRec, k)
	}
	live := map[*sim.CarrierRecord]bool{}
	for _, c := range w.Carriers {
		live[c] = true
		id := m.shaftID(c)
		m.shaftRec[id] = c
		s := Shaft{ID: id, Kind: KindElevator, X: c.Column,
			Bottom: uiFloorOfSim(c.BottomServedFloor), Top: uiFloorOfSim(c.TopServedFloor)}
		switch c.CarrierMode {
		case 0:
			s.Kind = KindExpress
		case 2:
			s.Kind = KindService
		}
		m.fillCars(&s, c)
		for slot, q := range c.FloorQueues {
			if q == nil || (q.Up.IsEmpty() && q.Down.IsEmpty()) {
				continue
			}
			s.Queues = append(s.Queues, Queue{Floor: uiFloorOfSim(c.BottomServedFloor + slot),
				Up: m.ringStress(q.Up), Down: m.ringStress(q.Down)})
		}
		m.shafts = append(m.shafts, s)
	}
	for c := range m.shaftIDs {
		if !live[c] {
			delete(m.shaftIDs, c)
		}
	}
	for car := range m.carPrev {
		found := false
		for c := range live {
			for _, cc := range c.Cars {
				if cc == car {
					found = true
				}
			}
		}
		if !found {
			delete(m.carPrev, car)
		}
	}
}

func (m *SimModel) ringStress(r *sim.RouteRequestRing) []Stress {
	if r == nil || r.Count == 0 {
		return nil
	}
	out := make([]Stress, 0, r.Count)
	n := len(r.Items)
	for i := 0; i < r.Count && n > 0; i++ {
		key := r.Items[(r.Head+i)%n]
		st := StressNormal
		if s := m.simByKey[key]; s != nil {
			st = m.stressOf(s)
		}
		out = append(out, st)
	}
	return out
}

// fillCars lists the active cars (the order HandleSetElevatorHomeFloor and
// the elevator window index by), interpolating positions between ticks.
func (m *SimModel) fillCars(s *Shaft, c *sim.CarrierRecord) {
	load := make([]int, len(c.Cars))
	for _, r := range c.PendingRoutes {
		if r.Boarded && r.AssignedCarIndex >= 0 && r.AssignedCarIndex < len(load) {
			load[r.AssignedCarIndex]++
		}
	}
	for ci, car := range c.Cars {
		if !car.Active {
			continue
		}
		e := carEffective(car)
		if p, ok := m.carPrev[car]; ok && m.frac > 0 && math.Abs(p-e) < 4 {
			e = p + (e-p)*m.frac
		}
		pos := int(math.Round((e - 10) * 3))
		lo, hi := s.Bottom*3, s.Top*3
		pos = clampInt(pos, lo, hi)
		dir := 0
		if car.CurrentFloor != car.TargetFloor || car.SettleCounter > 0 {
			dir = -1
			if car.DirectionFlag != 0 {
				dir = 1
			}
		}
		s.Cars = append(s.Cars, Car{Pos: pos, Dir: dir, Passengers: load[ci], Capacity: c.AssignmentCapacity,
			DoorsOpen: car.DwellCounter > 0 && car.SettleCounter == 0, Home: uiFloorOfSim(car.HomeFloor)})
	}
}

// refreshCars updates only the car positions (interpolation between ticks).
func (m *SimModel) refreshCars() {
	for i := range m.shafts {
		s := &m.shafts[i]
		c := m.shaftRec[s.ID]
		if c == nil {
			continue
		}
		s.Cars = s.Cars[:0]
		m.fillCars(s, c)
	}
}

// ---- people ----

// buildPeople lists the walkers: sims standing in a lobby between legs of a
// trip (in transit, not riding a car, not queued at a shaft) and sims on a
// stairs/escalator leg (drawn on the flight's entry floor). People in rooms
// are part of the tile art, riders are the car's count, queued sims are the
// shaft's Queues.
func (m *SimModel) buildPeople() {
	w := m.world()
	m.people = m.people[:0]
	// lobby extents per internal floor
	type run struct{ l, r int }
	lobbies := map[int][]run{}
	for i := range m.items {
		it := &m.items[i]
		if it.Kind == KindLobby {
			sf := simFloorOfUI(it.Floor)
			lobbies[sf] = append(lobbies[sf], run{it.X, it.X + it.W})
		}
	}
	tick := m.ts.Time().DayTick
	for _, s := range w.Sims {
		if !sim.HasTransitFlag(s.StateCode) {
			continue
		}
		key := sim.SimKey(s)
		var x, sf int
		switch s.Route.Mode {
		case sim.RouteModeCarrier:
			continue // queued or riding
		case sim.RouteModeSegment:
			// on a stairs/escalator leg: the segment record has no column, so
			// the sim is drawn near its own column on the flight's floor
			sf = s.SelectedFloor
			x = s.HomeColumn + s.BaseOffset
		default:
			// between legs: standing on SelectedFloor. In a lobby the crowd
			// mills across the run; elsewhere the sim walks near its room.
			sf = s.SelectedFloor
			h := len(key)*7 + s.HomeColumn*13 + s.BaseOffset*29 + s.FloorAnchor*3
			dir := 1
			if h&1 == 1 {
				dir = -1
			}
			if runs := lobbies[sf]; len(runs) > 0 {
				r := runs[0]
				for _, rr := range runs {
					if s.HomeColumn >= rr.l && s.HomeColumn < rr.r {
						r = rr
					}
				}
				span := r.r - r.l - 2
				if span < 2 {
					continue
				}
				x = r.l + 1 + ((h+dir*(tick/6))%span+span)%span
			} else {
				x = s.HomeColumn + s.BaseOffset + dir*((tick/8)%4)
			}
		}
		if sf < 0 || sf >= sim.GridHeight {
			continue
		}
		id := m.personID(key)
		_, named := m.names[key]
		m.people = append(m.people, Person{ID: id, X: clampInt(x, 0, sim.GridWidth-1), Floor: uiFloorOfSim(sf),
			Stress: m.stressOf(s), Named: named})
		if len(m.people) >= 400 {
			break
		}
	}
}
