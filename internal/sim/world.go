package sim

import (
	"encoding/json"
	"fmt"
)

// Port of reference/tower-together/sim/world.ts: grid constants, RNG, and
// every record type that lives inside WorldState.

// Grid and floor model constants.
const (
	GridWidth  = 375
	GridHeight = 120 // floor indices 0-119; floor 10 = ground ("0"), floor 119 = top
)

// YToFloor converts a grid Y coordinate to a floor index (0=bottom underground, 119=top).
// TS: world.ts yToFloor
func YToFloor(y int) int {
	return GridHeight - 1 - y
}

// FloorToY converts a floor index to a grid Y coordinate.
// TS: world.ts floorToY
func FloorToY(floor int) int {
	return GridHeight - 1 - floor
}

const (
	UndergroundFloors = 10                                 // floors 0-9 underground; floor 10 = ground ("0")
	UndergroundY      = GridHeight - UndergroundFloors     // Y=110: first underground row
	GroundY           = GridHeight - 1 - UndergroundFloors // Y=109: ground lobby row
)

// LobbyMode is the lobby placement / express-stop spacing mode.
//
//	"perfect-parity": ground + sky lobbies at floors 0, 14, 29, 44, ... (binary parity).
//	"modern":         ground + sky lobbies at floors 0, 15, 30, 45, ...
type LobbyMode string

const (
	LobbyModePerfectParity LobbyMode = "perfect-parity"
	LobbyModeModern        LobbyMode = "modern"
	DefaultLobbyMode                 = LobbyModeModern
)

// IsValidLobbyY reports whether y is a valid lobby row for the lobby mode.
// TS: world.ts isValidLobbyY
func IsValidLobbyY(y int, mode LobbyMode) bool {
	floorsAboveGround := GroundY - y
	if floorsAboveGround < 0 {
		return false
	}
	if floorsAboveGround == 0 {
		return true
	}
	offset := 14
	if mode == LobbyModeModern {
		offset = 0
	}
	return floorsAboveGround%15 == offset
}

// ─── PRNG ────────────────────────────────────────────────────────────────────

// SampleRng samples a 15-bit LCG value from the world RNG and advances it.
// 32-bit LCG: state = state * 0x15a4e35 + 1 (mod 2^32); returns
// (state >>> 16) & 0x7fff. rngState is kept as a signed int32 like the TS
// `| 0` result so saves round-trip identically.
// TS: world.ts sampleRng
// PORT-NOTE: the TS __RNG_TRACE debug shim (stack-trace logging) is not ported.
func SampleRng(world *WorldState) int {
	world.RngState = int32(uint32(world.RngState)*0x15a4e35 + 1)
	world.RngCallCount++
	return int((uint32(world.RngState) >> 16) & 0x7fff)
}

// ─── Carrier types ────────────────────────────────────────────────────────────

// CarrierCar is one car of a carrier shaft. TS: world.ts CarrierCar
type CarrierCar struct {
	Active                               bool `json:"active"`
	CurrentFloor                         int  `json:"currentFloor"`
	SettleCounter                        int  `json:"settleCounter"`
	DwellCounter                         int  `json:"dwellCounter"`
	AssignedCount                        int  `json:"assignedCount"`
	PendingAssignmentCount               int  `json:"pendingAssignmentCount"`
	DwellStartPendingAssignmentCount     int  `json:"dwellStartPendingAssignmentCount"`
	DirectionFlag                        int  `json:"directionFlag"` // 0 = downward, 1 = upward
	TargetFloor                          int  `json:"targetFloor"`
	PrevFloor                            int  `json:"prevFloor"`
	HomeFloor                            int  `json:"homeFloor"`
	NearestWorkFloor                     int  `json:"nearestWorkFloor"` // binary -0x51
	ScheduleFlag                         int  `json:"scheduleFlag"`
	ArrivalSeen                          int  `json:"arrivalSeen"` // binary -0x57
	ArrivalTick                          int  `json:"arrivalTick"` // binary -0x56
	ArrivalDispatchThisTick              bool `json:"arrivalDispatchThisTick"`
	ArrivalDispatchStartingAssignedCount int  `json:"arrivalDispatchStartingAssignedCount"`
	SuppressDwellOppositeDirectionFlip   bool `json:"suppressDwellOppositeDirectionFlip"`
	// WaitingCount is the waiting sim count indexed by floor slot.
	WaitingCount             []int               `json:"waitingCount"`
	DestinationCountByFloor  []int               `json:"destinationCountByFloor"`
	NonemptyDestinationCount int                 `json:"nonemptyDestinationCount"`
	ActiveRouteSlots         []*CarrierRouteSlot `json:"activeRouteSlots"`
	PendingRouteIDs          []string            `json:"pendingRouteIds"`
}

