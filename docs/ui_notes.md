# UI notes (internal/ui)

How the presentation layer is put together, and what the sim adapter must
provide. The binding visual spec is `docs/graphics.md` plus the mock
`docs/graphics_plan.html`; this file describes the Go side.

## Running it

```
go run ./cmd/simtower -demo -size 80x24     # local tty, Q quits
go run ./cmd/simtower -demo                 # probe / use the tty size
go test ./internal/ui/...                   # goldens + byte safety + input paths
go test ./internal/ui/ -update              # accept new renders into testdata/
```

Without `-demo` the door opens the start menu (splash + Continue / New tower
/ Demo tower / Quit) and runs the real simulation through `SimModel`
(`simmodel*.go`, see "Sim adapter" below). `-demo` skips the menu and runs
`DemoModel` directly. `python3 tools/pty_smoke.py <binary>` drives a built
door headless on a pty (build, let time pass, finance, quit) and checks for
panics, unsafe bytes and the save file.

`internal/ui/testdata/*.txt` holds one dump per view (80x24 for every view,
132x50 for the main ones): the glyph grid as Unicode, then a fg grid (hex
digit per cell) and a bg grid (digit per cell). `less` them to review visuals
in a terminal.

## Files

| File | What |
|---|---|
| `model.go` | the `Model` interface and every data type the UI reads |
| `demo.go` | `DemoModel`: the mock's `TOWER` with scripted motion, accepts build commands, raises a fire (1:00 PM day 1) and a bomb (10:00 AM day 2) prompt |
| `tiles.go` | tile tables (ported from the mock's `TILES`), palette `C`, cache, `Blit` |
| `cp437.go` | Unicode <-> CP437 tables; `CP()` converts authored strings at init |
| `app.go` | `App`, layout, frame loop, hotspots, drawing primitives, popup stack |
| `render.go` | the viewport (`scene()` port), status row, footer, eval overlay, ghost, cursor, zoom |
| `input.go` | key/mouse routing, drag/pan, actions |
| `views.go` | build palette, facility / tenant / elevator windows, map, finance, rating, event prompt, message log |
| `startmenu.go` | `Launcher`, the start menu over the splash, `App.setModel`, the `modalView` rule |
| `simmodel.go` | `SimModel`: construction, stepping/cadence, notifications -> messages, prompts -> events, clock |
| `simmodel_view.go` | sim state -> `Item`/`FloorSpan`/`Shaft`/`Person` (refresh per tick), fire/burned, cars, queues, walkers |
| `simmodel_place.go` | build catalogue, `CanPlace`/`Place` (read-only port of the `HandlePlaceTile` rules + catalogue pricing) |
| `simmodel_cmd.go` | remove, elevator/rent/movie/rename commands, Inspect/Schedule/Tenant, finance/rating, save/load |

## Model interface (summary)

Reads: `Bounds` (floor range, lot width), `Catalog` (palette rows), `Floors`
(built span per floor), `Items`, `Shafts` (cars + queues), `People` (walkers),
`Clock`, `Funds`, `Population`, `Stars`, `Speed`, `Messages`, `Event`,
`Finance`, `Rating`, `Inspect(itemID)`, `Schedule(shaftID)`, `Tenant(personID)`.

Commands: `Step(dt)`, `SetSpeed`, `CanPlace` / `Place(kind, x, floor, w, h)`,
`Remove(x, floor)`, `AddCar`, `RemoveCar`, `SetCarHome`, `ToggleServe`,
`SetPeriodMode`, `SetResponse`, `SetDeparture`, `SetRent`, `ChangeMovie`,
`RenameTenant`, `Respond(choice)`, `Save`.

Conventions:
- Floor index 0 = 1F, -1 = B1. Multi-floor items report their BOTTOM floor
  and H in floors. X is the left unit.
- `Car.Pos` is in thirds of a floor: `floor*3 + 0..2`, the car's bottom.
- `Queue.Up/Down` are one `Stress` per waiting person, front first.
- `Item.State` is the UI's own enum (Vacant, Occupied, ForSale, Asleep, Dirty,
  Infested, VIP, Open, Closed, Busy, Active, Burning, Burned); `tileStateFor`
  maps it (plus `Lit`) to a tile state. `Active` covers cinema showing, party,
  metro train, wedding and stairs in use.
