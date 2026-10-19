package sim

// Port of reference/tower-together/sim/trace.test.ts: build towers from
// fixture JSON specs, step the simulation, and compare against the
// reference JSONL traces dumped from SIMTOWER.EXE.
//
// Usage:
//
//	go test ./internal/sim -run TestTrace                 # every fixture, full length
//	go test ./internal/sim -run TestTrace -short          # first traceShortEntries entries each
//	go test ./internal/sim -run TestTrace -fixture=condo  # one fixture (or SIMTOWER_FIXTURE=condo)
//	go test ./internal/sim -run TestTrace -trace-entries=500
//
// Extra (not in the TS suite): SIMTOWER_TRACE_SIM_STATES=1 also compares
// the per-sim `sim_states` array against SimsToArray() state codes in order.
//
// The JSONL files are streamed line by line (288MB total; never loaded
// whole). A fixture that reaches a not-yet-ported function fails with the
// stub's "TODO port: ..." message.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var (
	fixtureFlag      = flag.String("fixture", "", "run only this trace fixture (name without build_ prefix); env SIMTOWER_FIXTURE")
	traceEntriesFlag = flag.Int("trace-entries", 0, "compare at most N trace entries per fixture (0 = all, or traceShortEntries under -short)")
)

const traceShortEntries = 400

var fixtureDir = filepath.Join("..", "..", "reference", "tower-together", "sim", "fixtures")

// ─── Types ──────────────────────────────────────────────────────────────────

type facilitySpec struct {
	Type   string `json:"type"`
	Floor  *int   `json:"floor"`
	Left   int    `json:"left"`
	Right  *int   `json:"right"`
	Bottom *int   `json:"bottom"`
	Top    *int   `json:"top"`
	Cars   *int   `json:"cars"`
}

type scheduledBatch struct {
	Day        int            `json:"day"`
	Tick       int            `json:"tick"`
	Facilities []facilitySpec `json:"facilities"`
}

type floorExtent struct {
	Left  int `json:"left"`
	Right int `json:"right"`
}

type buildSpec struct {
	FloorExtent         map[string]floorExtent `json:"floor_extent"`
	Facilities          []facilitySpec         `json:"facilities"`
	ScheduledFacilities []scheduledBatch       `json:"scheduled_facilities"`
}

type traceCar struct {
	CurrentFloor     int  `json:"currentFloor"`
	DirectionFlag    int  `json:"directionFlag"`
	TargetFloor      int  `json:"targetFloor"`
	StabilizeCounter *int `json:"stabilizeCounter"`
	DwellCounter     *int `json:"dwellCounter"`
	AssignedCount    *int `json:"assignedCount"`
	PrevFloor        *int `json:"prevFloor"`
	ArrivalSeen      *int `json:"arrivalSeen"`
	ArrivalTick      *int `json:"arrivalTick"`
}

type traceCarrier struct {
	Column      int        `json:"column"`
	Mode        int        `json:"mode"`
	Capacity    int        `json:"capacity"`
	BottomFloor int        `json:"bottomFloor"`
	TopFloor    int        `json:"topFloor"`
	Cars        []traceCar `json:"cars"`
}

type traceSimGroup struct {
	Count     int            `json:"count"`
	StressAvg *int           `json:"stress_avg"`
	StressMin *int           `json:"stress_min"`
	StressMax *int           `json:"stress_max"`
	States    map[string]int `json:"states"`
}

type traceFire struct {
	Flags                   int            `json:"flags"`
	FireActive              bool           `json:"fire_active"`
	BombActive              bool           `json:"bomb_active"`
	SecurityCount           int            `json:"security_count"`
	EvalEntityIdx           int            `json:"eval_entity_idx"`
	FireFloor               int            `json:"fire_floor"`
	FireTile                int            `json:"fire_tile"`
	FireStartTick           int            `json:"fire_start_tick"`
	RescueCountdown         int            `json:"rescue_countdown"`
	HelicopterExtinguishPos int            `json:"helicopter_extinguish_pos"`
	FireLeftPos             map[string]int `json:"fire_left_pos"`
	FireRightPos            map[string]int `json:"fire_right_pos"`
}

