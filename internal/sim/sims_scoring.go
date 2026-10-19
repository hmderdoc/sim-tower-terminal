package sim

// Port of reference/tower-together/sim/sims/scoring.ts: operational
// scoring, stress readouts and the SimStateRecord view.

var entertainmentNoiseFamilies = map[int]bool{
	FamilyCinema:            true,
	FamilyCinemaLower:       true,
	FamilyCinemaStairsUpper: true,
	FamilyCinemaStairsLower: true,
	FamilyPartyHall:         true,
	FamilyPartyHallLower:    true,
}

// operationalScoreDivisor: binary compute_object_operational_score (1138:040f)
// divisor per family.
var operationalScoreDivisor = map[int]int{
	FamilyHotelSingle: 1,
	FamilyHotelTwin:   2,
	FamilyHotelSuite:  2,
	FamilyOffice:      6,
	FamilyCondo:       3,
}

// Stress level strings.
const (
	StressLow    = "low"
	StressMedium = "medium"
	StressHigh   = "high"
)

// SimStateRecord is the display/test view of one sim.
// TS: sims/scoring.ts SimStateRecord
type SimStateRecord struct {
	ID                     string `json:"id"`
	FloorAnchor            int    `json:"floorAnchor"`
	SelectedFloor          int    `json:"selectedFloor"`
	DestinationFloor       int    `json:"destinationFloor"`
	HomeColumn             int    `json:"homeColumn"`
	BaseOffset             int    `json:"baseOffset"`
	FamilyCode             int    `json:"familyCode"`
	StateCode              int    `json:"stateCode"`
	RouteMode              int    `json:"routeMode"`
	CarrierID              *int   `json:"carrierId"` // null when not on a carrier
	AssignedCarIndex       int    `json:"assignedCarIndex"`
	BoardedOnCarrier       bool   `json:"boardedOnCarrier"`
	CurrentTripStressTicks int    `json:"currentTripStressTicks"`
	CurrentTripStressLevel string `json:"currentTripStressLevel"`
	AverageTripStressTicks int    `json:"averageTripStressTicks"`
	StressLevel            string `json:"stressLevel"`
	TripCount              int    `json:"tripCount"`
	AccumulatedTicks       int    `json:"accumulatedTicks"`
	ElapsedTicks           int    `json:"elapsedTicks"`
}

// TS: sims/scoring.ts isNoiseSource
func isNoiseSource(originFamilyCode, targetFamilyCode int) bool {
	if entertainmentNoiseFamilies[targetFamilyCode] {
		return EvaluatableFamilies[originFamilyCode]
	}
	if HotelFamilies[originFamilyCode] {
		return targetFamilyCode == FamilyRestaurant || targetFamilyCode == FamilyOffice ||
			targetFamilyCode == FamilyFastFood || targetFamilyCode == FamilyRetail
	}
	if originFamilyCode == FamilyOffice {
		return targetFamilyCode == FamilyRestaurant || targetFamilyCode == FamilyFastFood ||
			targetFamilyCode == FamilyRetail
	}
	if originFamilyCode == FamilyCondo {
		return targetFamilyCode == FamilyHotelSingle || targetFamilyCode == FamilyHotelTwin ||
			targetFamilyCode == FamilyHotelSuite || targetFamilyCode == FamilyRestaurant ||
			targetFamilyCode == FamilyOffice || targetFamilyCode == FamilyFastFood ||
			targetFamilyCode == FamilyRetail
	}
	return targetFamilyCode == FamilyRestaurant || targetFamilyCode == FamilyFastFood ||
		targetFamilyCode == FamilyRetail
}

