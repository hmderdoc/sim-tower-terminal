package sim

// Port of reference/tower-together/sim/index.ts: the TowerSim facade.
//
// The facade owns one (*TimeState, *WorldState, *LedgerState) triple. Step
// bundles them into a ServiceIdleTasksContext (the TS `ctx`), runs the tick
// and reads ctx.Time back, because the tick replaces the TimeState pointer.

// CarrierCarStateRecord TS: index.ts CarrierCarStateRecord
type CarrierCarStateRecord struct {
	CarrierID     int  `json:"carrierId"`
	CarIndex      int  `json:"carIndex"`
	CarCount      int  `json:"carCount"`
	Column        int  `json:"column"`
	CarrierMode   int  `json:"carrierMode"`
	CurrentFloor  int  `json:"currentFloor"`
	TargetFloor   int  `json:"targetFloor"`
	SettleCounter int  `json:"settleCounter"`
	DirectionFlag int  `json:"directionFlag"`
	DwellCounter  int  `json:"dwellCounter"`
	AssignedCount int  `json:"assignedCount"`
	PrevFloor     int  `json:"prevFloor"`
	ArrivalSeen   int  `json:"arrivalSeen"`
	ArrivalTick   int  `json:"arrivalTick"`
	HomeFloor     int  `json:"homeFloor"`
	Active        bool `json:"active"`
}

// StepNotification is a drained notification (message defaults to "").
type StepNotification struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// StepResult TS: index.ts StepResult
type StepResult struct {
	SimTime        int                `json:"simTime"`
	EconomyChanged bool               `json:"economyChanged"`
	CellPatches    []CellPatch        `json:"cellPatches"`
	Notifications  []StepNotification `json:"notifications"`
	Prompts        []SimPrompt        `json:"prompts"`
}

// TowerSim TS: index.ts TowerSim
type TowerSim struct {
	time      *TimeState
	world     *WorldState
	ledger    *LedgerState
	FreeBuild bool
}

func newTowerSim(time *TimeState, world *WorldState, ledger *LedgerState) *TowerSim {
	return &TowerSim{time: time, world: world, ledger: ledger}
}

// ── Factory methods ────────────────────────────────────────────────────────

// Create builds a new tower. Empty lobbyMode = DEFAULT_LOBBY_MODE.
// TS: index.ts TowerSim.create
func Create(towerID, name string, lobbyMode LobbyMode) *TowerSim {
	return FromSnapshot(CreateInitialSnapshot(towerID, name, StartingCash, lobbyMode))
}

// FromSnapshot deep-clones snap (TS structuredClone) so later steps never
// bleed into the caller's snapshot, then hydrates it.
// TS: index.ts TowerSim.fromSnapshot
func FromSnapshot(snap *SimSnapshot) *TowerSim {
	hydrated := HydrateSnapshot(CloneSnapshot(snap))
	return newTowerSim(hydrated.Time, hydrated.World, hydrated.Ledger)
}

// ── Read access for the door ───────────────────────────────────────────────

// World returns the live world (do not retain across Step if you need a
// stable view; it is mutated in place).
func (s *TowerSim) World() *WorldState { return s.world }

// Ledger returns the live ledger.
func (s *TowerSim) Ledger() *LedgerState { return s.ledger }

// Time returns the current time state. Step replaces it with a new pointer.
func (s *TowerSim) Time() *TimeState { return s.time }

// ── Tick ──────────────────────────────────────────────────────────────────

