# Phase A2 port log

Phase A2 (transport) of the TS -> Go port of `reference/tower-together/sim`:
reachability, route scoring, carriers, the route queue and the carrier
tick. Package `sim` in `internal/sim`. This log covers the file map, every
place the port is not a literal transliteration, the pending declarations,
and the iteration-order decisions.

## Files

| TS | Go |
|---|---|
| reachability/mask-tests.ts | reachability_mask_tests.go |
| reachability/rebuild-tables.ts | reachability_rebuild_tables.go |
| reachability/span-checks.ts | reachability_span_checks.go |
| reachability/special-link-records.ts | reachability_special_link_records.go |
| route-scoring/constants.ts | route_scoring_constants.go |
| route-scoring/delay-table.ts | route_scoring_delay_table.go |
| route-scoring/route-mode.ts | route_scoring_route_mode.go |
| route-scoring/score-carrier.ts | route_scoring_score_carrier.go |
| route-scoring/score-local.ts | route_scoring_score_local.go |
| route-scoring/score-special-link.ts | route_scoring_score_special_link.go |
| route-scoring/select-candidate.ts | route_scoring_select_candidate.go |
| route-scoring/select-cathedral.ts | route_scoring_select_cathedral.go |
| route-scoring/select-housekeeping.ts | route_scoring_select_housekeeping.go |
| carriers.ts | carriers.go |
| carriers/advance.ts | carriers_advance.go |
| carriers/arrival.ts | carriers_arrival.go |
| carriers/assign.ts | carriers_assign.go |
| carriers/depart.ts | carriers_depart.go |
| carriers/index.ts | carriers_index.go (comment only: a re-export barrel) |
| carriers/motion.ts | carriers_motion.go |
| carriers/pending.ts | carriers_pending.go |
| carriers/position.ts | carriers_position.go |
| carriers/slot.ts | carriers_slot.go |
| carriers/sync.ts | carriers_sync.go |
| carriers/target.ts | carriers_target.go |
| queue/cancel.ts | queue_cancel.go |
| queue/dequeue.ts | queue_dequeue.go |
| queue/dispatch-arrivals.ts | queue_dispatch_arrivals.go |
| queue/encoding.ts | queue_encoding.go |
| queue/enqueue.ts | queue_enqueue.go |
| queue/index.ts | queue_index.go (comment only: a re-export barrel) |
| queue/process-travel.ts | queue_process_travel.go |
| queue/resolve.ts | queue_resolve.go |
| queue/scan.ts | queue_scan.go |
| tick/carrier-tick.ts | tick_carrier_tick.go |
| tick/service-idle-tasks.ts | tick_service_idle_tasks.go (now holds `ServiceIdleTasksContext`) |
| (new) | jsutil_a2.go (transport JS-idiom helpers) |
| carriers.test.ts (+ Go-only transport tests) | carriers_a2_test.go |

`stubs_a2.go` is deleted: every stub in it now has a real body.

## Public API other phases call

- `ResolveSimRouteBetweenFloors(world, sim, src, dst, dir int, time *TimeState, options *ResolveSimRouteOptions) int`.
  - `time` may be nil (TS `undefined`). A nil `options` means the TS default
    `= {}`.
  - `ResolveSimRouteOptions{TargetHeightMetric *int, IsPassengerRoute *bool, EmitDistanceFeedback *bool}`:
    a nil field takes the TS default (`sim.HomeColumn`, `true`, `true`).
    Build the pointers with `intPtr` and `trBoolPtr`.
  - The return codes are named `RouteResolutionNoRoute` (-1),
    `RouteResolutionQueueFull` (0), `RouteResolutionSegment` (1),
    `RouteResolutionQueued` (2) and `RouteResolutionSameFloor` (3). The
    values are the TS literals, so callers that compare with plain ints
    still work.
- `CancelRuntimeRouteRequest(carrier, simID)` and its alias `EvictCarrierRoute`.
- `EnqueueRequestIntoRouteQueue(carrier, simID, src, dst, dir, lobbyMode) bool` and its alias `EnqueueCarrierRoute`.
- `DecrementRouteQueueDirectionLoad(carrier, srcFloor, dir)` (empty upstream).
- `PopUnitQueueRequest(queue, dir) (string, bool)`.
- `FloorToSlot`, `IsExpressStopFloor`, `CarrierServesFloor`, `CarrierSpansFloor`.
- `MakeCarrier(id, col, mode, bottom, top, numCars)` and `MakeCarrierCar(numSlots, homeFloor)`.
  - The TS default for `numCars` is 1. Go has no default arguments, so
    pass 1 explicitly.
- `RebuildCarrierList`, `InitCarrierState`, `FlushCarriersEndOfDay` and `SyncAssignmentStatus`.
- `RebuildRouteReachabilityTables`, `RebuildTransferGroupCache` and `RebuildSpecialLinkRouteRecords`.
- `CarrierTick(world, ledger, time)`.
- `ServiceIdleTasks(ctx)` with `ServiceIdleTasksContext{World, Ledger, Time}`.