// CarrierRouteSlot TS: world.ts CarrierRouteSlot
type CarrierRouteSlot struct {
	RouteID          string `json:"routeId"`
	SourceFloor      int    `json:"sourceFloor"`
	DestinationFloor int    `json:"destinationFloor"`
	Boarded          bool   `json:"boarded"`
	Active           bool   `json:"active"`
}

// CarrierFloorQueue TS: world.ts CarrierFloorQueue
type CarrierFloorQueue struct {
	Up   *RouteRequestRing `json:"up"`
	Down *RouteRequestRing `json:"down"`
}

// CarrierPendingRoute TS: world.ts CarrierPendingRoute
type CarrierPendingRoute struct {
	SimID            string `json:"simId"`
	SourceFloor      int    `json:"sourceFloor"`
	DestinationFloor int    `json:"destinationFloor"`
	Boarded          bool   `json:"boarded"`
	DirectionFlag    int    `json:"directionFlag"`
	AssignedCarIndex int    `json:"assignedCarIndex"`
}

// CarrierRecord is one elevator shaft. TS: world.ts CarrierRecord
type CarrierRecord struct {
	CarrierID int `json:"carrierId"`
	// Column is the X column of the shaft.
	Column int `json:"column"`
	// CarrierMode: 0 = Express, 1 = Standard, 2 = Service.
	CarrierMode       int   `json:"carrierMode"`
	TopServedFloor    int   `json:"topServedFloor"`
	BottomServedFloor int   `json:"bottomServedFloor"`
	ServedFloorFlags  []int `json:"servedFloorFlags"` // 14 entries: 7 dayparts x 2 calendar phases
	// Per-floor route status tags (car index + 1, 0 = none).
	PrimaryRouteStatusByFloor   []int `json:"primaryRouteStatusByFloor"`
	SecondaryRouteStatusByFloor []int `json:"secondaryRouteStatusByFloor"`
	ServiceScheduleFlags        []int `json:"serviceScheduleFlags"`
	// DwellDelay: 14 entries, per-daypart departure-dwell multiplier (x30 ticks).
	DwellDelay []int `json:"dwellDelay"`
	// ExpressDirectionFlags: 14 entries; 0 normal, 1 express to top, 2 express to bottom.
	ExpressDirectionFlags       []int                  `json:"expressDirectionFlags"`
	WaitingCarResponseThreshold int                    `json:"waitingCarResponseThreshold"`
	AssignmentCapacity          int                    `json:"assignmentCapacity"`
	FloorQueues                 []*CarrierFloorQueue   `json:"floorQueues"`
	PendingRoutes               []*CarrierPendingRoute `json:"pendingRoutes"`
	CompletedRouteIDs           []string               `json:"completedRouteIds"`
	SuppressedFloorAssignments  []string               `json:"suppressedFloorAssignments"`
	// StopFloorEnabled: per-slot stop flag. 1 = stops, 0 = passes through.
	StopFloorEnabled []int         `json:"stopFloorEnabled"`
	Cars             []*CarrierCar `json:"cars"`
}

// ─── Runtime sims ────────────────────────────────────────────────────────

// Route modes for RouteState.Mode.
const (
	RouteModeIdle    = "idle"
	RouteModeSegment = "segment"
	RouteModeCarrier = "carrier"
)

