package sim

import (
	"sort"
	"strconv"
)

// Port of reference/tower-together/sim/sims/population.ts.

// TS: sims/population.ts makeSim
func makeSim(floorAnchor, homeColumn, baseOffset, familyCode, population, facilitySlot int) *SimRecord {
	return &SimRecord{
		FloorAnchor:           floorAnchor,
		HomeColumn:            homeColumn,
		BaseOffset:            baseOffset,
		FacilitySlot:          facilitySlot,
		FamilyCode:            familyCode,
		StateCode:             initialStateForFamily(familyCode, baseOffset, population),
		Route:                 RouteIdle,
		SelectedFloor:         floorAnchor,
		OriginFloor:           floorAnchor,
		DestinationFloor:      -1,
		VenueReturnState:      0,
		QueueTick:             0,
		ElapsedTicks:          0,
		TransitTicksRemaining: 0,
		LastDemandTick:        -1,
		TripCount:             0,
		AccumulatedTicks:      0,
		TargetRoomFloor:       -1,
		TargetRoomColumn:      -1,
		SpawnFloor:            floorAnchor,
		PostClaimCountdown:    0,
		EncodedTargetFloor:    0,
		CommercialVenueSlot:   -1,
	}
}

// TS: sims/population.ts initialStateForFamily
func initialStateForFamily(familyCode, baseOffset, _ int) int {
	if HotelFamilies[familyCode] {
		// Binary NIGHT_B handler: base_offset == 0 -> HOTEL_PARKED, others -> MORNING_GATE.
		if baseOffset == 0 {
			return StateHotelParked
		}
		return StateMorningGate
	}
	if CathedralFamilies[familyCode] {
		return StateParked
	}
	if familyCode == FamilyOffice {
		return StateMorningGate
	}
	if familyCode == FamilyCondo {
		return StateMorningGate
	}
	if CommercialFamilies[familyCode] {
		return StateMorningGate
	}
	if familyCode == FamilyRecyclingCenterUpper || familyCode == FamilySecurity {
		return StateActive
	}
	if familyCode == FamilyHousekeeping {
		return HkStateSearch
	}
	return StateParked
}

// SimKey is the string identity of a sim: "floor:col:family:offset".
// TS: sims/population.ts simKey
func SimKey(sim *SimRecord) string {
	return strconv.Itoa(sim.FloorAnchor) + ":" + strconv.Itoa(sim.HomeColumn) + ":" +
		strconv.Itoa(sim.FamilyCode) + ":" + strconv.Itoa(sim.BaseOffset)
}

// TS: sims/population.ts objectKey
func objectKey(sim *SimRecord) string {
	y := GridHeight - 1 - sim.FloorAnchor
	return cellKey(sim.HomeColumn, y)
}

// FindObjectForSim returns the placed object owning sim, or nil. The first
// successful lookup is cached on the sim (the TS simOwnerCache WeakMap);
// a cached owner is returned even if the map entry later changed, exactly
// like the TS.
// TS: sims/population.ts findObjectForSim
func FindObjectForSim(world *WorldState, sim *SimRecord) *PlacedObjectRecord {
	if sim.ownerCache != nil {
		return sim.ownerCache
	}
	owner, _ := world.PlacedObjects.Get(objectKey(sim))
	if owner != nil {
		sim.ownerCache = owner
	}
	return owner
}

// FindSiblingSims TS: sims/population.ts findSiblingSims
func FindSiblingSims(world *WorldState, sim *SimRecord) []*SimRecord {
	var out []*SimRecord
	for _, c := range world.Sims {
		if c.FloorAnchor == sim.FloorAnchor && c.HomeColumn == sim.HomeColumn && c.FamilyCode == sim.FamilyCode {
			out = append(out, c)
		}
	}
	return out
}

// ClearSimRoute resets the route to idle. State-code bits are deliberately
// NOT cleared (see the TS comment: family arrival handlers read the
// _TRANSIT byte right after this).
// TS: sims/population.ts clearSimRoute
func ClearSimRoute(sim *SimRecord) {
	sim.Route = RouteIdle
}