type traceEntry struct {
	Day           int  `json:"day"`
	Tick          int  `json:"tick"`
	Daypart       int  `json:"daypart"`
	Stars         int  `json:"stars"`
	Cash          int  `json:"cash"`
	CalendarPhase int  `json:"calendar_phase"`
	MetroFloor    int  `json:"metro_floor"`
	Population    int  `json:"population"`
	RngCalls      *int `json:"rng_calls"`
	Gates         struct {
		Security  bool `json:"security"`
		Office    bool `json:"office"`
		Recycling bool `json:"recycling"`
		Route     bool `json:"route"`
	} `json:"gates"`
	SimStates []int                    `json:"sim_states"`
	Sims      map[string]traceSimGroup `json:"sims"`
	// simKeyOrder keeps the JSON member order of "sims" (the TS iterates
	// Object.entries(entry.sims) and fails on the first mismatch).
	simKeyOrder []string
	Carriers    []traceCarrier `json:"carriers"`
	Fire        *traceFire     `json:"fire"`

	raw []byte
}

// ─── Fixture tile name -> sim tile name mapping ────────────────────────────

var fixtureTileMap = map[string]string{
	"office":       "office",
	"condo":        "condo",
	"restaurant":   "restaurant",
	"fast-food":    "fastFood",
	"retail":       "retail",
	"cinema":       "cinema",
	"party-hall":   "partyHall",
	"single":       "hotelSingle",
	"twin":         "hotelTwin",
	"suite":        "hotelSuite",
	"stairs":       "stairs",
	"security":     "security",
	"housekeeping": "housekeeping",
	"medical":      "medical",
	"metro":        "metro",
	"cathedral":    "cathedral",
	"lobby":        "lobby",
}

var traceSimKeyToFamily = map[string]int{
	"office":       7,
	"condo":        9,
	"restaurant":   6,
	"fast-food":    12,
	"retail":       10,
	"cinema":       18,
	"party-hall":   30,
	"single":       3,
	"twin":         4,
	"suite":        5,
	"security":     14,
	"housekeeping": 15,
	"cathedral-a":  0x24,
	"cathedral-b":  0x24,
	"cathedral-c":  0x24,
	"cathedral-d":  0x24,
	"cathedral-e":  0x24,
}

// ─── Fixture loading (streamed) ─────────────────────────────────────────────

func loadBuildSpec(t *testing.T, name string) buildSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtureDir, "build_"+name+".json"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var spec buildSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	return spec
}

type traceReader struct {
	f  *os.File
	br *bufio.Reader
}

func openTrace(t *testing.T, name string) *traceReader {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtureDir, "build_"+name+".jsonl"))
	if err != nil {
		t.Fatalf("open trace: %v", err)
	}
	return &traceReader{f: f, br: bufio.NewReaderSize(f, 1<<20)}
}

func (r *traceReader) Close() { r.f.Close() }

// next returns the next entry, or nil at EOF.
func (r *traceReader) next(t *testing.T) *traceEntry {
	t.Helper()
	for {
		line, err := r.br.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			var e traceEntry
			if jerr := json.Unmarshal(line, &e); jerr != nil {
				t.Fatalf("parse trace line: %v", jerr)
			}
			e.raw = append([]byte(nil), line...)
			e.simKeyOrder = simsKeyOrder(t, line)
			return &e
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			t.Fatalf("read trace: %v", err)
		}
	}
}

// simsKeyOrder extracts the member order of the "sims" object.
func simsKeyOrder(t *testing.T, line []byte) []string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(line, &top); err != nil {
		t.Fatalf("parse trace line: %v", err)
	}
	raw, ok := top["sims"]
	if !ok {
		return nil
	}
	var om OrderedMap[string, json.RawMessage]
	if err := json.Unmarshal(raw, &om); err != nil {
		t.Fatalf("parse sims: %v", err)
	}
	return om.Keys()
}