// RouteState is the TS discriminated union
//
//	{ mode: "idle" } | { mode: "segment"; segmentId; destination }
//	| { mode: "carrier"; carrierId; direction: "up"|"down"; source }
//
// flattened into one struct. Only the fields of the active Mode are
// meaningful; the others stay zero.
type RouteState struct {
	Mode        string `json:"mode"`
	SegmentID   int    `json:"segmentId,omitempty"`
	Destination int    `json:"destination,omitempty"`
	CarrierID   int    `json:"carrierId,omitempty"`
	Direction   string `json:"direction,omitempty"` // "up" | "down"
	Source      int    `json:"source,omitempty"`
}

// SimRecord is one runtime sim. TS: world.ts SimRecord
type SimRecord struct {
	FloorAnchor int `json:"floorAnchor"`
	HomeColumn  int `json:"homeColumn"`
	BaseOffset  int `json:"baseOffset"`
	// FacilitySlot: per-floor local object index (binary sim+1 / BP+0xc).
	FacilitySlot int `json:"facilitySlot"`
	FamilyCode   int `json:"familyCode"`
	// StateCode: SimRecord+5 state_code byte. bits 0..3 phase, 0x20 waiting, 0x40 in transit.
	StateCode             int        `json:"stateCode"`
	Route                 RouteState `json:"route"`
	SelectedFloor         int        `json:"selectedFloor"`
	OriginFloor           int        `json:"originFloor"`
	DestinationFloor      int        `json:"destinationFloor"`
	VenueReturnState      int        `json:"venueReturnState"`
	QueueTick             int        `json:"queueTick"`
	ElapsedTicks          int        `json:"elapsedTicks"`
	TransitTicksRemaining int        `json:"transitTicksRemaining"`
	LastDemandTick        int        `json:"lastDemandTick"`
	TripCount             int        `json:"tripCount"`
	AccumulatedTicks      int        `json:"accumulatedTicks"`
	// Housekeeping helper (family 0x0f) fields.
	TargetRoomFloor    int `json:"targetRoomFloor"`
	TargetRoomColumn   int `json:"targetRoomColumn"`
	SpawnFloor         int `json:"spawnFloor"`
	PostClaimCountdown int `json:"postClaimCountdown"`
	EncodedTargetFloor int `json:"encodedTargetFloor"`
	// CommercialVenueSlot: binary sim[+6] venue sidecar index; -1 = lobby fallback.
	CommercialVenueSlot int `json:"commercialVenueSlot"`

	// ownerCache mirrors the TS module-level `simOwnerCache` WeakMap in
	// sims/population.ts (findObjectForSim). Not serialized; a sim object
	// created by hydration or rebuild starts with an empty cache, exactly
	// like a fresh JS object misses the WeakMap.
	ownerCache *PlacedObjectRecord
}

// ─── Routing types ────────────────────────────────────────────────────────────

const (
	MaxSpecialLinks       = 64
	MaxSpecialLinkRecords = 8
	MaxTransferGroups     = 16
)

// SpecialLinkSegment TS: world.ts SpecialLinkSegment
type SpecialLinkSegment struct {
	Active bool `json:"active"`
	// Flags: bit 0 = stairs cost bit; bits 7:1 = inclusive span length in floors.
	Flags                 int `json:"flags"`
	HeightMetric          int `json:"heightMetric"`
	EntryFloor            int `json:"entryFloor"`
	ReservedByte          int `json:"reservedByte"`
	DescendingLoadCounter int `json:"descendingLoadCounter"`
	AscendingLoadCounter  int `json:"ascendingLoadCounter"`
}

// SpecialLinkRecord TS: world.ts SpecialLinkRecord
type SpecialLinkRecord struct {
	Active                   bool  `json:"active"`
	LowerFloor               int   `json:"lowerFloor"`
	UpperFloor               int   `json:"upperFloor"`
	ReachabilityMasksByFloor []int `json:"reachabilityMasksByFloor"`
}

// TransferGroupEntry TS: world.ts TransferGroupEntry
type TransferGroupEntry struct {
	Active      bool `json:"active"`
	TaggedFloor int  `json:"taggedFloor"`
	CarrierMask int  `json:"carrierMask"`
}