// TS: sims/population.ts clearCarrierSlotsForRemovedSims
func clearCarrierSlotsForRemovedSims(carrier *CarrierRecord, removedIDs map[string]bool) {
	kept := carrier.PendingRoutes[:0:0]
	for _, route := range carrier.PendingRoutes {
		if !removedIDs[route.SimID] {
			kept = append(kept, route)
		}
	}
	carrier.PendingRoutes = kept
	for _, car := range carrier.Cars {
		for _, slot := range car.ActiveRouteSlots {
			if !slot.Active {
				continue
			}
			if !removedIDs[slot.RouteID] {
				continue
			}
			if slot.Boarded {
				car.AssignedCount = max(0, car.AssignedCount-1)
			}
			slot.Active = false
			slot.RouteID = ""
			slot.SourceFloor = 0xff
			slot.DestinationFloor = 0xff
			slot.Boarded = false
		}
		ids := make([]string, 0, len(car.PendingRouteIDs))
		for _, id := range car.PendingRouteIDs {
			if !removedIDs[id] {
				ids = append(ids, id)
			}
		}
		car.PendingRouteIDs = ids
	}
}

// Binary place_object_on_floor maintains a 10-entry FIFO of pending sim-slot
// reservations; see the TS comment block (medical placeholders reserve 6
// slots and are zeroed on finalize, letting a later placement reclaim them).
const (
	placementQueueSize = 10
	medicalSlotWidth   = 6
)

type placementQueueEntry struct {
	slotIndices    []int
	zeroOnFinalize bool
}

// TS: sims/population.ts firstFitSlot
func firstFitSlot(occupied []bool, width int) int {
	run := 0
	runStart := -1
	for i := range occupied {
		if !occupied[i] {
			if run == 0 {
				runStart = i
			}
			run++
			if run == width {
				return runStart
			}
		} else {
			run = 0
		}
	}
	return len(occupied)
}

// RebuildRuntimeSims rebuilds world.Sims from placedObjects, preserving
// prior runtime state by simKey.
// TS: sims/population.ts rebuildRuntimeSims
func RebuildRuntimeSims(world *WorldState) {
	previous := make(map[string]*SimRecord, len(world.Sims))
	for _, s := range world.Sims {
		previous[SimKey(s)] = s // later duplicates win, like new Map(entries)
	}

	// Above-grade floors ascending (y descending), then below-grade floors
	// ascending (y ascending), then x ascending. Stable like JS sort.
	type keyed struct {
		key  string
		x, y int
		obj  *PlacedObjectRecord
	}
	entries := world.PlacedObjects.Entries()
	sorted := make([]keyed, len(entries))
	for i, e := range entries {
		x, y := parseKeyXY(e.Key)
		sorted[i] = keyed{e.Key, x, y, e.Val}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		aBelow, bBelow := 0, 0
		if a.y > GroundY {
			aBelow = 1
		}
		if b.y > GroundY {
			bBelow = 1
		}
		if aBelow != bBelow {
			return aBelow < bBelow
		}
		if aBelow == 0 {
			if a.y != b.y {
				return a.y > b.y
			}
		} else {
			if a.y != b.y {
				return a.y < b.y
			}
		}
		return a.x < b.x
	})

	slotByFloor := map[int]int{}
	var slots []*SimRecord
	var occupied []bool
	var queue []placementQueueEntry

	for _, ent := range sorted {
		object := ent.obj
		familyCode := object.ObjectTypeCode
		runtimeFamilyCode := familyCode
		if CathedralFamilies[familyCode] {
			runtimeFamilyCode = FamilyCathedralBase
		}
		isMedical := familyCode == FamilyMedical
		population := EntityPopulationByType[familyCode]
		if population == 0 && !isMedical {
			continue
		}

		x := ent.x
		floorAnchor := YToFloor(ent.y)
		facilitySlot := slotByFloor[floorAnchor]
		slotByFloor[floorAnchor] = facilitySlot + 1

		slotWidth := population
		if isMedical {
			slotWidth = medicalSlotWidth
		}
		start := firstFitSlot(occupied, slotWidth)
		slotIndices := make([]int, 0, slotWidth)
		for j := 0; j < slotWidth; j++ {
			idx := start + j
			for len(slots) <= idx {
				slots = append(slots, nil)
				occupied = append(occupied, false)
			}
			occupied[idx] = true
			slotIndices = append(slotIndices, idx)
			if isMedical {
				slots[idx] = nil
				continue
			}
			fresh := makeSim(floorAnchor, x, j, runtimeFamilyCode, population, facilitySlot)
			if prior, ok := previous[SimKey(fresh)]; ok {
				// { ...fresh, ...prior, floorAnchor, homeColumn: x, facilitySlot }:
				// a NEW object copying every prior field (fresh is fully
				// shadowed because prior carries all fields).
				merged := *prior
				merged.FloorAnchor = floorAnchor
				merged.HomeColumn = x
				merged.FacilitySlot = facilitySlot
				merged.ownerCache = nil // new JS object: WeakMap miss
				slots[idx] = &merged
			} else {
				fresh.TripCount = 0
				fresh.AccumulatedTicks = 0
				slots[idx] = fresh
			}
		}

		queue = append(queue, placementQueueEntry{slotIndices: slotIndices, zeroOnFinalize: isMedical})
		if len(queue) > placementQueueSize {
			oldest := queue[0]
			queue = queue[1:]
			if oldest.zeroOnFinalize {
				for _, i := range oldest.slotIndices {
					occupied[i] = false
					slots[i] = nil
				}
			}
		}
	}

	out := make([]*SimRecord, 0, len(slots))
	for _, s := range slots {
		if s != nil {
			out = append(out, s)
		}
	}
	world.Sims = out
}

