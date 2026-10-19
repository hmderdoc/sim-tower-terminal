# Porting tower-together's sim to Go

`internal/sim` is a line-faithful port of `reference/tower-together/sim/`
(TypeScript, ~20.7k lines, MIT). That sim reproduces SIMTOWER.EXE tick for
tick; the per-tick reference traces in `reference/tower-together/sim/fixtures`
are the acceptance test (`internal/sim/trace_test.go`). Read
`reference/tower-together/sim/AGENTS.md` first for the tour, and the matching
`reference/tower-together/specs/*.md` for the why behind each file.

## Rules that keep parity

1. **One Go package.** Everything goes in `package sim` (directory
   `internal/sim`), one Go file per TS file, named after the TS path with the
   directory flattened: `carriers/advance.ts` -> `carriers_advance.go`,
   `sims/hotel.ts` -> `sims_hotel.go`, `queue/resolve.ts` -> `queue_resolve.go`.
   Keep the TS function names (exported, CamelCase). Put a comment above every
   ported function naming the TS source function and, when the TS comment
   gives one, the binary address (e.g. `// TS: carriers/target.ts recomputeCarTargetAndDirection (binary 1098:xxxx)`).
2. **Integer semantics.** The TS uses JS numbers but every value is a binary
   integer. Use `int` (and `int32`/`uint32` where the TS does `| 0`, `>>> 0`,
   `Math.imul`). The RNG is `state = state*0x15a4e35 + 1` on uint32, output
   `(state >> 16) & 0x7fff`. Division in the TS is `Math.floor`/`Math.trunc`
   or JS float division: read each site and pick Go's `/` (truncating) only
   when the operands are non-negative or the TS truncates; otherwise write a
   `floorDiv` helper. Never silently change a formula.
3. **Iteration order is simulation state.** JS `Map`, `Set` and objects with
   non-integer string keys iterate in insertion order; objects with integer-like
   keys iterate those keys in ascending numeric order first. Go maps are
   random. Wherever the TS iterates a Map/object and the order can affect
   results (dispatch, RNG draws, queue draining, scoring ties), use the
   insertion-ordered `OrderedMap[K,V]` helper in `ordered_map.go` (slice of
   keys + map), or a slice. When in doubt, use the ordered helper.
4. **Stable sorts.** JS `Array.prototype.sort` is stable: use
   `sort.SliceStable` / `slices.SortStableFunc`.
5. **String keys stay strings** where the TS uses them as identity
   (`"x,y"` cell keys, `simKey`, `routeId`). Do not "improve" them into ints
   during the port; that can be done later behind the same API once the trace
   test passes.
6. **No behaviour fixes during the port.** If the TS looks wrong, port it as
   is and add a `// PORT-NOTE:` comment. The trace test decides.
7. **Tests travel with the code.** Port the TS unit tests that cover your
   files (`*.test.ts`) into `*_test.go` where practical; the fixture replay
   test is the big one.
8. **Deterministic.** No `time.Now`, no `math/rand`, no goroutines inside the
   sim. The only randomness is the world RNG.

## Save format

The Go save is JSON of the snapshot struct (port of `snapshot.ts`
`SimSnapshot`), written per BBS user by the door. Hydration/migration logic
from `snapshot.ts` is ported too, but legacy-version migrations can be
skipped; our saves start at the current layout.

## File map and phases

Phase A1 - foundation (one agent):
- `world.ts`, `resources.ts`, `time.ts`, `ledger.ts`, `ring-buffer.ts`,
  `queue/route-record.ts`, `sims/states.ts`, `sims/scoring.ts` (types +
  scoring), `sims/trip-counters.ts`, `sims/population.ts`, `sim-access/*`,
  `stress/*`, `snapshot.ts`, `commands.ts`, `index.ts` (the `TowerSim`
  facade; it may call not-yet-ported functions through stubs that panic with
  `"TODO port: <ts function>"`), plus `ordered_map.go` and `trace_test.go`
  (fixture loader + harness mirroring `trace.test.ts`; skip fixtures whose
  features are still stubbed).

Phase A2 - transport (one agent, after A1):
- `reachability/*`, `route-scoring/*`, `carriers.ts`, `carriers/*`,
  `queue/*` (except route-record), `tick/carrier-tick.ts`.

Phase B - behaviour (parallel after A1, each agent owns its files; cross-file
calls into other agents' files are allowed and a final integration pass fixes
signatures):
- B1 residential: `sims/index.ts`, `sims/office.ts`, `sims/condo.ts`,
  `sims/hotel.ts`, `sims/hotel-facilities.ts`, `sims/housekeeping.ts`,
  `sims/facility-refunds.ts`, `families/office.ts`, `families/condo.ts`,
  `families/hotel.ts`, `families/housekeeping.ts`, `families/state-tables/*`,
  `families/finalize.ts`, `families/reset.ts`, `families/tile-spans.ts`,
  `families/shared-dispatch.ts`, `families/dispatch-sim-behavior.ts`,
  `families/maybe-dispatch-after-wait.ts`, `families/force-dispatch.ts`,
  `sim-refresh/*`.
- B2 commercial and services: `sims/commercial.ts`, `sims/medical.ts`,
  `sims/parking.ts`, `families/restaurant.ts`, `families/retail.ts`,
  `families/medical.ts`, `families/parking.ts`, `families/recycling.ts`,
  `families/entertainment.ts`, `entertainment.ts`, `recycling.ts`,
  `cathedral.ts`.
- B3 world events and scheduling: `events.ts`, `progression.ts`,
  `daily/*`, `tick/day-scheduler.ts`, `tick/service-idle-tasks.ts`,
  `scheduler.ts`.

Phase C - integration: make `go build ./... && go vet ./...` clean, run
`trace_test.go` over every fixture, fix divergences earliest-tick-first.

Phase D - the door UI (`internal/ui`, `cmd/simtower`) consumes the sim
through the `TowerSim` facade: `Step()`, `SubmitCommand()`, and read access
to the world (cells, placed objects, carriers, sims, ledger, time).

## Gameplay catalogue vs. trace parity

`resources.ts` has a few values the upstream author got wrong for real play
(party hall width 27, metro $45,000, lobby $3,000, express/service shaft
$200,000). The fixture replays run with `freeBuild` so costs do not affect
parity. Keep the TS tables verbatim in `resources.go` for the port, and put
the verified gameplay numbers (docs/mechanics.md section 2) in
`catalog.go` as the table the door's build menu and `Place` cost path use;
`catalog_test.go` asserts the two agree except for the listed known errors.