// withoutTimeEqual compares two raw entries ignoring day/tick
// (TS dropTerminalDuplicateDump's JSON.stringify(withoutTime(...)) check).
func withoutTimeEqual(a, b []byte) bool {
	var ma, mb map[string]any
	if json.Unmarshal(a, &ma) != nil || json.Unmarshal(b, &mb) != nil {
		return false
	}
	delete(ma, "day")
	delete(ma, "tick")
	delete(mb, "day")
	delete(mb, "tick")
	return reflect.DeepEqual(ma, mb)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// TS: trace.test.ts traceTickToTotalTicks
func traceTickToTotalTicks(day, tick int) int {
	if day == 0 {
		return (tick - NewGameDayTick + DayTickMax) % DayTickMax
	}
	day0Length := DayTickMax - NewGameDayTick + DayTickNewDay
	return day0Length + (day-1)*DayTickMax + ((tick - DayTickNewDay + DayTickMax) % DayTickMax)
}

func intOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// TS: trace.test.ts placeFacilityList
func placeFacilityList(t *testing.T, sim *TowerSim, facilities []facilitySpec) {
	t.Helper()
	for _, fac := range facilities {
		if fac.Type == "elevator" || fac.Type == "elevatorExpress" || fac.Type == "elevatorService" {
			bottom := intOr(fac.Bottom, intOr(fac.Floor, 0))
			top := intOr(fac.Top, intOr(fac.Floor, 0))
			for f := bottom; f <= top; f++ {
				sim.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: fac.Left, Y: GroundY - f, TileType: fac.Type})
			}
			numCars := intOr(fac.Cars, 1)
			var homeFloors []int
			if fac.Type == "elevatorExpress" {
				validStops := []int{bottom}
				for f := bottom + 1; f <= top; f++ {
					exe := f + 10
					if exe >= 11 && (exe-10)%15 == 14 {
						validStops = append(validStops, f)
					}
				}
				for i := 0; i < numCars-1; i++ {
					homeFloors = append(homeFloors, validStops[(i+1)%len(validStops)])
				}
			} else {
				span := top - bottom
				for i := 0; i < numCars-1; i++ {
					if numCars <= 1 {
						homeFloors = append(homeFloors, bottom)
					} else {
						homeFloors = append(homeFloors, bottom+floorDiv(span*(i+1), numCars-1))
					}
				}
			}
			for _, homeFloor := range homeFloors {
				sim.SubmitCommand(SimCommand{Type: CmdAddElevatorCar, X: fac.Left, Y: GroundY - homeFloor})
			}
			continue
		}
		tileType, ok := fixtureTileMap[fac.Type]
		if !ok {
			t.Fatalf("Unknown fixture tile type: %s", fac.Type)
		}
		if fac.Floor == nil {
			t.Fatalf("Facility %s missing 'floor'", fac.Type)
		}
		y := GroundY - *fac.Floor
		// Wide lobby placements span [left, right); iterate single tiles.
		if tileType == "lobby" && fac.Right != nil {
			for x := fac.Left; x < *fac.Right; x++ {
				sim.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: x, Y: y, TileType: tileType})
			}
			continue
		}
		sim.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: fac.Left, Y: y, TileType: tileType})
	}
}

// TS: trace.test.ts placeTilesFromSpec
func placeTilesFromSpec(t *testing.T, sim *TowerSim, spec buildSpec) {
	t.Helper()
	sim.FreeBuild = true

	if ground, ok := spec.FloorExtent["0"]; ok {
		for x := ground.Left; x < ground.Right; x++ {
			sim.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: x, Y: GroundY, TileType: "lobby"})
		}
	}

	var floors []int
	for k := range spec.FloorExtent {
		f, err := strconv.Atoi(k)
		if err != nil {
			t.Fatalf("bad floor key %q", k)
		}
		if f != 0 {
			floors = append(floors, f)
		}
	}
	sort.Ints(floors)
	for _, floor := range floors {
		extent := spec.FloorExtent[strconv.Itoa(floor)]
		y := GroundY - floor
		tileType := "floor"
		if floor%15 == 14 {
			tileType = "lobby"
		}
		for x := extent.Left; x < extent.Right; x++ {
			sim.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: x, Y: y, TileType: tileType})
		}
	}

	placeFacilityList(t, sim, spec.Facilities)
	sim.FreeBuild = false
}

// TS: trace.test.ts computeRngState
func computeRngState(seed int32, calls int) int32 {
	state := seed
	for i := 0; i < calls; i++ {
		state = int32(uint32(state)*0x15a4e35 + 1)
	}
	return state
}

// TS: trace.test.ts prepareFromTrace
func prepareFromTrace(t *testing.T, spec buildSpec, first *traceEntry) *TowerSim {
	t.Helper()
	sim := Create("trace-test", "Trace Test", LobbyModePerfectParity)
	placeTilesFromSpec(t, sim, spec)
	snap := sim.SaveState()
	snap.Ledger.CashBalance = first.Cash
	snap.World.StarCount = first.Stars
	snap.World.CurrentPopulation = first.Population
	if first.RngCalls != nil {
		snap.World.RngCallCount = *first.RngCalls
		snap.World.RngState = computeRngState(1, *first.RngCalls)
	}
	snap.World.EventState.DisableNewsEvents = true
	for _, refCarrier := range first.Carriers {
		var carrier *CarrierRecord
		for _, c := range snap.World.Carriers {
			if c.Column == refCarrier.Column {
				carrier = c
				break
			}
		}
		if carrier == nil {
			continue
		}
		for c := 0; c < len(refCarrier.Cars) && c < len(carrier.Cars); c++ {
			refCar := refCarrier.Cars[c]
			ourCar := carrier.Cars[c]
			ourCar.CurrentFloor = refCar.CurrentFloor
			ourCar.TargetFloor = refCar.TargetFloor
			ourCar.PrevFloor = intOr(refCar.PrevFloor, refCar.CurrentFloor)
		}
	}
	return FromSnapshot(snap)
}