// ─── PlacedObjectRecord ───────────────────────────────────────────────────────

// PlacedObjectRecord is the per-object record for every placed
// non-infrastructure tile, keyed in WorldState.PlacedObjects by "anchorX,y".
// TS: world.ts PlacedObjectRecord
type PlacedObjectRecord struct {
	LeftTileIndex   int `json:"leftTileIndex"`
	RightTileIndex  int `json:"rightTileIndex"`
	ObjectTypeCode  int `json:"objectTypeCode"`
	UnitStatus      int `json:"unitStatus"`
	AuxValueOrTimer int `json:"auxValueOrTimer"`
	// LinkedRecordIndex indexes WorldState.Sidecars; -1 when none.
	LinkedRecordIndex int `json:"linkedRecordIndex"`
	// DirtyFlag: binary +0x13 cashflow-needed flag.
	DirtyFlag int `json:"dirtyFlag"`
	// OccupiedFlag: binary +0x14 occupied / scored flag.
	OccupiedFlag int `json:"occupiedFlag"`
	// EvalLevel: 0 bad, 1 ok, 2 good; 0xff until scored.
	EvalLevel int `json:"evalLevel"`
	// EvalScore: raw average stress, -1 when no trip yet (display only).
	EvalScore int `json:"evalScore"`
	// RentLevel: pricing tier 0-3; 4 = no payout.
	RentLevel               int  `json:"rentLevel"`
	ActivationTickCount     int  `json:"activationTickCount"`
	HousekeepingClaimedFlag int  `json:"housekeepingClaimedFlag"`
	VipFlag                 bool `json:"vipFlag,omitempty"`
}

// ─── Gate flags ───────────────────────────────────────────────────────────────

// GateFlags are the global simulation gate flags. TS: world.ts GateFlags
type GateFlags struct {
	UnknownC198              int `json:"unknownC198"`
	MetroPlaced              int `json:"metroPlaced"`
	MetroStationFloorIndex   int `json:"metroStationFloorIndex"`
	OfficePlaced             int `json:"officePlaced"`
	SecurityPlaced           int `json:"securityPlaced"`
	OfficeServiceOk          int `json:"officeServiceOk"`
	OfficeServiceOkMedical   int `json:"officeServiceOkMedical"`
	RecyclingAdequate        int `json:"recyclingAdequate"`
	RoutesViable             int `json:"routesViable"`
	VipSuiteFloor            int `json:"vipSuiteFloor"`
	EvalSimIndex             int `json:"evalSimIndex"`
	RecyclingCenterCount     int `json:"recyclingCenterCount"`
	FacilityProgressOverride int `json:"facilityProgressOverride"`
	Family345SaleCount       int `json:"family345SaleCount"`
	NewspaperTrigger         int `json:"newspaperTrigger"`
}

// CreateGateFlags TS: world.ts createGateFlags
func CreateGateFlags() GateFlags {
	return GateFlags{
		UnknownC198:            0xffffffff,
		MetroStationFloorIndex: -1,
		VipSuiteFloor:          0xffff, // no VIP suite
		EvalSimIndex:           0xffff, // no cathedral placed
	}
}

// ─── Sidecar records ──────────────────────────────────────────────────────────

// Sidecar kind strings (the TS `kind` discriminant).
const (
	SidecarKindCommercialVenue   = "commercial_venue"
	SidecarKindServiceRequest    = "service_request"
	SidecarKindEntertainmentLink = "entertainment_link"
	SidecarKindMedicalCenter     = "medical_center"
)

// SidecarRecord is the TS union CommercialVenueRecord | ServiceRequestEntry |
// EntertainmentLinkRecord | MedicalCenterRecord. Concrete values are always
// pointers; switch on type (`s.(*CommercialVenueRecord)`) where the TS
// tests `sidecar.kind === "..."`.
type SidecarRecord interface {
	SidecarKind() string
	// GetOwnerSubtypeIndex / SetOwnerSubtypeIndex expose the field every
	// variant shares (freeSidecar writes 0xff through the union).
	GetOwnerSubtypeIndex() int
	SetOwnerSubtypeIndex(v int)
}

