# Phase B3 port log

Phase B3 covers world events, progression and the day scheduler. It is part
of the TS -> Go port of `reference/tower-together/sim`, in package `sim`
(`internal/sim`).

## Files

| TS | Go |
|---|---|
| events.ts | events.go |
| progression.ts | progression.go |
| daily/drain-active-requests.ts | daily_drain_active_requests.go |
| tick/day-scheduler.ts | tick_day_scheduler.go |
| scheduler.ts | no file. It only re-exports day-scheduler.ts, and Go code in one package needs no re-export. |
| progression.test.ts | progression_test.go (all cases, plus a bucket test) |
| (new) | events_test.go, tick_day_scheduler_test.go |
| (new) | pending_b3.go (declarations owned by other phases, see below) |

`stubs_b3.go` is deleted. `RunSimulationDayScheduler`, `HandlePromptResponse`
and `TickVipSpecialVisitor` now have their real implementations, with the
signatures the stubs had.

Exported API:
- events.go: `TryTriggerBombEvent`, `TickBombEvent`, `CheckSecurityPatrolBomb`,
  `TryTriggerFireEvent`, `TickFireEvent`, `TickVipSpecialVisitor`,
  `TriggerRandomNewsEvent`, `CheckDailyEvents`, `HandlePromptResponse`.
- progression.go: `ComputeStarCountFromPopulation`,
  `AddToPopulationBucket`, `ClearPopulationBucket`,
  `CheckStarAdvancementConditions`, `ResetStarGateState` and
  `TryAdvanceStarCount`. A2's carrier tick calls `TryAdvanceStarCount` every
  tick.
- daily_drain_active_requests.go: `DispatchActiveRequestsByFamily`.
- tick_day_scheduler.go: the `SimState{Time, World, Ledger}` struct,
  `RunCheckpoints(state, prevTick, currTick)` and
  `RunSimulationDayScheduler(world, ledger, time) *TimeState`.

The file-private TS helpers are unexported. These include
`floorTileBoundsForFloor` (returns `left, right, ok`), `selectRandomLiveFloor`,
`deleteObjectCoveringFloorTile`, `applyTeardownSideEffects`, the bomb search
helpers, the fire spread, helicopter and helper functions, and the
`checkpoint*` bodies.

## Checkpoint table (tick_day_scheduler.go `dayCheckpoints`, TS order)

A checkpoint fires when its tick is in the half-open window
`(prevTick, currTick]`. When `currTick < prevTick` the day wrapped, so it
fires when `tick > prev || tick <= curr`.

| Tick | Body | Calls |
|---|---|---|
| 0x000 | checkpointStartOfDay | progress override latch (dayCounter%8==4, stars<5, flag 0x10), medical flag when stars>2, ActivateEvalSims |
| 0x020 | checkpointRecyclingReset | ResetRecyclingCenterDutyTier |
| 0x0f0 | checkpointFacilityLedgerRebuild | CheckDailyEvents, RebuildFacilityLedger, RebuildCommercialVenueRuntime, SeedEntertainmentBudgets |
| 0x3e8 | checkpointEntertainmentHalf1 | ActivateEntertainmentUpperHalf |
| 0x4b0 | checkpointHotelSaleReset | family345SaleCount=0, DispatchEvalMiddayReturn, PromoteCinemaAndActivatePartyHall; with no carriers, a parked hotel sim moves to the morning gate when its family has exactly one hotel |
| 0x578 | checkpointEntertainmentHalf2 | ActivateEntertainmentLowerHalf |
| 0x5dc | checkpointEntertainmentPhase1 | AdvanceEntertainmentUpperPhase |
| 0x640 | checkpointMidday | clears the override when flag 0x10 is set, then RebuildRestaurantFacilityRecords, SpreadCockroachInfestation, UpdateHotelOperationalAndOccupancy, AdvancePartyHallPhaseAndAccrue, PromoteCinemaLinksToReadyPhase, UpdateRecyclingCenterState(0), AdvanceObjectStayPhaseTiers |
| 0x708 | checkpointNoop | none |
| 0x76c | checkpointEntertainmentPhase2 | AdvanceEntertainmentLowerPairedPhaseAndAccrue |
| 0x7d0 | checkpointLateFacility | CloseCommercialVenuesByFamily for retail, then fast food, then UpdateRecyclingCenterState(2) |
| 0x898 | checkpointType6Advance | CloseCommercialVenuesByFamily(restaurant) |
| 0x8fc | checkpointDayCounter | none (AdvanceOneTick already updated the day counter) |
| 0x9c4 | checkpointRuntimeRefresh | RebuildAllSimTileSpans, ResetSimRuntimeState, NormalizeUnitStatusEndOfDay, DispatchActiveRequestsByFamily |
| 0x9e5 | checkpointLedgerRollover | ResetSimRuntimeState, DoLedgerRollover, RecomputeAllObjectOperationalStatus; when dayCounter%3==0 also ActivateThreeDayCashflow, DoExpenseSweep, RefundUnhappyFacilities and an office/condo ResetSimTripCounters |
| 0x9f6 | checkpointEndOfDay | FlushCarriersEndOfDay |
| 0xa06 | checkpointRecyclingFinal | UpdateRecyclingCenterState(5) |