func advanceTo(sim *TowerSim, targetTotalTicks int) {
	for sim.SimTime() < targetTotalTicks {
		sim.Step()
	}
}

func traceSimTotal(entry *traceEntry) int {
	total := 0
	for _, g := range entry.Sims {
		total += g.Count
	}
	return total
}

// runGuarded runs a trace body; a "TODO port" panic fails the test with the
// stub name, any other panic fails with its stack.
func runGuarded(t *testing.T, fixture string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprint(r)
			if strings.Contains(msg, "TODO port:") {
				t.Fatalf("fixture %s needs unported code: %s", fixture, msg)
			}
			t.Fatalf("fixture %s panicked: %v\n%s", fixture, r, debug.Stack())
		}
	}()
	fn()
}

func selectedFixture() string {
	if *fixtureFlag != "" {
		return *fixtureFlag
	}
	return os.Getenv("SIMTOWER_FIXTURE")
}

func entryLimit() int {
	if *traceEntriesFlag > 0 {
		return *traceEntriesFlag
	}
	if testing.Short() {
		return traceShortEntries
	}
	return 0
}

func wantFixture(name string) bool {
	sel := selectedFixture()
	return sel == "" || sel == name || sel == "build_"+name
}

// ─── Fixture list ───────────────────────────────────────────────────────────

var fixtureNames = []string{
	"commercial",
	"condo",
	"condo_office_elevator",
	"condo_stuck",
	"dense_hotel",
	"dense_office",
	"elevator",
	"hotel",
	"lobby_only",
	"mixed",
	"mixed_elevator",
	"mixed_elevator_delayed",
	"mixed_multicar",
	"metro",
	"offices",
	"sky_office",
}

// ─── trace: build_<fixture> ─────────────────────────────────────────────────

func TestTrace(t *testing.T) {
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Skipf("fixtures not present: %v", err)
	}
	for _, name := range fixtureNames {
		if !wantFixture(name) {
			continue
		}
		t.Run("build_"+name, func(t *testing.T) {
			runGuarded(t, name, func() { runTraceFixture(t, name) })
		})
	}
}

func runTraceFixture(t *testing.T, fixtureName string) {
	spec := loadBuildSpec(t, fixtureName)
	r := openTrace(t, fixtureName)
	defer r.Close()

	first := r.next(t)
	if first == nil {
		return
	}
	cur := r.next(t)
	if cur == nil {
		return // no simulation entries
	}

	sim := prepareFromTrace(t, spec, first)
	var prevSimCalls, prevTraceCalls *int

	pending := make([]scheduledBatch, len(spec.ScheduledFacilities))
	copy(pending, spec.ScheduledFacilities)

	checkSimStates := os.Getenv("SIMTOWER_TRACE_SIM_STATES") == "1"
	limit := entryLimit()
	prevRaw := first.raw
	compared := 0

	for cur != nil {
		next := r.next(t)
		// dropTerminalDuplicateDump: the emulator appends one final dump
		// identical (minus day/tick) to the previous one.
		if next == nil && withoutTimeEqual(prevRaw, cur.raw) {
			break
		}
		entry := cur

		advanceTo(sim, traceTickToTotalTicks(entry.Day, entry.Tick))
		for i := len(pending) - 1; i >= 0; i-- {
			batch := pending[i]
			if batch.Day == entry.Day && batch.Tick == entry.Tick {
				placeFacilityList(t, sim, batch.Facilities)
				pending = append(pending[:i], pending[i+1:]...)
			}
		}
		ctx := fmt.Sprintf("day=%d tick=%d", entry.Day, entry.Tick)
		compareTraceEntry(t, fixtureName, sim, entry, ctx, &prevSimCalls, &prevTraceCalls, checkSimStates)

		compared++
		if limit > 0 && compared >= limit {
			t.Logf("stopped after %d entries (limit)", compared)
			return
		}
		prevRaw = cur.raw
		cur = next
	}
	t.Logf("compared %d entries", compared)
}

