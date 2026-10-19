package sim

// Port of reference/tower-together/sim/sims/medical.ts: medical-center
// service slots and the office-worker medical trip state machine.

// MedicalNotificationMessage TS: sims/medical.ts MEDICAL_NOTIFICATION_MESSAGE
const MedicalNotificationMessage = "Medical Center demanded near Lobby"

// Office worker state codes used during a medical trip.
const (
	StateMedicalTrip        = 0x30
	StateMedicalTripTransit = 0x70
	StateMedicalDwell       = 0x31
)

// medicalCenterCandidate TS: sims/medical.ts MedicalCenterCandidate
type medicalCenterCandidate struct {
	sidecarIndex int
	floor        int
	record       *MedicalCenterRecord
}

// collectMedicalCenters TS: sims/medical.ts collectMedicalCenters.
// Enumerates placed medical centers, bucketed by zone key floor((floor - 9) / 15).
// byZone is a lookup-only TS Map (only .get is used), so a Go map is safe;
// each bucket slice keeps placedObjects insertion order.
func collectMedicalCenters(world *WorldState) (map[int][]medicalCenterCandidate, []medicalCenterCandidate) {
	byZone := map[int][]medicalCenterCandidate{}
	var all []medicalCenterCandidate
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if object.ObjectTypeCode != FamilyMedical {
			continue
		}
		if object.LinkedRecordIndex < 0 {
			continue
		}
		sidecar, ok := GetSidecar(world, object.LinkedRecordIndex).(*MedicalCenterRecord)
		if !ok || sidecar == nil {
			continue
		}
		if sidecar.OwnerSubtypeIndex == 0xff {
			continue
		}
		_, y := parseKeyXY(e.Key)
		floor := YToFloor(y)
		zoneKey := floorDiv(floor-9, 15)
		candidate := medicalCenterCandidate{sidecarIndex: object.LinkedRecordIndex, floor: floor, record: sidecar}
		byZone[zoneKey] = append(byZone[zoneKey], candidate)
		all = append(all, candidate)
	}
	return byZone, all
}

// pickMedicalCenter TS: sims/medical.ts pickMedicalCenter
func pickMedicalCenter(world *WorldState, sourceFloor int) *medicalCenterCandidate {
	byZone, all := collectMedicalCenters(world)
	if len(all) == 0 {
		return nil
	}
	zoneKey := floorDiv(sourceFloor-9, 15)
	bucket := all
	if zoneBucket := byZone[zoneKey]; len(zoneBucket) > 0 {
		bucket = zoneBucket
	}
	pick := bucket[SampleRng(world)%len(bucket)]
	return &pick
}

// allocMedicalSlot TS: sims/medical.ts allocMedicalSlot (first-fit scan)
func allocMedicalSlot(world *WorldState, sim *SimRecord, targetSidecarIndex, sourceFloor int) int {
	for i := 0; i < MaxMedicalServiceSlots; i++ {
		slot := world.MedicalServiceSlots[i]
		if slot.Active {
			continue
		}
		slot.Active = true
		slot.SimID = SimKey(sim)
		slot.SourceFloor = sourceFloor
		slot.TargetSidecarIndex = targetSidecarIndex
		slot.RetryCounter = 0
		return i
	}
	return -1
}

// findMedicalSlotForSim TS: sims/medical.ts findMedicalSlotForSim
func findMedicalSlotForSim(world *WorldState, sim *SimRecord) int {
	key := SimKey(sim)
	for i, slot := range world.MedicalServiceSlots {
		if slot.Active && slot.SimID == key {
			return i
		}
	}
	return -1
}

// freeMedicalSlot TS: sims/medical.ts freeMedicalSlot
func freeMedicalSlot(world *WorldState, slotIndex int) {
	if slotIndex < 0 || slotIndex >= len(world.MedicalServiceSlots) {
		return
	}
	slot := world.MedicalServiceSlots[slotIndex]
	if slot == nil || !slot.Active {
		return
	}
	if sidecar, ok := GetSidecar(world, slot.TargetSidecarIndex).(*MedicalCenterRecord); ok && sidecar != nil {
		sidecar.PendingVisitorsCount = max(0, sidecar.PendingVisitorsCount-1)
	}
	slot.Active = false
	slot.SimID = ""
	slot.SourceFloor = -1
	slot.TargetSidecarIndex = -1
	slot.RetryCounter = 0
}

// fireMedicalFailureBanner TS: sims/medical.ts fireMedicalFailureBanner
func fireMedicalFailureBanner(world *WorldState) {
	world.GateFlags.OfficeServiceOkMedical = 0
	world.PendingNotifications = append(world.PendingNotifications, SimNotification{
		Kind:    NotificationRouteFailure,
		Message: MedicalNotificationMessage,
	})
}

// routeOfficeWorkerHome TS: sims/medical.ts routeOfficeWorkerHome
func routeOfficeWorkerHome(world *WorldState, time *TimeState, sim *SimRecord) {
	result := ResolveSimRouteBetweenFloors(world, sim, sim.SelectedFloor, LobbyFloor,
		b2Bool01(LobbyFloor > sim.SelectedFloor), time, nil)
	if result == -1 {
		sim.StateCode = StateNightB
		sim.DestinationFloor = -1
		ClearSimRoute(sim)
		ReleaseServiceRequest(world, sim)
		return
	}
	if result == 3 {
		sim.DestinationFloor = -1
		sim.SelectedFloor = LobbyFloor
		sim.StateCode = StateParked
		ReleaseServiceRequest(world, sim)
		return
	}
	sim.DestinationFloor = LobbyFloor
	sim.StateCode = StateDepartureTransit
}

