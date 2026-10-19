package sim

import (
	"strconv"
	"strings"
	"testing"
)

// ─── Faithful ports of commands.test.ts ─────────────────────────────────────
//
// These build towers through TowerSim.Create / placements, which reach the
// A2/B stubs (HydrateSnapshot -> InitCarrierState, RunGlobalRebuilds ->
// RebuildParkingCoverage ...). They skip with the stub name until those
// land, then assert automatically.

func makeSimWithLobbyStrip(t *testing.T, from, to int) *TowerSim {
	t.Helper()
	sim := Create("spacing-test", "Spacing", "")
	for x := from; x <= to; x++ {
		if r := place(sim, x, GroundY, "lobby"); !r.Accepted {
			t.Fatalf("lobby placement at x=%d failed: %s", x, r.Reason)
		}
	}
	return sim
}

func placeElevatorSegment(sim *TowerSim, x, floor int, tileType string) CommandResult {
	return place(sim, x, GroundY-floor, tileType)
}

func buildFloors(t *testing.T, sim *TowerSim, floor, from, to int) {
	t.Helper()
	for x := from; x <= to; x++ {
		if r := place(sim, x, GroundY-floor, "floor"); !r.Accepted {
			t.Fatalf("floor placement at x=%d,floor=%d failed: %s", x, floor, r.Reason)
		}
	}
}

func expectAccepted(t *testing.T, r CommandResult, what string) {
	t.Helper()
	if !r.Accepted {
		t.Fatalf("%s rejected: %s", what, r.Reason)
	}
}

func expectRejected(t *testing.T, r CommandResult, reasonSubstr string) {
	t.Helper()
	if r.Accepted {
		t.Fatalf("expected rejection matching %q, got accepted", reasonSubstr)
	}
	if !strings.Contains(strings.ToLower(r.Reason), strings.ToLower(reasonSubstr)) {
		t.Fatalf("reason %q does not match %q", r.Reason, reasonSubstr)
	}
}

func TestCommandsTSElevatorShaftSpacing(t *testing.T) {
	t.Run("extend same shaft", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			for f := 0; f <= 2; f++ {
				expectAccepted(t, placeElevatorSegment(sim, 100, f, "elevator"), "segment")
			}
		})
	})
	t.Run("rejects within 4 tiles", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, placeElevatorSegment(sim, 100, 0, "elevator"), "first")
			expectRejected(t, placeElevatorSegment(sim, 107, 0, "elevator"), "too close")
		})
	})
	t.Run("accepts exactly 4 tiles away", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, placeElevatorSegment(sim, 100, 0, "elevator"), "first")
			expectAccepted(t, placeElevatorSegment(sim, 108, 0, "elevator"), "second")
		})
	})
	t.Run("8 clear tiles beside express", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, placeElevatorSegment(sim, 100, 0, "elevatorExpress"), "express")
			expectRejected(t, placeElevatorSegment(sim, 113, 0, "elevator"), "too close")
			expectAccepted(t, placeElevatorSegment(sim, 114, 0, "elevator"), "standard")
		})
	})
	t.Run("8 clear tiles when new shaft is express", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, placeElevatorSegment(sim, 100, 0, "elevator"), "standard")
			expectRejected(t, placeElevatorSegment(sim, 111, 0, "elevatorExpress"), "too close")
			expectAccepted(t, placeElevatorSegment(sim, 112, 0, "elevatorExpress"), "express")
		})
	})
}

var undergroundBlocked = []string{"hotelSingle", "hotelTwin", "hotelSuite", "office", "condo", "housekeeping", "lobby"}

func TestCommandsTSUndergroundRestrictions(t *testing.T) {
	t.Run("rejects rooms underground", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			for _, tile := range undergroundBlocked {
				expectRejected(t, place(sim, 100, UndergroundY, tile), "underground")
			}
		})
	})
	t.Run("allows infrastructure underground", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, place(sim, 100, UndergroundY, "floor"), "floor")
			expectAccepted(t, place(sim, 120, UndergroundY, "parking"), "parking")
			expectAccepted(t, place(sim, 140, UndergroundY-1, "elevator"), "elevator ground")
			expectAccepted(t, place(sim, 140, UndergroundY, "elevator"), "elevator B1")
		})
	})
}