func compareTraceEntry(t *testing.T, fixtureName string, sim *TowerSim, entry *traceEntry, ctx string,
	prevSimCalls, prevTraceCalls **int, checkSimStates bool) {
	t.Helper()
	gf := sim.GateFlags()
	eq := func(got, want any, what string) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s at %s: got %v want %v", what, ctx, got, want)
		}
	}

	// ── Scalar fields ──
	eq(sim.DaypartIndex(), entry.Daypart, "daypart mismatch")
	eq(sim.StarCount(), entry.Stars, "stars mismatch")
	eq(sim.CurrentPopulation(), entry.Population, "population mismatch")
	eq(gf.MetroStationFloorIndex, entry.MetroFloor, "metro_floor mismatch")
	eq(gf.OfficeServiceOk != 0, entry.Gates.Office, "gates.office")
	eq(gf.RecyclingAdequate != 0, entry.Gates.Recycling, "gates.recycling")
	eq(gf.RoutesViable != 0, entry.Gates.Route, "gates.route")
	eq(entry.Gates.Security, false, "unexpected gates.security")

	// ── Cash ──
	eq(sim.Cash(), entry.Cash, "cash mismatch fx="+fixtureName)

	// ── Sim counts & states (only for entries with named families) ──
	isActive := false
	for _, k := range entry.simKeyOrder {
		if !strings.HasPrefix(k, "0x") {
			isActive = true
			break
		}
	}
	if isActive {
		simArray := sim.SimsToArray()
		eq(len(simArray), traceSimTotal(entry), "sim total mismatch")

		byFamily := map[int]int{}
		byFamilyState := map[int]map[string]int{}
		byFamilyStress := map[int][]int{}
		for _, sm := range simArray {
			byFamily[sm.FamilyCode]++
			m := byFamilyState[sm.FamilyCode]
			if m == nil {
				m = map[string]int{}
				byFamilyState[sm.FamilyCode] = m
			}
			m[strconv.Itoa(sm.StateCode)]++
			stress := 0
			if sm.TripCount > 0 {
				stress = sm.AccumulatedTicks / sm.TripCount
			}
			if stress > 0 {
				byFamilyStress[sm.FamilyCode] = append(byFamilyStress[sm.FamilyCode], stress)
			}
		}

		for _, key := range entry.simKeyOrder {
			refGroup := entry.Sims[key]
			familyCode, ok := traceSimKeyToFamily[key]
			if !ok {
				continue
			}
			eq(byFamily[familyCode], refGroup.Count, "family "+key+" count mismatch")
			ourStates := byFamilyState[familyCode]
			if ourStates == nil {
				ourStates = map[string]int{}
			}
			refStates := refGroup.States
			if refStates == nil {
				refStates = map[string]int{}
			}
			eq(ourStates, refStates, "family "+key+" state counts mismatch")

			if refGroup.StressAvg != nil && refGroup.StressMin != nil && refGroup.StressMax != nil {
				stresses := byFamilyStress[familyCode]
				avg, mn, mx := 0, 0, 0
				if len(stresses) > 0 {
					sum := 0
					mn, mx = stresses[0], stresses[0]
					for _, s := range stresses {
						sum += s
						mn = min(mn, s)
						mx = max(mx, s)
					}
					avg = sum / len(stresses)
				}
				eq(avg, *refGroup.StressAvg, "family "+key+" stress_avg mismatch")
				eq(mn, *refGroup.StressMin, "family "+key+" stress_min mismatch")
				eq(mx, *refGroup.StressMax, "family "+key+" stress_max mismatch")
			}
		}

		if checkSimStates && entry.SimStates != nil {
			ours := make([]int, len(simArray))
			for i, sm := range simArray {
				ours[i] = sm.StateCode
			}
			eq(ours, entry.SimStates, "sim_states (opt-in) mismatch")
		}
	}

	// ── RNG call deltas ──
	if entry.RngCalls != nil {
		if *prevSimCalls != nil && *prevTraceCalls != nil {
			simDelta := sim.RngCallCount() - **prevSimCalls
			traceDelta := *entry.RngCalls - **prevTraceCalls
			if simDelta != traceDelta {
				t.Fatalf("rng_calls delta mismatch at %s: sim=%d trace=%d", ctx, simDelta, traceDelta)
			}
		}
		sc := sim.RngCallCount()
		tc := *entry.RngCalls
		*prevSimCalls = &sc
		*prevTraceCalls = &tc
	}

	// ── Carrier car positions ──
	if len(entry.Carriers) > 0 {
		ourByCol := map[string][]CarrierCarStateRecord{}
		for _, rec := range sim.CarriersToArray() {
			k := fmt.Sprintf("%d:%d", rec.Column, rec.CarrierMode)
			ourByCol[k] = append(ourByCol[k], rec)
		}
		for _, refCarrier := range entry.Carriers {
			k := fmt.Sprintf("%d:%d", refCarrier.Column, refCarrier.Mode)
			cars, ok := ourByCol[k]
			if !ok {
				t.Fatalf("carrier %s missing at %s", k, ctx)
			}
			eq(len(cars), len(refCarrier.Cars), "car count for "+k)
			for i, ref := range refCarrier.Cars {
				ours := cars[i]
				pre := fmt.Sprintf("car %d ", i)
				eq(ours.CurrentFloor, ref.CurrentFloor, pre+"currentFloor at "+k)
				eq(ours.TargetFloor, ref.TargetFloor, pre+"targetFloor at "+k)
				eq(ours.DirectionFlag, ref.DirectionFlag, pre+"directionFlag at "+k)
				if ref.StabilizeCounter != nil {
					eq(ours.SettleCounter, *ref.StabilizeCounter, pre+"stabilizeCounter at "+k)
				}
				if ref.DwellCounter != nil {
					eq(ours.DwellCounter, *ref.DwellCounter, pre+"dwellCounter at "+k)
				}
				if ref.AssignedCount != nil {
					eq(ours.AssignedCount, *ref.AssignedCount, pre+"assignedCount at "+k)
				}
				if ref.PrevFloor != nil {
					eq(ours.PrevFloor, *ref.PrevFloor, pre+"prevFloor at "+k)
				}
				if ref.ArrivalSeen != nil {
					eq(ours.ArrivalSeen, *ref.ArrivalSeen, pre+"arrivalSeen at "+k)
				}
				if ref.ArrivalTick != nil {
					eq(ours.ArrivalTick, *ref.ArrivalTick, pre+"arrivalTick at "+k)
				}
			}
		}
	}
}