- `Rent` is the tier 0 (high) .. 3 (very low), -1 when not rentable.
- Returned slices may be reused by the model next call; the UI only reads
  them within one frame.
- `Step` is called every render tick even while paused; the model applies
  its own speed and should not advance while an `Event` prompt is open.
- Text from the model must be plain ASCII (no Unicode punctuation).

What the sim adapter must provide beyond a straight field copy:
- **Lobby runs**: return each contiguous lobby run on a floor as ONE item
  (pillars are drawn every 16 units from the item's X; splitting a run into
  segments restarts the pattern).
- **Walkers**: only people standing/walking on a floor (lobby crowd, people
  walking to a shaft or stairs). People in rooms are part of the tile art;
  people in cars are the car's passenger count; queued people are `Queues`.
- **Lit** for offices/condos at night (windows yellow), from the sim's
  lighting state.
- **Customers** for fast food / restaurant (shown instead of the eval bar)
  and a `Busy` state for the fast-food rush.
- **Burning/Burned** item states for fire.
- **Event** with `Floor`/`X` set: the UI scrolls there when it pops up.
  `Floor = -100` means no location.
- `CanPlace` is called on every ghost-footprint change while a tool is armed
  (hover), so it must be cheap and side-effect free, and its `Reason` is
  shown on the message line in red.
- Bulldozer drags call `Remove(x, floor)` once per unit across the run;
  `Remove` on an empty cell must return `Result{}` quietly.
- The palette's `BuildDef.W/H` decide the placement footprint; lobby/floor
  footprints come from horizontal drags (min `W`), shafts from vertical drags.

## Frame loop

`App.Run`: a `time.Ticker` at 8 fps. Input events are handled as they arrive
(and the queue drained); on each tick: `model.Step(dt)`, advance the frame
counters, open an event popup if `model.Event()` changed, render the whole
frame into the reused `screen.Screen`, `Flush` the diff. The tile animation
clock `tframe` runs at 4 per second (the mock's `t`) and freezes while paused;
the cursor blink uses the raw frame counter. Render cost at 132x50 is about
55 us and ~40 small allocations per frame (`BenchmarkRender132x50`).

Resize: `term.Event{Resize}` re-lays out (viewport floors = (rows-3)/3,
units = cols-4); below 80x24 a "too small" screen is shown until it grows.
Local tty: SIGWINCH feeds Resize events. Door socket: `Config.ProbeEvery`
(3 s) re-sends the parked-cursor CPR probe; the reply decodes as Resize.

## Tiles

`tiles.go` ports every entry of the mock's `TILES` as a `tileDef`: width
(units, including the wall column), height (3 rows per floor), flags (no
wall: lobby, stairs, escalator; shaft car), the declared states, an animation
period, and a `draw(state, t)` returning glyph rows (authored in Unicode,
exactly as the mock) and paint rows (keys into the palette `pal`, the mock's
`C`). At init every (kind, state, frame 0..23) is drawn once, converted to
CP437 and cached as cells with the mock's blit rules ('.' and blank-on-blank
are transparent; rows padded/clipped to w-1; wall column appended).
`Blit(dst, x, y, kind, state, frame)` copies from the cache. `TBurning` /
`TBurned` are procedural for any footprint. `previewRow` draws one tile row
for the palette.

Glyph substitutions (CP437 has no such glyph, or it would land on a control
byte): `♪` -> `♫` (0x0E), `◙` wheels -> `o`, `▪` -> `■` (0xFE), `★` -> `*`.
`screen.SafeByte` defines what may be emitted: everything except NUL, BEL,
BS, TAB, LF, CR, ESC, DEL and 0xFF (telnet IAC); `TestNoUnsafeBytes` sweeps
every view at four sizes and 30 frames.

## Input model

