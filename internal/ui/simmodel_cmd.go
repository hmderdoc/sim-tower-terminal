package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"simtower/internal/sim"
)

// ---- removal ----

// nonRemovable: items the original will not bulldoze (docs/mechanics.md
// section 2 "Removable").
var nonRemovable = map[string]string{
	"lobby":        "The lobby cannot be removed.",
	"housekeeping": "Housekeeping cannot be removed.",
	"security":     "The security office cannot be removed.",
	"metro":        "The metro station cannot be removed.",
	"cathedral":    "The cathedral cannot be removed.",
}

// Remove bulldozes whatever is at the cell: a burned area, a whole elevator
// shaft, a stairs/escalator flight or a room. Plain floor and empty cells
// return Result{} quietly (bulldozer drags sweep across them).
func (m *SimModel) Remove(x, f int) Result {
	m.refresh()
	for id, b := range m.burned {
		if x >= b.X && x < b.X+b.W && f >= b.Floor && f < b.Floor+b.H {
			delete(m.burned, id)
			m.touch()
			return Result{OK: true}
		}
	}
	w := m.world()
	y := yOfUI(f)
	if y < 0 || y >= w.Height || x < 0 || x >= w.Width {
		return Result{}
	}
	k := ckey(x, y)
	if a := w.OverlayToAnchor.At(k); a != "" || w.Overlays.At(k) != "" {
		if a == "" {
			a = k
		}
		ax, ay := parseXY(a)
		t := w.Overlays.At(a)
		if t == "elevator" || t == "elevatorExpress" || t == "elevatorService" {
			var c *sim.CarrierRecord
			for _, cc := range w.Carriers {
				if cc.Column == ax && sim.YToFloor(ay) >= cc.BottomServedFloor && sim.YToFloor(ay) <= cc.TopServedFloor {
					c = cc
				}
			}
			if c != nil {
				bottom, top := c.BottomServedFloor, c.TopServedFloor
				for fl := bottom; fl <= top; fl++ {
					m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdRemoveTile, X: ax, Y: sim.FloorToY(fl)})
				}
				m.after()
				return Result{OK: true}
			}
		}
		r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdRemoveTile, X: ax, Y: ay})
		m.after()
		if !r.Accepted {
			return Result{Reason: r.Reason}
		}
		return Result{OK: true}
	}
	anchor := w.CellToAnchor.At(k)
	if anchor == "" {
		anchor = k
	}
	tile := w.Cells.At(anchor)
	switch tile {
	case "", "floor":
		return Result{}
	}
	if why, ok := nonRemovable[tile]; ok {
		return Result{Reason: why}
	}
	ax, ay := parseXY(anchor)
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdRemoveTile, X: ax, Y: ay})
	m.after()
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	return Result{OK: true}
}

// after is the bookkeeping every mutating command shares: collect what it
// queued and rebuild the read caches so the change is not mistaken for fire
// damage.
func (m *SimModel) after() {
	m.drainAfterCommand()
	m.touch()
	m.noBurn = true
	m.refresh()
	m.noBurn = false
}

// ---- elevators ----

func (m *SimModel) carrier(id int) *sim.CarrierRecord {
	m.refresh()
	return m.shaftRec[id]
}

func (m *SimModel) AddCar(id, home int) Result {
	c := m.carrier(id)
	if c == nil {
		return Result{Reason: "No such elevator."}
	}
	n := 0
	for _, car := range c.Cars {
		if car.Active {
			n++
		}
	}
	if n >= 8 {
		return Result{Reason: "A shaft holds at most 8 cars."}
	}
	cost := sim.CatalogCarCosts[c.CarrierMode]
	if cost > m.ts.Cash() {
		return Result{Reason: "Not enough money for a car (" + money(int64(cost)) + ")."}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdAddElevatorCar, X: c.Column, Y: sim.FloorToY(simFloorOfUI(home))})
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	m.ts.Ledger().CashBalance -= cost
	m.construction += int64(cost)
	m.after()
	return Result{OK: true, Cost: int64(cost)}
}

func (m *SimModel) RemoveCar(id int) Result {
	c := m.carrier(id)
	if c == nil {
		return Result{Reason: "No such elevator."}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdRemoveElevatorCar, X: c.Column, Y: sim.FloorToY(c.BottomServedFloor)})
	m.after()
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	return Result{OK: true}
}