Each tick, `RunSimulationDayScheduler` runs, in order:
1. `AdvanceOneTick`
2. `TriggerRandomNewsEvent` (1 RNG draw when its gates pass)
3. `TickVipSpecialVisitor` (1 draw when its gates pass)
4. `RunCheckpoints`
5. `TickBombEvent`
6. `TickFireEvent`

It returns the new time pointer.

## PORT-NOTEs

- **Time mutated in place.** In the TS, `bombCleanup` and `resolveFireEvent`
  cast away readonly and move `time.dayTick` forward to 1500 in place. The
  Go versions mutate `*TimeState` the same way. In the scheduler the pointer
  is the fresh `nextTime`, so the fast-forward reaches the returned time,
  exactly as in the TS. `HandlePromptResponse` (ransom paid) mutates
  `TowerSim`'s current time in place, as the TS does.
- **Bomb flag 1 stays set after a paid ransom.** `bombCleanup` clears only
  0x60, so the active-search bit (1) survives. This is TS behavior, kept as
  is; see `TestBombRansomPrompt`.
- **Floor index compared with a y.** `applyTeardownSideEffects` compares
  `sim.floorAnchor` (a floor index) with `objY` (a grid y), as the TS does.
  Kept verbatim.
- **Locale formatting.** `ransom.toLocaleString()` is ported as
  `jsToLocaleString`, which assumes the en-US locale ("300,000").
- **Bounds without infinities.** The TS `floorTileBoundsForFloor` uses ±Infinity
  sentinels. Go uses a first-hit flag and returns `ok=false` where the TS
  returns null. The scan computes a min and max, so iteration order does not
  matter.
- **Empty helper list.** `es.fireRescueHelpers = []` is ported as
  `[]*FireRescueHelper{}` (non-nil), so the JSON keeps `[]`.
- **Live iteration of sims.** `DispatchActiveRequestsByFamily` ranges over
  `world.Sims`. The TS `for..of` is live and would also visit sims appended in
  place during the loop; Go's range would not. Reassigning `world.Sims`
  behaves the same in both. Revisit this if a trace diverges at 0x9c4.
- **Iteration order:**
  - `deleteObjectCoveringFloorTile` uses `PlacedObjects.Entries()` (a
    snapshot in insertion order) and stops at the first match.
  - `spawnFireRescueHelpers` walks the same order, so the helper order
    matches the TS.
  - `TickVipSpecialVisitor` toggles metro objects in `Values()` order.
  - The lookup-only tables are Go maps: `BOMB_RANSOM`, the three
    `FIRE_*_FAMILIES` sets and `hotelFamilyCounts`.
- **Map writes on a nil map.** `AddToPopulationBucket` and
  `ClearPopulationBucket` create `CurrentPopulationBuckets` if it is nil, so
  they never write to a nil map. The arithmetic is unchanged.

## Pending declarations (pending_b3.go)

Every declaration panics with `"TODO port: <ts path> <fn>"`. The integration
pass deletes each one once its owner ports it. All are called only from the
day scheduler or the daily drain.

