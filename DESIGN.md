# SimTower door - design

A faithful SimTower (Maxis, 1994) for a BBS terminal, written in Go as a
Synchronet external program (DOOR32.SYS socket). Gameplay is 1:1 with the
original wherever the rules are known; the presentation is CP437 at one cell
per unit and one row per floor (see the plan page and `docs/graphics.md`).

Mechanics sources: `docs/mechanics.md` (compiled from tower-together's
reverse-engineered specs, the manual, the wiki and FAQs) and
`reference/tower-together/specs`. The port log files under `docs/` record
every place the Go code deliberately mirrors a quirk of the binary.

## Packages

```
cmd/simtower        main: flags, dropfile, term open, size probe, event loop
internal/term       Term interface (socket/stdio), DOOR32 parsing, key + SGR mouse decoder
internal/screen     CP437 cell buffer with diff flush
internal/sim        the whole game simulation; no I/O, no terminal knowledge
internal/ui         views: tower viewport, status/footer, palette, popups, map, finance
data/               per-user saves  <alias>.json  (gitignored)
```

The split that matters: `sim` is pure and deterministic given (state,
inputs). `ui` talks to it only through the `ui.Model` interface
(`internal/ui/model.go`), implemented by `simmodel.go` (real sim) and
`demo.go` (scripted demo tower for `-demo`).

## Simulation

`internal/sim` is a line-faithful Go port of tower-together's TypeScript sim
(`docs/PORT.md`, `docs/port_log_*.md`). It reproduces SIMTOWER.EXE tick for
tick: `internal/sim/trace_test.go` replays the 16 per-tick reference traces
captured from the original binary (`reference/tower-together/sim/fixtures`)
and every one passes to its last entry (`go test ./internal/sim/ -run TestTrace`).

Facade (`internal/sim/index.go`): `Create(towerID, name, lobbyMode)`,
`FromSnapshot`, `Step()` (one tick), `SubmitCommand(SimCommand)` (place_tile,
remove_tile, prompt_response, set_rent_level, add/remove_elevator_car,
set_elevator_dwell_delay / waiting_car_response / home_floor,
toggle_elevator_floor_stop, set_cinema_movie_pool), `SaveState()` (JSON
snapshot), `QueryCell`, `LiveSims`, `LiveCarriers`, `FireFronts`,
`DrainNotifications`, `DrainPrompts`, plus `World()/Ledger()/Time()`.

Coordinates inside the sim are the binary's: grid 375 x 120, y grows
downward, floor index raw 0..119 with raw 10 = 1F (`world.go` helpers
`yToFloor`/`floorToY`, `GROUND_Y`). The UI converts to its own 0 = 1F,
-1 = B1 floors in the adapter (`internal/ui/simmodel.go`).

Lobby mode: the door creates towers in "perfect-parity" mode (sky lobbies at
15F, 30F, ... in 1-based floors, matching the binary); "modern" is the
upstream's own variant and is not used.

`resources.go` keeps the upstream tables verbatim for parity (the trace
replays run with free build). `catalog.go` holds the verified gameplay costs
from `docs/mechanics.md` and is what the build menu charges.

## Time and the main loop

The adapter converts real seconds to sim ticks with an accumulator (cadence
documented in `docs/ui_notes.md`); the tick -> clock mapping is the
original's non-linear one (2600 ticks per day, slow lunch hour), so the
clock already runs slower at rush hour. The UI loop runs at 8 fps: drain
input, `Step(dt)`, render into the cell buffer, `Flush` the diff.

## Rendering rules (ui)

See `docs/graphics.md` (binding spec) and `docs/graphics_plan.html` (the
approved animated mock). One column per unit, three rows per floor, drawn
tiles per item with states and animation, 16 colors, responsive from 80x24
up. `docs/ui_notes.md` describes the Go side (Model interface, tiles as
data, frame loop, input model).

## Saves

One JSON file per BBS user: `data/<alias>.json` (sanitized alias). Autosave
on quit and at every quarter boundary. A new game starts with the original's
funds and an empty lot; the player picks continue/new on entry.

## Input

Mouse (SGR 1006, click + drag) is the primary building input; the keyboard
cursor is a full fallback. Hotkeys are listed on the plan page and in the
palette.