// ─── trace: cathedral recovered path ────────────────────────────────────────

func TestTraceCathedralRecoveredPath(t *testing.T) {
	if !wantFixture("cathedral") {
		t.Skip("not selected")
	}
	runGuarded(t, "cathedral", func() {
		extent := map[string]floorExtent{}
		for floor := 0; floor < 104; floor++ {
			extent[strconv.Itoa(floor)] = floorExtent{Left: 80, Right: 180}
		}
		floor := 103
		sim := Create("trace-cathedral", "Trace Cathedral", LobbyModePerfectParity)
		placeTilesFromSpec(t, sim, buildSpec{FloorExtent: extent, Facilities: []facilitySpec{{Type: "cathedral", Floor: &floor, Left: 100}}})

		countBy := func(recs []*SimStateRecord, key func(*SimStateRecord) int) map[int]int {
			out := map[int]int{}
			for _, r := range recs {
				out[key(r)]++
			}
			return out
		}
		before := sim.SimsToArray()
		if len(before) != 40 {
			t.Fatalf("sims %d", len(before))
		}
		if got := countBy(before, func(r *SimStateRecord) int { return r.FamilyCode }); !reflect.DeepEqual(got, map[int]int{36: 40}) {
			t.Fatalf("families %v", got)
		}
		if got := countBy(before, func(r *SimStateRecord) int { return r.StateCode }); !reflect.DeepEqual(got, map[int]int{39: 40}) {
			t.Fatalf("states %v", got)
		}
		advanceTo(sim, traceTickToTotalTicks(0, 0))
		after := sim.SimsToArray()
		if sim.StarCount() != 1 {
			t.Fatalf("stars %d", sim.StarCount())
		}
		if got := countBy(after, func(r *SimStateRecord) int { return r.StateCode }); !reflect.DeepEqual(got, map[int]int{32: 40}) {
			t.Fatalf("states after %v", got)
		}
	})
}

// ─── trace: build_fire / build_fire_accept ──────────────────────────────────

func readAllTrace(t *testing.T, name string) []*traceEntry {
	// The fire fixtures are small (~130KB); the TS indexes them backwards.
	r := openTrace(t, name)
	defer r.Close()
	var out []*traceEntry
	for e := r.next(t); e != nil; e = r.next(t) {
		out = append(out, e)
	}
	return out
}

func seedFireSnapshot(t *testing.T, sim *TowerSim, seed *traceEntry, withPopulation bool) *SimSnapshot {
	fire := seed.Fire
	if fire == nil {
		t.Fatal("fixture must include fire state")
	}
	snap := sim.SaveState()
	snap.Time.DayCounter = seed.Day
	snap.Time.DayTick = seed.Tick
	snap.Time.DaypartIndex = seed.Daypart
	if withPopulation {
		if seed.Day%3 == 2 {
			snap.Time.WeekendFlag = 1
		} else {
			snap.Time.WeekendFlag = 0
		}
		snap.World.CurrentPopulation = seed.Population
		snap.World.EventState.RescueCountdown = fire.RescueCountdown
		snap.World.EventState.HelicopterExtinguishPos = fire.HelicopterExtinguishPos
	}
	snap.World.StarCount = seed.Stars
	snap.Ledger.CashBalance = seed.Cash
	es := &snap.World.EventState
	es.DisableNewsEvents = true
	es.GameStateFlags = fire.Flags
	es.FireFloor = fire.FireFloor
	es.FireTile = fire.FireTile
	es.FireStartTick = fire.FireStartTick
	for i := range es.FireLeftPos {
		es.FireLeftPos[i] = 0xffff
	}
	for i := range es.FireRightPos {
		es.FireRightPos[i] = 0xffff
	}
	for k, v := range fire.FireLeftPos {
		i, _ := strconv.Atoi(k)
		es.FireLeftPos[i] = v
	}
	for k, v := range fire.FireRightPos {
		i, _ := strconv.Atoi(k)
		es.FireRightPos[i] = v
	}
	return snap
}