| Declaration | TS source | Owner |
|---|---|---|
| `DispatchSimBehavior(world, ledger, time, sim)` | families/dispatch-sim-behavior.ts | B1 |
| `RebuildAllSimTileSpans(world)` | families/tile-spans.ts | B1 |
| `RebuildCommercialVenueRuntime(world, time)` | sims/facility-refunds.ts | B1 |
| `RebuildRestaurantFacilityRecords(world, time)` | sims/facility-refunds.ts | B1 |
| `CloseCommercialVenuesByFamily(world, ledger, familyCode)` | sims/facility-refunds.ts | B1 |
| `RefundUnhappyFacilities(world, ledger, time)` | sims/facility-refunds.ts | B1 |
| `SpreadCockroachInfestation(world, time)` | sims/hotel-facilities.ts | B1 |
| `UpdateHotelOperationalAndOccupancy(world, time)` | sims/hotel-facilities.ts | B1 |
| `AdvanceObjectStayPhaseTiers(world)` | sims/hotel-facilities.ts | B1 |
| `NormalizeUnitStatusEndOfDay(world)` | sims/hotel-facilities.ts | B1 |
| `ActivateEvalSims(world)`, `DispatchEvalMiddayReturn(world)` | cathedral.ts | B2 |
| `ResetRecyclingCenterDutyTier(world)`, `UpdateRecyclingCenterState(world, ledger, param)` | recycling.ts | B2 |
| `SeedEntertainmentBudgets`, `ActivateEntertainmentUpperHalf`, `ActivateEntertainmentLowerHalf`, `PromoteCinemaAndActivatePartyHall`, `AdvanceEntertainmentUpperPhase`, `PromoteCinemaLinksToReadyPhase` (all `(world)`) | entertainment.ts | B2 |
| `AdvancePartyHallPhaseAndAccrue(world, ledger, dayCounter)`, `AdvanceEntertainmentLowerPairedPhaseAndAccrue(world, ledger, dayCounter)` | entertainment.ts | B2 |

`FlushCarriersEndOfDay` used to be in this list. I removed it once A2 ported
it in `carriers.go`.

Events also call functions other phases own. When B3 finished, these
already existed (stub or real): `ReleaseServiceRequest` (B1,
`sims_index.go`), `RebuildParkingCoverage` and `RebuildParkingDemandLog` (B2
stubs), `ApplyRemoveElevatorCar` (A1) and `RemoveCashflowFromFamilyResource`
(A1).

**Duplicate to remove in integration.** `pending_b2.go` declares panicking
`AddToPopulationBucket` and `ClearPopulationBucket` (progression.ts, owned
by B3). The real versions are in `progression.go` with the same signatures,
so the two `pending_b2.go` entries must go. This is the only redeclaration
that involves B3 files.

## Test status

The full package does not compile while the other phases are mid-port.
B3's files compile, and their only remaining errors are the two
`pending_b2.go` duplicates above. To test B3, I copied the A1 files, the
stubs, the B3 files and a one-line `ReleaseServiceRequest` shim into a
scratch directory. In that copy, `go build`, `go vet` and the B3 tests are
clean:

- **progression_test.go:** every progression.test.ts case passes, plus a
  bucket add/clear test.
- **events_test.go:** passes. It covers:
  - `jsToLocaleString`;
  - floor bounds and the live-floor scan, including the RNG draw count;
  - the bomb trigger (floor, tile, RNG, prompt text and cost) and its
    guards;
  - detonation at 1200, the blast, and cleanup at 1250 with the fast-forward
    to 1500;
  - paying and refusing the ransom;
  - the fire trigger, the helicopter prompt at start+2, accepting the
    helicopter, and resolution at 2000;
  - declining the fire prompt, which spawns helpers from a security office;
  - the cathedral guard;
  - the VIP metro toggle and its notification;
  - the random-news RNG gate;
  - clearing a carrier_remove prompt.
- **tick_day_scheduler_test.go:**
  - The checkpoint window and wrap logic, plus the exact tick list, pass.
  - A between-checkpoint scheduler step passes: it returns a fresh pointer
    and makes one news draw.
  - A full-day run skips with `skipOnTODO`. It needs the pending B1/B2
    functions.
- **Existing tests:** `TestScoringRunsAtTick2533Checkpoint` now gets
  through the 0x9e5 checkpoint up to
  `sims/facility-refunds.ts refundUnhappyFacilities` (B1) and skips there.
  The `commands_test.go` VIP case is still blocked on A2 hydration.
