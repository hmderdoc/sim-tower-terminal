package sim

import (
	"encoding/json"
	"fmt"
)

// Port of reference/tower-together/sim/snapshot.ts: snapshot creation,
// normalization, hydration and persistence cloning.
//
// PORT-NOTE: legacy-version migrations are skipped (PORT.md "Save
// format"): snake_case -> camelCase renames, starCount-in-time, the
// primary/secondary/tertiaryLedger renames, occupiableFlag / stayPhase /
// pairingStatus / variantIndex / pairingPendingFlag placed-object renames,
// evalEntityIndex / securityAdequate / securityLedgerScale gate renames,
// forwardBudget/reverseBudget, the flat floor-queue format, the
// `entities` -> `sims` rename, the pre-`world` flat snapshot shape, and the
// `??=` field defaults for individual numeric/boolean fields (a typed Go
// struct cannot tell "missing" from zero; our saves always carry every
// field). Kept: nil-section defaults, array-length repairs, the VIP tile
// normalization, metro gate recompute, ring normalization, and the hydrate
// rebuild calls.

// SimSnapshot TS: snapshot.ts SimSnapshot. This is the save format.
type SimSnapshot struct {
	Time   *TimeState   `json:"time"`
	World  *WorldState  `json:"world"`
	Ledger *LedgerState `json:"ledger"`
}

// TS: snapshot.ts createEmptySpecialLinks
func createEmptySpecialLinks() []*SpecialLinkSegment {
	out := make([]*SpecialLinkSegment, MaxSpecialLinks)
	for i := range out {
		out[i] = &SpecialLinkSegment{}
	}
	return out
}

// TS: snapshot.ts createEmptySpecialLinkRecords
func createEmptySpecialLinkRecords() []*SpecialLinkRecord {
	out := make([]*SpecialLinkRecord, MaxSpecialLinkRecords)
	for i := range out {
		out[i] = &SpecialLinkRecord{ReachabilityMasksByFloor: make([]int, GridHeight)}
	}
	return out
}

// TS: snapshot.ts createEmptyTransferGroupEntries
func createEmptyTransferGroupEntries() []*TransferGroupEntry {
	out := make([]*TransferGroupEntry, MaxTransferGroups)
	for i := range out {
		out[i] = &TransferGroupEntry{TaggedFloor: -1}
	}
	return out
}

// newEmptyWorld builds the WorldState literal shared by
// createInitialSnapshot and the normalizeSnapshot no-world branch.
func newEmptyWorld(towerID, name string, lobbyMode LobbyMode) *WorldState {
	return &WorldState{
		TowerID:                  towerID,
		Name:                     name,
		Width:                    GridWidth,
		Height:                   GridHeight,
		LobbyHeight:              1,
		LobbyMode:                lobbyMode,
		GateFlags:                CreateGateFlags(),
		Sidecars:                 SidecarList{},
		Sims:                     []*SimRecord{},
		Carriers:                 []*CarrierRecord{},
		SpecialLinks:             createEmptySpecialLinks(),
		SpecialLinkRecords:       createEmptySpecialLinkRecords(),
		FloorWalkabilityFlags:    make([]int, GridHeight),
		TransferGroupEntries:     createEmptyTransferGroupEntries(),
		TransferGroupCache:       make([]int, GridHeight),
		ParkingDemandLog:         []int{},
		MedicalServiceSlots:      CreateMedicalServiceSlots(),
		StarCount:                1,
		CurrentPopulation:        0,
		CurrentPopulationBuckets: map[int]int{},
		CommercialVenueBuckets:   CreateCommercialVenueBuckets(),
		RngState:                 1,
		RngCallCount:             0,
		EventState:               CreateEventState(),
		PendingNotifications:     []SimNotification{},
		PendingPrompts:           []SimPrompt{},
	}
}

// CreateInitialSnapshot TS: snapshot.ts createInitialSnapshot. An empty
// lobbyMode means the TS default (DEFAULT_LOBBY_MODE).
func CreateInitialSnapshot(towerID, name string, startingCash int, lobbyMode LobbyMode) *SimSnapshot {
	if lobbyMode == "" {
		lobbyMode = DefaultLobbyMode
	}
	return &SimSnapshot{
		Time:   CreateNewGameTimeState(),
		World:  newEmptyWorld(towerID, name, lobbyMode),
		Ledger: CreateLedgerState(startingCash),
	}
}