func sparseFronts(pos []int) map[string]int {
	out := map[string]int{}
	for f, v := range pos {
		if v != 0xffff {
			out[strconv.Itoa(f)] = v
		}
	}
	return out
}

func TestTraceFire(t *testing.T) {
	if !wantFixture("fire") {
		t.Skip("not selected")
	}
	runGuarded(t, "fire", func() {
		spec := loadBuildSpec(t, "fire")
		trace := readAllTrace(t, "fire")
		if len(trace) == 0 {
			return
		}
		sim := Create("trace-fire", "Trace Fire", LobbyModePerfectParity)
		placeTilesFromSpec(t, sim, spec)
		seed := trace[0]
		seeded := FromSnapshot(seedFireSnapshot(t, sim, seed, true))
		baseTotal := seeded.SimTime()

		declineAtTick := seed.Fire.FireStartTick + 2
		declineSubmitted := false

		lastFireActive := -1
		for i := len(trace) - 1; i >= 0; i-- {
			if trace[i].Fire != nil && trace[i].Fire.FireActive {
				lastFireActive = i
				break
			}
		}
		lastIdx := len(trace) - 1
		if lastFireActive >= 0 {
			lastIdx = lastFireActive + 1
		}
		for entryIdx := 1; entryIdx <= lastIdx; entryIdx++ {
			entry := trace[entryIdx]
			prev := trace[entryIdx-1]
			var delta int
			if entry.Day == prev.Day {
				delta = entry.Tick - prev.Tick
			} else {
				delta = DayTickMax - prev.Tick + entry.Tick
			}
			// Cleanup boundary: binary fast-forwards dayTick 2000 -> 1500.
			if prev.Fire != nil && prev.Fire.FireActive && entry.Fire != nil && !entry.Fire.FireActive &&
				entry.Day == prev.Day && entry.Tick >= 1500 {
				delta += 500
			}
			for i := 0; i < delta; i++ {
				seeded.Step()
				// PORT-NOTE: the TS reads saveState().time.dayTick (a full
				// deep copy per tick); Time() is the same value.
				if !declineSubmitted && seeded.Time().DayTick >= declineAtTick {
					seeded.SubmitCommand(SimCommand{Type: CmdPromptResponse, PromptID: "fire_" + strconv.Itoa(seed.Day), Accepted: false})
					declineSubmitted = true
				}
			}
			ctx := fmt.Sprintf("entry %d day=%d tick=%d", entryIdx, entry.Day, entry.Tick)
			ourFire := &seeded.World().EventState
			refFire := entry.Fire
			if refFire == nil {
				t.Fatalf("trace entry %d missing fire", entryIdx)
			}
			check := func(got, want int, what string) {
				if got != want {
					t.Fatalf("%s at %s: got %d want %d", what, ctx, got, want)
				}
			}
			check(ourFire.GameStateFlags, refFire.Flags, "gameStateFlags")
			check(ourFire.FireFloor, refFire.FireFloor, "fireFloor")
			check(ourFire.FireTile, refFire.FireTile, "fireTile")
			check(ourFire.FireStartTick, refFire.FireStartTick, "fireStartTick")
			check(ourFire.HelicopterExtinguishPos, refFire.HelicopterExtinguishPos, "helicopterExtinguishPos")
			wantLeft, wantRight := refFire.FireLeftPos, refFire.FireRightPos
			if wantLeft == nil {
				wantLeft = map[string]int{}
			}
			if wantRight == nil {
				wantRight = map[string]int{}
			}
			if got := sparseFronts(ourFire.FireLeftPos); !reflect.DeepEqual(got, wantLeft) {
				t.Fatalf("fireLeftPos at %s: got %v want %v", ctx, got, wantLeft)
			}
			if got := sparseFronts(ourFire.FireRightPos); !reflect.DeepEqual(got, wantRight) {
				t.Fatalf("fireRightPos at %s: got %v want %v", ctx, got, wantRight)
			}
		}
		if seeded.SimTime() <= baseTotal {
			t.Fatal("test never stepped")
		}

		// Post-fire coherence smoke check.
		for i := 0; i < 200; i++ {
			seeded.Step()
		}
		post := seeded.SaveState()
		if post.World.EventState.GameStateFlags&8 != 0 {
			t.Fatal("fire flag re-armed after cleanup")
		}
		if len(post.World.EventState.FireRescueHelpers) != 0 {
			t.Fatal("firefighter helpers not drained at cleanup")
		}
		for _, s := range post.World.Sims {
			if s.FamilyCode == 0 {
				continue
			}
			// PORT-NOTE: the TS compares the placed-object key's y against
			// sim.floorAnchor (a floor index, not a y); kept verbatim.
			objY := s.FloorAnchor
			var home *PlacedObjectRecord
			for _, e := range post.World.PlacedObjects.Entries() {
				_, ky := parseKeyXY(e.Key)
				if ky != objY {
					continue
				}
				if s.HomeColumn >= e.Val.LeftTileIndex && s.HomeColumn <= e.Val.RightTileIndex {
					home = e.Val
				}
			}
			if home == nil {
				continue
			}
			if home.ObjectTypeCode != s.FamilyCode {
				t.Fatalf("orphaned sim post-fire: family=%d home=%d,%d", s.FamilyCode, s.FloorAnchor, s.HomeColumn)
			}
		}
		if post.Ledger.CashBalance < 0 {
			t.Fatalf("cash went negative post-fire: %d", post.Ledger.CashBalance)
		}
	})
}