func (m *SimModel) SetCarHome(id, car, f int) Result {
	c := m.carrier(id)
	if c == nil {
		return Result{Reason: "No such elevator."}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdSetElevatorHomeFloor, X: c.Column, CarIndex: car, Floor: simFloorOfUI(f)})
	m.after()
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	return Result{OK: true}
}

func (m *SimModel) ToggleServe(id, f int) Result {
	c := m.carrier(id)
	if c == nil {
		return Result{Reason: "No such elevator."}
	}
	sf := simFloorOfUI(f)
	if sf < c.BottomServedFloor || sf > c.TopServedFloor {
		return Result{Reason: "Floor not in this shaft."}
	}
	if c.CarrierMode == 0 {
		return Result{Reason: "Express floors cannot be toggled."}
	}
	for _, car := range c.Cars {
		if car.Active && car.HomeFloor == sf {
			return Result{Reason: "Cannot stop serving a car's home floor."}
		}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdToggleElevatorFloorStop, X: c.Column, Floor: sf})
	m.after()
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	return Result{OK: true}
}

// SetPeriodMode writes the shaft's per-daypart express flag (the binary's
// schedule byte the sim reads at terminal floors; there is no command for it
// in the sim's command set, so the adapter sets the player's choice directly).
func (m *SimModel) SetPeriodMode(id int, weekend bool, p int, mode PeriodMode) {
	c := m.carrier(id)
	if c == nil || p < 0 || p >= 7 {
		return
	}
	i := p
	if weekend {
		i += 7
	}
	for len(c.ExpressDirectionFlags) < 14 {
		c.ExpressDirectionFlags = append(c.ExpressDirectionFlags, 0)
	}
	c.ExpressDirectionFlags[i] = int(mode) % 3
}

func (m *SimModel) SetResponse(id, floors int) {
	if c := m.carrier(id); c != nil {
		m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdSetElevatorWaitingCarResponse, X: c.Column,
			Y: sim.FloorToY(c.BottomServedFloor), Value: clampInt(floors, 0, 99)})
	}
}

// Departure: the sim's dwell multiplier is 0..5 (x30 ticks, the upstream
// inspector's range); the window shows it as 10 s steps.
const departureStep = 10

func (m *SimModel) SetDeparture(id, seconds int) {
	c := m.carrier(id)
	if c == nil {
		return
	}
	cur := 0
	if len(c.DwellDelay) > 0 {
		cur = c.DwellDelay[0]
	}
	d := seconds - cur*departureStep
	next := cur
	switch {
	case d > 0 && d <= 30, d < -30:
		next = cur + 1
	case d < 0 || d > 30:
		next = cur - 1
	}
	if next > 5 {
		next = 0
	}
	if next < 0 {
		next = 5
	}
	m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdSetElevatorDwellDelay, X: c.Column, Y: sim.FloorToY(c.BottomServedFloor), Value: next})
}

func (m *SimModel) Schedule(id int) Schedule {
	c := m.carrier(id)
	if c == nil {
		return Schedule{ShaftID: id}
	}
	n := 0
	for _, o := range m.world().Carriers {
		if o.CarrierMode == c.CarrierMode {
			n++
		}
		if o == c {
			break
		}
	}
	title := [3]string{"EXPRESS ELEVATOR #", "STANDARD ELEVATOR #", "SERVICE ELEVATOR #"}[clampInt(c.CarrierMode, 0, 2)]
	sc := Schedule{ShaftID: id, Title: title + itoa(n), Response: c.WaitingCarResponseThreshold,
		Capacity: c.AssignmentCapacity, CarCost: int64(sim.CatalogCarCosts[c.CarrierMode])}
	if len(c.DwellDelay) > 0 {
		sc.Departure = c.DwellDelay[0] * departureStep
	}
	for f := c.BottomServedFloor; f <= c.TopServedFloor; f++ {
		slot := f - c.BottomServedFloor
		on := slot >= len(c.StopFloorEnabled) || c.StopFloorEnabled[slot] != 0
		if c.CarrierMode == 0 && !sim.IsExpressStopFloor(f, m.world().LobbyMode) {
			on = false
		}
		sc.Served = append(sc.Served, on)
	}
	for i := 0; i < 14 && i < len(c.ExpressDirectionFlags); i++ {
		sc.Modes[i/7][i%7] = PeriodMode(clampInt(c.ExpressDirectionFlags[i], 0, 2))
	}
	return sc
}