// Step advances one tick. TS: index.ts TowerSim.step
func (s *TowerSim) Step() StepResult {
	balanceBefore := s.ledger.CashBalance

	// Snapshot display-facing room fields before tick to detect changes.
	type before struct{ occupied, unitStatus, evalLevel, evalScore int }
	prev := make(map[string]before, s.world.PlacedObjects.Len())
	for _, e := range s.world.PlacedObjects.Entries() {
		r := e.Val
		prev[e.Key] = before{r.OccupiedFlag, r.UnitStatus, r.EvalLevel, r.EvalScore}
	}

	// Binary 1268:01a6 service_idle_tasks: day scheduler + carrier tick.
	ctx := &ServiceIdleTasksContext{World: s.world, Ledger: s.ledger, Time: s.time}
	ServiceIdleTasks(ctx)
	s.time = ctx.Time

	cellPatches := []CellPatch{}
	for _, e := range s.world.PlacedObjects.Entries() {
		record := e.Val
		p, ok := prev[e.Key]
		if !ok {
			continue
		}
		if p.occupied != record.OccupiedFlag || p.unitStatus != record.UnitStatus ||
			p.evalLevel != record.EvalLevel || p.evalScore != record.EvalScore {
			x, y := parseKeyXY(e.Key)
			cellPatches = append(cellPatches, CellPatch{
				X:              x,
				Y:              y,
				TileType:       s.world.Cells.At(e.Key),
				IsAnchor:       true,
				EvalActiveFlag: intPtr(record.OccupiedFlag),
				UnitStatus:     intPtr(record.UnitStatus),
				EvalLevel:      intPtr(record.EvalLevel),
				EvalScore:      intPtr(record.EvalScore),
			})
		}
	}

	notifications := s.DrainNotifications()
	prompts := s.DrainPrompts()

	return StepResult{
		SimTime:        s.time.TotalTicks,
		EconomyChanged: s.ledger.CashBalance != balanceBefore,
		CellPatches:    cellPatches,
		Notifications:  notifications,
		Prompts:        prompts,
	}
}

// ── Commands ──────────────────────────────────────────────────────────────

// SubmitCommand dispatches a command. TS: index.ts TowerSim.submitCommand
func (s *TowerSim) SubmitCommand(cmd SimCommand) CommandResult {
	switch cmd.Type {
	case CmdPlaceTile:
		return HandlePlaceTile(cmd.X, cmd.Y, cmd.TileType, s.world, s.ledger, s.FreeBuild, s.time)
	case CmdRemoveTile:
		return HandleRemoveTile(cmd.X, cmd.Y, s.world, s.ledger)
	case CmdPromptResponse:
		HandlePromptResponse(s.world, s.ledger, s.time, cmd.PromptID, cmd.Accepted)
		return CommandResult{Accepted: true, Patch: []CellPatch{}, EconomyChanged: true}
	case CmdSetRentLevel:
		return HandleSetRentLevel(cmd.X, cmd.Y, cmd.RentLevel, s.world, s.time)
	case CmdAddElevatorCar:
		return HandleAddElevatorCar(cmd.X, cmd.Y, s.world, s.ledger, s.FreeBuild)
	case CmdRemoveElevatorCar:
		return HandleRemoveElevatorCar(cmd.X, cmd.Y, s.world)
	case CmdSetElevatorDwellDelay:
		return HandleSetElevatorDwellDelay(cmd.X, cmd.Y, cmd.Value, s.world)
	case CmdSetElevatorWaitingCarResponse:
		return HandleSetElevatorWaitingCarResponse(cmd.X, cmd.Y, cmd.Value, s.world)
	case CmdSetElevatorHomeFloor:
		return HandleSetElevatorHomeFloor(cmd.X, cmd.CarIndex, cmd.Floor, s.world)
	case CmdToggleElevatorFloorStop:
		return HandleToggleElevatorFloorStop(cmd.X, cmd.Floor, s.world)
	case CmdSetCinemaMoviePool:
		return HandleSetCinemaMoviePool(cmd.X, cmd.Y, cmd.Pool, s.world, s.ledger, s.FreeBuild)
	}
	// PORT-NOTE: the TS switch is exhaustive over the union (returns
	// undefined otherwise); Go rejects unknown types explicitly.
	return rejected("Unknown command type")
}

// ── Cell inspection ──────────────────────────────────────────────────────────

// ObjectInfo is QueryCellResult.ObjectInfo.
type ObjectInfo struct {
	ObjectTypeCode          int  `json:"objectTypeCode"`
	RentLevel               int  `json:"rentLevel"`
	EvalLevel               int  `json:"evalLevel"`
	UnitStatus              int  `json:"unitStatus"`
	ActivationTickCount     int  `json:"activationTickCount"`
	VenueAvailability       *int `json:"venueAvailability,omitempty"`
	HousekeepingClaimedFlag int  `json:"housekeepingClaimedFlag"`
}

