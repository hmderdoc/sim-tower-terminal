package sim

import (
	"slices"
	"testing"
)

// TestRunCheckpointsInterval pins the (prev, curr] window and wrap handling
// against the real table's tick list.
func TestRunCheckpointsInterval(t *testing.T) {
	saved := dayCheckpoints
	defer func() { dayCheckpoints = saved }()
	var fired []int
	dayCheckpoints = nil
	for _, cp := range saved {
		tick := cp.tick
		dayCheckpoints = append(dayCheckpoints, dayCheckpoint{tick, func(*SimState) { fired = append(fired, tick) }})
	}
	run := func(prev, curr int) []int {
		fired = nil
		RunCheckpoints(&SimState{}, prev, curr)
		return fired
	}
	if got := run(0x0ef, 0x0f0); !slices.Equal(got, []int{0x0f0}) {
		t.Fatalf("0xf0: %v", got)
	}
	if got := run(0x0f0, 0x0f1); len(got) != 0 {
		t.Fatalf("0xf1: %v", got)
	}
	// Day wrap 2599 -> 0 fires tick 0 only.
	if got := run(2599, 0); !slices.Equal(got, []int{0x000}) {
		t.Fatalf("wrap: %v", got)
	}
	// A whole-day sweep in one call still fires in table order.
	if got := run(0xa06, 0xa05); len(got) != len(saved)-1 || got[0] != 0 || got[len(got)-1] != 0x9f6 {
		t.Fatalf("sweep: %v", got)
	}
	want := []int{0x000, 0x020, 0x0f0, 0x3e8, 0x4b0, 0x578, 0x5dc, 0x640, 0x708, 0x76c, 0x7d0, 0x898, 0x8fc, 0x9c4, 0x9e5, 0x9f6, 0xa06}
	var ticks []int
	for _, cp := range saved {
		ticks = append(ticks, cp.tick)
	}
	if !slices.Equal(ticks, want) {
		t.Fatalf("table ticks %v", ticks)
	}
}

// Between checkpoints the scheduler only advances time and runs the event
// gates (one random-news draw when daytime).
func TestRunSimulationDaySchedulerBetweenCheckpoints(t *testing.T) {
	w := newEmptyWorld("t", "T", LobbyModePerfectParity)
	ledger := CreateLedgerState(0)
	tm := &TimeState{DayTick: 0x700, DaypartIndex: 0x700 / 400, TotalTicks: 9}
	next := RunSimulationDayScheduler(w, ledger, tm)
	if next == tm || next.DayTick != 0x701 || next.TotalTicks != 10 || tm.DayTick != 0x700 {
		t.Fatalf("next %+v orig %+v", next, tm)
	}
	if w.RngCallCount != 1 {
		t.Fatalf("draws %d", w.RngCallCount)
	}
}

// A full day over an empty tower; blocked until the other phases land.
func TestRunSimulationDaySchedulerFullDay(t *testing.T) {
	skipOnTODO(t, func() {
		w := newEmptyWorld("t", "T", LobbyModePerfectParity)
		ledger := CreateLedgerState(2_000_000)
		tm := CreateNewGameTimeState()
		for i := 0; i < DayTickMax; i++ {
			tm = RunSimulationDayScheduler(w, ledger, tm)
		}
		if tm.DayTick != NewGameDayTick || tm.DayCounter != 1 {
			t.Fatalf("time %+v", tm)
		}
	})
}