// ---- facilities ----

func (m *SimModel) itemByID(id int) (*Item, itemRef, bool) {
	m.refresh()
	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i], m.itemRec[id], true
		}
	}
	return nil, itemRef{}, false
}

func (m *SimModel) SetRent(id, tier int) Result {
	_, ref, ok := m.itemByID(id)
	if !ok || ref.key == "" {
		return Result{Reason: "That facility is gone."}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdSetRentLevel, X: ref.x, Y: ref.y, RentLevel: clampInt(tier, 0, 3)})
	m.touch()
	if !r.Accepted {
		reason := r.Reason
		if reason == "" {
			reason = "The price cannot be changed."
		}
		m.msg(reason, true)
		return Result{Reason: reason}
	}
	return Result{OK: true}
}

func (m *SimModel) ChangeMovie(id int, classic bool) Result {
	it, ref, ok := m.itemByID(id)
	if !ok || it.Kind != KindCinema {
		return Result{Reason: "Not a cinema."}
	}
	cost, pool := sim.CinemaNewMovieCost, "new"
	if classic {
		cost, pool = sim.CinemaClassicMovieCost, "classic"
	}
	if cost > m.ts.Cash() {
		return Result{Reason: "Not enough money (" + money(int64(cost)) + ")."}
	}
	r := m.ts.SubmitCommand(sim.SimCommand{Type: sim.CmdSetCinemaMoviePool, X: ref.x, Y: ref.y, Pool: pool})
	if !r.Accepted {
		return Result{Reason: r.Reason}
	}
	m.ts.Ledger().CashBalance -= cost
	m.msg("New movie booked: -"+money(int64(cost)), false)
	m.touch()
	return Result{OK: true, Cost: int64(cost)}
}

func (m *SimModel) RenameTenant(id int, name string) Result {
	key, ok := m.personKey[id]
	if !ok {
		return Result{Reason: "That person has left the tower."}
	}
	name = strings.TrimSpace(asciiOnly(name))
	if name == "" {
		delete(m.names, key)
		return Result{OK: true}
	}
	if _, had := m.names[key]; !had && len(m.names) >= 20 {
		return Result{Reason: "At most 20 people can be named."}
	}
	if len(name) > 20 {
		name = name[:20]
	}
	m.names[key] = name
	m.touch()
	return Result{OK: true}
}

var rentLabels = map[Kind]string{KindOffice: "Rent per quarter", KindRetail: "Rent per quarter",
	KindCondo: "Sale price", KindHotelSingle: "Per stay", KindHotelTwin: "Per stay", KindHotelSuite: "Per stay"}

