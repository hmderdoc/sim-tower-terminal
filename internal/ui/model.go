package ui

// Model is everything the presentation layer reads from the game and every
// command it issues. The UI never touches internal/sim directly: a thin
// adapter maps the real simulation onto this interface (DemoModel in demo.go
// is the stand-in used for development and the golden tests).
//
// Coordinates: X is the left unit (0..Bounds().Width-1), Floor is the floor
// index (0 = 1F, 1 = 2F, -1 = B1). Multi-floor items give their BOTTOM floor.
// Slices returned by the read methods may be reused by the model on the next
// call; the UI only reads them within one frame.
type Model interface {
	// ---- reads ----
	Bounds() Bounds      // legal world extent (scroll clamp, map)
	Catalog() []BuildDef // build palette rows, in display order
	Floors() []FloorSpan // built floor area per floor (black interior)
	Items() []Item       // every room, lobby segment run, stairs, escalator
	Shafts() []Shaft     // elevator shafts with cars and queues
	People() []Person    // people walking on floors / lobbies (2-cell sprites)
	Clock() Clock        //
	Funds() int64        // dollars, may be negative
	Population() int     //
	Stars() int          // 1..5, 6 = TOWER
	Speed() int          // 0 paused, 1 normal, 2 fast
	Messages() []Message // oldest first; the message line shows the last
	Event() *Event       // active prompt (fire, bomb, VIP...) or nil
	Finance() Finance    // current quarter report
	Rating() Rating      // star rating report
	Inspect(itemID int) InspectInfo
	Schedule(shaftID int) Schedule
	Tenant(personID int) TenantInfo

	// ---- commands ----
	Step(dt float64) // advance by dt real seconds (the model applies Speed)
	SetSpeed(n int)
	CanPlace(k Kind, x, floor, w, h int) Result // w,h used by lobby/floor/shaft drags
	Place(k Kind, x, floor, w, h int) Result
	Remove(x, floor int) Result // bulldoze whatever is at the cell
	AddCar(shaftID, homeFloor int) Result
	RemoveCar(shaftID int) Result
	SetCarHome(shaftID, car, floor int) Result
	ToggleServe(shaftID, floor int) Result
	SetPeriodMode(shaftID int, weekend bool, period int, mode PeriodMode)
	SetResponse(shaftID, floors int)
	SetDeparture(shaftID, seconds int)
	SetRent(itemID, tier int)
	ChangeMovie(itemID int, classic bool) Result
	RenameTenant(personID int, name string) Result
	Respond(choice int) // answer the active Event's prompt (index into Choices)
	Save() error
}

// Kind is the UI's item vocabulary (the adapter maps sim kinds onto it).
type Kind uint8

const (
	KindNone Kind = iota
	KindFloor
	KindLobby
	KindOffice
	KindCondo
	KindHotelSingle
	KindHotelTwin
	KindHotelSuite
	KindFastFood
	KindRestaurant
	KindRetail
	KindCinema
	KindPartyHall
	KindParkingRamp
	KindParking
	KindMetro
	KindCathedral
	KindMedical
	KindSecurity
	KindHousekeeping
	KindRecycling
	KindStairs
	KindEscalator
	KindElevator // standard shaft
	KindExpress
	KindService
	KindBulldozer // palette tool only
	numKinds
)

// IsShaft reports whether k is an elevator kind.
func (k Kind) IsShaft() bool { return k == KindElevator || k == KindExpress || k == KindService }

// ShaftWidth is the shaft footprint in units including both walls.
func (k Kind) ShaftWidth() int {
	if k == KindExpress {
		return 6
	}
	return 4
}

// State is what an item is doing right now; the renderer picks a tile state
// from it (plus Lit and the clock).
type State uint8

const (
	StateIdle     State = iota // default / nothing special
	StateVacant                // for rent, unoccupied hotel room, empty parking
	StateOccupied              // rented, guest in room, car parked
	StateForSale               // condo
	StateAsleep                // hotel guest sleeping
	StateDirty                 // hotel room awaiting housekeeping
	StateInfested              // cockroaches
	StateVIP                   // VIP in the suite
	StateOpen                  // shop/restaurant open
	StateClosed                // shop/restaurant closed
	StateBusy                  // fast food rush
	StateActive                // cinema showing, party, metro train, wedding, stairs in use
	StateBurning
	StateBurned
)

// Eval is the tenant satisfaction color of the original's Eval view.
type Eval uint8

const (
	EvalNone   Eval = iota // not rated (gray)
	EvalBlue               // excellent
	EvalYellow             // good
	EvalRed                // terrible
)

// Stress tier of a person (sprite color).
type Stress uint8

const (
	StressNormal Stress = iota
	StressPink
	StressRed
)

type Bounds struct {
	MinFloor, MaxFloor int // e.g. -10 .. 99
	Width              int // units, e.g. 375
}

type FloorSpan struct {
	Floor       int
	Left, Right int // built units [Left, Right)
}

type Item struct {
	ID        int
	Kind      Kind
	X, Floor  int // left unit, bottom floor
	W, H      int // units, floors
	State     State
	Eval      Eval
	Lit       bool // lights on (night windows)
	Occupants int
	Capacity  int
	Customers int // fast food / restaurant / shop: today's customers
	Name      string
	Rent      int // tier 0 high .. 3 very low; -1 = not applicable
}

