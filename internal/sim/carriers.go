package sim

import "sort"

// Port of reference/tower-together/sim/carriers.ts: the carrier module hub.
//
// Retains carrier record / car constructors and world-level lifecycle
// (MakeCarrier, MakeCarrierCar, RebuildCarrierList, InitCarrierState,
// FlushCarriersEndOfDay) and ResetCarrierTickBookkeeping. The TS re-exports
// of the per-car state machine (carriers/*.ts) and queue ops (queue/*.ts)
// need no Go equivalent: everything is in package sim. The two binary-alias
// re-exports are kept as thin functions below.

// EnqueueCarrierRoute is the TS alias `enqueueCarrierRoute =
// enqueueRequestIntoRouteQueue` (binary 1218:1002).
// TS: carriers.ts enqueueCarrierRoute
func EnqueueCarrierRoute(carrier *CarrierRecord, simID string, sourceFloor, destinationFloor, directionFlag int, lobbyMode LobbyMode) bool {
	return EnqueueRequestIntoRouteQueue(carrier, simID, sourceFloor, destinationFloor, directionFlag, lobbyMode)
}

// EvictCarrierRoute is the TS alias `evictCarrierRoute =
// cancelRuntimeRouteRequest` (binary 1218:1a86).
// TS: carriers.ts evictCarrierRoute
func EvictCarrierRoute(carrier *CarrierRecord, simID string) {
	CancelRuntimeRouteRequest(carrier, simID)
}

// TS: carriers.ts createFloorQueue
func createFloorQueue() *CarrierFloorQueue {
	return &CarrierFloorQueue{Up: NewRouteRequestRing(), Down: NewRouteRequestRing()}
}

func newFloorQueues(numSlots int) []*CarrierFloorQueue {
	out := make([]*CarrierFloorQueue, numSlots)
	for i := range out {
		out[i] = createFloorQueue()
	}
	return out
}

func newIdleRouteSlots() []*CarrierRouteSlot {
	out := make([]*CarrierRouteSlot, ActiveSlotCapacity)
	for i := range out {
		out[i] = newIdleRouteSlot()
	}
	return out
}

// MakeCarrierCar TS: carriers.ts makeCarrierCar
func MakeCarrierCar(numSlots int, homeFloor int) *CarrierCar {
	return &CarrierCar{
		Active:                  true,
		CurrentFloor:            homeFloor,
		DirectionFlag:           1,
		TargetFloor:             homeFloor,
		PrevFloor:               homeFloor,
		HomeFloor:               homeFloor,
		NearestWorkFloor:        homeFloor,
		WaitingCount:            make([]int, numSlots),
		DestinationCountByFloor: make([]int, numSlots),
		ActiveRouteSlots:        newIdleRouteSlots(),
		PendingRouteIDs:         []string{},
	}
}

// MakeCarrier builds a fresh carrier record. numCars is clamped to 1..8
// (TS default 1).
// TS: carriers.ts makeCarrier
func MakeCarrier(id, col, mode, bottom, top, numCars int) *CarrierRecord {
	numSlots := top - bottom + 1
	clampedCars := max(1, min(8, numCars))
	// Binary place_carrier_shaft / _add_shaft_cars writes
	// bottom_served_floor to every car's home_floor byte at
	// carrier+0xBA..0xC1; the idle-home test observes this all-bottom layout.
	cars := make([]*CarrierCar, clampedCars)
	for i := range cars {
		cars[i] = MakeCarrierCar(numSlots, bottom)
	}
	assignmentCapacity := 0x15
	if mode == 0 {
		assignmentCapacity = 0x2a
	}
	return &CarrierRecord{
		CarrierID:                   id,
		Column:                      col,
		CarrierMode:                 mode,
		TopServedFloor:              top,
		BottomServedFloor:           bottom,
		ServedFloorFlags:            filledInts(14, 1),
		PrimaryRouteStatusByFloor:   make([]int, numSlots),
		SecondaryRouteStatusByFloor: make([]int, numSlots),
		ServiceScheduleFlags:        filledInts(14, 1),
		DwellDelay:                  make([]int, 14),
		ExpressDirectionFlags:       make([]int, 14),
		WaitingCarResponseThreshold: 5,
		AssignmentCapacity:          assignmentCapacity,
		FloorQueues:                 newFloorQueues(numSlots),
		PendingRoutes:               []*CarrierPendingRoute{},
		CompletedRouteIDs:           []string{},
		SuppressedFloorAssignments:  []string{},
		StopFloorEnabled:            filledInts(numSlots, 1),
		Cars:                        cars,
	}
}