func (m *SimModel) Inspect(id int) InspectInfo {
	it, ref, ok := m.itemByID(id)
	if !ok {
		return InspectInfo{ID: id, Title: "Gone", Status: "Removed", Rent: -1}
	}
	in := InspectInfo{ID: id, Kind: it.Kind, Title: kindName(it.Kind) + ", Floor " + floorLabel(it.Floor),
		Eval: it.Eval, Rent: it.Rent}
	if it.Name != "" {
		in.Title = it.Name + " (" + kindName(it.Kind) + "), Floor " + floorLabel(it.Floor)
	}
	in.EvalText = [4]string{"Not rated", "Excellent", "Good", "Terrible"}[it.Eval]
	switch it.State {
	case StateVacant:
		in.Status = "For Rent"
		if isHotel(it.Kind) {
			in.Status = "Vacant"
		}
	case StateForSale:
		in.Status = "For Sale"
	case StateClosed:
		in.Status = "Closed"
	case StateOpen:
		in.Status = "Open"
	case StateBusy:
		in.Status = "Open (rush)"
	case StateDirty:
		in.Status = "Needs cleaning"
	case StateInfested:
		in.Status = "Cockroaches!"
	case StateAsleep:
		in.Status = "Guest asleep"
	case StateVIP:
		in.Status = "VIP guest"
	case StateBurning:
		in.Status = "ON FIRE"
	case StateBurned:
		in.Status = "Burned out; bulldoze to rebuild"
	case StateActive:
		in.Status = "In use"
	case StateOccupied:
		in.Status = "Occupied"
		if it.Kind == KindCondo {
			in.Status = "Sold"
		}
	default:
		in.Status = "Operating"
	}
	in.Message = [4]string{"", "Conditions are excellent.", "Conditions are good.", "Conditions are terrible."}[it.Eval]
	w := m.world()
	rec := w.PlacedObjects.At(ref.key)
	if payouts, ok := sim.Yen1001[ref.tile]; ok && it.Rent >= 0 {
		for _, p := range payouts {
			n := int64(p * 1000)
			in.Tiers = append(in.Tiers, RentTier{Label: money(n), Amount: n})
		}
		in.Income = rentLabels[it.Kind] + " " + money(in.Tiers[it.Rent].Amount)
		if it.Kind == KindCondo && rec != nil && rec.UnitStatus < 0x18 {
			in.RentFixed = "sold at this price"
		}
	} else {
		in.Rent = -1
	}
	switch it.Kind {
	case KindOffice, KindCondo, KindHotelSingle, KindHotelTwin, KindHotelSuite:
		if it.State != StateVacant && it.State != StateForSale {
			for i, s := range m.simsByObj[id] {
				if i >= it.Capacity {
					break
				}
				in.Occupants = append(in.Occupants, m.personID(sim.SimKey(s)))
			}
		}
	case KindFastFood, KindRestaurant, KindRetail:
		in.ShowCount, in.Customers = true, it.Customers
		if rec != nil {
			if v, ok := sim.GetSidecar(w, rec.LinkedRecordIndex).(*sim.CommercialVenueRecord); ok {
				in.Income = "Customers today " + itoa(v.TodayVisitCount) + ", yesterday " + itoa(v.YesterdayVisitCount)
				switch {
				case v.YesterdayVisitCount >= 50 || v.TodayVisitCount >= 50:
					in.Message = "Business is good."
				case v.TodayVisitCount >= 25:
					in.Message = "Business is average."
				default:
					in.Message = "Business is poor."
				}
			}
		}
		if it.State == StateVacant {
			in.Message = "Waiting for a tenant."
		}
	case KindCinema:
		if rec != nil {
			if e, ok := sim.GetSidecar(w, rec.LinkedRecordIndex).(*sim.EntertainmentLinkRecord); ok && e.FamilySelectorOrSingleLinkFlag != 0xff {
				sel := e.FamilySelectorOrSingleLinkFlag
				pool := "new"
				if sel < 7 {
					pool = "classic"
				}
				title := "?"
				if sel >= 0 && sel < len(sim.MovieTitles) {
					title = sim.MovieTitles[sel]
				}
				in.Movie = "\"" + title + "\" (" + pool + "), day " + itoa(e.LinkAgeCounter+1) + " of its run"
				in.Income = "Attendance today " + itoa(e.AttendanceCounter)
				switch {
				case e.AttendanceCounter >= 100:
					in.Message = "It's a Sellout!"
				case e.LinkAgeCounter == 0:
					in.Message = "New Movie showing!"
				case e.AttendanceCounter >= 40:
					in.Message = "Ticket Sales are average."
				default:
					in.Message = "Terrible Sales, change the movie."
				}
			}
		}
	}
	if in.Message == "" && it.State == StateBurned {
		in.Message = "Destroyed. Use the bulldozer to clear it."
	}
	return in
}

func familyLabel(code int) string {
	switch code {
	case sim.FamilyOffice:
		return "Office worker"
	case sim.FamilyCondo:
		return "Condo resident"
	case sim.FamilyHotelSingle, sim.FamilyHotelTwin, sim.FamilyHotelSuite:
		return "Hotel guest"
	case sim.FamilyRestaurant:
		return "Diner"
	case sim.FamilyFastFood:
		return "Fast food customer"
	case sim.FamilyRetail:
		return "Shopper"
	case sim.FamilyCinema, sim.FamilyCinemaLower:
		return "Movie goer"
	case sim.FamilyPartyHall, sim.FamilyPartyHallLower:
		return "Party guest"
	case sim.FamilyHousekeeping:
		return "Housekeeper"
	case sim.FamilySecurity:
		return "Security guard"
	case sim.FamilyRecyclingCenterUpper, sim.FamilyRecyclingCenterLower:
		return "Recycling worker"
	}
	if code >= sim.FamilyCathedralBase && code <= sim.FamilyCathedralMax {
		return "Wedding guest"
	}
	return "Visitor"
}

