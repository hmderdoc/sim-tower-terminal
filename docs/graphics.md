# Graphics and interaction spec (approved 2026-10-19)

The approved mock is `docs/graphics_plan.html` (open it in a browser; the
`TILES` object and `scene()` function in its script are the reference for
every tile and for the viewport composition). This file states the rules the
Go renderer in `internal/ui` must follow. Where this file and the mock
disagree, the mock wins.

## Geometry

- One terminal column = one SimTower unit. **Three rows = one floor.**
- Viewport = everything between the status row (row 0) and the two footer
  rows (message line, hotkey bar). A 4-column gutter on the left carries the
  floor label (`12F`, `1F`, `B3`) on the middle row of each floor; `1F` is
  bright yellow, others dark grey.
- Responsive: 80x24 is the minimum (7 floors x 76 units). The door probes
  the terminal (parked-cursor CPR) and uses the whole size; on a live resize
  report it re-lays out. Nothing scales; only the visible floor count and
  unit count change.
- Sky above the top built floor: bg blue by day, magenta at dusk, black with
  dots for stars and a 2x2 block moon at night, dark grey with falling `/`
  in rain. Clouds are `░▒▒░` / `░░` drifting slowly. Ground level outside the
  tower footprint: a row of green `▓` grass. Underground outside built floors:
  brown `░` dirt pattern.
- Built floor area is black background. Each room ends in a dark grey `║`
  wall column. Shafts: `│` walls (standard/service) or `║` (express), dark
  grey, with `·` on the middle row for cable; a car is a 3-row block (`▄▄` /
  passengers or `▐▌` doors / `▀▀`) that moves in thirds of a floor;
  passenger count is drawn as digits on the car's middle row while moving.
  Express cars are 4 wide inside a 6-wide shaft.
- People: two cells tall (`☺` head on the middle row, `╨`/`∩` alternating
  legs on the bottom row) in the lobby and in elevator queues; one cell `☺`
  inside cars and rooms. Queues form on the LEFT of a shaft for up, RIGHT for
  down (manual p.47). Stress color: normal (black on the amber lobby, bright
  white elsewhere), pink, red.
- Lobby: amber (bg brown 6) band three rows tall, white `║` pillar every 16
  units, yellow `☼` lamps on the top row, a black reception desk `▬▬▬` /
  `▀▀▀` at the centre, green `♣` plants. Sky lobbies identical.
- Tiles: see the mock's `TILES`. Port them as data where static (rows of
  glyphs + paint map keyed through the shared palette table `C`) and as small
  functions where a state needs the sim (customer counts, time of day).
  Animated tiles: escalator (running), stairs (walker), cinema (screen
  flicker during a show), party hall (dancers), hotel (rising z), metro
  (train), infested room (blinking dots), fire (red/yellow flicker).

## Chrome

- **Status row (row 0, bg blue):** funds, population, stars (`★` x n, or
  `TOWER`), `Y1 Q2 WD1 10:42 AM`, speed glyphs, `[BUILD]` and `[MAP]`
  buttons. Every element is a click target: funds opens Finance, population
  opens the rating report, speed cycles pause/normal/fast, the buttons open
  their panels.
- **Message line (second to last row):** the original's info-bar messages
  (build errors, "Office workers demand Parking", route failures, income
  notices), yellow on black, newest message; click to open the message log.
- **Hotkey bar (last row, black on light grey):** `B)uild I)nspect V)eval
  M)ap F)inance Z)oom  Space pause  Arrows/Wheel scroll  Q)uit`. **Each
  entry is a click target** that performs the same action as the key.
- Popups are double-line boxes with a title tab, `[BUTTONS]` drawn as black
  on light grey; every button is clickable and has a keyboard focus ring (bg
  blue) moved with Tab/arrows and activated with Enter/Space.

## Input model

The whole game must be playable with only the mouse and with only the
keyboard.

Mouse (SGR 1006, press/release/drag, wheel):
- Wheel scrolls floors; Shift+wheel scrolls units; left-drag (no tool) or right-drag pans.
- Left click on the tower with no tool: inspect (facility / shaft / tenant
  window). With a build tool armed: place; the tool stays armed; right click
  or Esc disarms. Drag with the elevator tool sets shaft height; drag with
  lobby/floor extends; drag with bulldoze clears a run.
- Hover with a tool armed shows the item outline as a ghost: bright white
  where legal, red where illegal with the reason on the message line.

Keyboard:
- Arrows move a cursor cell (blinking bright block) inside the viewport;
  the viewport scrolls when the cursor reaches an edge. PgUp/PgDn move ten
  floors, Home returns to the lobby. Enter = left click at the cursor,
  Shift+arrows = drag, Backspace/Esc = right click.
- Hotkeys as on the bar; each build item has a letter shown in the palette.
- Tab cycles focus between viewport, status row, hotkey bar; inside popups
  it cycles the controls.

## Views

- Build palette (B): two columns of items with hotkey letter, name, cost,
  star requirement, and a one-row live preview of the tile; locked items
  grey; selected row highlighted blue; footer describes the selected item.
- Inspect: facility window (type + floor, leasing status, eval bar
  blue/yellow/red or customer count, occupants, length, rent/price dial with
  the 4 tiers, message), elevator window (floor column with serve toggles and
  car home floors, WD/WE x 7 periods grid with Local / Express Up / Express
  Down, Waiting Car Response and Standard Floor Departure dials, ADD CAR /
  REMOVE CAR), tenant window (name, from, going to, stress, rename).
- Eval overlay (V): recolor room fills blue/yellow/red/grey per evaluation;
  second press shows pricing colors; third shows hotel dirty rooms; fourth
  off.
- Map (M): whole tower at one cell per 5 units x 5 floors, elevators as
  vertical lines (express blue, standard grey, service red), viewport
  rectangle, click to jump.
- Zoom (Z): one-row-per-floor overview of the tower (the first mock's
  renderer) for navigation; Z again returns.
- Finance (F): the original's report, 10 income + 10 maintenance lines,
  other income, construction, quarter balance, total.
- Events: fire, bomb, VIP, treasure, Santa, wedding as popups with the
  original's choices; burning cells flicker red/yellow and spread.