- Mouse: SGR 1006 click/drag (1000/1002) always on; 1003 (any motion) only
  while a build tool is armed, for the hover ghost. Wheel scrolls floors,
  Shift+wheel units, left-drag (no tool armed) or right-drag pans, right click (no drag) disarms the tool
  or closes the top popup.
- Every click target is a `hotspot` registered while drawing (status row
  elements, hotkey-bar entries, message line, popup buttons, grid cells).
  While a popup is open only its hotspots (plus the status row and footer)
  take clicks.
- Keyboard: Tab cycles viewport / status row / hotkey bar (Left/Right picks
  an element, Enter activates). In the viewport, arrows move the cursor
  (scrolling at edges), PgUp/PgDn 10 floors, Home the lobby, Enter = left
  click, Shift+arrows drag with a tool armed, Esc/Backspace = right click.
  Inside popups Tab/arrows move the blue focus ring, Enter/Space activates,
  Esc closes (not for event prompts, which must be answered). Main hotkeys
  (B I V M F Z L) close the popup and act. `L` opens the message log.

## Deviations from the mock (deliberate)

- Parking cars keep one color per space (the mock cycles it every frame).
- Items are clipped per cell instead of culled by bottom floor, so tall
  rooms partly in view are drawn. The demo moves the metro to B4..B2 so it
  no longer covers the B1 parking spaces (the mock's TOWER overlaps them).
  Other TOWER overlaps (stairs under the 3F security office, the medical
  center over the third shaft) are kept as in the mock.
- Home floors of cars are marked with a red car number on the shaft (from
  the manual), not in the mock.
- Elevator window: columns re-spaced to fit 72 columns, period headers use
  CP437 `½` ("12-12½", "12½-1"); the per-period mode is whole-shaft (as
  mechanics.md says), shown on every served floor's row. No shaft preview /
  SIMULATE button yet.
- Build palette: the selection highlight covers the selected column only;
  the catalog pages (PgUp/PgDn) when it does not fit.
- Status row pause glyph is `||`.

## Open items

- Cinema/party/metro/cathedral "Active" scheduling and all report numbers in
  the demo are placeholders (marked DEMO).
- Find Person, car particulars window, Options menu, sound: not built.
- Ghost legality uses `CanPlace` per footprint change; very long drags call
  it a lot, so the adapter should keep it O(items on those floors).

## Sim adapter (simmodel*.go)

`SimModel` wraps one `*sim.TowerSim` and is the only UI code that imports
`internal/sim`. It reads the live world through the facade (`World()`,
`Ledger()`, `Time()`, `LiveCarriers()`, ...) and changes it only through
`SubmitCommand`, with two exceptions noted below (cost debits and the
express period flags). `internal/sim` is unchanged.

### Coordinates and identity

- Sim grid row `y` (0 = top), internal floor 0..119 with 10 = 1F. UI floor
  `f = simFloor - 10`, `y = 109 - f` (`sim.GroundY - f`). Bounds are
  -10..109 x 375 (the cathedral reaches 104F).
- Item IDs are derived from the anchor cell (`1 + y*1024 + x`; stairs and
  escalators add `1<<20`), so they are stable across frames and saves. Lobby
  runs use the run's left cell. Shaft IDs are adapter-assigned per
  `*CarrierRecord` (the sim renumbers `carrierId` on every rebuild, but keeps
  the record). Person IDs are adapter-assigned per `SimKey` (sims are
  rebuilt on every placement; the key survives).

### Mapping

- Items: every anchor cell of `world.Cells` that is not `floor`; multi-row
  stacks (cinema, party hall, recycling, metro, cathedral) become one item
  with H = 2/2/2/3/5 and their bottom floor (recycling's lower slice is
  skipped). Widths are the sim's (`TileWidths`: party hall 27 against a
  24-unit tile, cathedral 5 slices against a 4-floor tile; the art is drawn
  at its own size inside the footprint). Lobby cells are merged into one item
  per contiguous run per floor. Floor spans are the min..max cell per row.