// CommercialVenueRecord TS: world.ts CommercialVenueRecord
type CommercialVenueRecord struct {
	OwnerSubtypeIndex    int `json:"ownerSubtypeIndex"` // 0xff = invalid / demolished
	Capacity             int `json:"capacity"`
	VisitCount           int `json:"visitCount"`
	TodayVisitCount      int `json:"todayVisitCount"`     // binary +0x7
	YesterdayVisitCount  int `json:"yesterdayVisitCount"` // binary +0x8
	AcquireCount         int `json:"acquireCount"`        // binary +0x10
	AvailabilityState    int `json:"availabilityState"`
	CurrentPopulation    int `json:"currentPopulation"`    // binary +0x09
	LastAcquireTick      int `json:"lastAcquireTick"`      // binary +0x0a
	EligibilityThreshold int `json:"eligibilityThreshold"` // binary +0x0c (signed)
	RemainingCapacity    int `json:"remainingCapacity"`    // binary +0x06
	PhaseASeed           int `json:"phaseASeed"`           // binary +0x03
	PhaseBSeed           int `json:"phaseBSeed"`           // binary +0x04
	OverrideSeed         int `json:"overrideSeed"`         // binary +0x05
}

// ServiceRequestEntry TS: world.ts ServiceRequestEntry. The two optional TS
// fields are pointers: nil == undefined (parking.ts tests
// `floorIndex === undefined`).
type ServiceRequestEntry struct {
	OwnerSubtypeIndex int  `json:"ownerSubtypeIndex"`
	FloorIndex        *int `json:"floorIndex,omitempty"`
	CoverageFlag      *int `json:"coverageFlag,omitempty"` // 0 uncovered, 1 covered
}

// EntertainmentLinkRecord TS: world.ts EntertainmentLinkRecord
type EntertainmentLinkRecord struct {
	OwnerSubtypeIndex              int `json:"ownerSubtypeIndex"`
	PairedSubtypeIndex             int `json:"pairedSubtypeIndex"`             // 0xff = no pair
	FamilySelectorOrSingleLinkFlag int `json:"familySelectorOrSingleLinkFlag"` // 0xff single; 0..13 selector
	LinkAgeCounter                 int `json:"linkAgeCounter"`
	UpperBudget                    int `json:"upperBudget"`
	LowerBudget                    int `json:"lowerBudget"`
	LinkPhaseState                 int `json:"linkPhaseState"`
	PendingTransitionFlag          int `json:"pendingTransitionFlag"`
	AttendanceCounter              int `json:"attendanceCounter"`
	ActiveRuntimeCount             int `json:"activeRuntimeCount"`
	LowerHalfFloor                 int `json:"lowerHalfFloor"` // binary +0x03
	UpperHalfFloor                 int `json:"upperHalfFloor"` // binary +0x00
}

// MedicalCenterRecord TS: world.ts MedicalCenterRecord
type MedicalCenterRecord struct {
	OwnerSubtypeIndex    int `json:"ownerSubtypeIndex"`
	PendingVisitorsCount int `json:"pendingVisitorsCount"`
}

func (*CommercialVenueRecord) SidecarKind() string   { return SidecarKindCommercialVenue }
func (*ServiceRequestEntry) SidecarKind() string     { return SidecarKindServiceRequest }
func (*EntertainmentLinkRecord) SidecarKind() string { return SidecarKindEntertainmentLink }
func (*MedicalCenterRecord) SidecarKind() string     { return SidecarKindMedicalCenter }

