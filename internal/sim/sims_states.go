package sim

// Port of reference/tower-together/sim/sims/states.ts: shared sim state
// codes, family sets and constants.

// LobbyFloor is the internal floor-slot index of the lobby.
const (
	LobbyFloor    = 10
	EvalZoneFloor = 109 // floor 0x6d
)

const (
	StateTransitFlag = 0x40
	StateBaseMask    = 0x3f
)

// WithTransitFlag TS: sims/states.ts withTransitFlag
func WithTransitFlag(baseStateCode int) int { return StateTransitFlag | baseStateCode }

// StateBaseCode TS: sims/states.ts stateBaseCode
func StateBaseCode(stateCode int) int { return stateCode & StateBaseMask }

// HasTransitFlag TS: sims/states.ts hasTransitFlag
func HasTransitFlag(stateCode int) bool { return stateCode&StateTransitFlag != 0 }

const (
	StateCommute       = 0x00 // commuting to destination
	StateActive        = 0x01 // active / in-stay / venue selection
	StateActiveAlt     = 0x02 // office alternate lunch/venue-selection state
	StateArrived       = 0x03 // arrived at destination
	StateCheckoutQueue = 0x04 // hotel checkout queue (non-last sibling)
	StateDeparture     = 0x05 // departing / returning
	StateTransition    = 0x10 // unit status transition (hotel checking out)
	StateMorningGate   = 0x20 // morning activation gate
	StateAtWork        = 0x21 // at work (office, post-commute)
	StateVenueTrip     = 0x22 // commercial venue trip in transit
	stateDwellReturn   = 0x23
	StateHotelParked   = 0x24 // hotel parked (awaiting guest)
	StateNightA        = 0x25 // night park variant A
	StateNightB        = 0x26 // night park / venue unavailable
	StateParked        = 0x27 // parked / idle
)

const (
	StateCommuteTransit     = StateTransitFlag | StateCommute
	StateActiveTransit      = StateTransitFlag | StateActive
	StateVenueTripTransit   = StateTransitFlag | StateVenueTrip
	StateDepartureTransit   = StateTransitFlag | StateDeparture
	StateEvalReturn         = StateDepartureTransit
	StateEvalOutbound       = StateTransitFlag | StateMorningGate
	StateMorningTransit     = StateEvalOutbound
	StateAtWorkTransit      = StateTransitFlag | StateAtWork
	StateVenueHomeTransit   = StateTransitFlag | StateVenueTrip
	StateDwellReturnTransit = StateTransitFlag | stateDwellReturn
)

const (
	UnitStatusOfficeOccupied     = 0x0f
	UnitStatusCondoOccupied      = 0x17
	UnitStatusCondoVacant        = 0x18
	UnitStatusCondoVacantEvening = 0x20
	UnitStatusHotelSoldOut       = 0x37
)

// RouteIdle TS: ROUTE_IDLE. RouteState is a value type in Go, so sharing
// the constant is safe.
var RouteIdle = RouteState{Mode: RouteModeIdle}

const (
	NoEvalEntity        = 0xffff
	EntityRefreshStride = 16
	ActivationTickCap   = 0x78
)

// EntityPopulationByType: sims allocated per placed object, by family.
// Missing families read as 0 (TS `?? 0`).
var EntityPopulationByType = map[int]int{
	FamilyHotelSingle: 2,
	FamilyHotelTwin:   3,
	FamilyHotelSuite:  3,
	FamilyRestaurant:  48,
	FamilyOffice:      6,
	FamilyCondo:       3,
	FamilyRetail:      48,
	FamilyFastFood:    48,
	// Cinema primary (0x12): 56 guest sims for the theater span.
	FamilyCinema:               56,
	FamilyRecyclingCenterUpper: 6,
	FamilySecurity:             6,
	// Housekeeping helpers (family 0x0f).
	FamilyHousekeeping: 6,
	// Party hall lower (0x1e): 40-slot occupant span.
	FamilyPartyHallLower: 40,
	// Cathedral object slices: 5 floor types x 8 slots = 40 guests.
	36: 8, // 0x24
	37: 8, // 0x25
	38: 8, // 0x26
	39: 8, // 0x27
	40: 8, // 0x28
}

var HotelFamilies = map[int]bool{
	FamilyHotelSingle: true,
	FamilyHotelTwin:   true,
	FamilyHotelSuite:  true,
}

// EvaluatableFamilies: placed objects carrying an evaluation score.
var EvaluatableFamilies = map[int]bool{
	FamilyHotelSingle: true,
	FamilyHotelTwin:   true,
	FamilyHotelSuite:  true,
	FamilyOffice:      true,
	FamilyCondo:       true,
}

var CommercialFamilies = map[int]bool{
	FamilyRestaurant: true,
	FamilyFastFood:   true,
	FamilyRetail:     true,
}

var CathedralFamilies = map[int]bool{0x24: true, 0x25: true, 0x26: true, 0x27: true, 0x28: true}

// BinaryAllocOrder: binary entity-table allocation order by family.
var BinaryAllocOrder = map[int]int{
	FamilyRecyclingCenterUpper: 0, // 0xf2
	FamilyRestaurant:           1, // 0xf4
	FamilyFastFood:             2, // 0xf6
	FamilyCondo:                3, // 0xf7
	FamilyOffice:               4, // 0xf9
	FamilyRetail:               5, // 0xfa
	FamilyHotelTwin:            6, // 0xfb
	FamilyHotelSuite:           7, // 0xfc
	FamilyHotelSingle:          8, // 0xfd
	FamilyHousekeeping:         14,
	36:                         9,
	37:                         10,
	38:                         11,
	39:                         12,
	40:                         13,
}

var ElevatorDemandStates = map[int]bool{
	StateCommute:            true,
	StateActive:             true,
	StateActiveAlt:          true,
	StateCheckoutQueue:      true,
	StateDeparture:          true,
	StateVenueTrip:          true,
	StateCommuteTransit:     true,
	StateActiveTransit:      true,
	StateVenueTripTransit:   true,
	StateDepartureTransit:   true,
	StateEvalOutbound:       true,
	StateAtWorkTransit:      true,
	StateDwellReturnTransit: true,
	// STATE_EVAL_RETURN == STATE_DEPARTURE_TRANSIT and
	// STATE_VENUE_HOME_TRANSIT == STATE_VENUE_TRIP_TRANSIT (duplicates in the TS set).
}

// Housekeeping helper (family 0x0f) state codes.
const (
	HkStateSearch                  = 0
	HkStateRouteToCandidate        = 1
	HkStateCountdown               = 2
	HkStateRouteToTarget           = 3
	HkStateRouteToCandidateTransit = 4
	HkSearchingSentinel            = -1
	HkPostClaimCountdown           = 3
	HkClaimDayTickCutoff           = 1500
	HkFloorClassMod                = 6
)

// InvalidFloor: carrier slot cleanup / unset cathedral owner sentinel.
const InvalidFloor = 0xff
const CommercialVenueDwellTicks = 60

// CommercialDwellState: the binary encodes this as 0x62 (transit bit set).
const CommercialDwellState = StateVenueHomeTransit

// Venue selector family sets (select_random_commercial_venue_record_from_bucket).
var (
	CondoSelectorRetail     = map[int]bool{FamilyRetail: true}
	CondoSelectorRestaurant = map[int]bool{FamilyRestaurant: true}
	CondoSelectorFastFood   = map[int]bool{FamilyFastFood: true}
	// HotelRoomSelector: hotel rooms only ever visit restaurants.
	HotelRoomSelector = map[int]bool{FamilyRestaurant: true}
)
