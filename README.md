# SimTower door

A faithful SimTower (Maxis / OPeNBooK, 1994) for BBS terminals, written in Go
as a Synchronet external program (DOOR32.SYS over the inherited socket; also
runs on a local tty). The simulation is a port of tower-together's
reverse-engineered, trace-verified SimTower sim (see `docs/PORT.md` and
`reference/README.md`); the presentation is CP437 at one column per unit and
three rows per floor (`docs/graphics.md`, approved mock in
`docs/graphics_plan.html`).

## Build and run

    go build -o simtower ./cmd/simtower
    ./simtower                 # local play on this tty (probes the size)
    ./simtower -demo           # the graphics demo tower, no simulation
    ./simtower -dropfile /sbbs/node1/DOOR32.SYS   # how the BBS runs it

Flags: `-size 132x50` forces a terminal size, `-user NAME` overrides the
player name, `-data DIR` moves the save directory, `-log FILE` the debug log.

Saves are one JSON file per player in `data/`.

## Synchronet

`install-xtrn.ini` registers the door (`jsexec install-xtrn ../xtrn/sim_tower`)
or add the `[prog:GAMES:SIMTOWER]` section to `ctrl/xtrn.ini` by hand.

## Layout

    cmd/simtower     entry: flags, dropfile, terminal probe, event loop
    internal/term    Term (socket/stdio), DOOR32 parsing, key + SGR mouse decoder
    internal/screen  CP437 cell buffer with diff flush
    internal/sim     the simulation (port of reference/tower-together/sim)
    internal/ui      viewport, tiles, chrome, popups, input
    docs/            mechanics spec, port notes, graphics spec + mock
    reference/       upstream sim source, specs and trace fixtures (gitignored)