func (r *CommercialVenueRecord) GetOwnerSubtypeIndex() int    { return r.OwnerSubtypeIndex }
func (r *ServiceRequestEntry) GetOwnerSubtypeIndex() int      { return r.OwnerSubtypeIndex }
func (r *EntertainmentLinkRecord) GetOwnerSubtypeIndex() int  { return r.OwnerSubtypeIndex }
func (r *MedicalCenterRecord) GetOwnerSubtypeIndex() int      { return r.OwnerSubtypeIndex }
func (r *CommercialVenueRecord) SetOwnerSubtypeIndex(v int)   { r.OwnerSubtypeIndex = v }
func (r *ServiceRequestEntry) SetOwnerSubtypeIndex(v int)     { r.OwnerSubtypeIndex = v }
func (r *EntertainmentLinkRecord) SetOwnerSubtypeIndex(v int) { r.OwnerSubtypeIndex = v }
func (r *MedicalCenterRecord) SetOwnerSubtypeIndex(v int)     { r.OwnerSubtypeIndex = v }

func marshalWithKind(kind string, v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	kb, _ := json.Marshal(kind)
	out := make([]byte, 0, len(b)+len(kb)+8)
	out = append(out, `{"kind":`...)
	out = append(out, kb...)
	if len(b) > 2 {
		out = append(out, ',')
		out = append(out, b[1:]...)
	} else {
		out = append(out, '}')
	}
	return out, nil
}

func (r CommercialVenueRecord) MarshalJSON() ([]byte, error) {
	type plain CommercialVenueRecord
	return marshalWithKind(SidecarKindCommercialVenue, plain(r))
}
func (r ServiceRequestEntry) MarshalJSON() ([]byte, error) {
	type plain ServiceRequestEntry
	return marshalWithKind(SidecarKindServiceRequest, plain(r))
}
func (r EntertainmentLinkRecord) MarshalJSON() ([]byte, error) {
	type plain EntertainmentLinkRecord
	return marshalWithKind(SidecarKindEntertainmentLink, plain(r))
}
func (r MedicalCenterRecord) MarshalJSON() ([]byte, error) {
	type plain MedicalCenterRecord
	return marshalWithKind(SidecarKindMedicalCenter, plain(r))
}

// SidecarList is WorldState.Sidecars; it exists only to decode the
// polymorphic JSON array by its "kind" member.
type SidecarList []SidecarRecord

// UnmarshalJSON decodes each element by its "kind".
func (l *SidecarList) UnmarshalJSON(data []byte) error {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return err
	}
	out := make(SidecarList, 0, len(raws))
	for _, raw := range raws {
		var head struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			return err
		}
		var rec SidecarRecord
		switch head.Kind {
		case SidecarKindCommercialVenue:
			rec = &CommercialVenueRecord{}
		case SidecarKindServiceRequest:
			rec = &ServiceRequestEntry{}
		case SidecarKindEntertainmentLink:
			rec = &EntertainmentLinkRecord{}
		case SidecarKindMedicalCenter:
			rec = &MedicalCenterRecord{}
		default:
			return fmt.Errorf("sidecar: unknown kind %q", head.Kind)
		}
		if err := json.Unmarshal(raw, rec); err != nil {
			return err
		}
		out = append(out, rec)
	}
	*l = out
	return nil
}

// GetSidecar returns world.Sidecars[index], or nil when index is out of
// range (JS `world.sidecars[i]` yielding undefined, e.g. for -1).
func GetSidecar(world *WorldState, index int) SidecarRecord {
	if index < 0 || index >= len(world.Sidecars) {
		return nil
	}
	return world.Sidecars[index]
}

// CommercialVenueBuckets: per-family runtime path buckets (7 zone rows each)
// holding sidecar indices. TS: world.ts CommercialVenueBuckets
type CommercialVenueBuckets struct {
	Retail     [][]int `json:"retail"`
	Restaurant [][]int `json:"restaurant"`
	FastFood   [][]int `json:"fastFood"`
}

const CommercialVenueBucketRows = 7

// CreateCommercialVenueBuckets TS: world.ts createCommercialVenueBuckets
func CreateCommercialVenueBuckets() CommercialVenueBuckets {
	empty := func() [][]int {
		rows := make([][]int, CommercialVenueBucketRows)
		for i := range rows {
			rows[i] = []int{}
		}
		return rows
	}
	return CommercialVenueBuckets{Retail: empty(), Restaurant: empty(), FastFood: empty()}
}

