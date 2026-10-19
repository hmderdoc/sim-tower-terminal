package sim

import (
	"encoding/json"
	"testing"
)

// Port of the B2-relevant parts of determinism.test.ts: the fixture-based
// "hydration is idempotent" and "server vs hydrated client stay in lockstep"
// cases for the fixtures that exercise commercial venues (commercial, mixed).
// The small-mixed-tower and the hotel/office/condo/elevator fixtures belong
// to the phases that own those families. File named *_b2_test.go so it does
// not collide with another phase's determinism_test.go.

// TS: determinism.test.ts buildFromFixture
func buildDeterminismFixture(t *testing.T, name string) *TowerSim {
	t.Helper()
	spec := loadBuildSpec(t, name)
	sim := Create("determinism-fixture", "Determinism Fixture", LobbyModePerfectParity)
	placeTilesFromSpec(t, sim, spec)
	sim.SetStarCount(3)
	return sim
}

func b2StepN(sim *TowerSim, ticks int) {
	for i := 0; i < ticks; i++ {
		sim.Step()
	}
}

func b2SnapshotJSON(t *testing.T, sim *TowerSim) string {
	t.Helper()
	b, err := json.Marshal(sim.SaveState())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestDeterminismFixturesB2(t *testing.T) {
	for _, fixture := range []string{"mixed", "commercial"} {
		t.Run(fixture+"/idempotent", func(t *testing.T) {
			skipOnTODO(t, func() {
				server := buildDeterminismFixture(t, fixture)
				b2StepN(server, 1_000)
				before := b2SnapshotJSON(t, server)
				after := b2SnapshotJSON(t, FromSnapshot(server.SaveState()))
				if before != after {
					t.Fatalf("%s idempotent: snapshots diverge", fixture)
				}
			})
		})
		t.Run(fixture+"/lockstep", func(t *testing.T) {
			skipOnTODO(t, func() {
				server := buildDeterminismFixture(t, fixture)
				b2StepN(server, 1_000)
				client := FromSnapshot(server.SaveState())
				b2StepN(server, 500)
				b2StepN(client, 500)
				if b2SnapshotJSON(t, server) != b2SnapshotJSON(t, client) {
					t.Fatalf("%s lockstep: snapshots diverge", fixture)
				}
			})
		})
	}
}