func placeMetro(sim *TowerSim, x, topY int) CommandResult {
	return place(sim, x, topY, "metro")
}

func TestCommandsTSMetroStation(t *testing.T) {
	t.Run("singleton three-floor stack", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 150)
			r := placeMetro(sim, 100, GroundY+1)
			expectAccepted(t, r, "metro")
			if len(r.Patch) != 90 {
				t.Fatalf("patch len %d", len(r.Patch))
			}
			snap := sim.SaveState()
			w := snap.World
			if w.GateFlags.MetroPlaced != 1 || w.GateFlags.MetroStationFloorIndex != 9 {
				t.Fatalf("gates %+v", w.GateFlags)
			}
			if w.PlacedObjects.At("100,110").ObjectTypeCode != FamilyMetroTop ||
				w.PlacedObjects.At("100,111").ObjectTypeCode != FamilyMetroMiddle ||
				w.PlacedObjects.At("100,112").ObjectTypeCode != FamilyMetroBottom {
				t.Fatal("metro families")
			}
			if w.CellToAnchor.At("129,112") != "100,110" {
				t.Fatal("anchor")
			}
		})
	})
	t.Run("rejects second metro, resets on removal", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectAccepted(t, placeMetro(sim, 100, GroundY+1), "metro")
			expectRejected(t, placeMetro(sim, 140, GroundY+1), "already placed")
			expectAccepted(t, sim.SubmitCommand(SimCommand{Type: CmdRemoveTile, X: 110, Y: GroundY + 2}), "remove")
			w := sim.SaveState().World
			if w.GateFlags.MetroPlaced != 0 || w.GateFlags.MetroStationFloorIndex != -1 {
				t.Fatal("gates not reset")
			}
			for _, k := range []string{"100,110", "100,111", "100,112"} {
				if w.PlacedObjects.Has(k) {
					t.Fatalf("%s still placed", k)
				}
			}
		})
	})
	t.Run("charges regardless of daypart and seeds status", func(t *testing.T) {
		skipOnTODO(t, func() {
			paid := makeSimWithLobbyStrip(t, 90, 150)
			paidSnap := paid.SaveState()
			paidSnap.Time.DaypartIndex = 4
			paidSim := FromSnapshot(paidSnap)
			before := paidSim.Cash()
			expectAccepted(t, placeMetro(paidSim, 100, GroundY+1), "metro")
			if paidSim.Cash() != before-TileCosts["metro"] || paidSim.SaveState().World.PlacedObjects.At("100,110").UnitStatus != 1 {
				t.Fatal("paid metro")
			}
			early := makeSimWithLobbyStrip(t, 90, 150)
			earlySnap := early.SaveState()
			earlySnap.Time.DayTick = 1200
			earlySnap.Time.DaypartIndex = 3
			earlySim := FromSnapshot(earlySnap)
			before = earlySim.Cash()
			expectAccepted(t, placeMetro(earlySim, 100, GroundY+1), "metro")
			if earlySim.Cash() != before-TileCosts["metro"] || earlySim.SaveState().World.PlacedObjects.At("100,110").UnitStatus != 0 {
				t.Fatal("early metro")
			}
		})
	})
	t.Run("blocks carrier extension below metro middle", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 150)
			expectAccepted(t, placeMetro(sim, 100, GroundY+1), "metro")
			expectAccepted(t, placeElevatorSegment(sim, 140, -1, "elevator"), "B1")
			expectAccepted(t, placeElevatorSegment(sim, 140, -2, "elevator"), "B2")
			expectRejected(t, placeElevatorSegment(sim, 140, -3, "elevator"), "below the metro")
		})
	})
	t.Run("toggles display variant and emits arrival", func(t *testing.T) {
		skipOnTODO(t, func() {
			base := makeSimWithLobbyStrip(t, 90, 150)
			baseSnap := base.SaveState()
			baseSnap.Time.DaypartIndex = 3
			sim := FromSnapshot(baseSnap)
			expectAccepted(t, placeMetro(sim, 100, GroundY+1), "metro")
			snap := sim.SaveState()
			snap.World.RngState = 54
			snap.World.RngCallCount = 0
			tm := CreateTimeState()
			tm.DayTick = 0xf1
			tm.DaypartIndex = 3
			TickVipSpecialVisitor(snap.World, tm)
			for _, k := range []string{"100,110", "100,111", "100,112"} {
				if snap.World.PlacedObjects.At(k).UnitStatus != 2 {
					t.Fatalf("%s status %d", k, snap.World.PlacedObjects.At(k).UnitStatus)
				}
			}
			found := false
			for _, n := range snap.World.PendingNotifications {
				if n.Kind == "event" && n.Message == "metro_train_arrival" {
					found = true
				}
			}
			if !found {
				t.Fatal("no metro_train_arrival notification")
			}
		})
	})
}

