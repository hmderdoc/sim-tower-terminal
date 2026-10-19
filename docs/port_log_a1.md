# Phase A1 port log

Phase A1 (foundation) of the TS -> Go port of `reference/tower-together/sim`.
Package `sim` in `internal/sim`. This log records every place the port is not
a literal transliteration, every stub, and every iteration-order decision.

## Files

| TS | Go |
|---|---|
| world.ts | world.go |
| resources.ts | resources.go (verbatim) + catalog.go (gameplay costs, mechanics.md section 2) |
| time.ts | time.go |
| ledger.ts | ledger.go |
| ring-buffer.ts | ring_buffer.go |
| queue/route-record.ts | queue_route_record.go |
| sims/states.ts | sims_states.go |
| sims/scoring.ts | sims_scoring.go |
| sims/trip-counters.ts | sims_trip_counters.go |
| sims/population.ts | sims_population.go |
| sim-access/selectors.ts | sim_access_selectors.go |
| sim-access/state-bits.ts | sim_access_state_bits.go |
| stress/*.ts (5 files) | stress_accumulate_elapsed.go, stress_add_delay.go, stress_lobby_reduction.go, stress_rebase_elapsed.go, stress_trip_counters.go |
| snapshot.ts | snapshot.go |
| commands.ts | commands.go |
| index.ts | index.go |
| (new) | ordered_map.go, jsutil.go, stubs_phase_a1.go |
| trace.test.ts | trace_test.go |
| commands.test.ts | commands_test.go |
| sims/scoring.test.ts | sims_scoring_test.go |
| queue/route-record.test.ts | queue_route_record_test.go |
| (new) | ordered_map_test.go, world_test.go, ledger_test.go, sims_population_test.go, snapshot_test.go, catalog_test.go, helpers_test.go |

`sim.test.ts`, which AGENTS.md mentions, does not exist in the reference
tree, so there was nothing to port from it.

## Conventions later phases must follow

- **State passing.** Pass `*WorldState`, `*LedgerState` and `*TimeState`
  exactly where the TS passes `world`, `ledger` and `time`. The TS never
  mutates a `TimeState` in place: `advanceOneTick` returns a new one. So
  `AdvanceOneTick(t *TimeState) (*TimeState, bool)` returns a fresh pointer,
  and `RunSimulationDayScheduler` returns `*TimeState`.
- **The tick `ctx`.** It is `ServiceIdleTasksContext{World, Ledger, Time}`,
  declared in `stubs_phase_a1.go`. Move it to `tick_service_idle_tasks.go`
  when you port that file. `ServiceIdleTasks(ctx)` must assign
  `ctx.Time = RunSimulationDayScheduler(...)`. `TowerSim.Step` reads
  `ctx.Time` back.
- **OrderedMap.** `OrderedMap[K, V]` (`ordered_map.go`) is used for every JS
  object with `"x,y"` keys: `WorldState.Cells`, `CellToAnchor`, `Overlays`,
  `OverlayToAnchor`, `PlacedObjects`.
  - The API is `Get`, `At` (zero value when absent), `Has`, `Set`, `Delete`,
    `Len`, `Keys`, `Values`, `Entries`, `Range`, `Clone`, plus JSON in
    insertion order.
  - `Set` on an existing key keeps its position. `Delete` followed by `Set`
    appends the key at the end, which is the JS behavior.
  - `Keys`, `Values`, `Entries` and `Range` iterate a **snapshot**, like
    `Object.keys/values/entries`. A JS `Map` iterated with `for..of` is
    live; if you port such a loop and the loop body mutates the map, reason
    about it separately.
  - Use it for any TS `Map`/`Set` whose iteration order matters. A Go map is
    only safe for lookup-only use.
- **Truthiness.** The JS check `world.cells[key]` is ported as
  `hasStr(&world.Cells, key)` (the value is non-empty). Use `world.Cells.At(key)`
  to read a value.
- **Keys.** Use `cellKey(x, y)` to build an `"x,y"` key and `parseKeyXY(key)`
  to split one; both live in `jsutil.go`. `SimKey(sim)` builds a sim key.
  Keys stay strings, per PORT.md rule 5.
- **Sidecars.** `SidecarRecord` is an interface. The concrete pointer types
  are `*CommercialVenueRecord`, `*ServiceRequestEntry`,
  `*EntertainmentLinkRecord` and `*MedicalCenterRecord`.
  - Port `sidecar.kind === "x"` as a type assertion or type switch.
  - `GetSidecar(world, i)` returns nil when `i` is out of range, which covers
    the TS `world.sidecars[-1]` case.
  - JSON keeps the `"kind"` field (`SidecarList` decodes it).
  - `ServiceRequestEntry.FloorIndex` and `ServiceRequestEntry.CoverageFlag`
    are `*int`; nil means the TS value is `undefined`.
- **RouteState.** The TS union is flattened into one struct:
  `RouteState{Mode, SegmentID, Destination, CarrierID, Direction, Source}`.
  `Mode` is one of `RouteModeIdle`, `RouteModeSegment`, `RouteModeCarrier`.
  `RouteIdle` is the `ROUTE_IDLE` value. It is a value type, so sharing it
  is safe.
- **Sims, carriers and other records** are pointers: `[]*SimRecord`,
  `[]*CarrierRecord`, `[]*CarrierCar`, `[]*CarrierRouteSlot`,
  `[]*CarrierPendingRoute` and `[]*CarrierFloorQueue`. This keeps JS object
  identity (`candidate === object` becomes a pointer comparison).
- **Integer helpers** live in `jsutil.go`:
  - Go `/` and `%` truncate, which matches `Math.trunc` and JS `%`.
  - Use `floorDiv(a, b)` for `Math.floor` on a quotient that can be negative.
  - `absInt`, `intPtr` and `filledInts` (`new Array(n).fill(v)`) are also
    there.
  - Use the builtins `min` and `max` for `Math.min` and `Math.max`.
- **RNG.** `WorldState.RngState` is an `int32`, matching the TS `| 0`.
  `SampleRng(world)` advances it.
- **Sets.** Static TS `Set` constants are `map[int]bool` or `map[string]bool`,
  used only for lookup.
- **Exported names.** TS names become CamelCase: `GridWidth`, `GroundY`,
  `FamilyOffice`, `StateMorningGate`, `TileWidths`, `TileCosts`,
  `EntityPopulationByType`, `HotelFamilies` and so on. A TS function `fooBar`
  becomes `FooBar`. A few file-private TS helpers are unexported (for
  example `makeSim`, `allocSidecar`, `findCarrierAtClick`).
- **Tests.** Use `skipOnTODO(t, fn)` for ported TS tests that need unported
  code; the test starts asserting once the stub is gone. Use
  `expectTODOPanic(t, "<stub>", fn)` to check that A1 code ran up to a stub.

## Stubs (stubs_phase_a1.go)

Every stub panics with `"TODO port: <ts path> <fn>"`.

| Stub | TS source | Owner |
|---|---|---|
| `FloorToSlot(carrier, floor) int` | carriers/slot.ts floorToSlot | A2 |
| `MakeCarrierCar(numSlots, homeFloor) *CarrierCar` | carriers.ts makeCarrierCar | A2 |
| `RebuildCarrierList(world)` | carriers.ts rebuildCarrierList | A2 |
| `InitCarrierState(world)` | carriers.ts initCarrierState | A2 |
| `CancelRuntimeRouteRequest(carrier, simID)` | queue/cancel.ts | A2 |
| `RebuildRouteReachabilityTables(world)` | reachability/rebuild-tables.ts | A2 |
| `RebuildTransferGroupCache(world)` | reachability/rebuild-tables.ts | A2 |
| `RebuildSpecialLinkRouteRecords(world)` | reachability/special-link-records.ts | A2 |
| `ServiceIdleTasksContext` + `ServiceIdleTasks(ctx)` | tick/service-idle-tasks.ts | A2/B3 |
| `RunSimulationDayScheduler(world, ledger, time) *TimeState` | tick/day-scheduler.ts | B3 |
| `ActivateObjectFamilyHousekeepingConnectionState(world, time, sim, arrivalFloor)` | families/housekeeping.ts | B1 |
| `RebuildParkingCoverage(world)`, `RebuildParkingDemandLog(world)` | sims/parking.ts | B2 |
| `InvalidateMedicalSlotsForSidecar(world, idx)` | sims/medical.ts | B2 |
| `RecomputeCommercialVenueBuckets(world)` | families/entertainment.ts | B2 |
| `HandlePromptResponse(world, ledger, time, promptID, accepted) bool` | events.ts | B3 |
| `TickVipSpecialVisitor(world, time)` | events.ts (used only by a ported test) | B3 |

`ActivateObjectFamilyHousekeepingConnectionState` is called with a **nil**
time from `MaybeStartHousekeepingRoomClaim`, because the TS passes
`null as never`. The real port must not dereference time on that path.

## Iteration order decisions

- `cells`, `cellToAnchor`, `overlays`, `overlayToAnchor` and
  `placedObjects` are OrderedMaps. Every `Object.entries/keys/values` over
  them uses the snapshot methods.
- These loops depend on that order:
  - `doExpenseSweep`: cash is clamped at 0 after every charge.
  - `rebuildRuntimeSims`: input to a stable sort.
  - `hasNearbyNoise` and `refreshOccupiedFlagAndTripCounters` (branch B
    picks the first donor).
  - `runGlobalRebuilds`: the metro floor comes from the last metro top.
  - `appendParkingCoveragePatches`, `cellsToArray`, `hasRecyclingStackOverlap`
    (breaks on the first hit) and `hasOverlappingMisalignedStairs`.
  - The `subRecordKeys` filter in `handleRemoveTile`.
  - The VIP and metro scans in `normalizeSnapshot`.
  - `Step` cell patches.
- `rebuildRuntimeSims` uses `sort.SliceStable` with the TS comparator,
  because `Array.prototype.sort` is stable.
- Local JS `Set`s that are iterated (`stackCells` and `structureCells` in
  the metro, recycling and entertainment placement helpers, and
  `adjacentAnchors` in `hasMisalignedAdjacentOverlay`) are ported as a slice
  for order plus a map for membership.
- Lookup-only JS `Map`s and `Set`s are Go maps. These are `previous` (last
  duplicate wins, like `new Map(entries)`), `slotByFloor`, `removedIds`,
  `freedSidecars`, `evictedSet`, `pendingBySimId` (later `set` wins) and
  `evalBefore` in `Step`.
- `currentPopulationBuckets` (TS `Record<number, number>`) is
  `map[int]int`. It is never iterated in the TS.
- `TILE_TO_FAMILY_CODE` is built from `FAMILY_CODE_TO_TILE` with integer
  keys in ascending order (the JS integer-key order), and the last write
  wins. This gives `metro` -> 0x20 and `lobby` -> 24 (`FAMILY_PARKING`). Both
  quirks are kept.
- In `dispatchEntertainmentGuestSubstate`, the first matching link in
  sidecar order gets the attendance increment.
- Trace harness:
  - `floor_extent` keys are sorted numerically, as in the TS.
  - `entry.sims` is walked in JSON member order (`simKeyOrder`), so the
    first mismatch reported is the same one the TS reports.
  - Scheduled batches are scanned from the end, as in the TS.

## PORT-NOTEs (non-literal spots)

### World and resources
- `world.ts`: the `__RNG_TRACE` stack-logging debug shim is not ported.
- `world.ts`: optional TS fields:
  - `SimNotification.Message`: `""` means undefined.
  - `SimPrompt.Cost`: `*int`.
  - `PlacedObjectRecord.VipFlag`: `bool` with `omitempty`.
  - `EventState.DisableNewsEvents`: `bool` with `omitempty`.
- `resources.ts` `YEN_1001` has fractional entries (1.5, 0.5, 0.8), so it is
  stored as `float64`. Each entry times 1000 is an exact integer, so
  `int(x * 1000)` is exact.
- `resources.ts`: a comment in the TS maps 0x21 to metro bottom, but the TS
  literal only lists 0x21 as `FAMILY_CATHEDRAL_ANCHOR`, so 0x21 maps to
  `"cathedral"`.

### Sims, scoring and stress
- `sims/population.ts`: the module-level `simOwnerCache` WeakMap is an
  unexported `ownerCache` field on `SimRecord`. It is not serialized.
  Records produced by hydration or `rebuildRuntimeSims` start with an empty
  cache, which matches a fresh JS object missing the WeakMap.
- `sims/population.ts` `{...fresh, ...prior, floorAnchor, homeColumn,
  facilitySlot}` is ported as a copy of `*prior` with the three overrides.
  `prior` carries every field, so `fresh` is fully shadowed.
- `stress/lobby-reduction.ts` takes a structural `{ elapsedTicks }` argument.
  The Go version takes `*int`, so callers pass `&sim.ElapsedTicks`.
- `sims/scoring.ts` `SimStateRecord.carrierId` (`number | null`) is `*int`.

### Commands
- `commands.ts` `hasMisalignedAdjacentOverlay`: the TS always passes
  `type = "elevator"`, so express and service neighbours never count for
  alignment. Kept verbatim.
- `commands.ts` elevator placement: on a mid-loop "requires a base tile"
  rejection, floor cells written for earlier columns are not rolled back,
  same as the TS.
- `commands.ts`: the recycling-overlap check in the standard placement path
  is unreachable, because recycling tiles return early. Kept verbatim.
- `commands.ts` `handleToggleElevatorFloorStop`: JS assignment past the end
  of a short `stopFloorEnabled` array grows it with holes that read as
  `?? 1`. Go grows the slice with 1s, which is equivalent.
- `commands.ts`: `time: { daypartIndex }` defaults to `{ daypartIndex: 0 }`.
  In Go a nil `*TimeState` means that default (`daypartOf`).
- `CellPatch` optional fields are `*int` with `omitempty`. `CommandResult`
  and `SimCommand` are flat structs; `SimCommand.Type` is one of the `Cmd*`
  constants.
- `index.ts` `submitCommand`: the TS switch is exhaustive and falls through
  to `undefined`. Go returns a rejection with "Unknown command type".

### Snapshot and facade
- `snapshot.ts`: per PORT.md, legacy migrations are skipped. That covers:
  - snake_case renames and starCount-in-time;
  - the primary/secondary/tertiary ledger renames;
  - the occupiableFlag, stayPhase, pairingStatus, variantIndex and
    pairingPendingFlag placed-object renames;
  - the gate renames;
  - forwardBudget/reverseBudget;
  - the flat floor-queue format, `entities` -> `sims`, and the pre-`world`
    flat layout.
- `snapshot.ts`: the per-field `??=` defaults on numbers and booleans are
  skipped, because a typed struct cannot tell "missing" from zero. Our saves
  always write every field.
- `snapshot.ts`: these repairs are kept:
  - nil-section defaults and array-length repairs;
  - the VIP tile normalization and the metro gate recompute;
  - `RouteRequestRing.from` normalization of floor queues;
  - the `stopFloorEnabled` and `dwellDelay` resizing in hydrate;
  - all of hydrate's rebuild calls.
- `snapshot.ts`: a missing `eventState` is detected heuristically (both
  fire-position arrays empty) and replaced with `CreateEventState()`.
- `snapshot.ts`: the branch that rebuilds a world from a flat legacy
  snapshot only creates an empty perfect-parity world.
- `index.ts`: `structuredClone` and `JSON.parse(JSON.stringify(...))` become
  `CloneSnapshot`, a JSON round trip. OrderedMap order survives it.
  `SaveState` returns a detached deep copy.
- `queue/route-record.ts`: `RouteRequestRing` serializes its `capacity` own
  property, as `JSON.stringify` does.
- `index.ts`: `Create(id, name, "")` uses the default lobby mode (modern).
  The trace harness passes `LobbyModePerfectParity`.

### Trace harness
- In the fire tests, the TS reads `saveState().time.dayTick` every tick (a
  full deep copy). The Go harness reads `Time().DayTick`, which is the same
  value.
- The post-fire coherence check in the TS compares a placed-object key's y
  with `sim.floorAnchor`, which is a floor index, not a y. Kept verbatim.
- `sim_states` is not compared by the TS suite. The Go harness compares it
  only when `SIMTOWER_TRACE_SIM_STATES=1` is set (opt-in extra).
- These fields are not checked, the same as in the TS: `calendar_phase` and
  `sim_allocated/initialized/uninitialized`.

## Test status at end of A1

- `go build ./internal/sim/` and `go vet ./internal/sim/` are clean.
- Unit tests pass: ordered map, ring/route ring, RNG, lobby cadence, time,
  ledger, population rebuild (including the medical placeholder reclaim),
  scoring, stress, state bits, selectors, snapshot JSON round trip and
  normalization, catalogue, and command validation on a bare world. The
  placement tests also confirm each placement mutates state, sims, ledger
  and sidecars before reaching the first rebuild stub.
- The ported `commands.test.ts` cases and the tick-2533 scoring case skip
  with "blocked on unported code: TODO port: carriers.ts initCarrierState"
  (or the next stub), because `TowerSim.Create` hydrates, and hydrating
  needs A2.
- `TestTrace/*`, `TestTraceFire`, `TestTraceFireAccept` and
  `TestTraceCathedralRecoveredPath` **fail on purpose** with "fixture X needs
  unported code: TODO port: carriers.ts initCarrierState". As a result,
  `go test ./internal/sim/` is red until A2/B land.
- Use `-run 'TestCommands|TestScoring|...'` for the A1-only suite.
- Harness check (temporary scratch copy, not committed): I replaced the
  rebuild stubs with no-ops and the tick with `AdvanceOneTick`.
  `build_lobby_only` then passed its full trace, and `build_condo` diverged
  at day 0 tick 0 (population), which is where the unported day scheduler
  starts doing work. So the streaming, seeding and comparison mechanics work.
