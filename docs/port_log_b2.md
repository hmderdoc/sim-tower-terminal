# Phase B2 port log

Phase B2 (commercial and services) of the TS -> Go port of
`reference/tower-together/sim`. Package `sim` in `internal/sim`. This log
records each place where the port is not a literal transliteration, every
pending declaration, and every iteration-order decision.

## Files

| TS | Go |
|---|---|
| sims/commercial.ts | sims_commercial.go |
| sims/medical.ts | sims_medical.go |
| sims/parking.ts | sims_parking.go |
| families/restaurant.ts | families_restaurant.go |
| families/retail.ts | families_retail.go |
| families/medical.ts | families_medical.go (comment only: the TS file only re-exports sims/medical.ts) |
| families/parking.ts | families_parking.go |
| families/recycling.ts | families_recycling.go |
| families/entertainment.ts | families_entertainment.go |
| entertainment.ts | entertainment.go |
| recycling.ts | recycling.go |
| cathedral.ts | cathedral.go |
| determinism.test.ts (the commercial and mixed fixture cases) | determinism_b2_test.go |
| (new) | b2_test_helpers_test.go, sims_parking_test.go, sims_medical_test.go, sims_commercial_test.go, entertainment_test.go, families_entertainment_test.go, recycling_test.go, cathedral_test.go |

`stubs_b2.go` is deleted. All four of its stubs are now real functions:

- `RebuildParkingCoverage`
- `RebuildParkingDemandLog`
- `InvalidateMedicalSlotsForSidecar`
- `RecomputeCommercialVenueBuckets`

`pending_b2.go` is also deleted. Every function B2 needed from another phase
now exists in that phase's file, with the same signature B2 calls:

| Function | Defined in |
|---|---|
| `ResolveSimRouteBetweenFloors`, `ResolveSimRouteOptions` | queue_resolve.go (A2) |
| `DecrementRouteQueueDirectionLoad` | queue_cancel.go (A2) |
| `ReleaseServiceRequest`, `TryAcquireOfficeVenueSlot`, `VenueSlotFull` | sims_index.go (B1) |
| `ActivateRetailShop`, `IncrementVenueSeed` | sims_facility_refunds.go (B1) |
| `MaybeDispatchQueuedRouteAfterWait` | families_maybe_dispatch_after_wait.go (B1) |
| `AddToPopulationBucket`, `ClearPopulationBucket` | progression.go (B3) |

## Exported API (for callers in other phases)

**sims_commercial.go**
- `ProcessCommercialSim(world, ledger, time, sim)`
- `HandleCommercialSimArrival(world, sim, arrivalFloor, time)`

**sims_medical.go**
- `TryStartMedicalTrip(world, time, sim) bool`
- `ProcessMedicalSim(world, time, sim)`
- `HandleMedicalSimArrival(world, sim, arrivalFloor)`
- `InvalidateMedicalSlotsForSidecar(world, idx)`
- `MedicalNotificationMessage`
- `StateMedicalTrip`, `StateMedicalTripTransit`, `StateMedicalDwell`

**sims_parking.go**
- `RebuildParkingCoverage(world)`
- `RebuildParkingDemandLog(world)`
- `TryAssignParkingService(world, time, sim) bool`

**families_restaurant.go and families_retail.go**
- `GateObjectFamilyRestaurantFastFoodStateHandler(world, ledger, time, sim)`
- `DispatchObjectFamilyRestaurantFastFoodStateHandler(world, sim, arrivalFloor, time)`
- `GateObjectFamilyRetailStateHandler` and `DispatchObjectFamilyRetailStateHandler`, with the same parameters

**families_parking.go**
- `GateObjectFamilyParkingStateHandler(world, time, sim)`
- `DispatchObjectFamilyParkingStateHandler(world, time, sim)`

**families_recycling.go**
- `GateObjectFamilyRecyclingCenterLowerStateHandler(world, ledger, time, sim)`
- `DispatchObjectFamilyRecyclingCenterLowerStateHandler(world, time, sim)`

**families_entertainment.go**
- `GateEntertainmentGuestState(world, ledger, time, sim)`
- `DispatchEntertainmentGuestState(world, time, sim)`
- `HandleEntertainmentSimArrival(world, time, sim, arrivalFloor)`
- `RecomputeCommercialVenueBuckets(world)`
- `RecomputeRestaurantVenueBuckets(world)`