// CinemaInfo is QueryCellResult.CinemaInfo.
type CinemaInfo struct {
	Selector          int `json:"selector"`
	LinkAgeCounter    int `json:"linkAgeCounter"`
	AttendanceCounter int `json:"attendanceCounter"`
	LinkPhaseState    int `json:"linkPhaseState"`
}

// CarInfo is one entry of CarrierInfo.CarInfos.
type CarInfo struct {
	HomeFloor int  `json:"homeFloor"`
	Active    bool `json:"active"`
}

// CarrierInfo is QueryCellResult.CarrierInfo.
type CarrierInfo struct {
	CarrierID                   int       `json:"carrierId"`
	Column                      int       `json:"column"`
	CarrierMode                 int       `json:"carrierMode"`
	TopServedFloor              int       `json:"topServedFloor"`
	BottomServedFloor           int       `json:"bottomServedFloor"`
	CarCount                    int       `json:"carCount"`
	MaxCars                     int       `json:"maxCars"`
	ServedFloors                []int     `json:"servedFloors"`
	DwellDelay                  int       `json:"dwellDelay"`
	WaitingCarResponseThreshold int       `json:"waitingCarResponseThreshold"`
	StopFloorEnabled            []bool    `json:"stopFloorEnabled"`
	CarInfos                    []CarInfo `json:"carInfos"`
}

// QueryCellResult TS: index.ts queryCell return type.
type QueryCellResult struct {
	AnchorX     int          `json:"anchorX"`
	TileType    string       `json:"tileType"`
	ObjectInfo  *ObjectInfo  `json:"objectInfo,omitempty"`
	CinemaInfo  *CinemaInfo  `json:"cinemaInfo,omitempty"`
	CarrierInfo *CarrierInfo `json:"carrierInfo,omitempty"`
}

