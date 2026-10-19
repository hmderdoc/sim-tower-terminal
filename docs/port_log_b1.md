# Phase B1 port log (residential families)

Phase B1 of the TS -> Go port of `reference/tower-together/sim`: the runtime
sims facade, office/condo/hotel/housekeeping state machines, hotel/condo
facility sweeps, commercial venue refunds, and the binary-named `families/*`
entry points. Package `sim`, directory `internal/sim`.

## File map

| TS | Go |
|---|---|
| sims/index.ts | sims_index.go |
| sims/office.ts | sims_office.go |
| sims/condo.ts | sims_condo.go |
| sims/hotel.ts | sims_hotel.go |
| sims/hotel-facilities.ts | sims_hotel_facilities.go |
| sims/housekeeping.ts | sims_housekeeping.go |
| sims/facility-refunds.ts | sims_facility_refunds.go |
| families/office.ts | families_office.go (also holds `prologueCarrierForSim`, shared by the three refresh prologues) |
| families/condo.ts | families_condo.go |
| families/hotel.ts | families_hotel.go |
| families/housekeeping.ts | families_housekeeping.go (replaces the A1 stub; `stubs_b1.go` deleted) |
| families/state-tables/office.ts | families_state_tables_office.go |
| families/state-tables/condo.ts | families_state_tables_condo.go |
| families/state-tables/hotel.ts | families_state_tables_hotel.go |
| families/state-tables/family-prologue.ts | families_state_tables_family_prologue.go |
| families/finalize.ts | families_finalize.go |
| families/reset.ts | families_reset.go (comment only: pure re-export of A1's `ResetSimRuntimeState`) |
| families/tile-spans.ts | families_tile_spans.go |
| families/shared-dispatch.ts | families_shared_dispatch.go |
| families/dispatch-sim-behavior.ts | families_dispatch_sim_behavior.go |
| families/maybe-dispatch-after-wait.ts | families_maybe_dispatch_after_wait.go |
| families/force-dispatch.ts | families_force_dispatch.go |
| sim-refresh/refresh-stride.ts | sim_refresh_refresh_stride.go (`RefreshRuntimeEntitiesForTickStride`, alias of `AdvanceSimRefreshStride`) |
| sims/housekeeping.test.ts | sims_housekeeping_test.go |
| (new) | sims_b1_test.go (Go-only checks: office presence counter, end-of-day bands, shared-dispatch floors, venue seed, venue slot acquire/release) |

The TS re-export shims (`families/*.ts` re-exporting `sims/*.ts`,
`state-tables/*.ts` re-exporting the handler tables, `sims/index.ts`
re-exporting population/scoring/parking/trip-counters, `sim-office.ts`) need
no Go wrappers because the package is flat. The only thin wrappers kept are
the binary-named functions that have their own TS bodies
(`RefreshObjectFamily*StateHandler`, `DispatchObjectFamily*StateHandler`,
`GateHousekeepingRoomClaimState`, `UpdateObjectFamilyHousekeepingConnectionState`,
`ActivateObjectFamilyHousekeepingConnectionState`,
`RefreshRuntimeEntitiesForTickStride`).

## API shapes other phases call

- `ResolveSimRouteBetweenFloors(..., time, opts *ResolveSimRouteOptions)`:
  callers pass `nil` for the TS default `{}`; `resolveNoFeedback()`
  (sims_office.go) builds `{emitDistanceFeedback: false}`.
- `DispatchCommercialVenueVisit(world, time, sim, CommercialVenueVisitOptions)`:
  `TripState` / `UnavailableState` are `*int` (nil = undefined; the trip state
  then defaults to `StateVenueTrip`); `OnVenueReserved` is a nil-able `func()`.
- `HandleCommercialVenueArrival(..., tripState int)`: the TS default parameter
  is explicit in Go (pass `StateVenueTrip`).
- `HandleOfficeSimArrival(world, time, sim, arrivalFloor, alreadyAdvanced bool)`:
  the TS default `false` is explicit.
- `TryAcquireOfficeVenueSlot` returns `VenueSlotAcquired` (3), `VenueSlotFull`
  (2) or `VenueSlotUnavailable` (-1).
- `SelectFacilityProgressSlot` returns the TS literal union `3 | 4 | 5` as int.
- Handler tables are Go maps (`OfficeRefreshHandlerTable`,
  `CondoRefreshHandlerTable`, `HotelRefreshHandlerTable`) filled in `init()`
  to avoid an initialization cycle. The prologue tables are `map[int]string`.

## Pending declarations

None. Every cross-phase function B1 calls already existed when B1 finished:
A2 (`ResolveSimRouteBetweenFloors`, `ResolveSimRouteOptions`,
`DecrementRouteQueueDirectionLoad`, `PopUnitQueueRequest`,
`SyncAssignmentStatus`, `FloorToSlot`), B2 (`TryAssignParkingService`,
`TryStartMedicalTrip`, `ProcessMedicalSim`, `HandleMedicalSimArrival`,
`StateMedical*`, `ProcessCommercialSim`, `HandleCommercialSimArrival`,
`GateEntertainmentGuestState`, `HandleEntertainmentSimArrival`,
`ProcessCathedralSim`, `HandleCathedralSimArrival`,
`RecomputeCommercialVenueBuckets`, `RecomputeRestaurantVenueBuckets`,
`GateObjectFamilyRetailStateHandler`,
`GateObjectFamilyRestaurantFastFoodStateHandler`) and B3
(`AddToPopulationBucket`, `ClearPopulationBucket`). So there is no
`pending_b1.go`.

Other phases' `pending_*.go` files declared B1 functions as panicking
placeholders (`pending_b3.go`: `DispatchSimBehavior`,
`RebuildAllSimTileSpans`, the facility-refunds and hotel-facilities sweeps;
`pending_b2.go`: `MaybeDispatchQueuedRouteAfterWait`). Those now collide with
the real B1 definitions and must be deleted from those files by their owner or
the integration pass. B1 did not edit them.

Shared-helper notes:
- `hotelFamilyCodes` (the HOTEL_FAMILY_CODES set) is reused from
  sims_trip_counters.go.
- `findWaitCarrier` is a local `carriers.find` because the A2 helper of
  that shape moved while B1 was in progress.
- `dirFlag(a, b)` is the `a > b ? 1 : 0` idiom (sims_office.go), and
  `commercialVenueAt` is the typed sidecar lookup (sims_index.go).

## Iteration-order decisions

- Every TS `Object.entries/values(world.placedObjects)` loop walks the
  OrderedMap snapshot (`Entries()` / `Values()`) in insertion order:
  - `pickAvailableVenue`: bucket order decides which venue
    `sampleRng % len` picks, so it is RNG-relevant.
  - `findCommercialVenueAtFloor`: first match wins.
  - `floorLocalRoomRank`, which then stable-sorts the columns.
  - `normalizeUnitStatusEndOfDay`, `handleExtendedVacancyExpiry`,
    `advanceObjectStayPhaseTiers`, `rebuildCommercialVenueRuntime` and
    `rebuildRestaurantFacilityRecords`: population-bucket adds are
    order-independent, but the order is kept anyway.
  - `closeCommercialVenuesByFamily`: cash is clamped per add.
  - `refundUnhappyFacilities`: cash is clamped at 0 per removal.
- `spreadCockroachInfestation` uses one `Entries()` snapshot for both the
  outer and inner loops, like the TS `objectsByKey` array. A room infected
  earlier in the pass can then spread again later in the same pass.
- `findVacantHotelRoomForClaim`:
  - `byFloor` is a lookup-only Go map.
  - Each per-floor list is built in placedObjects order and then sorted
    with `sort.SliceStable` by x, so equal-x ties keep insertion order.
  - The upward and downward floor scans are explicit loops.
- `world.sims.find(...)` (housekeeping occupant lookups and the popped-sim
  lookup in `dispatchQueuedRouteUntilRequest`) is a forward scan with the
  first match winning. `carrier.pendingRoutes.find` and
  `medicalServiceSlots.find` work the same way.
- `reconcileSimTransport`'s `completed` Set is lookup-only (a Go map). Both
  sim sweeps walk `world.sims` in order.
- `advanceSimRefreshStride` walks `world.Sims` by index, the 1-of-16
  stride.
- Static TS `Set` constants and the handler and prologue `Map`s are
  lookup-only Go maps.

## RNG draw order

Every `sampleRng` call is in the same position and under the same guard as
in the TS. Calls of note:
- the office morning gate and commute 1/12 gates;
- `nextOfficeMorningState`, which draws only for baseOffset != 0 with
  starCount >= 3;
- the condo and hotel 1/12 and 1/6 gates;
- `pickAvailableVenue`, which draws whenever the bucket is non-empty, even
  when the pick is then rejected.

The draws inside `TryStartMedicalTrip` and `TryAssignParkingService` belong
to B2.

## PORT-NOTEs (non-literal spots)

- sims/office.ts `OFFICE_REFRESH_HANDLER_TABLE`:
  - The TS lists both `STATE_VENUE_TRIP_TRANSIT` and
    `STATE_VENUE_HOME_TRANSIT`. Both are 0x62, so they are one Go map key.
  - The TS comment labels the entry "0x42", but no office handler is
    registered for 0x42.
- sims/index.ts `shouldFinalizeSegmentTrip`: the duplicate 0x62 state is
  listed once.
- sims/index.ts `finalizePendingRouteLeg`: housekeeping is not in its skip
  list (it is in `shouldFinalizeSegmentTrip`'s), so HK segment sims get
  `selectedFloor` synced. Kept verbatim.
- sims/facility-refunds.ts `applyClosureCash`:
  - The TS writes `incomeLedger[familyCode]` on a 10-slot array.
  - Restaurant (6) hits slot 6. Fast food (12) grows the JS array with a NaN
    entry, which the next rollover zeroes.
  - Go drops that out-of-range write. incomeLedger is display-only and cash
    is unaffected.
- sims/hotel.ts `handleHotelVenueTrip` / `handleHotelCommercialDwell`: the
  direction flag is computed from `floorAnchor > LOBBY` even when the source
  is `selectedFloor`. Kept verbatim.
- families/housekeeping.ts `activateObjectFamilyHousekeepingConnectionState`
  is called with a nil `*TimeState` from `MaybeStartHousekeepingRoomClaim`
  (the TS passes `null as never`):
  - That caller only passes sims in state 0, and
    `HandleHousekeepingSimArrival` reads no time field for state 0, so nil
    is safe.
  - States 1, 3 and 4 would dereference time, just as the TS would throw on
    `null.dayTick`.
  - Covered by `TestActivateHousekeepingConnectionStateNilTime`.
- families/*.ts refresh prologues: the TS falls back to `world.carriers[0]`.
  `c.carrierId === undefined` never matches for a non-carrier route, so
  `prologueCarrierForSim` only searches by id in carrier mode.
- families/maybe-dispatch-after-wait.ts: `carrier.floorQueues[slot]` out of
  range (TS `undefined`) is a bounds check. `popUnitQueueRequest` returning
  `undefined`/`""` maps to `!ok || id == ""`.
- families/office.ts: the TS `void simBaseState(...)` is kept as `_ =`.

## Test status at end of B1

- `sims_housekeeping_test.go` ports all 11 `housekeeping.test.ts` cases and
  adds a nil-time check. All pass, including the cross-floor route-fail case,
  which runs through A2's real resolver.
- `sims_b1_test.go`: all pass.
- `sim.test.ts` does not exist upstream (see port_log_a1.md), so there was
  nothing more to port.
- In-tree `go build ./internal/sim/` fails right now **only** because of the
  colliding placeholders in `pending_b3.go` described above. That file also
  collides with B2's real cathedral/entertainment/recycling functions.
- On a scratch copy with the colliding pending entries stripped:
  - `go build` and `go vet` are clean.
  - The full `go test ./internal/sim/` run passes `TestTrace` (all 16
    fixtures), `TestTraceFire`, `TestTraceFireAccept` and
    `TestTraceCathedralRecoveredPath`.
  - The only failure is B2's `TestGateEntertainmentGuestState`
    (families_entertainment_test.go:116). That test expected a TODO skip and
    now runs against the real resolver.