**entertainment.go**
- `SeedEntertainmentBudgets`
- `ActivateEntertainmentUpperHalf`
- `PromoteCinemaAndActivatePartyHall`
- `PromoteCinemaLinksToReadyPhase`
- `ActivateEntertainmentLowerHalf`
- `AdvanceEntertainmentUpperPhase`
- `AdvancePartyHallPhaseAndAccrue(world, ledger, dayCounter)`
- `AdvanceEntertainmentLowerPairedPhaseAndAccrue(world, ledger, dayCounter)`

**recycling.go**
- `ResetRecyclingCenterDutyTier(world)`
- `UpdateRecyclingCenterState(world, ledger, param)`

**cathedral.go**
- `ActivateEvalSims(world)`
- `DispatchEvalMiddayReturn(world)`
- `ProcessCathedralSim(world, time, sim)`
- `CheckEvalCompletionAndAward(world, time, sim)`
- `HandleCathedralSimArrival(world, time, sim, arrivalFloor)`

The signatures in `pending_b3.go` for the entertainment, recycling and
cathedral functions match the ones above exactly.

## Naming decisions in the flat package

Some TS file-private names would be ambiguous or collide in the flat
package, so they were renamed:

| TS name | Go name |
|---|---|
| families/parking.ts `runDispatchJumpTable` | `runParkingDispatchJumpTable` |
| cathedral.ts `dispatchOutbound` | `cathedralDispatchOutbound` |
| cathedral.ts `dispatchReturn` | `cathedralDispatchReturn` |
| entertainment.ts `CINEMA_FAMILY_CODES` | `entertainmentCinemaFamilyCodes` (commands.go already has `cinemaFamilyCodes`) |

Other renames and helpers:

- B1's `sims_index.go` also defines a helper named `commercialVenueAt`, so
  the commercial.ts helper is named `b2CommercialVenueAt` instead.
- The B2-only helpers are `b2Bool01`, `resolveOptsFeedback`,
  `resolveOptsFeedbackHeight`, `liveEntertainmentLinks` and `b2AddIncome`.
- Constants private to the TS files are unexported: `entState*`, `ent*`,
  `stateRecycling*`, `recycling*` and `evalSimCount`.

## Iteration-order decisions

- **placedObjects (OrderedMap snapshots).** All placedObjects loops use the
  snapshot methods (`Entries`/`Values`), in insertion order. These loops
  depend on that order:
  - `collectMedicalCenters`: the bucket order is the RNG pick index.
  - `getCurrentCommercialVenueDestinationFloor`: the first linked object wins.
  - `rebuildParkingDemandLog`: an inner existence test, so order does not
    change the result.
  - `RebuildParkingCoverage`: collects ramp anchors in this order.
- **world.Sidecars and world.Sims.** These are walked by index or by range
  over the slice, which is the TS array order. `findEntertainmentLink`
  returns the first matching link. `InvalidateMedicalSlotsForSidecar`
  replaces `world.sims.find` with a first-match loop.
- **Lookup-only TS Maps.** These are ported as Go maps:
  - medical `byZone`: only `.get` is used, and each bucket is a slice
    appended in placedObjects order.
  - entertainment `sidecarToFloor` and `sidecarToFamily`: a later `set`
    wins, which is the same as a Go map overwrite.
  - parking `parkingByKey`: has one `.values()` pass that only writes
    coverage 0, so order is irrelevant. It is iterated through an
    insertion-order key slice anyway.
- **Static TS Sets.** These are `map[int]bool`, used for lookup only:
  `entertainmentGuestFamilies`, `entertainmentCinemaFamilyCodes`, and the
  family sets passed to `appendBucketsForFamilies`.
- **`liveEntertainmentLinks`.** It takes a slice snapshot of the live
  entertainment links before each pass. The TS `for..of` over
  `world.sidecars` is live, but no loop body adds or removes sidecars or
  changes `ownerSubtypeIndex`, so the result is the same.
- **RNG draw order.** Each `&&` chain that guards a `sampleRng` call keeps
  Go's short-circuit evaluation, which matches the JS order. These are:
  - the recycling dispatch gate;
  - the entertainment state-0x20 gate;
  - the cathedral `dayTick > 0x50 && rng % 12`.

  The commercial-venue picker draws before its availability check, as in
  the TS and the binary.