Changed shapes:

| TS | Go |
|---|---|
| `findBestAvailableCarForFloor` returns `{carIndex, fullAssign}` | `FindBestAvailableCarForFloor` returns `(carIndex int, fullAssign bool)` |
| `popActiveRouteSlotRequest` returns `string \| undefined` | `PopActiveRouteSlotRequest` returns `(string, bool)` |
| `decodeEncodedRouteTargetByte` / `decodeRuntimeRouteTarget` return an object or `null` | they return `*RuntimeRouteTarget`, nil for null |
| `selectBestRouteCandidate` and the cathedral/housekeeping wrappers return `RouteCandidate \| null` | they return `*RouteCandidate`, nil for null |
| `scoreSpecialLinkRoute` returns `{cost, direction}` | `ScoreSpecialLinkRoute` returns `SpecialLinkRouteResult{Cost, Direction}` |

## Pending declarations

None remain. I wrote `pending_a2.go` for the four functions this phase
calls from other phases, then deleted it, because all four already exist
with matching signatures:

| Function | Owner | Defined in |
|---|---|---|
| `DispatchSimArrival` | B1 | sims_index.go |
| `RefreshRuntimeEntitiesForTickStride` | B1 | sim_refresh_refresh_stride.go |
| `ReconcileSimTransport` | B1 | sims_index.go |
| `TryAdvanceStarCount` | B3 | progression.go |

`RunSimulationDayScheduler` (B3) also exists, in tick_day_scheduler.go.

## Helpers (jsutil_a2.go)

The helpers carry a `tr` prefix so they cannot collide with helpers that
other phases add.

- `trAt(s, i, def)` is the JS `arr[i] ?? def`. It returns `def` when `i` is
  out of range, which is how JS reads `undefined` past either end.
- `trSet(&s, i, v)` is the JS `arr[i] = v`. Writing past the end grows the
  slice with zeros (JS leaves holes, which are only ever read through
  `?? 0`).
- `trBit(n)` is the JS `1 << n`:
  - The shift count is taken mod 32.
  - The result is a sign-extended int32, so `trBit(31)` is negative.
  - Every carrier/record mask is built from `trBit` values combined with
    `|`, `&` and `&^`. The results stay sign-extended int32 values, so
    they equal the JS results bit for bit. This includes the bit-31 record
    7 and the JSON numbers in `transferGroupCache`, `carrierMask` and
    `reachabilityMasksByFloor`.
- `trFill` is the JS `arr.fill(v)`.
- `trBoolPtr` returns a pointer to a bool, for the optional TS boolean
  options.

Two other helpers are shared across files:

- `trFindCarrier(world, id)` is `world.carriers.find(c => c.carrierId === id)`.
  It was renamed from `findCarrierByID` because B1 has a function with that
  name.
- `findSimByKey(world, id)` is `world.sims.find(s => simKey(s) === id)`.

## PORT-NOTEs (non-literal spots)

- **Debug logging dropped.** The `__DRAIN_TRACE__` console logging in
  `enqueueRequestIntoRouteQueue` and `drainFloorQueueForCar` is not ported.
- **Duplicated TS privates merged.** The TS re-declares some file-private
  helpers in several files. Go has one copy of each:
  - `getCarCapacity` becomes `carCapacity`.
  - `getScheduleIndex` becomes `carrierScheduleIndex`.
  - `DEPARTURE_SEQUENCE_TICKS` becomes `departureSequenceTicks`.
  - `ACTIVE_SLOT_CAPACITY` becomes `ActiveSlotCapacity`.
  - `getQueueState` becomes `queueState` and `getDirectionQueue` becomes
    `directionQueue`.
  - The two identical walkability loops (in `rebuildRouteReachabilityTables`
    and inside `rebuildSpecialLinkRouteRecords`) share
    `markSpecialLinkWalkability`.
- **Infinity.** `Number.POSITIVE_INFINITY` in
  `findBestAvailableCarForFloor` is `math.MaxInt`. It is only used with
  `<`, and the subtraction runs only when both candidates are finite.
- **Optional car index.** In `assignRequestToRuntimeRoute`, the optional
  `carIndex?: number` is an int and a negative value means undefined. The
  only TS caller always passes a real index.
- **Legacy defaults skipped.** In `rebuildCarrierList`, the numeric `??=`
  defaults (`waitingCarResponseThreshold`, `assignmentCapacity`) and the
  `nearestWorkFloor === undefined` reset are legacy-snapshot repairs. A
  typed struct cannot express them, so they are skipped, the same as A1's
  snapshot policy. The nil-slice `??=` repairs are kept.
- **Zero-car carrier.** In `assignCarToFloorRequest`, the degenerate
  car-0 fallback on a zero-car carrier throws in the TS
  (`undefined.pendingAssignmentCount`). The Go port panics on the index
  instead. Carriers always have at least one car, so this path is
  unreachable in practice.
