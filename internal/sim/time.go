package sim

// Port of reference/tower-together/sim/time.ts.

// Day cycle constants.
const (
	DayTickMax     = 2600  // 2600 ticks per day
	DayTickNewDay  = 2300  // checkpoint where dayCounter increments
	DayCounterWrap = 11988 // 12 days/year * 999 years
	// NewGameDayTick is the starting dayTick for a new game
	// (new_game_initializer at 0x10d8_07f6): daypart 6.
	NewGameDayTick = 0x9e5 // 2533
)

// TimeState TS: time.ts TimeState. The TS never mutates a TimeState in
// place inside the sim (advanceOneTick returns a fresh one), so callers may
// share *TimeState freely; the tick loop swaps in a new pointer.
type TimeState struct {
	// DayTick: position within the day (0-2599).
	DayTick int `json:"dayTick"`
	// DaypartIndex: dayTick / 400 (0-6).
	DaypartIndex int `json:"daypartIndex"`
	// DayCounter increments at checkpoint 2300 each day.
	DayCounter int `json:"dayCounter"`
	// WeekendFlag: dayCounter % 3 == 2 ? 1 : 0
	WeekendFlag int `json:"weekendFlag"`
	// TotalTicks: monotonically increasing tick counter since game start.
	TotalTicks int `json:"totalTicks"`
}

// CreateTimeState returns a zero time state.
// TS: time.ts createTimeState
func CreateTimeState() *TimeState {
	return &TimeState{}
}

// CreateNewGameTimeState matches new_game_initializer at 0x10d8_07f6.
// TS: time.ts createNewGameTimeState
func CreateNewGameTimeState() *TimeState {
	return &TimeState{
		DayTick:      NewGameDayTick,
		DaypartIndex: NewGameDayTick / 400, // = 6
	}
}

// AdvanceOneTick advances time by one tick, returning the new state and
// whether the DAY_TICK_NEW_DAY checkpoint was just crossed.
// TS: time.ts advanceOneTick
func AdvanceOneTick(t *TimeState) (*TimeState, bool) {
	totalTicks := t.TotalTicks + 1
	dayTick := t.DayTick + 1
	dayCounter := t.DayCounter
	weekendFlag := t.WeekendFlag
	incomeCheckpoint := false

	if dayTick >= DayTickMax {
		dayTick = 0
	}

	if dayTick == DayTickNewDay {
		dayCounter = t.DayCounter + 1
		if dayCounter >= DayCounterWrap {
			dayCounter = 0
		}
		if dayCounter%3 == 2 {
			weekendFlag = 1
		} else {
			weekendFlag = 0
		}
		incomeCheckpoint = true
	}

	return &TimeState{
		DayTick:      dayTick,
		DaypartIndex: dayTick / 400,
		DayCounter:   dayCounter,
		WeekendFlag:  weekendFlag,
		TotalTicks:   totalTicks,
	}, incomeCheckpoint
}

// PreDay4 TS: time.ts preDay4
func PreDay4(t *TimeState) bool {
	return t.DaypartIndex < 4
}