- States: office vacant when `unitStatus > 0x0f`; condo for sale when
  `> 0x17`; hotel `< 0x18` occupied (asleep 11 PM-6 AM, VIP when `vipFlag`),
  `< 0x28` vacant, `< 0x38` dirty, else infested (the upstream inspector's
  thresholds). Shops/restaurants from the venue sidecar's
  `availabilityState` (dormant retail = For Rent, closed = Closed, near-full
  fast food = Busy). `Customers` = today's visit count. Cinema Active while
  its link is live between ticks 1000 and 1900 (12:45-8:00 PM), party hall
  1200-1600 (1-5 PM). Metro Active when its train flag (`unitStatus 2`) is
  up. Parking Occupied when covered by a ramp, 7 AM-10 PM. Stairs Active
  when a stairs segment entering that floor carries anyone.
- Eval: `evalLevel` 2/1/0 = blue/yellow/red, 0xff = none; shops without a
  level are graded by today's customers (< 25 red, >= 50 blue).
- Lit: offices after 5 PM while any worker is still at work; sold condos
  6 PM-midnight. The sim has no lighting state of its own.
- Fire: rooms overlapping a floor's active front span `[left, right+12)`
  (`FireFronts`, the upstream overlay rule) are Burning. The sim deletes
  burned rooms outright; the adapter keeps any room that vanished while a
  fire/bomb flag was set as a Burned item until something is built over it
  or it is bulldozed (not saved).
- Shafts: one per carrier; cars are the active cars (the index the sim's
  home-floor command uses). Car position = `currentFloor +- settle/6` (the
  upstream client's interpolation rule), interpolated between the last two
  ticks by the step accumulator's fraction, in thirds of a floor. Doors
  open while dwelling; passengers = boarded pending routes on that car.
  Queues come from each floor slot's up/down request rings, front first.
- Stress tiers use the binary's fixed person colors on the live trip
  stress (`CurrentTripStressTicks`): < 80 normal, 80-119 pink, >= 120 red.
- Walkers: sims in transit with an idle route (between legs) stand on their
  current floor: in a lobby they mill across the run, elsewhere near their
  room; sims on a stairs/escalator leg are drawn near their column. Riders
  and queued sims are not walkers. The sim has no horizontal positions, so x
  is synthetic.
- Messages: `route_failure` notifications are red; `event` text as is
  (the cosmetic `metro_train_arrival` is dropped); `star_advanced` adds a
  message and a CONGRATULATIONS popup. Repeats within a day are folded.
- Events: sim prompts (bomb ransom, fire rescue, car-removal confirmation)
  become `Event`s with `Floor`/`X` from `EventState` (bomb/fire floor and
  tile); choice 0 accepts. They queue; one is shown at a time.
- Finance: income lines are the ledger's 10 slots, maintenance the expense
  ledger by family code (lobby = 24, elevators 1/42/43, escalator 27,
  parking ramp 44, recycling 20/21, metro 0x1f-0x21, housekeeping 15,
  security 14). The ledger rolls over every 3 days at 4:59 AM, so the
  window shows the current quarter. Previous balance = the rollover base;
  construction is tracked by the adapter and reset at each rollover.
- Rating: next threshold from `StarThresholds`, needs from the gate flags
  the sim's `CheckStarAdvancementConditions` tests.

### Commands

- Costs: the sim runs with `FreeBuild` and the adapter charges the verified
  catalogue (`sim.CatalogCosts`, `CatalogCarCosts`, movie costs) by debiting
  the ledger after an accepted command, plus $500 for every bare-air segment
  a room or lobby covers (mechanics.md section 1, auto-floor; shafts pay it
  for the cells they auto-floor). Extending a shaft costs
  `CarrierExtendFloorCost` per floor. Star unlocks follow mechanics.md (twin
  and suite at 3 stars).
- `CanPlace` is a read-only port of `HandlePlaceTile`'s validation (bounds,
  underground list, lobby cadence, occupancy, support, metro limits,
  parking-ramp stacking, recycling adjacency, cinema/party/cathedral/metro
  stacks, every elevator rule including spacing, alignment, mode and the
  31-floor span, stairs/escalator base and alignment) plus star and funds
  checks; it allocates a little and never mutates. Door-only rules from
  mechanics.md: parking and recycling are underground only. Lobby/floor
  drags fill what is missing and skip built cells; shaft drags skip floors
  the shaft already has and validate later floors against the earlier ones.
