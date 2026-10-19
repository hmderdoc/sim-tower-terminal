package sim

import (
	"fmt"
	"strings"
	"testing"
)

// todoPanic runs fn and returns the recovered "TODO port" panic message, or
// "" when fn returned normally. Any other panic is re-raised.
func todoPanic(fn func()) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			s := fmt.Sprint(r)
			if strings.Contains(s, "TODO port:") {
				msg = s
				return
			}
			panic(r)
		}
	}()
	fn()
	return ""
}

// skipOnTODO runs a ported TS test body; if it reaches a not-yet-ported
// stub the test is skipped with the stub name (visible under -v), so the
// test starts asserting automatically once the stub is replaced.
func skipOnTODO(t *testing.T, fn func()) {
	t.Helper()
	if msg := todoPanic(fn); msg != "" {
		t.Skipf("blocked on unported code: %s", msg)
	}
}

// expectTODOPanic asserts fn panics on the named stub (A1 code ran up to
// the first later-phase call).
func expectTODOPanic(t *testing.T, want string, fn func()) {
	t.Helper()
	msg := todoPanic(fn)
	if msg == "" {
		// Stub already ported: fine, the caller's post-conditions still apply.
		return
	}
	if !strings.Contains(msg, want) {
		t.Fatalf("expected TODO panic %q, got %q", want, msg)
	}
}

// newBareSim builds a TowerSim from CreateInitialSnapshot without hydration
// (HydrateSnapshot calls A2 stubs), for exercising A1 validation paths.
func newBareSim(mode LobbyMode) *TowerSim {
	snap := CreateInitialSnapshot("bare", "Bare", StartingCash, mode)
	return newTowerSim(snap.Time, snap.World, snap.Ledger)
}

func setRow(w *WorldState, y, from, to int, tile string) {
	for x := from; x <= to; x++ {
		w.Cells.Set(cellKey(x, y), tile)
	}
}

func place(s *TowerSim, x, y int, tile string) CommandResult {
	return s.SubmitCommand(SimCommand{Type: CmdPlaceTile, X: x, Y: y, TileType: tile})
}

func testCarrier(column, mode, bottom, top int, cars ...*CarrierCar) *CarrierRecord {
	return &CarrierRecord{
		Column: column, CarrierMode: mode, BottomServedFloor: bottom, TopServedFloor: top,
		Cars: cars, DwellDelay: make([]int, 14),
	}
}