func (m *SimModel) Tenant(id int) TenantInfo {
	m.refresh()
	key, ok := m.personKey[id]
	s := m.simByKey[key]
	if !ok || s == nil {
		return TenantInfo{ID: id, Name: "(left the tower)", From: "-", GoingTo: "-"}
	}
	ti := TenantInfo{ID: id, Name: familyLabel(s.FamilyCode), Stress: m.stressOf(s)}
	if n, ok := m.names[key]; ok {
		ti.Name = n
	}
	home := itemID(s.HomeColumn, sim.FloorToY(s.FloorAnchor))
	if it, _, ok := m.itemByID(home); ok {
		ti.From = kindName(it.Kind) + ", Floor " + floorLabel(it.Floor)
		ti.Eval = it.Eval
	} else {
		ti.From = "Floor " + floorLabel(uiFloorOfSim(s.FloorAnchor))
	}
	switch {
	case s.DestinationFloor >= 0 && sim.HasTransitFlag(s.StateCode):
		ti.GoingTo = "Floor " + floorLabel(uiFloorOfSim(s.DestinationFloor))
	case sim.HasTransitFlag(s.StateCode):
		ti.GoingTo = "On the way (" + floorLabel(uiFloorOfSim(s.SelectedFloor)) + ")"
	default:
		ti.GoingTo = "Staying put"
	}
	return ti
}

// ---- reports ----

var incomeLabels = [10]string{"Office", "Single room", "Twin room", "Suite", "Shop", "Fast food",
	"Restaurant", "Party hall", "Cinema", "Condominium"}

// maintenance lines: label and the expense-ledger codes (YEN #1002 family
// codes; 24 = lobby, the "parking expense" routine).
var maintLines = [10]struct {
	label string
	codes []int
}{
	{"Lobby", []int{24}}, {"Elevator", []int{1}}, {"Express elevator", []int{42}}, {"Service elevator", []int{43}},
	{"Escalator", []int{27}}, {"Parking ramp", []int{44}}, {"Recycling center", []int{20, 21}},
	{"Metro station", []int{0x1f, 0x20, 0x21}}, {"Housekeeping", []int{15}}, {"Security", []int{14}},
}

func (m *SimModel) Finance() Finance {
	led := m.ts.Ledger()
	c := m.Clock()
	f := Finance{Quarter: "Y" + itoa(c.Year) + " Q" + itoa(c.Quarter), PrevBalance: int64(led.CashBalanceCycleBase),
		Balance: int64(led.CashBalance), Construction: m.construction}
	for i, l := range incomeLabels {
		v := 0
		if i < len(led.IncomeLedger) {
			v = led.IncomeLedger[i]
		}
		f.Income = append(f.Income, Line{l, int64(v)})
	}
	for _, ml := range maintLines {
		v := 0
		for _, code := range ml.codes {
			if code < len(led.ExpenseLedger) {
				v += led.ExpenseLedger[code]
			}
		}
		f.Maintenance = append(f.Maintenance, Line{ml.label, int64(v)})
	}
	return f
}

func (m *SimModel) Rating() Rating {
	w := m.world()
	st := m.Stars()
	r := Rating{Stars: st, Population: m.Population()}
	if st >= 6 {
		r.Next = "Your tower has the TOWER rating."
		return r
	}
	if st >= 5 {
		r.Next = "TOWER at population " + commas(sim.StarThresholds[4])
		r.Needs = append(r.Needs, "A cathedral on 100F and a wedding in it (weekend morning)")
	} else {
		r.Next = itoa(st+1) + " stars at population " + commas(sim.StarThresholds[st-1])
	}
	if st < 5 && m.Population() < sim.StarThresholds[st-1] {
		r.Needs = append(r.Needs, "Population "+commas(sim.StarThresholds[st-1]))
	}
	g := &w.GateFlags
	need := func(ok bool, s string) {
		if !ok {
			r.Needs = append(r.Needs, s)
		}
	}
	switch st {
	case 2:
		need(g.SecurityPlaced != 0, "A security office")
	case 3:
		need(g.OfficePlaced != 0, "Offices")
		need(g.RecyclingAdequate != 0, "Enough recycling centers")
		need(g.OfficeServiceOk != 0, "A favorable VIP visit (hotel suites)")
		need(g.OfficeServiceOkMedical != 0, "A medical center within reach")
		need(g.RoutesViable != 0, "Working routes to every floor")
	case 4:
		need(g.MetroPlaced != 0, "A metro station")
		need(g.RecyclingAdequate != 0, "Enough recycling centers")
		need(g.OfficeServiceOkMedical != 0, "A medical center within reach")
		need(g.RoutesViable != 0, "Working routes to every floor")
	}
	if st >= 3 && st < 5 {
		r.Needs = append(r.Needs, "Ratings rise on weekdays from 5:00 PM")
	}
	return r
}