// ResetCarrierTickBookkeeping runs the per-carrier bookkeeping reset
// (completedRouteIds + SyncAssignmentStatus) at the head of each carrier's
// pass in CarrierTick.
// TS: carriers.ts resetCarrierTickBookkeeping
func ResetCarrierTickBookkeeping(carrier *CarrierRecord) {
	carrier.CompletedRouteIDs = []string{}
	for _, car := range carrier.Cars {
		car.ArrivalDispatchThisTick = false
		car.ArrivalDispatchStartingAssignedCount = 0
	}
	SyncAssignmentStatus(carrier)
}

// FlushCarriersEndOfDay is the end-of-day carrier flush (checkpoint 0x9f6).
// Drains all floor queues (count only; heads are kept) and clears pending
// route tracking. Per-car state is left untouched at the day boundary.
// TS: carriers.ts flushCarriersEndOfDay
func FlushCarriersEndOfDay(world *WorldState) {
	for _, carrier := range world.Carriers {
		for _, queue := range carrier.FloorQueues {
			queue.Up.Count = 0
			queue.Down.Count = 0
		}
		carrier.PendingRoutes = []*CarrierPendingRoute{}
		carrier.CompletedRouteIDs = []string{}
	}
}

// RebuildCarrierList rebuilds world.Carriers from the elevator overlays,
// matching each contiguous run to the existing same-column carrier that
// overlaps it most (preserving per-car state).
// TS: carriers.ts rebuildCarrierList
//
// Iteration order: overlayKeys is a JS Set built from Object.keys(overlays)
// then Object.keys(overlayToAnchor) (insertion order, first occurrence
// wins); `columns` is a JS Map keyed by first-seen column. Both are ported
// as slice + membership map. Runs are therefore emitted in first-seen column
// order, and carrierIds follow that order.
func RebuildCarrierList(world *WorldState) {
	type column struct {
		floors   []int
		floorSet map[int]bool
		mode     int
	}
	columnOrder := []int{}
	columns := map[int]*column{}

	overlayKeys := []string{}
	seenKeys := map[string]bool{}
	for _, k := range world.Overlays.Keys() {
		if !seenKeys[k] {
			seenKeys[k] = true
			overlayKeys = append(overlayKeys, k)
		}
	}
	for _, k := range world.OverlayToAnchor.Keys() {
		if !seenKeys[k] {
			seenKeys[k] = true
			overlayKeys = append(overlayKeys, k)
		}
	}

	for _, key := range overlayKeys {
		anchorKey, ok := world.OverlayToAnchor.Get(key)
		if !ok {
			anchorKey = key
		}
		typ := world.Overlays.At(anchorKey)
		var mode int
		switch typ {
		case "elevator":
			mode = 1
		case "elevatorExpress":
			mode = 0
		case "elevatorService":
			mode = 2
		default:
			continue
		}

		x, _ := parseKeyXY(anchorKey)
		_, y := parseKeyXY(key)
		floor := YToFloor(y)

		c, exists := columns[x]
		if !exists {
			c = &column{floorSet: map[int]bool{}, mode: mode}
			columns[x] = c
			columnOrder = append(columnOrder, x)
		}
		if !c.floorSet[floor] {
			c.floorSet[floor] = true
			c.floors = append(c.floors, floor)
		}
	}

	// Split each column into contiguous floor runs. Two elevator overlays in
	// the same column with a vertical gap form two independent carriers.
	type run struct{ col, mode, bottom, top int }
	runs := []run{}
	for _, col := range columnOrder {
		c := columns[col]
		sorted := append([]int(nil), c.floors...)
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		runBottom := sorted[0]
		prev := sorted[0]
		for i := 1; i < len(sorted); i++ {
			f := sorted[i]
			if f == prev+1 {
				prev = f
				continue
			}
			runs = append(runs, run{col, c.mode, runBottom, prev})
			runBottom = f
			prev = f
		}
		runs = append(runs, run{col, c.mode, runBottom, prev})
	}

	newCarriers := []*CarrierRecord{}
	consumed := map[*CarrierRecord]bool{}
	id := 0

	for _, r := range runs {
		col, mode, bottom, top := r.col, r.mode, r.bottom, r.top
		numSlots := top - bottom + 1

		// Match this run to whichever existing same-column carrier overlaps
		// it most; the larger overlap wins on merge/split.
		var existing *CarrierRecord
		bestOverlap := 0
		for _, c := range world.Carriers {
			if c.Column != col || consumed[c] {
				continue
			}
			overlap := min(c.TopServedFloor, top) - max(c.BottomServedFloor, bottom) + 1
			if overlap > bestOverlap {
				existing = c
				bestOverlap = overlap
			}
		}
		if existing != nil {
			consumed[existing] = true
		}

		if existing != nil {
			existing.CarrierID = id
			id++
			existing.CarrierMode = mode
			existing.TopServedFloor = top
			existing.BottomServedFloor = bottom
			// PORT-NOTE: `waitingCarResponseThreshold ??= 5` and
			// `assignmentCapacity ??= ...` are legacy-snapshot defaults on
			// numbers; a typed struct cannot tell missing from zero, and our
			// saves always carry the field (see port_log_a1.md snapshot notes).
			if len(existing.ServedFloorFlags) != 14 {
				existing.ServedFloorFlags = filledInts(14, 1)
			}
			if len(existing.ServiceScheduleFlags) != 14 {
				existing.ServiceScheduleFlags = filledInts(14, 1)
			}
			if len(existing.DwellDelay) != 14 {
				existing.DwellDelay = make([]int, 14)
			}
			if len(existing.ExpressDirectionFlags) != 14 {
				existing.ExpressDirectionFlags = make([]int, 14)
			}
			if existing.CompletedRouteIDs == nil {
				existing.CompletedRouteIDs = []string{}
			}
			if existing.SuppressedFloorAssignments == nil {
				existing.SuppressedFloorAssignments = []string{}
			}
			if len(existing.StopFloorEnabled) != numSlots {
				existing.StopFloorEnabled = filledInts(numSlots, 1)
			}
			if len(existing.PrimaryRouteStatusByFloor) != numSlots {
				existing.PrimaryRouteStatusByFloor = make([]int, numSlots)
				existing.SecondaryRouteStatusByFloor = make([]int, numSlots)
			}
			if len(existing.FloorQueues) != numSlots {
				existing.FloorQueues = newFloorQueues(numSlots)
			}
			for _, car := range existing.Cars {
				if len(car.WaitingCount) != numSlots {
					car.WaitingCount = make([]int, numSlots)
				}
				if len(car.DestinationCountByFloor) != numSlots {
					car.DestinationCountByFloor = make([]int, numSlots)
				}
				if car.ActiveRouteSlots == nil {
					car.ActiveRouteSlots = newIdleRouteSlots()
				}
				car.HomeFloor = min(top, max(bottom, car.HomeFloor))
				// PORT-NOTE: the TS also resets when nearestWorkFloor is
				// undefined (legacy snapshot); not representable in Go.
				if car.NearestWorkFloor < bottom || car.NearestWorkFloor > top {
					car.NearestWorkFloor = car.HomeFloor
				}
				if car.CurrentFloor < bottom || car.CurrentFloor > top {
					ResetOutOfRangeCar(existing, car)
				}
			}
			newCarriers = append(newCarriers, existing)
		} else {
			newCarriers = append(newCarriers, MakeCarrier(id, col, mode, bottom, top, 1))
			id++
		}
	}

	world.Carriers = newCarriers
	for _, carrier := range world.Carriers {
		SyncAssignmentStatus(carrier)
	}
}

// InitCarrierState TS: carriers.ts initCarrierState
func InitCarrierState(world *WorldState) {
	if world.Carriers == nil {
		world.Carriers = []*CarrierRecord{}
	}
	if world.FloorWalkabilityFlags == nil {
		world.FloorWalkabilityFlags = make([]int, GridHeight)
	}
	if world.TransferGroupCache == nil {
		world.TransferGroupCache = make([]int, GridHeight)
	}
}