// TS: sims/scoring.ts hasNearbyNoise
func hasNearbyNoise(world *WorldState, object *PlacedObjectRecord, floorAnchor, radius int) bool {
	for _, e := range world.PlacedObjects.Entries() {
		candidate := e.Val
		if candidate == object {
			continue
		}
		_, y := parseKeyXY(e.Key)
		if YToFloor(y) != floorAnchor {
			continue
		}
		if !isNoiseSource(object.ObjectTypeCode, candidate.ObjectTypeCode) {
			continue
		}
		leftDelta := absInt(candidate.LeftTileIndex - object.RightTileIndex)
		rightDelta := absInt(object.LeftTileIndex - candidate.RightTileIndex)
		if min(leftDelta, rightDelta) <= radius {
			return true
		}
	}
	return false
}

// opThresholds returns OP_SCORE_THRESHOLDS[min(star,5)] ?? [80, 200].
func opThresholds(world *WorldState) (int, int) {
	if t, ok := OpScoreThresholds[min(world.StarCount, 5)]; ok {
		return t[0], t[1]
	}
	return 80, 200
}

// RecomputeObjectOperationalStatus TS: sims/scoring.ts recomputeObjectOperationalStatus
func RecomputeObjectOperationalStatus(world *WorldState, sim *SimRecord, object *PlacedObjectRecord) {
	if !EvaluatableFamilies[object.ObjectTypeCode] {
		return
	}

	siblings := FindSiblingSims(world, sim)
	populationCount, ok := operationalScoreDivisor[object.ObjectTypeCode]
	if !ok {
		populationCount, ok = EntityPopulationByType[object.ObjectTypeCode]
		if !ok {
			populationCount = 1
		}
	}
	stressSum := 0
	hasTripData := false
	for _, sibling := range siblings {
		if sibling.BaseOffset == 0 && HotelFamilies[object.ObjectTypeCode] {
			// Binary skips bo=0 in the hotel loop (occupant index starts at 1).
			continue
		}
		if sibling.TripCount > 0 {
			stressSum += sibling.AccumulatedTicks / sibling.TripCount // non-negative: trunc
			hasTripData = true
		}
	}
	rawAverageStress := -1
	if hasTripData {
		rawAverageStress = stressSum / populationCount
	}
	object.EvalScore = rawAverageStress

	if HotelFamilies[object.ObjectTypeCode] && object.UnitStatus > UnitStatusHotelSoldOut {
		object.EvalLevel = 0xff
		return
	}
	if object.ObjectTypeCode == FamilyOffice && object.UnitStatus > UnitStatusOfficeOccupied && object.OccupiedFlag != 0 {
		object.EvalLevel = 0xff
		return
	}
	if object.ObjectTypeCode == FamilyCondo && object.UnitStatus > UnitStatusCondoOccupied && object.OccupiedFlag != 0 {
		object.EvalLevel = 0xff
		return
	}

	if rawAverageStress < 0 {
		object.EvalLevel = 0xff
		return
	}

	score := 0
	if hasTripData {
		score = rawAverageStress
	}
	switch object.RentLevel {
	case 0:
		score += 30
	case 2:
		score = max(0, score-30)
	case 3:
		score = 0
	}

	noiseRadius := 20
	if object.ObjectTypeCode == FamilyOffice {
		noiseRadius = 10
	} else if object.ObjectTypeCode == FamilyCondo {
		noiseRadius = 30
	}
	if hasNearbyNoise(world, object, sim.FloorAnchor, noiseRadius) {
		score += 60
	}

	lower, upper := opThresholds(world)
	if score < lower {
		object.EvalLevel = 2
	} else if score < upper {
		object.EvalLevel = 1
	} else {
		object.EvalLevel = 0
	}
	// Binary tail (1138:093c / 1138:09d6): if occupiedFlag == 0 && evalLevel != 0
	// set occupiedFlag = 1; hotels only while unitStatus <= 0x27.
	if object.OccupiedFlag == 0 && object.EvalLevel > 0 {
		if !HotelFamilies[object.ObjectTypeCode] || object.UnitStatus <= 0x27 {
			object.OccupiedFlag = 1
		}
	}
}