- **Live iteration.** In `carrierTick`, the TS iterates `world.carriers`
  and `carrier.cars` with live array iterators. Nothing in the tick
  reassigns or appends to them, so ranging over the slice is equivalent.
- **Fresh slices from `filter`.** `pendingRoutes.filter(...)` becomes
  `filterOutPendingRoute`, which always returns a fresh slice, like JS
  `filter`. Route records stay shared pointers, so a `route` held across a
  filter is still the same object.
- **JSON-visible slices are never nil.** Slices the TS writes as `[]`
  (`pendingRouteIds`, `completedRouteIds`, `pendingRoutes`) are written as
  empty non-nil slices, so they marshal as `[]`, not `null`.
- **Kept as upstream.** These TS no-ops and stand-ins are ported as is:
  - `decrementCarPendingAssignmentCount` and
    `decrementRouteQueueDirectionLoad` are intentionally empty.
  - `dispatchQueuedRouteUntilRequest` only removes from the ring.
  - `getCurrentSimRouteMode` is unused.

## Iteration-order decisions

- **`rebuildCarrierList`:**
  - `overlayKeys` is a JS `Set` of `Object.keys(overlays)` followed by
    `Object.keys(overlayToAnchor)`. It is ported as an ordered slice plus a
    seen-map, so the first occurrence wins.
  - `columns` is a JS `Map` keyed in first-seen column order. It is ported
    as a column-order slice plus a map, with per-column floors as a slice
    plus a set.
  - Each column's floors are stable-sorted ascending.
  - As a result, runs are emitted, and carrierIds are assigned, in
    first-seen column order, exactly as in the TS.
  - Matching against existing carriers scans `world.Carriers` in order. A
    strict `>` keeps the first carrier on overlap ties, and the `consumed`
    set is lookup-only.
- **`rebuildTransferGroupCache`:** parking tiles come from the
  `PlacedObjects` OrderedMap snapshot. They are then sorted with
  `sort.SliceStable` by (floor, left), so ties keep insertion order like
  the stable `Array.prototype.sort`.
- **`rebuildSpecialLinkRouteRecords`:** overlays come from the `Overlays`
  OrderedMap snapshot. They are then sorted with `sort.SliceStable` by
  (column, entryFloor).
- **Lookup-only sets are Go maps:**
  - `STATE_BIT_FAMILIES`, which appears in both `process-travel.ts` and
    `resolve.ts`;
  - `SHARED_ROUTE_SELECTOR_FAMILIES`;
  - the `consumed` set.
- **Everything else iterates slices in index order:**
  - the transfer-group entries, special-link segments and records;
  - each car's `activeRouteSlots`, `pendingRoutes` and `floorQueues`;
  - `carriers` and `sims`.
- **The 40-slot `RouteRequestRing` (A1) is used unchanged:**
  - `FlushCarriersEndOfDay` zeroes only `count` and keeps `head`
    (carriers.test.ts).
  - `RemoveFirst` is the compaction used by cancel.
  - Enqueue refuses a full ring (`IsFull`), as the TS does upstream of the
    silent wrap.

## Test and build status

- **Isolated build is clean.** I built the A1 files, the A2 files and a
  scratch-only stub file for every other-phase symbol in a scratch copy.
  `go build` and `go vet` pass there.
- **`carriers_a2_test.go` passes:**
  - the ported carriers.test.ts flush case;
  - Go-only tests for the constructors;
  - `RebuildCarrierList` run splitting, ordering and record preservation;
  - enqueue -> assign -> travel -> board -> deliver;
  - cancel and the express non-stop enqueue;
  - the `trBit` int32 semantics;
  - stairs and elevator route selection, and the transfer floor;
  - the resolver codes and side effects;
  - route-target encoding.
- **A1 tests pass, and hydration now gets further.** The ported
  `commands.test.ts` cases got past every A2 function and stopped at B2's
  parking stub in the isolated build, which is expected.
- **The full package in place does not build yet.** `go build
  ./internal/sim/` fails only on duplicate declarations in `pending_b3.go`
  (B3's file). The functions it declares have since been defined by B1 and
  B2. None of the errors involve A2 code.
- **The integrated build is clean.** I made a scratch copy of the whole
  package and dropped every `pending_*.go` declaration that is defined
  elsewhere, which is what the integration pass will do. `go build` and
  `go vet` are clean there, and `TestTrace` passes every fixture in full:
  - `build_lobby_only` compared 9998 entries.
  - `build_condo` compared 9998 entries.
  - All 16 `TestTrace` fixtures (commercial, condo, condo_office_elevator,
    condo_stuck, dense_hotel, dense_office, elevator, hotel, lobby_only,
    mixed, mixed_elevator, mixed_elevator_delayed, mixed_multicar, metro,
    offices, sky_office) compare their full trace.
  - `TestTraceFire`, `TestTraceFireAccept` and
    `TestTraceCathedralRecoveredPath` also pass.
- **One other-phase unit test fails.** The full suite's only failure is
  B2's `TestGateEntertainmentGuestState` ("budget consumed or refunded").