func TestCommandsTSStairsAlignment(t *testing.T) {
	t.Run("stacks at same column", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			for f := 1; f <= 3; f++ {
				buildFloors(t, sim, f, 90, 180)
			}
			for f := 0; f <= 2; f++ {
				expectAccepted(t, place(sim, 100, GroundY-f, "stairs"), "stairs")
			}
		})
	})
	t.Run("non-overlapping allowed", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			buildFloors(t, sim, 1, 90, 180)
			buildFloors(t, sim, 2, 90, 180)
			expectAccepted(t, place(sim, 100, GroundY, "stairs"), "a")
			expectAccepted(t, place(sim, 108, GroundY-1, "stairs"), "b")
		})
	})
	t.Run("rejects unaligned overlap", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			buildFloors(t, sim, 1, 90, 180)
			buildFloors(t, sim, 2, 90, 180)
			expectAccepted(t, place(sim, 100, GroundY, "stairs"), "a")
			expectRejected(t, place(sim, 104, GroundY-1, "stairs"), "align")
		})
	})
	t.Run("rejects without floor above", func(t *testing.T) {
		skipOnTODO(t, func() {
			sim := makeSimWithLobbyStrip(t, 90, 180)
			expectRejected(t, place(sim, 100, GroundY, "stairs"), "floor above")
		})
	})
}

// ─── A1 validation tests on a bare (unhydrated) world ───────────────────────
//
// Same rules as above, with setup written straight into the world so the
// rejection paths run without reaching later-phase stubs.

func bareLobbySim(from, to int) *TowerSim {
	s := newBareSim(LobbyModeModern)
	setRow(s.world, GroundY, from, to, "lobby")
	return s
}

func TestCommandsBasicRejections(t *testing.T) {
	s := bareLobbySim(90, 180)
	expectRejected(t, place(s, 100, 100, "spaceship"), "invalid tile")
	expectRejected(t, place(s, -1, 100, "office"), "out of bounds")
	expectRejected(t, place(s, 370, 100, "office"), "out of bounds") // 370+9-1 >= 375
	expectRejected(t, place(s, 100, GroundY-3, "office"), "no support")
	expectRejected(t, place(s, 100, GroundY, "office"), "already occupied")
	expectRejected(t, place(s, 100, GroundY-14, "lobby"), "+15")
	expectRejected(t, place(s, 100, 5, "cathedral"), "floor 103")
	expectRejected(t, place(s, 100, GroundY-1, "parkingRamp"), "only be placed underground")
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdRemoveTile, X: 10, Y: 10}), "empty")
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetRentLevel, X: 10, Y: 10, RentLevel: 4}), "0-3")
	expectRejected(t, s.SubmitCommand(SimCommand{Type: "bogus"}), "unknown")
	if s.Cash() != StartingCash {
		t.Fatalf("rejections must not charge: %d", s.Cash())
	}
	pp := newBareSim(LobbyModePerfectParity)
	expectRejected(t, place(pp, 100, GroundY-15, "lobby"), "14, then 15")
}

func TestCommandsUndergroundRestrictionsBare(t *testing.T) {
	s := bareLobbySim(90, 180)
	for _, tile := range undergroundBlocked {
		expectRejected(t, place(s, 100, UndergroundY, tile), "underground")
	}
}