// RecomputeAllObjectOperationalStatus TS: sims/scoring.ts recomputeAllObjectOperationalStatus
func RecomputeAllObjectOperationalStatus(world *WorldState) {
	for _, e := range world.PlacedObjects.Entries() {
		object := e.Val
		if !EvaluatableFamilies[object.ObjectTypeCode] {
			continue
		}
		x, y := parseKeyXY(e.Key)
		floorAnchor := YToFloor(y)
		var sim *SimRecord
		for _, c := range world.Sims {
			if c.FamilyCode == object.ObjectTypeCode && c.FloorAnchor == floorAnchor && c.HomeColumn == x {
				sim = c
				break
			}
		}
		if sim == nil {
			continue
		}
		RecomputeObjectOperationalStatus(world, sim, object)
	}
}

// ActiveStressThresholds TS: sims/scoring.ts activeStressThresholds
func ActiveStressThresholds(world *WorldState) (medium, high int) {
	return opThresholds(world)
}

// AverageTripStressTicks TS: sims/scoring.ts averageTripStressTicks
func AverageTripStressTicks(sim *SimRecord) int {
	if sim.TripCount > 0 {
		return sim.AccumulatedTicks / sim.TripCount
	}
	return 0
}

// CurrentTripStressTicks TS: sims/scoring.ts currentTripStressTicks
func CurrentTripStressTicks(world *WorldState, time *TimeState, sim *SimRecord) int {
	liveElapsed := 0
	if sim.LastDemandTick >= 0 {
		liveElapsed = (time.DayTick - sim.LastDemandTick + DayTickMax) % DayTickMax
		reduced := liveElapsed
		ReduceElapsedForLobbyBoarding(&reduced, sim.OriginFloor, world)
		liveElapsed = reduced
	}
	return min(300, sim.ElapsedTicks+liveElapsed)
}

// CurrentTripStressLevel TS: sims/scoring.ts currentTripStressLevel
func CurrentTripStressLevel(world *WorldState, time *TimeState, sim *SimRecord) string {
	elapsed := CurrentTripStressTicks(world, time, sim)
	medium, high := ActiveStressThresholds(world)
	if elapsed >= high {
		return StressHigh
	}
	if elapsed >= medium {
		return StressMedium
	}
	return StressLow
}

// RefreshOccupiedFlagAndTripCounters: binary
// refresh_occupied_flag_and_trip_counters @ 1138:0f79 (branches A/B/C).
// TS: sims/scoring.ts refreshOccupiedFlagAndTripCounters
func RefreshOccupiedFlagAndTripCounters(world *WorldState, sim *SimRecord, object *PlacedObjectRecord) {
	if object.EvalLevel != 0xff && object.EvalLevel >= 1 {
		object.OccupiedFlag = 1
		ResetFacilitySimTripCounters(world, sim)
		return
	}
	if object.EvalLevel == 0 {
		y := GridHeight - 1 - sim.FloorAnchor
		for _, e := range world.PlacedObjects.Entries() {
			candidate := e.Val
			if candidate == object {
				continue
			}
			if candidate.ObjectTypeCode != object.ObjectTypeCode {
				continue
			}
			_, cy := parseKeyXY(e.Key)
			if cy != y {
				continue
			}
			if candidate.EvalLevel != 2 {
				continue
			}
			candidate.EvalLevel = 1
			candidate.OccupiedFlag = 1
			object.EvalLevel = 1
			object.OccupiedFlag = 1
			ResetFacilitySimTripCounters(world, sim)
			return
		}
		object.OccupiedFlag = 0
	}
}

// TS: sims/scoring.ts simStressLevel
func simStressLevel(world *WorldState, time *TimeState, sim *SimRecord, _ *PlacedObjectRecord) string {
	return CurrentTripStressLevel(world, time, sim)
}

// TS: sims/scoring.ts distanceFeedbackPenalty
func distanceFeedbackPenalty(routeHeightMetric, targetHeightMetric int) int {
	delta := absInt(routeHeightMetric - targetHeightMetric)
	if delta >= 125 {
		return 60
	}
	if delta > 79 {
		return 30
	}
	return 0
}

