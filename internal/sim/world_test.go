package sim

import "testing"

func TestSampleRngMatchesLCG(t *testing.T) {
	w := newEmptyWorld("t", "t", LobbyModeModern)
	// Reference: state = imul(state, 0x15a4e35) + 1 | 0 from seed 1.
	state := int64(1)
	for i := 0; i < 1000; i++ {
		state = int64(int32(uint32(state)*0x15a4e35 + 1))
		want := int((uint32(int32(state)) >> 16) & 0x7fff)
		if got := SampleRng(w); got != want {
			t.Fatalf("draw %d: got %d want %d", i, got, want)
		}
	}
	if w.RngCallCount != 1000 || int64(w.RngState) != state {
		t.Fatalf("count %d state %d", w.RngCallCount, w.RngState)
	}
	// First draw from seed 1 is (0x15a4e36 >> 16) & 0x7fff = 0x15a.
	w2 := newEmptyWorld("t", "t", LobbyModeModern)
	if v := SampleRng(w2); v != 0x15a {
		t.Fatalf("first draw %#x", v)
	}
}

func TestLobbyCadence(t *testing.T) {
	cases := []struct {
		floor int
		mode  LobbyMode
		ok    bool
	}{
		{0, LobbyModeModern, true}, {15, LobbyModeModern, true}, {14, LobbyModeModern, false},
		{0, LobbyModePerfectParity, true}, {14, LobbyModePerfectParity, true}, {29, LobbyModePerfectParity, true},
		{15, LobbyModePerfectParity, false}, {-1, LobbyModeModern, false},
	}
	for _, c := range cases {
		if got := IsValidLobbyY(GroundY-c.floor, c.mode); got != c.ok {
			t.Errorf("floor %d mode %s: got %v", c.floor, c.mode, got)
		}
	}
}

func TestTileToFamilyCodeQuirks(t *testing.T) {
	if TileToFamilyCode["lobby"] != FamilyParking {
		t.Errorf("lobby -> %d", TileToFamilyCode["lobby"])
	}
	if TileToFamilyCode["metro"] != FamilyMetroMiddle {
		t.Errorf("metro -> %#x (TS last-write-wins gives 0x20)", TileToFamilyCode["metro"])
	}
	if TileToFamilyCode["cathedral"] != 0x21 {
		t.Errorf("cathedral -> %#x", TileToFamilyCode["cathedral"])
	}
	if _, ok := TileToFamilyCode["floor"]; ok {
		t.Errorf("floor should have no family")
	}
}

func TestAdvanceOneTick(t *testing.T) {
	tm := CreateNewGameTimeState()
	if tm.DayTick != 2533 || tm.DaypartIndex != 6 {
		t.Fatalf("new game %+v", tm)
	}
	crossed := 0
	for i := 0; i < DayTickMax*3; i++ {
		var c bool
		tm, c = AdvanceOneTick(tm)
		if c {
			crossed++
			if tm.DayTick != DayTickNewDay {
				t.Fatalf("checkpoint at tick %d", tm.DayTick)
			}
		}
		if tm.DaypartIndex != tm.DayTick/400 {
			t.Fatalf("daypart %d at tick %d", tm.DaypartIndex, tm.DayTick)
		}
	}
	if crossed != 3 || tm.DayCounter != 3 || tm.WeekendFlag != 0 || tm.TotalTicks != DayTickMax*3 {
		t.Fatalf("after 3 days %+v crossed=%d", tm, crossed)
	}
	tm2 := &TimeState{DayTick: DayTickNewDay - 1, DayCounter: 1}
	tm2, _ = AdvanceOneTick(tm2)
	if tm2.DayCounter != 2 || tm2.WeekendFlag != 1 {
		t.Fatalf("weekend flag %+v", tm2)
	}
}