func TestCommandsShaftSpacingBare(t *testing.T) {
	s := bareLobbySim(90, 180)
	s.world.Carriers = []*CarrierRecord{testCarrier(100, 1, 10, 10)}
	expectRejected(t, placeElevatorSegment(s, 107, 0, "elevator"), "too close")

	s = bareLobbySim(90, 180)
	s.world.Carriers = []*CarrierRecord{testCarrier(100, 0, 10, 10)}
	expectRejected(t, placeElevatorSegment(s, 113, 0, "elevator"), "too close")

	s = bareLobbySim(90, 180)
	s.world.Carriers = []*CarrierRecord{testCarrier(100, 1, 10, 10)}
	expectRejected(t, placeElevatorSegment(s, 111, 0, "elevatorExpress"), "too close")

	// Max shafts.
	s = bareLobbySim(0, 374)
	for i := 0; i < elevatorMaxShafts; i++ {
		s.world.Carriers = append(s.world.Carriers, testCarrier(i, 1, 50, 50))
	}
	expectRejected(t, placeElevatorSegment(s, 300, 0, "elevator"), "maximum 24")

	// Overlay collision and misalignment with the segment above.
	s = bareLobbySim(90, 180)
	setRow(s.world, GroundY-1, 90, 180, "floor")
	s.world.Overlays.Set(cellKey(100, GroundY), "elevator")
	for dx := 1; dx < 4; dx++ {
		s.world.OverlayToAnchor.Set(cellKey(100+dx, GroundY), cellKey(100, GroundY))
	}
	s.world.Carriers = []*CarrierRecord{testCarrier(100, 1, 10, 10)}
	expectRejected(t, placeElevatorSegment(s, 102, 0, "elevator"), "already has an overlay")
	carriers := s.world.Carriers
	s.world.Carriers = nil // keep tooCloseToExistingShaft out of the way
	expectRejected(t, placeElevatorSegment(s, 102, 1, "elevator"), "align")
	s.world.Carriers = carriers
	expectRejected(t, placeElevatorSegment(s, 100, 1, "elevatorService"), "mode must match")
}

func TestCommandsStairsBare(t *testing.T) {
	s := bareLobbySim(90, 180)
	expectRejected(t, place(s, 100, GroundY, "stairs"), "floor above")
	expectRejected(t, place(s, 100, GroundY-1, "stairs"), "require a base tile")

	s = bareLobbySim(90, 180)
	setRow(s.world, GroundY-1, 90, 180, "floor")
	setRow(s.world, GroundY-2, 90, 180, "floor")
	s.world.Overlays.Set(cellKey(100, GroundY), "stairs")
	expectRejected(t, place(s, 104, GroundY-1, "stairs"), "align")
}

func TestCommandsMetroBare(t *testing.T) {
	s := bareLobbySim(90, 180)
	expectRejected(t, place(s, 100, GroundY, "metro"), "only be placed underground")
	s.world.GateFlags.MetroStationFloorIndex = 9
	expectRejected(t, place(s, 100, GroundY+1, "metro"), "already placed")
	expectRejected(t, placeElevatorSegment(s, 140, -3, "elevator"), "below the metro")
	expectRejected(t, place(s, 100, GroundY+3, "cinema"), "below the metro")
}

func TestCommandsRentLevel(t *testing.T) {
	s := bareLobbySim(90, 180)
	condo := addObject(s.world, 100, 11, FamilyCondo, 16)
	condo.UnitStatus = 0x10
	for dx := 1; dx < 16; dx++ {
		s.world.CellToAnchor.Set(cellKey(100+dx, FloorToY(11)), cellKey(100, FloorToY(11)))
	}
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetRentLevel, X: 105, Y: FloorToY(11), RentLevel: 2}), "sold condos")
	condo.UnitStatus = 0x18
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdSetRentLevel, X: 105, Y: FloorToY(11), RentLevel: 2}), "condo rent")
	if condo.RentLevel != 2 {
		t.Fatalf("rent %d", condo.RentLevel)
	}
	addObject(s.world, 140, 11, FamilySecurity, 16)
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetRentLevel, X: 140, Y: FloorToY(11), RentLevel: 1}), "adjustable")
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetRentLevel, X: 170, Y: FloorToY(11), RentLevel: 1}), "no facility")
}