// MaybeApplyDistanceFeedback TS: sims/scoring.ts maybeApplyDistanceFeedback
func MaybeApplyDistanceFeedback(_ *WorldState, sim *SimRecord, routeHeightMetric, targetHeightMetric int, canApplyForRouteKind bool) {
	if !canApplyForRouteKind {
		return
	}
	penalty := distanceFeedbackPenalty(routeHeightMetric, targetHeightMetric)
	if penalty == 0 {
		return
	}
	AddDelayToCurrentSim(sim, penalty)
}

// PendingRouteRef is one value of the TS `pendingBySimId` map.
type PendingRouteRef struct {
	Carrier *CarrierRecord
	Route   *CarrierPendingRoute
}

// BuildPendingBySimID pre-indexes pending routes by simId; later carriers /
// routes overwrite earlier ones, like the TS Map.set loop.
func BuildPendingBySimID(world *WorldState) map[string]PendingRouteRef {
	out := map[string]PendingRouteRef{}
	for _, carrier := range world.Carriers {
		for _, route := range carrier.PendingRoutes {
			out[route.SimID] = PendingRouteRef{Carrier: carrier, Route: route}
		}
	}
	return out
}

// CreateSimStateRecord returns nil when the sim has no owning object.
// TS: sims/scoring.ts createSimStateRecord
func CreateSimStateRecord(world *WorldState, time *TimeState, sim *SimRecord, pendingBySimID map[string]PendingRouteRef) *SimStateRecord {
	object := FindObjectForSim(world, sim)
	if object == nil {
		return nil
	}
	id := SimKey(sim)
	pending, hasPending := pendingBySimID[id]
	var pendingRoute *CarrierPendingRoute
	if hasPending {
		pendingRoute = pending.Route
	}
	var carrierID *int
	if pendingRoute != nil || sim.Route.Mode == RouteModeCarrier {
		if hasPending {
			v := pending.Carrier.CarrierID
			carrierID = &v
		} else if sim.Route.Mode == RouteModeCarrier {
			v := sim.Route.CarrierID
			carrierID = &v
		}
	}
	routeModeNum := 0
	if sim.Route.Mode == RouteModeCarrier {
		routeModeNum = 2
	} else if sim.Route.Mode == RouteModeSegment {
		routeModeNum = 1
	}
	currentTicks := CurrentTripStressTicks(world, time, sim)
	currentLevel := simStressLevel(world, time, sim, object)
	assignedCarIndex := -1
	boarded := false
	if pendingRoute != nil {
		assignedCarIndex = pendingRoute.AssignedCarIndex
		boarded = pendingRoute.Boarded
	}
	return &SimStateRecord{
		ID:                     id,
		FloorAnchor:            sim.FloorAnchor,
		SelectedFloor:          sim.SelectedFloor,
		HomeColumn:             sim.HomeColumn,
		BaseOffset:             sim.BaseOffset,
		FamilyCode:             sim.FamilyCode,
		StateCode:              sim.StateCode,
		RouteMode:              routeModeNum,
		DestinationFloor:       sim.DestinationFloor,
		CarrierID:              carrierID,
		AssignedCarIndex:       assignedCarIndex,
		BoardedOnCarrier:       boarded,
		CurrentTripStressTicks: currentTicks,
		CurrentTripStressLevel: currentLevel,
		AverageTripStressTicks: AverageTripStressTicks(sim),
		StressLevel:            currentLevel,
		TripCount:              sim.TripCount,
		AccumulatedTicks:       sim.AccumulatedTicks,
		ElapsedTicks:           sim.ElapsedTicks,
	}
}

// CreateSimStateRecords TS: sims/scoring.ts createSimStateRecords
func CreateSimStateRecords(world *WorldState, time *TimeState) []*SimStateRecord {
	pendingBySimID := BuildPendingBySimID(world)
	result := make([]*SimStateRecord, 0, len(world.Sims))
	for _, sim := range world.Sims {
		if record := CreateSimStateRecord(world, time, sim, pendingBySimID); record != nil {
			result = append(result, record)
		}
	}
	return result
}