// TryStartMedicalTrip TS: sims/medical.ts tryStartMedicalTrip.
// Returns true iff the sim was routed onto a medical trip. The 1-in-10 RNG
// sample is only consumed at starCount >= 3.
func TryStartMedicalTrip(world *WorldState, time *TimeState, sim *SimRecord) bool {
	if world.StarCount < 3 {
		return false
	}
	roll := SampleRng(world) % 10
	if roll != 0 {
		return false
	}

	target := pickMedicalCenter(world, sim.FloorAnchor)
	if target == nil {
		fireMedicalFailureBanner(world)
		return false
	}

	slotIndex := allocMedicalSlot(world, sim, target.sidecarIndex, sim.FloorAnchor)
	if slotIndex < 0 {
		fireMedicalFailureBanner(world)
		return false
	}

	target.record.PendingVisitorsCount++

	if target.floor == sim.FloorAnchor {
		sim.StateCode = StateMedicalDwell
		sim.SelectedFloor = target.floor
		sim.DestinationFloor = -1
		ClearSimRoute(sim)
		return true
	}

	result := ResolveSimRouteBetweenFloors(world, sim, sim.FloorAnchor, target.floor,
		b2Bool01(target.floor > sim.FloorAnchor), time, nil)
	if result == -1 {
		freeMedicalSlot(world, slotIndex)
		fireMedicalFailureBanner(world)
		return false
	}
	sim.SelectedFloor = sim.FloorAnchor
	sim.DestinationFloor = target.floor
	if result == 3 {
		sim.SelectedFloor = target.floor
		sim.DestinationFloor = -1
		sim.StateCode = StateMedicalDwell
		ClearSimRoute(sim)
		return true
	}
	sim.StateCode = StateMedicalTripTransit
	return true
}

// medicalTargetGone is the TS `!sidecar || sidecar.kind !== "medical_center"
// || sidecar.ownerSubtypeIndex === 0xff` test.
func medicalTargetGone(world *WorldState, index int) bool {
	sidecar, ok := GetSidecar(world, index).(*MedicalCenterRecord)
	return !ok || sidecar == nil || sidecar.OwnerSubtypeIndex == 0xff
}

// ProcessMedicalSim TS: sims/medical.ts processMedicalSim
func ProcessMedicalSim(world *WorldState, time *TimeState, sim *SimRecord) {
	if sim.StateCode == StateMedicalTripTransit {
		return
	}
	if sim.StateCode != StateMedicalDwell {
		return
	}

	slotIndex := findMedicalSlotForSim(world, sim)
	if slotIndex < 0 {
		sim.StateCode = StateDeparture
		return
	}
	slot := world.MedicalServiceSlots[slotIndex]

	if medicalTargetGone(world, slot.TargetSidecarIndex) {
		// Binary 1178:02e1 (site 6 of 6 add_or_update_sim_trip_counters
		// callers): add_delay_to_current_sim(g_venue_unavailable_delay) then
		// advance_sim_trip_counters in the target-gone branch.
		AddDelayToCurrentSim(sim, 300)
		AdvanceSimTripCounters(sim)
		freeMedicalSlot(world, slotIndex)
		fireMedicalFailureBanner(world)
		routeOfficeWorkerHome(world, time, sim)
		return
	}

	slot.RetryCounter++
	if slot.RetryCounter >= MedicalRetryOverflow {
		freeMedicalSlot(world, slotIndex)
		routeOfficeWorkerHome(world, time, sim)
	}
}

// HandleMedicalSimArrival TS: sims/medical.ts handleMedicalSimArrival
func HandleMedicalSimArrival(world *WorldState, sim *SimRecord, arrivalFloor int) {
	if sim.StateCode != StateMedicalTripTransit {
		return
	}
	slotIndex := findMedicalSlotForSim(world, sim)
	if slotIndex < 0 {
		sim.StateCode = StateDeparture
		sim.DestinationFloor = -1
		return
	}
	slot := world.MedicalServiceSlots[slotIndex]
	if medicalTargetGone(world, slot.TargetSidecarIndex) {
		freeMedicalSlot(world, slotIndex)
		fireMedicalFailureBanner(world)
		sim.StateCode = StateDeparture
		sim.SelectedFloor = arrivalFloor
		sim.DestinationFloor = -1
		ClearSimRoute(sim)
		return
	}
	sim.SelectedFloor = arrivalFloor
	sim.DestinationFloor = -1
	sim.StateCode = StateMedicalDwell
	ClearSimRoute(sim)
}

// InvalidateMedicalSlotsForSidecar TS: sims/medical.ts invalidateMedicalSlotsForSidecar.
// Called from the placement command layer after freeSidecar marks the target
// demolished. Attached sims go back to STATE_DEPARTURE.
func InvalidateMedicalSlotsForSidecar(world *WorldState, removedSidecarIndex int) {
	for i := 0; i < len(world.MedicalServiceSlots); i++ {
		slot := world.MedicalServiceSlots[i]
		if !slot.Active {
			continue
		}
		if slot.TargetSidecarIndex != removedSidecarIndex {
			continue
		}
		// world.sims.find: first sim whose key matches.
		var sim *SimRecord
		for _, s := range world.Sims {
			if SimKey(s) == slot.SimID {
				sim = s
				break
			}
		}
		freeMedicalSlot(world, i)
		fireMedicalFailureBanner(world)
		if sim == nil {
			continue
		}
		sim.StateCode = StateDeparture
		sim.DestinationFloor = -1
		ClearSimRoute(sim)
	}
}
