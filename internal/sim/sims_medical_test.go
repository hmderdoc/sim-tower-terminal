package sim

import "testing"

// B2 unit tests for sims/medical.ts.

func b2MedicalWorld(t *testing.T) (*WorldState, *SimRecord, *MedicalCenterRecord) {
	w := b2World(t)
	w.StarCount = 3
	center := &MedicalCenterRecord{OwnerSubtypeIndex: 0}
	w.Sidecars = append(w.Sidecars, center)
	b2PlaceObject(w, 40, 12, &PlacedObjectRecord{ObjectTypeCode: FamilyMedical, LinkedRecordIndex: 0})
	sim := b2Sim(12, 60, FamilyOffice, StateAtWork)
	w.Sims = append(w.Sims, sim)
	w.GateFlags.OfficeServiceOkMedical = 1
	return w, sim, center
}

func TestTryStartMedicalTripGates(t *testing.T) {
	w, sim, _ := b2MedicalWorld(t)
	w.StarCount = 2
	calls := w.RngCallCount
	if TryStartMedicalTrip(w, &TimeState{}, sim) || w.RngCallCount != calls {
		t.Fatal("star < 3 must return false without drawing RNG")
	}
	w.StarCount = 3
	w.RngState = b2SeedFor(t, 10, 3)
	if TryStartMedicalTrip(w, &TimeState{}, sim) {
		t.Fatal("roll != 0 must return false")
	}
}

func TestTryStartMedicalTripSameFloorDwell(t *testing.T) {
	w, sim, center := b2MedicalWorld(t)
	w.RngState = b2SeedFor(t, 10, 0)
	if !TryStartMedicalTrip(w, &TimeState{}, sim) {
		t.Fatal("expected a medical trip")
	}
	if sim.StateCode != StateMedicalDwell || sim.SelectedFloor != 12 || center.PendingVisitorsCount != 1 {
		t.Fatalf("state=%#x selected=%d pending=%d", sim.StateCode, sim.SelectedFloor, center.PendingVisitorsCount)
	}
	slot := w.MedicalServiceSlots[0]
	if !slot.Active || slot.SimID != SimKey(sim) || slot.TargetSidecarIndex != 0 {
		t.Fatalf("slot = %+v", slot)
	}

	// Dwell: retry counter climbs until overflow, then routes home.
	ProcessMedicalSim(w, &TimeState{}, sim)
	if slot.RetryCounter != 1 {
		t.Fatalf("retry = %d", slot.RetryCounter)
	}
	slot.RetryCounter = MedicalRetryOverflow - 1
	skipOnTODO(t, func() {
		ProcessMedicalSim(w, &TimeState{}, sim)
		if slot.Active || center.PendingVisitorsCount != 0 {
			t.Fatalf("overflow must free the slot: %+v pending=%d", slot, center.PendingVisitorsCount)
		}
	})
}

func TestTryStartMedicalTripNoCenterFiresBanner(t *testing.T) {
	w, sim, center := b2MedicalWorld(t)
	center.OwnerSubtypeIndex = 0xff
	w.RngState = b2SeedFor(t, 10, 0)
	if TryStartMedicalTrip(w, &TimeState{}, sim) {
		t.Fatal("no live center: expected false")
	}
	if w.GateFlags.OfficeServiceOkMedical != 0 || len(w.PendingNotifications) != 1 ||
		w.PendingNotifications[0].Message != MedicalNotificationMessage {
		t.Fatalf("banner not fired: %+v", w.PendingNotifications)
	}
}

func TestPickMedicalCenterPrefersZone(t *testing.T) {
	w := b2World(t)
	w.Sidecars = append(w.Sidecars, &MedicalCenterRecord{}, &MedicalCenterRecord{})
	b2PlaceObject(w, 40, 12, &PlacedObjectRecord{ObjectTypeCode: FamilyMedical, LinkedRecordIndex: 0}) // zone 0
	b2PlaceObject(w, 40, 30, &PlacedObjectRecord{ObjectTypeCode: FamilyMedical, LinkedRecordIndex: 1}) // zone 1
	for i := 0; i < 20; i++ {
		if got := pickMedicalCenter(w, 28); got == nil || got.sidecarIndex != 1 {
			t.Fatalf("source floor 28 must pick zone-1 center, got %+v", got)
		}
	}
	// Underground source floor 3: zone floor((3-9)/15) = -1 is empty -> global bucket.
	if got := pickMedicalCenter(w, 3); got == nil {
		t.Fatal("global fallback expected")
	}
}

func TestInvalidateMedicalSlotsForSidecar(t *testing.T) {
	w, sim, center := b2MedicalWorld(t)
	sim.StateCode = StateMedicalTripTransit
	sim.Route = RouteState{Mode: RouteModeCarrier, CarrierID: 0}
	if allocMedicalSlot(w, sim, 0, sim.FloorAnchor) != 0 {
		t.Fatal("first-fit slot 0 expected")
	}
	center.PendingVisitorsCount = 1
	// A second slot pointing elsewhere must survive.
	other := b2Sim(20, 70, FamilyOffice, StateMedicalDwell)
	allocMedicalSlot(w, other, 5, 20)

	InvalidateMedicalSlotsForSidecar(w, 0)

	if w.MedicalServiceSlots[0].Active || !w.MedicalServiceSlots[1].Active {
		t.Fatal("only the matching slot is freed")
	}
	if center.PendingVisitorsCount != 0 || w.GateFlags.OfficeServiceOkMedical != 0 || len(w.PendingNotifications) != 1 {
		t.Fatalf("pending=%d flag=%d notes=%d", center.PendingVisitorsCount, w.GateFlags.OfficeServiceOkMedical, len(w.PendingNotifications))
	}
	if sim.StateCode != StateDeparture || sim.DestinationFloor != -1 || sim.Route.Mode != RouteModeIdle {
		t.Fatalf("sim not reset: %+v", sim)
	}
}

func TestHandleMedicalSimArrival(t *testing.T) {
	w, sim, _ := b2MedicalWorld(t)
	sim.StateCode = StateMedicalTripTransit
	allocMedicalSlot(w, sim, 0, 5)
	HandleMedicalSimArrival(w, sim, 12)
	if sim.StateCode != StateMedicalDwell || sim.SelectedFloor != 12 {
		t.Fatalf("arrival: %+v", sim)
	}
	// No slot -> STATE_DEPARTURE.
	sim2 := b2Sim(14, 80, FamilyOffice, StateMedicalTripTransit)
	HandleMedicalSimArrival(w, sim2, 12)
	if sim2.StateCode != StateDeparture {
		t.Fatalf("slotless arrival state = %#x", sim2.StateCode)
	}
}