// TS: snapshot.ts normalizeLegacyTileNames
func normalizeLegacyTileNames(snapshot *SimSnapshot) {
	for _, e := range snapshot.World.Cells.Entries() {
		if alias, ok := LegacyTileAliases[e.Val]; ok {
			snapshot.World.Cells.Set(e.Key, alias)
		}
	}
}

// NormalizeSnapshot repairs a decoded snapshot into the current layout.
// TS: snapshot.ts normalizeSnapshot
func NormalizeSnapshot(snapshot *SimSnapshot) *SimSnapshot {
	if snapshot.World == nil {
		// PORT-NOTE: the TS rebuilds the world from a pre-`world` flat legacy
		// snapshot; we only create an empty perfect-parity world.
		snapshot.World = newEmptyWorld("", "", LobbyModePerfectParity)
		snapshot.World.SpecialLinks = nil
		snapshot.World.SpecialLinkRecords = nil
		snapshot.World.FloorWalkabilityFlags = nil
		snapshot.World.TransferGroupEntries = nil
		snapshot.World.TransferGroupCache = nil
	}
	if snapshot.World.CommercialVenueBuckets.Retail == nil {
		snapshot.World.CommercialVenueBuckets = CreateCommercialVenueBuckets()
	}
	if snapshot.Ledger == nil {
		snapshot.Ledger = CreateLedgerState(2_000_000)
	}
	if snapshot.Time == nil {
		snapshot.Time = &TimeState{}
	}

	normalizeLegacyTileNames(snapshot)

	w := snapshot.World
	if w.Height < GridHeight {
		w.Height = GridHeight
	}
	if w.Width == 0 || w.Width < GridWidth {
		w.Width = GridWidth
	}
	if w.LobbyMode == "" {
		// Pre-existing snapshots predate lobbyMode: default to perfect-parity.
		w.LobbyMode = LobbyModePerfectParity
	}
	if w.Sidecars == nil {
		w.Sidecars = SidecarList{}
	}
	if w.Sims == nil {
		w.Sims = []*SimRecord{}
	}
	if w.Carriers == nil {
		w.Carriers = []*CarrierRecord{}
	}
	if len(w.SpecialLinks) == 0 {
		w.SpecialLinks = createEmptySpecialLinks()
	}
	if len(w.SpecialLinkRecords) == 0 {
		w.SpecialLinkRecords = createEmptySpecialLinkRecords()
	}
	if len(w.FloorWalkabilityFlags) != GridHeight {
		w.FloorWalkabilityFlags = make([]int, GridHeight)
	}
	if len(w.TransferGroupEntries) == 0 {
		w.TransferGroupEntries = createEmptyTransferGroupEntries()
	}
	if len(w.TransferGroupCache) != GridHeight {
		w.TransferGroupCache = make([]int, GridHeight)
	}
	if w.CurrentPopulationBuckets == nil {
		w.CurrentPopulationBuckets = map[int]int{}
	}
	if w.EventState.FireRescueHelpers == nil {
		w.EventState.FireRescueHelpers = []*FireRescueHelper{}
	}
	if len(w.EventState.FireLeftPos) == 0 && len(w.EventState.FireRightPos) == 0 {
		// whole eventState missing (TS `eventState ??= createEventState()`)
		w.EventState = CreateEventState()
	}
	l := snapshot.Ledger
	if l.PopulationLedger == nil {
		l.PopulationLedger = make([]int, 256)
	}
	if l.IncomeLedger == nil {
		l.IncomeLedger = make([]int, 256)
	}
	if l.ExpenseLedger == nil {
		l.ExpenseLedger = make([]int, 256)
	}
	if len(w.MedicalServiceSlots) == 0 {
		w.MedicalServiceSlots = CreateMedicalServiceSlots()
	}

	// Legacy VIP tiles -> standard hotel tiles + vipFlag.
	vipAnchors := map[string]bool{}
	for _, e := range w.Cells.Entries() {
		standardTile, ok := LegacyVipTileToStandard[e.Val]
		if !ok {
			continue
		}
		w.Cells.Set(e.Key, standardTile)
		anchorKey := w.CellToAnchor.At(e.Key)
		if anchorKey == "" {
			anchorKey = e.Key
		}
		vipAnchors[anchorKey] = true
	}
	for _, e := range w.PlacedObjects.Entries() {
		if vipAnchors[e.Key] {
			record := e.Val
			switch record.ObjectTypeCode {
			case 31:
				record.ObjectTypeCode = 3
			case 32:
				record.ObjectTypeCode = 4
			case 33:
				record.ObjectTypeCode = 5
			}
			record.VipFlag = true
		}
	}
	w.GateFlags.MetroPlaced = 0
	w.GateFlags.MetroStationFloorIndex = -1
	for _, e := range w.PlacedObjects.Entries() {
		if e.Val.ObjectTypeCode != FamilyMetroTop {
			continue
		}
		_, y := parseKeyXY(e.Key)
		w.GateFlags.MetroPlaced = 1
		w.GateFlags.MetroStationFloorIndex = GridHeight - 1 - y
		break
	}

	// Floor queues: JSON gives plain {items, head, count}; normalize through
	// RouteRequestRing.from like the TS "already new shape" branch.
	for _, carrier := range w.Carriers {
		for i, q := range carrier.FloorQueues {
			if q == nil {
				continue
			}
			nq := &CarrierFloorQueue{}
			if q.Up != nil {
				nq.Up = RouteRequestRingFrom(q.Up.Items, q.Up.Head, q.Up.Count)
			} else {
				nq.Up = NewRouteRequestRing()
			}
			if q.Down != nil {
				nq.Down = RouteRequestRingFrom(q.Down.Items, q.Down.Head, q.Down.Count)
			} else {
				nq.Down = NewRouteRequestRing()
			}
			carrier.FloorQueues[i] = nq
		}
	}

	return snapshot
}