// CleanupSimsForRemovedTile drops the sims homed at (anchorX, y) and clears
// their carrier slots. TS: sims/population.ts cleanupSimsForRemovedTile
func CleanupSimsForRemovedTile(world *WorldState, anchorX, y int) {
	floorAnchor := YToFloor(y)
	removedIDs := map[string]bool{}

	kept := make([]*SimRecord, 0, len(world.Sims))
	for _, sim := range world.Sims {
		if sim.HomeColumn != anchorX || sim.FloorAnchor != floorAnchor {
			kept = append(kept, sim)
			continue
		}
		ClearSimRoute(sim)
		sim.DestinationFloor = -1
		removedIDs[SimKey(sim)] = true
	}
	world.Sims = kept

	if len(removedIDs) == 0 {
		return
	}
	for _, carrier := range world.Carriers {
		clearCarrierSlotsForRemovedSims(carrier, removedIDs)
	}
}

// ResetSimRuntimeState is the day-boundary sim reset.
// TS: sims/population.ts resetSimRuntimeState
func ResetSimRuntimeState(world *WorldState) {
	for _, sim := range world.Sims {
		object := FindObjectForSim(world, sim)
		if object == nil {
			continue
		}
		clearsDailyStress := CommercialFamilies[sim.FamilyCode] ||
			sim.FamilyCode == FamilySecurity ||
			sim.FamilyCode == FamilyHousekeeping ||
			sim.FamilyCode == FamilyCinema ||
			sim.FamilyCode == FamilyPartyHall ||
			sim.FamilyCode == 0x21 ||
			sim.FamilyCode == FamilyCathedralBase
		if clearsDailyStress {
			ResetSimTripCounters(sim)
		}

		if HotelFamilies[sim.FamilyCode] {
			// Binary 1228:0000 reset_sim_runtime_state, family 3/4/5 case.
			if sim.BaseOffset == 0 {
				sim.StateCode = StateHotelParked
			}
			continue
		} else if sim.FamilyCode == FamilyCondo {
			// Condos persist across day boundaries via CHECKOUT_QUEUE->TRANSITION.
			continue
		} else if sim.FamilyCode == FamilyHousekeeping {
			sim.StateCode = HkStateSearch
			sim.TargetRoomFloor = -1
			sim.SpawnFloor = sim.FloorAnchor
			sim.PostClaimCountdown = 0
		} else if sim.FamilyCode == FamilyOffice || CommercialFamilies[sim.FamilyCode] {
			// Spec TIME.md checkpoint 2500: family 6/7/10/12 -> 0x20 (MORNING_GATE).
			sim.StateCode = StateMorningGate
		} else if sim.FamilyCode == FamilyRecyclingCenterUpper || sim.FamilyCode == FamilySecurity {
			// Stationary sims keep their current state across day boundaries.
			continue
		} else {
			sim.StateCode = StateParked
		}

		sim.SelectedFloor = sim.FloorAnchor
		sim.OriginFloor = sim.FloorAnchor
		sim.Route = RouteIdle
		sim.DestinationFloor = -1
		sim.VenueReturnState = 0
		sim.QueueTick = 0
		sim.ElapsedTicks = 0
		sim.TransitTicksRemaining = 0
	}
}