func TestCommandsCinemaMoviePool(t *testing.T) {
	s := bareLobbySim(90, 180)
	cinema := addObject(s.world, 100, 12, FamilyCinemaStairsUpper, 7)
	s.world.Sidecars = SidecarList{&EntertainmentLinkRecord{FamilySelectorOrSingleLinkFlag: 3, LinkAgeCounter: 9}}
	cinema.LinkedRecordIndex = 0
	link := s.world.Sidecars[0].(*EntertainmentLinkRecord)
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdSetCinemaMoviePool, X: 100, Y: FloorToY(12), Pool: "classic"}), "classic")
	if link.FamilySelectorOrSingleLinkFlag != 4 || link.LinkAgeCounter != 0 || s.Cash() != StartingCash-150_000 {
		t.Fatalf("classic: sel %d cash %d", link.FamilySelectorOrSingleLinkFlag, s.Cash())
	}
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdSetCinemaMoviePool, X: 100, Y: FloorToY(12), Pool: "new"}), "new")
	if link.FamilySelectorOrSingleLinkFlag != 12 || s.Cash() != StartingCash-450_000 {
		t.Fatalf("new: sel %d cash %d", link.FamilySelectorOrSingleLinkFlag, s.Cash())
	}
	link.FamilySelectorOrSingleLinkFlag = 0xff
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetCinemaMoviePool, X: 100, Y: FloorToY(12), Pool: "new"}), "not a cinema")
}

func TestCommandsElevatorCars(t *testing.T) {
	s := bareLobbySim(90, 180)
	idle := &CarrierCar{}
	c := testCarrier(100, 1, 10, 20, &CarrierCar{Active: true}, idle)
	s.world.Carriers = []*CarrierRecord{c}
	// Click above the shaft: nearest segment at the column, home clamped to top.
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdAddElevatorCar, X: 100, Y: FloorToY(30)}), "add car")
	if !idle.Active || idle.HomeFloor != 20 || idle.CurrentFloor != 20 || s.Cash() != StartingCash-80_000 {
		t.Fatalf("activated car %+v cash %d", idle, s.Cash())
	}
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdAddElevatorCar, X: 101, Y: FloorToY(12)}), "no elevator")

	// Remove with live traffic -> confirmation prompt.
	c.PendingRoutes = []*CarrierPendingRoute{{SimID: "x"}}
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdRemoveElevatorCar, X: 100, Y: FloorToY(12)}), "remove prompt")
	if len(s.world.PendingPrompts) != 1 || s.world.PendingPrompts[0].PromptID != "carrier_remove_100_"+strconv.Itoa(FloorToY(12)) ||
		s.world.EventState.PendingCarrierEditColumn != 100 || !idle.Active {
		t.Fatalf("prompt %+v", s.world.PendingPrompts)
	}
	c.PendingRoutes = nil
	idle.AssignedCount = 3
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdRemoveElevatorCar, X: 100, Y: FloorToY(12)}), "remove prompt 2")
	expectAccepted(t, ApplyRemoveElevatorCar(s.world, 100, FloorToY(12)), "apply remove")
	if idle.Active || idle.AssignedCount != 0 {
		t.Fatalf("last car not removed %+v", idle)
	}
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdRemoveElevatorCar, X: 100, Y: FloorToY(12)}), "at least 1")

	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdSetElevatorDwellDelay, X: 100, Y: FloorToY(12), Value: 3}), "dwell")
	if len(c.DwellDelay) != 14 || c.DwellDelay[13] != 3 {
		t.Fatalf("dwell %v", c.DwellDelay)
	}
	expectAccepted(t, s.SubmitCommand(SimCommand{Type: CmdSetElevatorWaitingCarResponse, X: 100, Y: FloorToY(12), Value: 7}), "waiting")
	if c.WaitingCarResponseThreshold != 7 {
		t.Fatal("waiting threshold")
	}
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdSetElevatorHomeFloor, X: 100, CarIndex: 5, Floor: 12}), "car not found")

	full := testCarrier(140, 1, 10, 20)
	for i := 0; i < 8; i++ {
		full.Cars = append(full.Cars, &CarrierCar{Active: true})
	}
	s.world.Carriers = append(s.world.Carriers, full)
	expectRejected(t, s.SubmitCommand(SimCommand{Type: CmdAddElevatorCar, X: 140, Y: FloorToY(12)}), "maximum 8")
}

func TestFillRowGaps(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	setRow(w, GroundY, 0, 50, "lobby")
	w.Cells.Set(cellKey(10, GroundY-1), "office")
	w.Cells.Set(cellKey(20, GroundY-1), "office")
	var patch []CellPatch
	FillRowGaps(GroundY-1, w, &patch)
	if len(patch) != 9 || w.Cells.At(cellKey(15, GroundY-1)) != "floor" || w.Cells.Has(cellKey(21, GroundY-1)) {
		t.Fatalf("gaps %d", len(patch))
	}
}