// CommercialVenueRecord.AvailabilityState values.
const (
	VenueAvailable = 0
	VenuePartial   = 1    // active_assignment_count 1..9
	VenueNearFull  = 2    // active_assignment_count >= 10
	VenueClosed    = 3    // daily off-hours closure
	VenueDormant   = 0xff // inactive
)

// MedicalServiceSlot is one of the 10 global medical service-request slots.
// TS: world.ts MedicalServiceSlot
type MedicalServiceSlot struct {
	Active             bool   `json:"active"`
	SimID              string `json:"simId"`
	SourceFloor        int    `json:"sourceFloor"`
	TargetSidecarIndex int    `json:"targetSidecarIndex"`
	RetryCounter       int    `json:"retryCounter"`
}

const (
	MaxMedicalServiceSlots = 10
	MedicalRetryOverflow   = 0x28
)

// CreateMedicalServiceSlots TS: world.ts createMedicalServiceSlots
func CreateMedicalServiceSlots() []*MedicalServiceSlot {
	out := make([]*MedicalServiceSlot, MaxMedicalServiceSlots)
	for i := range out {
		out[i] = &MedicalServiceSlot{SourceFloor: -1, TargetSidecarIndex: -1}
	}
	return out
}

// ─── Event state ─────────────────────────────────────────────────────────────

// EventState TS: world.ts EventState
type EventState struct {
	// GameStateFlags: bit 0 bomb search, bit 3 fire, bit 5 bomb found, bit 6 detonated.
	GameStateFlags           int   `json:"gameStateFlags"`
	BombFloor                int   `json:"bombFloor"`
	BombTile                 int   `json:"bombTile"`
	BombDeadline             int   `json:"bombDeadline"`
	FireFloor                int   `json:"fireFloor"`
	FireTile                 int   `json:"fireTile"`
	FireStartTick            int   `json:"fireStartTick"`
	FireLeftPos              []int `json:"fireLeftPos"`  // 120 entries, 0xffff inactive
	FireRightPos             []int `json:"fireRightPos"` // 120 entries, 0xffff inactive
	RescueCountdown          int   `json:"rescueCountdown"`
	HelicopterExtinguishPos  int   `json:"helicopterExtinguishPos"`
	LcgState                 int   `json:"lcgState"`
	BombSearchLowerBound     int   `json:"bombSearchLowerBound"`
	BombSearchUpperBound     int   `json:"bombSearchUpperBound"`
	BombSearchCurrentFloor   int   `json:"bombSearchCurrentFloor"`
	BombSearchScanTile       int   `json:"bombSearchScanTile"`
	PendingCarrierEditColumn int   `json:"pendingCarrierEditColumn"`
	PendingCarrierEditY      int   `json:"pendingCarrierEditY"`
	// DisableNewsEvents suppresses triggerRandomNewsEvent (trace-test only).
	DisableNewsEvents bool                `json:"disableNewsEvents,omitempty"`
	FireRescueHelpers []*FireRescueHelper `json:"fireRescueHelpers"`
}

// FireRescueHelper TS: world.ts FireRescueHelper
type FireRescueHelper struct {
	Floor  int `json:"floor"`
	Column int `json:"column"`
	// Status: 0 walking, 1 winding up, 2 relocating, 3 idle/done.
	Status          int `json:"status"`
	WindupRemaining int `json:"windupRemaining"`
}

// CreateEventState TS: world.ts createEventState
func CreateEventState() EventState {
	return EventState{
		FireLeftPos:              filledInts(GridHeight, 0xffff),
		FireRightPos:             filledInts(GridHeight, 0xffff),
		LcgState:                 1,
		BombSearchLowerBound:     -1,
		BombSearchUpperBound:     -1,
		BombSearchCurrentFloor:   -1,
		BombSearchScanTile:       -1,
		PendingCarrierEditColumn: -1,
		PendingCarrierEditY:      -1,
		FireRescueHelpers:        []*FireRescueHelper{},
	}
}

// ─── WorldState ───────────────────────────────────────────────────────────────