// HydrateSnapshot normalizes and rebuilds derived tables.
// TS: snapshot.ts hydrateSnapshot
func HydrateSnapshot(raw *SimSnapshot) *SimSnapshot {
	snapshot := NormalizeSnapshot(raw)

	if snapshot.World.Height < GridHeight {
		snapshot.World.Height = GridHeight
	}

	InitCarrierState(snapshot.World)
	for _, carrier := range snapshot.World.Carriers {
		if carrier.CompletedRouteIDs == nil {
			carrier.CompletedRouteIDs = []string{}
		}
		if carrier.SuppressedFloorAssignments == nil {
			carrier.SuppressedFloorAssignments = []string{}
		}
		numSlots := max(0, carrier.TopServedFloor-carrier.BottomServedFloor+1)
		if len(carrier.StopFloorEnabled) != numSlots {
			carrier.StopFloorEnabled = filledInts(numSlots, 1)
		}
		if len(carrier.DwellDelay) != 14 {
			carrier.DwellDelay = make([]int, 14)
		}
		for _, car := range carrier.Cars {
			if car.DestinationCountByFloor == nil {
				car.DestinationCountByFloor = make([]int, numSlots)
			}
			if car.ActiveRouteSlots == nil {
				car.ActiveRouteSlots = []*CarrierRouteSlot{}
			}
		}
	}

	if snapshot.World.ParkingDemandLog == nil {
		snapshot.World.ParkingDemandLog = []int{}
	}

	RebuildSpecialLinkRouteRecords(snapshot.World)
	RebuildRouteReachabilityTables(snapshot.World)
	RebuildTransferGroupCache(snapshot.World)
	RebuildRuntimeSims(snapshot.World)

	return snapshot
}

// CloneSnapshot deep-copies a snapshot through its JSON form (the TS uses
// structuredClone / JSON.parse(JSON.stringify(...))). Order of every
// OrderedMap is preserved.
func CloneSnapshot(snap *SimSnapshot) *SimSnapshot {
	b, err := json.Marshal(snap)
	if err != nil {
		panic(fmt.Sprintf("sim: snapshot marshal: %v", err))
	}
	var out SimSnapshot
	if err := json.Unmarshal(b, &out); err != nil {
		panic(fmt.Sprintf("sim: snapshot unmarshal: %v", err))
	}
	return &out
}

// SerializeSimState returns a detached deep copy of the live state with the
// pending notification/prompt queues emptied.
// TS: snapshot.ts serializeSimState
func SerializeSimState(time *TimeState, world *WorldState, ledger *LedgerState) *SimSnapshot {
	out := CloneSnapshot(&SimSnapshot{Time: time, World: world, Ledger: ledger})
	out.World.PendingNotifications = []SimNotification{}
	out.World.PendingPrompts = []SimPrompt{}
	return out
}