// QueryCell TS: index.ts TowerSim.queryCell
func (s *TowerSim) QueryCell(x, y int) QueryCellResult {
	key := cellKey(x, y)
	anchorKey := s.world.CellToAnchor.At(key)
	if anchorKey == "" {
		anchorKey = key
	}
	tileType := s.world.Cells.At(anchorKey)
	if tileType == "" {
		tileType = "empty"
	}

	record := s.world.PlacedObjects.At(anchorKey)
	var venueAvailability *int
	var cinemaInfo *CinemaInfo
	if record != nil && record.LinkedRecordIndex >= 0 {
		switch sc := GetSidecar(s.world, record.LinkedRecordIndex).(type) {
		case *CommercialVenueRecord:
			venueAvailability = intPtr(sc.AvailabilityState)
		case *EntertainmentLinkRecord:
			if sc.FamilySelectorOrSingleLinkFlag != 0xff {
				cinemaInfo = &CinemaInfo{
					Selector:          sc.FamilySelectorOrSingleLinkFlag,
					LinkAgeCounter:    sc.LinkAgeCounter,
					AttendanceCounter: sc.AttendanceCounter,
					LinkPhaseState:    sc.LinkPhaseState,
				}
			}
		}
	}
	var objectInfo *ObjectInfo
	if record != nil {
		objectInfo = &ObjectInfo{
			ObjectTypeCode:          record.ObjectTypeCode,
			RentLevel:               record.RentLevel,
			EvalLevel:               record.EvalLevel,
			UnitStatus:              record.UnitStatus,
			ActivationTickCount:     record.ActivationTickCount,
			VenueAvailability:       venueAvailability,
			HousekeepingClaimedFlag: record.HousekeepingClaimedFlag,
		}
	}

	// Check for carrier at this column (elevator overlays)
	overlayKey := s.world.OverlayToAnchor.At(key)
	if overlayKey == "" && hasStr(&s.world.Overlays, key) {
		overlayKey = key
	}
	var carrierInfo *CarrierInfo
	if overlayKey != "" {
		col, _ := parseKeyXY(overlayKey)
		queriedFloor := YToFloor(y)
		var carrier *CarrierRecord
		for _, c := range s.world.Carriers {
			if c.Column == col && queriedFloor >= c.BottomServedFloor && queriedFloor <= c.TopServedFloor {
				carrier = c
				break
			}
		}
		if carrier != nil {
			servedFloors := []int{}
			stopFloorEnabled := []bool{}
			for f := carrier.BottomServedFloor; f <= carrier.TopServedFloor; f++ {
				slot := FloorToSlot(carrier, f)
				if slot >= 0 {
					servedFloors = append(servedFloors, f)
					v := 1
					if slot < len(carrier.StopFloorEnabled) {
						v = carrier.StopFloorEnabled[slot]
					}
					stopFloorEnabled = append(stopFloorEnabled, v != 0)
				}
			}
			dwell := 0
			if len(carrier.DwellDelay) > 0 {
				dwell = carrier.DwellDelay[0]
			}
			carInfos := make([]CarInfo, len(carrier.Cars))
			for i, car := range carrier.Cars {
				carInfos[i] = CarInfo{HomeFloor: car.HomeFloor, Active: car.Active}
			}
			carrierInfo = &CarrierInfo{
				CarrierID:                   carrier.CarrierID,
				Column:                      col,
				CarrierMode:                 carrier.CarrierMode,
				TopServedFloor:              carrier.TopServedFloor,
				BottomServedFloor:           carrier.BottomServedFloor,
				CarCount:                    len(activeCars(carrier)),
				MaxCars:                     8,
				ServedFloors:                servedFloors,
				DwellDelay:                  dwell,
				WaitingCarResponseThreshold: carrier.WaitingCarResponseThreshold,
				StopFloorEnabled:            stopFloorEnabled,
				CarInfos:                    carInfos,
			}
		}
	}

	anchorX, _ := parseKeyXY(anchorKey)
	return QueryCellResult{
		AnchorX:     anchorX,
		TileType:    tileType,
		ObjectInfo:  objectInfo,
		CinemaInfo:  cinemaInfo,
		CarrierInfo: carrierInfo,
	}
}

// ── Serialization ──────────────────────────────────────────────────────────

// SaveState returns a detached snapshot (the save format).
// TS: index.ts TowerSim.saveState
func (s *TowerSim) SaveState() *SimSnapshot {
	return SerializeSimState(s.time, s.world, s.ledger)
}

// DrainNotifications TS: index.ts TowerSim.drainNotifications
func (s *TowerSim) DrainNotifications() []StepNotification {
	out := make([]StepNotification, len(s.world.PendingNotifications))
	for i, n := range s.world.PendingNotifications {
		out[i] = StepNotification{Kind: n.Kind, Message: n.Message}
	}
	s.world.PendingNotifications = []SimNotification{}
	return out
}

// DrainPrompts TS: index.ts TowerSim.drainPrompts
func (s *TowerSim) DrainPrompts() []SimPrompt {
	out := s.world.PendingPrompts
	if out == nil {
		out = []SimPrompt{}
	}
	s.world.PendingPrompts = []SimPrompt{}
	return out
}

// ── Accessors ──────────────────────────────────────────────────────────────

func (s *TowerSim) TowerID() string { return s.world.TowerID }
func (s *TowerSim) Name() string    { return s.world.Name }
func (s *TowerSim) SimTime() int    { return s.time.TotalTicks }
func (s *TowerSim) Cash() int       { return s.ledger.CashBalance }

// Population sums the population ledger. TS: index.ts get population
func (s *TowerSim) Population() int {
	total := 0
	for _, v := range s.ledger.PopulationLedger {
		total += v
	}
	return total
}

func (s *TowerSim) CurrentPopulation() int { return s.world.CurrentPopulation }
func (s *TowerSim) StarCount() int         { return s.world.StarCount }

// SetStarCount (1..6). TS: index.ts setStarCount
func (s *TowerSim) SetStarCount(starCount int) { s.world.StarCount = starCount }