- `Place` submits one `place_tile` per unit (lobby, floor), per floor
  (shafts, bottom up) or once (rooms; y = lower row for 2-floor rooms, top
  row for metro and cathedral).
- `Remove`: a burned area is cleared; an elevator overlay removes the whole
  shaft (one `remove_tile` per floor); stairs/escalators and rooms are
  removed; floor and empty cells are quiet; lobby, housekeeping, security,
  metro and cathedral refuse (mechanics.md "Removable: No").
- Elevator window: serve toggles (`toggle_elevator_floor_stop`; refused on
  express shafts and on a car's home floor), home floor, add/remove car,
  Waiting Car Response (`waitingCarResponseThreshold`), Departure (the sim's
  dwell multiplier 0..5 x 30 ticks, shown as 10 s steps). The period modes
  have no sim command: the adapter writes the player's choice into the
  carrier's `expressDirectionFlags[weekend*7 + daypart]`, the byte the sim
  reads at terminal floors.
- Rent (`set_rent_level`; tiers from YEN #1001), movie
  (`set_cinema_movie_pool`), prompt answers (`prompt_response`).
  `RenameTenant` names are adapter state (max 20).

### Time

The upstream tower-together server ticks the sim every 50 ms at 1x
(`TowerRoom.startTick`: `setInterval(tick, 50 / speedMultiplier)`), and the
lockstep client uses the same `BASE_TICK_INTERVAL_MS = 50` with 1x/3x/10x.
The door uses that cadence: **20 ticks per real second** at normal speed, a
game day (2600 ticks) in 2 min 10 s, a quarter in 6.5 min, a year in 26 min.
The tick rate is constant; the manual's "clock runs slower at rush hours" is
the binary's non-linear tick -> clock table (`ClockOfTick`: 0.75 min/tick
in the morning, 0.075 over lunch, 0.6 afternoon/evening, 1.8 at night).
Fast = **10x** (200 ticks/s, 25 per frame; upstream's top multiplier, and
the original's Fast Mode runs as fast as the machine allows). `Step(dt)`
accumulates `dt * rate` and runs the whole ticks; it never steps while an
event prompt is open, drops any backlog beyond one second of fast mode, and
stops early if a frame's ticks take over 70 ms. Pause = no steps. A fast
frame on a 1,500-sim tower costs about 7.5 ms
(`BenchmarkSimModelFastFrame`).

### Saves

`<data>/<user>.json` is `json.Marshal(sim.SaveState())` (the snapshot);
`<data>/<user>.ui.json` holds what the snapshot does not: custom names,
construction this quarter, open/queued prompts (the snapshot drops pending
prompts), the last 50 messages and the speed. Both are written atomically
(temp file in the same directory + rename). Saves happen on quit and
disconnect, after New tower, and at every in-game quarter change (midnight
starting a quarter). New towers use lobby mode `perfect-parity` (1F, then
15F, 30F, ... in 1-based floors) and the original's $2,000,000.

### Start menu

`Config.Launcher` opens `startView` over the splash: Continue (only with a
readable save; focus starts there), New tower (asks before replacing a
save), Demo tower, Quit, by click, Tab/arrows + Enter, or C/N/D/Q. The
backdrop model does not step under it. `startView` and `eventView` are
`modalView`s: right click, Esc and the main hotkeys no longer close them and
the status row/footer under them are inert (before this, a right click or a
hotkey could dismiss a fire/bomb prompt and leave the sim waiting forever).

Also fixed while wiring: `leftRelease` computed the footprint after clearing
`dragging`, so every lobby/floor/shaft drag placed only the hover footprint.

### Not surfaced (no sim state for it)

Rain; VIP visits (no prompt/notification; only `vipFlag` rooms show VIP);
treasure, Santa and wedding events; occupancy "Length"; horizontal
positions of people and the column of a stairs segment; a burned-area
record (handled by the adapter as above).