func TestTraceFireAccept(t *testing.T) {
	if !wantFixture("fire_accept") {
		t.Skip("not selected")
	}
	runGuarded(t, "fire_accept", func() {
		spec := loadBuildSpec(t, "fire_accept")
		trace := readAllTrace(t, "fire_accept")
		if len(trace) == 0 {
			return
		}
		sim := Create("trace-fire-accept", "Trace Fire Accept", LobbyModePerfectParity)
		placeTilesFromSpec(t, sim, spec)
		seed := trace[0]
		seeded := FromSnapshot(seedFireSnapshot(t, sim, seed, false))

		acceptAt := seed.Fire.FireStartTick + 2
		accepted := false
		cashBefore := seeded.Cash()
		for i := 0; i < 250 && !accepted; i++ {
			seeded.Step()
			if seeded.Time().DayTick >= acceptAt {
				seeded.SubmitCommand(SimCommand{Type: CmdPromptResponse, PromptID: "fire_" + strconv.Itoa(seed.Day), Accepted: true})
				accepted = true
			}
		}
		if !accepted {
			t.Fatal("never submitted accept")
		}
		if cashBefore-seeded.Cash() != 500_000 {
			t.Fatalf("expected $500K rescue deduction, got %d", cashBefore-seeded.Cash())
		}
		if seeded.World().EventState.HelicopterExtinguishPos <= 0 {
			t.Fatal("helicopterExtinguishPos not set on accept")
		}
		for i := 0; i < 2500; i++ {
			seeded.Step()
		}
		if seeded.World().EventState.GameStateFlags&8 != 0 {
			t.Fatal("fire flag still set after helicopter extinguish + cleanup")
		}
	})
}

// Harness self-checks that do not need the simulation.
func TestTraceHarnessHelpers(t *testing.T) {
	if traceTickToTotalTicks(0, NewGameDayTick) != 0 || traceTickToTotalTicks(0, 0) != DayTickMax-NewGameDayTick {
		t.Fatal("day 0 ticks")
	}
	if traceTickToTotalTicks(1, DayTickNewDay) != DayTickMax-NewGameDayTick+DayTickNewDay {
		t.Fatal("day 1 start")
	}
	if computeRngState(1, 1) != 0x15a4e36 {
		t.Fatal("rng state")
	}
	if floorDiv(7, 2) != 3 || floorDiv(-7, 2) != -4 {
		t.Fatal("floorDiv")
	}
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Skipf("fixtures not present: %v", err)
	}
	r := openTrace(t, "condo")
	defer r.Close()
	e := r.next(t)
	if e == nil || e.Day != 0 || e.Tick != 2533 || e.RngCalls == nil || *e.RngCalls != 12 ||
		!reflect.DeepEqual(e.simKeyOrder, []string{"office", "condo"}) || e.Sims["condo"].States["32"] != 9 {
		t.Fatalf("first condo entry %+v", e)
	}
	e2 := r.next(t)
	if withoutTimeEqual(e.raw, e2.raw) != true {
		t.Fatal("entries 0 and 1 of build_condo differ only by tick")
	}
}