func (s *TowerSim) DaypartIndex() int     { return s.time.DaypartIndex }
func (s *TowerSim) GateFlags() *GateFlags { return &s.world.GateFlags }
func (s *TowerSim) RngCallCount() int     { return s.world.RngCallCount }
func (s *TowerSim) Width() int            { return s.world.Width }
func (s *TowerSim) Height() int           { return s.world.Height }

// CellsToArray TS: index.ts TowerSim.cellsToArray
func (s *TowerSim) CellsToArray() []CellPatch {
	result := []CellPatch{}
	for _, e := range s.world.Cells.Entries() {
		key, tileType := e.Key, e.Val
		x, y := parseKeyXY(key)
		isAnchor := !hasStr(&s.world.CellToAnchor, key)
		var record *PlacedObjectRecord
		if isAnchor {
			record = s.world.PlacedObjects.At(key)
		}
		var coverageFlag *int
		if record != nil && tileType == "parking" && record.LinkedRecordIndex >= 0 {
			if sr, ok := GetSidecar(s.world, record.LinkedRecordIndex).(*ServiceRequestEntry); ok {
				cov := 0
				if sr.CoverageFlag != nil {
					cov = *sr.CoverageFlag
				}
				coverageFlag = &cov
			}
		}
		p := CellPatch{X: x, Y: y, TileType: tileType, IsAnchor: isAnchor, CoverageFlag: coverageFlag}
		if record != nil {
			p.EvalActiveFlag = intPtr(record.OccupiedFlag)
			p.UnitStatus = intPtr(record.UnitStatus)
			p.EvalLevel = intPtr(record.EvalLevel)
			p.EvalScore = intPtr(record.EvalScore)
		}
		result = append(result, p)
	}
	for _, e := range s.world.Overlays.Entries() {
		x, y := parseKeyXY(e.Key)
		result = append(result, CellPatch{X: x, Y: y, TileType: e.Val, IsAnchor: true, IsOverlay: true})
	}
	return result
}

// SimsToArray TS: index.ts TowerSim.simsToArray
func (s *TowerSim) SimsToArray() []*SimStateRecord {
	return CreateSimStateRecords(s.world, s.time)
}

// SimToRecord TS: index.ts TowerSim.simToRecord
func (s *TowerSim) SimToRecord(sim *SimRecord) *SimStateRecord {
	return CreateSimStateRecord(s.world, s.time, sim, BuildPendingBySimID(s.world))
}

// LiveSims TS: index.ts get liveSims (live slice; do not mutate).
func (s *TowerSim) LiveSims() []*SimRecord { return s.world.Sims }

// LiveCarriers TS: index.ts get liveCarriers (live slice; do not mutate).
func (s *TowerSim) LiveCarriers() []*CarrierRecord { return s.world.Carriers }

// FireFronts returns the per-floor fire fronts (0xffff = inactive).
// TS: index.ts get fireFronts
func (s *TowerSim) FireFronts() (fireLeftPos, fireRightPos []int) {
	return s.world.EventState.FireLeftPos, s.world.EventState.FireRightPos
}

// CarriersToArray TS: index.ts TowerSim.carriersToArray
func (s *TowerSim) CarriersToArray() []CarrierCarStateRecord {
	var out []CarrierCarStateRecord
	for _, carrier := range s.world.Carriers {
		for carIndex, car := range carrier.Cars {
			out = append(out, CarrierCarStateRecord{
				CarrierID:     carrier.CarrierID,
				CarIndex:      carIndex,
				CarCount:      len(carrier.Cars),
				Column:        carrier.Column,
				CarrierMode:   carrier.CarrierMode,
				CurrentFloor:  car.CurrentFloor,
				TargetFloor:   car.TargetFloor,
				SettleCounter: car.SettleCounter,
				DirectionFlag: car.DirectionFlag,
				DwellCounter:  car.DwellCounter,
				AssignedCount: car.AssignedCount,
				PrevFloor:     car.PrevFloor,
				ArrivalSeen:   car.ArrivalSeen,
				ArrivalTick:   car.ArrivalTick,
				HomeFloor:     car.HomeFloor,
				Active:        car.Active,
			})
		}
	}
	return out
}