## PORT-NOTEs (non-literal spots)

- **Ledger growth (`b2AddIncome`).** `ledger.incomeLedger[18 | 29]` on a
  fresh 10-slot ledger extends the JS array with holes. Go grows the slice
  with zeros. Hydrated ledgers already have 256 slots.
- **`selectRandomCommercialVenueRecordFromBucket`.** The TS casts the sidecar
  without checking its kind. A non-venue sidecar's `availabilityState` is
  `undefined`, so the record is accepted. Go reproduces this: only a
  commercial venue can be rejected there.
- **`getCurrentCommercialVenueDestinationFloor`.** The 0xff route-fail
  sentinel is positive, so it does a real lookup for linkedRecordIndex 255.
  Kept as in the TS.
- **`runParkingDispatchJumpTable`.** The TS passes
  `world.carriers[0] ?? {}` to the empty `decrementRouteQueueDirectionLoad`.
  Go passes nil when there are no carriers.
- **`PromoteCinemaAndActivatePartyHall`.** The TS doc comment says it seeds
  lower-half sims to 0x20, but the TS body does not. Ported as the body
  reads.
- **families/recycling.ts.** The header comment says "family 33 / 0x21",
  but the code gates on `FAMILY_RECYCLING_CENTER_LOWER` (21 = 0x15). Ported
  as the code reads.
- **Medical zone keys.** `Math.floor((floor - 9) / 15)` can be negative
  for underground floors, so it uses `floorDiv`. The entertainment
  `Math.trunc` zone and bucket math uses Go `/` and `%`, which truncate the
  same way.
- **`Math.abs(sampleRng(world))`.** Ported as `absInt(SampleRng(world))`.
  `SampleRng` is never negative, so the call is a no-op kept for fidelity.
- **Optional route options.** `ResolveSimRouteOptions` fields are pointers,
  and a nil options pointer means the TS `= {}` default. This follows A2's
  `queue_resolve.go` and B1's call sites.
- **A1 test edit.** In `commands_test.go`,
  `TestCommandsPlaceOfficeMutatesBeforeRebuildStubs` expected the panic
  from the removed `rebuildParkingCoverage` stub. It now accepts any
  `"TODO port:"` panic, or none. Its post-condition checks are unchanged.

## Tests

- The new B2 unit tests cover:
  - parking coverage propagation, including the 3-tile gap rule, tombstones
    and the demand log;
  - parking assignment RNG use;
  - the medical gates, first-fit slot allocation, zone preference with
    global fallback, the failure banner, sidecar invalidation, arrival and
    retry overflow;
  - the commercial night -> morning gate, the retail dormant early exit
    with no RNG draw, the restaurant daypart windows and closed silent-park,
    try_consume bookkeeping, and the arrival fallback;
  - entertainment budget tiers, seeding with population buckets, the phase
    cycle, payouts including calendar-edge skip days, and income ledger
    growth;
  - venue bucket classification, rebuild, restaurant rebuild and the picker
    (RNG drawn before rejection, row-0 fallback);
  - acquire_commercial_venue_slot;
  - the entertainment, recycling and parking-family gates;
  - recycling adequacy tiers and the daily reset;
  - cathedral activation, midday return, the 40-arrival award and its tick
    800 cutoff, the gates, and arrival.
- Tests that route through `ResolveSimRouteBetweenFloors` or `TowerSim`
  hydration use `skipOnTODO` or `todoPanic`, so they skip until the code
  they need is ported.
- `sim.test.ts`, named in AGENTS.md, does not exist upstream. The only TS
  tests covering B2 code are the commercial and mixed fixture cases in
  `determinism.test.ts`, which are ported. `progression.test.ts` belongs
  to B3.

### Status at end of B2

- B3 removed `pending_b3.go`, and no `stubs_*` or `pending_*` files are left
  in the package.
- In the live tree, `go build ./internal/sim/` and `go vet ./internal/sim/`
  are clean.
- `go test ./internal/sim/` passes. That covers:
  - all B2 unit tests;
  - `TestDeterminismFixturesB2` (mixed and commercial, idempotent and
    lockstep);
  - every `TestTrace` fixture, run to full length (including
    build_commercial and build_mixed);
  - `TestTraceCathedralRecoveredPath`, `TestTraceFire` and
    `TestTraceFireAccept`.