// ---- save / load ----

// simExtras is the UI state the sim snapshot does not carry; it lives next
// to the save as <save>.ui.json.
type simExtras struct {
	Version      int               `json:"version"`
	Names        map[string]string `json:"names,omitempty"`
	Construction int64             `json:"construction"`
	Prompts      []sim.SimPrompt   `json:"prompts,omitempty"`
	Messages     []Message         `json:"messages,omitempty"`
	Speed        int               `json:"speed"`
}

func extrasPath(p string) string { return strings.TrimSuffix(p, ".json") + ".ui.json" }

// writeAtomic writes data via a temp file in the same directory + rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Save writes the sim snapshot (JSON of SaveState) and the UI extras.
func (m *SimModel) Save() error {
	if m.path == "" {
		return nil
	}
	data, err := json.Marshal(m.ts.SaveState())
	if err != nil {
		m.saveErr = err
		return err
	}
	if err := writeAtomic(m.path, data); err != nil {
		m.saveErr = err
		return err
	}
	ex := simExtras{Version: 1, Names: m.names, Construction: m.construction, Speed: m.speed}
	if m.eventPrompt != nil {
		ex.Prompts = append(ex.Prompts, *m.eventPrompt)
	}
	ex.Prompts = append(ex.Prompts, m.prompts...)
	if n := len(m.msgs); n > 50 {
		ex.Messages = m.msgs[n-50:]
	} else {
		ex.Messages = m.msgs
	}
	if b, err := json.Marshal(ex); err == nil {
		if err := writeAtomic(extrasPath(m.path), b); err != nil {
			m.saveErr = err
			return err
		}
	}
	m.saveErr = nil
	return nil
}

// LoadSimModel reads a save written by Save.
func LoadSimModel(path string) (m *SimModel, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap sim.SimSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("save %s: %w", path, err)
	}
	if snap.Time == nil || snap.World == nil || snap.Ledger == nil {
		return nil, fmt.Errorf("save %s: incomplete snapshot", path)
	}
	defer func() {
		if r := recover(); r != nil {
			m, err = nil, fmt.Errorf("save %s: %v", path, r)
		}
	}()
	m = newSimModel(sim.FromSnapshot(&snap), path)
	if b, err := os.ReadFile(extrasPath(path)); err == nil {
		var ex simExtras
		if json.Unmarshal(b, &ex) == nil {
			if ex.Names != nil {
				m.names = ex.Names
			}
			m.construction = ex.Construction
			m.prompts = ex.Prompts
			m.msgs = ex.Messages
			m.speed = clampInt(ex.Speed, 0, 2)
		}
	}
	m.msg("Welcome back. Tower loaded.", false)
	return m, nil
}

// SaveSummary describes a save for the start menu ("" if unreadable).
func SaveSummary(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var head struct {
		Time   sim.TimeState `json:"time"`
		Ledger struct {
			Cash int `json:"cashBalance"`
		} `json:"ledger"`
		World struct {
			Stars int `json:"starCount"`
			Pop   int `json:"currentPopulation"`
		} `json:"world"`
	}
	if json.Unmarshal(data, &head) != nil {
		return ""
	}
	d := head.Time.DayCounter
	return "Year " + strconv.Itoa(d/12+1) + " Q" + strconv.Itoa((d/3)%4+1) + ", " + strconv.Itoa(head.World.Stars) +
		" star, pop " + commas(head.World.Pop) + ", " + money(int64(head.Ledger.Cash))
}