// WorldState holds all placed tile data for one tower. TS: world.ts WorldState
//
// The four cell maps and PlacedObjects are JS objects keyed by "x,y" strings,
// which iterate in insertion order, so they are OrderedMaps (PORT.md rule 3).
type WorldState struct {
	TowerID     string    `json:"towerId"`
	Name        string    `json:"name"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	LobbyHeight int       `json:"lobbyHeight"`
	LobbyMode   LobbyMode `json:"lobbyMode"`
	GateFlags   GateFlags `json:"gateFlags"`
	// Cells: "x,y" -> tileType for every occupied cell.
	Cells OrderedMap[string, string] `json:"cells"`
	// CellToAnchor: extension cell key -> anchor cell key.
	CellToAnchor OrderedMap[string, string] `json:"cellToAnchor"`
	// Overlays: anchor cell key -> overlay tileType (e.g. "stairs").
	Overlays OrderedMap[string, string] `json:"overlays"`
	// OverlayToAnchor: extension cell key -> anchor cell key for overlays.
	OverlayToAnchor OrderedMap[string, string] `json:"overlayToAnchor"`
	// PlacedObjects: "anchorX,y" -> record for every simulated tile.
	PlacedObjects      OrderedMap[string, *PlacedObjectRecord] `json:"placedObjects"`
	Sidecars           SidecarList                             `json:"sidecars"`
	Sims               []*SimRecord                            `json:"sims"`
	Carriers           []*CarrierRecord                        `json:"carriers"`
	SpecialLinks       []*SpecialLinkSegment                   `json:"specialLinks"`
	SpecialLinkRecords []*SpecialLinkRecord                    `json:"specialLinkRecords"`
	// FloorWalkabilityFlags: per-floor bitmask, size GridHeight.
	FloorWalkabilityFlags []int                 `json:"floorWalkabilityFlags"`
	TransferGroupEntries  []*TransferGroupEntry `json:"transferGroupEntries"`
	// TransferGroupCache: per-floor bitmask of carrier ids, size GridHeight.
	TransferGroupCache  []int                 `json:"transferGroupCache"`
	ParkingDemandLog    []int                 `json:"parkingDemandLog"`
	MedicalServiceSlots []*MedicalServiceSlot `json:"medicalServiceSlots"`
	// RngState: signed 32-bit LCG state (TS `| 0`).
	RngState     int32 `json:"rngState"`
	RngCallCount int   `json:"rngCallCount"`
	// StarCount: 1-6 (6 = Tower).
	StarCount int `json:"starCount"`
	// CurrentPopulation: binary g_primary_family_ledger_total.
	CurrentPopulation int `json:"currentPopulation"`
	// CurrentPopulationBuckets: binary per-family ledger buckets. A JS
	// Record<number, number>; never iterated in the TS, so a Go map is safe.
	CurrentPopulationBuckets map[int]int            `json:"currentPopulationBuckets"`
	CommercialVenueBuckets   CommercialVenueBuckets `json:"commercialVenueBuckets"`
	EventState               EventState             `json:"eventState"`
	// Pending notifications / prompts for the current tick (drained by the facade).
	PendingNotifications []SimNotification `json:"pendingNotifications"`
	PendingPrompts       []SimPrompt       `json:"pendingPrompts"`
}

// Notification kinds.
const (
	NotificationRouteFailure = "route_failure"
	NotificationEvent        = "event"
	NotificationStarAdvanced = "star_advanced"
)

// SimNotification TS: world.ts SimNotification. Message "" == undefined.
type SimNotification struct {
	Kind    string `json:"kind"` // "route_failure" | "event" | "star_advanced"
	Message string `json:"message,omitempty"`
}

// Prompt kinds.
const (
	PromptBombRansom              = "bomb_ransom"
	PromptFireRescue              = "fire_rescue"
	PromptCarrierEditConfirmation = "carrier_edit_confirmation"
)

// SimPrompt TS: world.ts SimPrompt. Cost nil == undefined.
type SimPrompt struct {
	PromptID   string `json:"promptId"`
	PromptKind string `json:"promptKind"`
	Message    string `json:"message"`
	Cost       *int   `json:"cost,omitempty"`
}