// ─── Placement runs through all A1 mutation before the first stub ───────────

func TestCommandsPlaceOfficeMutatesBeforeRebuildStubs(t *testing.T) {
	s := bareLobbySim(90, 180)
	// B2 ported rebuildParkingCoverage; the first stub reached is now
	// whichever later rebuild is still unported (or none).
	expectTODOPanic(t, "TODO port:", func() {
		place(s, 100, GroundY-1, "office")
	})
	w := s.world
	if w.Cells.At("100,108") != "office" || w.CellToAnchor.At("108,108") != "100,108" {
		t.Fatal("cells not written")
	}
	o := w.PlacedObjects.At("100,108")
	if o == nil || o.ObjectTypeCode != FamilyOffice || o.UnitStatus != 0x10 || o.RentLevel != 1 || o.RightTileIndex != 108 {
		t.Fatalf("placed object %+v", o)
	}
	if s.Cash() != StartingCash-40_000 || w.GateFlags.OfficePlaced != 1 {
		t.Fatalf("cash %d gates %+v", s.Cash(), w.GateFlags)
	}
	// RebuildFacilityLedger + RebuildRuntimeSims ran before the stub.
	if s.Ledger().PopulationLedger[ledgerOffice] != 6 || len(w.Sims) != 6 || w.Sims[0].StateCode != StateMorningGate {
		t.Fatalf("ledger %v sims %d", s.Ledger().PopulationLedger, len(w.Sims))
	}
	// Lobby tiles are family 0x18 objects only when placed via commands; the
	// bare strip has none, so the office is the only placed object.
	if w.PlacedObjects.Len() != 1 {
		t.Fatalf("objects %d", w.PlacedObjects.Len())
	}
}

func TestCommandsPlaceVenuesAllocateSidecars(t *testing.T) {
	s := bareLobbySim(90, 180)
	expectTODOPanic(t, "TODO port", func() { place(s, 100, GroundY-1, "retail") })
	cv, ok := s.world.Sidecars[0].(*CommercialVenueRecord)
	if !ok || cv.AvailabilityState != VenueDormant || cv.TodayVisitCount != 10 || cv.OwnerSubtypeIndex != 100 {
		t.Fatalf("retail sidecar %+v", s.world.Sidecars[0])
	}
	expectTODOPanic(t, "TODO port", func() { place(s, 120, GroundY-1, "parking") })
	// The TS only restricts what goes underground; parking above ground is
	// accepted and gets a service_request sidecar with its floor index.
	sr, ok := s.world.Sidecars[1].(*ServiceRequestEntry)
	if !ok || sr.FloorIndex == nil || *sr.FloorIndex != 11 || sr.CoverageFlag == nil || *sr.CoverageFlag != 0 {
		t.Fatalf("parking sidecar %+v", s.world.Sidecars[1])
	}
}

func TestCommandsPlaceCinemaDrawsRng(t *testing.T) {
	s := bareLobbySim(90, 180)
	setRow(s.world, GroundY-1, 90, 180, "floor")
	expectTODOPanic(t, "TODO port", func() { place(s, 100, GroundY-2, "cinema") })
	if s.world.RngCallCount != 14 {
		t.Fatalf("rng calls %d", s.world.RngCallCount)
	}
	link := s.world.Sidecars[0].(*EntertainmentLinkRecord)
	ref := newEmptyWorld("t", "t", LobbyModeModern)
	if want := sampleCinemaInitialSelector(ref); link.FamilySelectorOrSingleLinkFlag != want {
		t.Fatalf("selector %d want %d", link.FamilySelectorOrSingleLinkFlag, want)
	}
	if link.UpperHalfFloor != YToFloor(GroundY-2) || link.LowerHalfFloor != YToFloor(GroundY-2)-1 {
		t.Fatalf("link floors %+v", link)
	}
	// 4 sub-records sharing the sidecar.
	n := 0
	for _, o := range s.world.PlacedObjects.Values() {
		if o.LinkedRecordIndex == 0 {
			n++
		}
	}
	if n != 4 || s.world.PlacedObjects.At(cellKey(107, GroundY-2)).ObjectTypeCode != FamilyCinema {
		t.Fatalf("sub-records %d", n)
	}
}