type Shaft struct {
	ID     int
	Kind   Kind // KindElevator, KindExpress or KindService
	X      int  // left wall unit; width from Kind.ShaftWidth()
	Bottom int  // lowest floor served (inclusive)
	Top    int  // highest floor (inclusive)
	Cars   []Car
	Queues []Queue
}

type Car struct {
	Pos        int // position in thirds of a floor: floor*3 + 0..2 (bottom of the car)
	Dir        int // -1 down, 0 idle, 1 up
	Passengers int
	Capacity   int
	DoorsOpen  bool
	Home       int // home (waiting) floor
}

// Queue is the people waiting at one floor of a shaft. Each slice holds one
// stress tier per waiting person, front of the queue first.
type Queue struct {
	Floor    int
	Up, Down []Stress
}

// Person is a walker drawn as a two-cell sprite (head + legs) on a floor.
type Person struct {
	ID     int
	X      int
	Floor  int
	Stress Stress
	Named  bool // renamed sims draw blue
}

type Daylight uint8

const (
	Day Daylight = iota
	Dusk
	Night
)

type Clock struct {
	Day      int    // days since start
	Tick     int    // tick within the day (model defined)
	Hour     int    // 0..23
	Minute   int    // 0..59
	DayLabel string // "WD1", "WD2", "WE"
	Weekend  bool
	Quarter  int // 1..4
	Year     int // 1..
	Light    Daylight
	Rain     bool
}

// TimeString formats the clock like the info bar: "10:42 AM".
func (c Clock) TimeString() string {
	h := c.Hour % 12
	if h == 0 {
		h = 12
	}
	ap := "AM"
	if c.Hour >= 12 {
		ap = "PM"
	}
	return itoa(h) + ":" + pad2(c.Minute) + " " + ap
}

type Message struct {
	Text  string // plain ASCII
	When  string // "Y1 Q2 WD1 10:42 AM"
	Alert bool   // drawn red (route failures, build errors)
}

type EventKind uint8

const (
	EventNone EventKind = iota
	EventFire
	EventBomb
	EventVIP
	EventTreasure
	EventSanta
	EventWedding
	EventInfo
)

type Event struct {
	Kind    EventKind
	Title   string
	Lines   []string
	Choices []string // button labels; Respond(index)
	Floor   int      // where it happens (the UI scrolls there); -100 = none
	X       int
}

type Line struct {
	Label  string
	Amount int64
}

type Finance struct {
	Income       []Line // up to 10
	Maintenance  []Line // up to 10
	OtherIncome  int64
	Construction int64
	PrevBalance  int64 // previous quarter's balance
	Balance      int64 // total balance (= funds)
	Quarter      string
}

type Rating struct {
	Stars      int
	Population int
	Next       string   // "3 stars at population 1,000"
	Needs      []string // missing requirements
}

// RentTier describes one of the four rent/price options.
type RentTier struct {
	Label  string // "$15,000"
	Amount int64
}

type InspectInfo struct {
	ID        int
	Kind      Kind
	Title     string // "Office, Floor 3"
	Status    string // "Occupied", "For Rent", "For Sale"
	Eval      Eval
	EvalText  string // "Good"
	Customers int    // fast food / restaurant: shown instead of the eval bar when >= 0
	ShowCount bool   // use Customers instead of the eval bar
	Occupants []int  // person IDs (click -> Tenant window)
	Length    string // "2 days"
	Rent      int    // selected tier, -1 if not rentable
	Tiers     []RentTier
	Income    string // "Daily sales $4,000"
	Message   string // "Stairs/Elevators are too far away"
	Movie     string // cinema only: current movie, "" otherwise
	Demo      bool   // placeholder data (DemoModel)
}

type PeriodMode uint8

const (
	ModeLocal PeriodMode = iota
	ModeExpressUp
	ModeExpressDown
)

// Schedule is the elevator window's data.
type Schedule struct {
	ShaftID   int
	Title     string           // "STANDARD ELEVATOR #1"
	Served    []bool           // index floor-Bottom
	Modes     [2][7]PeriodMode // [0]=weekday [1]=weekend, 7 periods
	Response  int              // Waiting Car Response, floors
	Departure int              // Standard Floor Departure, seconds
	Capacity  int
	CarCost   int64
}

// Periods are the elevator window's seven dayparts.
var Periods = [7]string{"7-12", "12-1230", "1230-1", "1-5", "5-9", "9-1", "1-7"}

// periodLabels are the 6-column CP437 headers the elevator window shows.
var periodLabels = [7]string{"7-12", CP("12-12½"), CP("12½-1"), "1-5", "5-9", "9-1", "1-7"}

type TenantInfo struct {
	ID      int
	Name    string
	From    string
	GoingTo string
	Stress  Stress
	Eval    Eval
	Demo    bool
}

// BuildDef is one palette row.
type BuildDef struct {
	Kind     Kind
	Name     string
	Hotkey   byte
	Cost     int64
	PerUnit  bool // cost is per unit (lobby, floor)
	W, H     int  // footprint (shafts: width only, H = 1)
	MinStars int
	Desc     string // footer hint
}

// Result of a command.
type Result struct {
	OK     bool
	Reason string
	Cost   int64
}
