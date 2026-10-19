# reference/

`tower-together/` is a read-only copy of https://github.com/phulin/tower-together
(MIT, Patrick Hulin): `specs/` (reverse-engineered notes on SIMTOWER.EXE),
`sim/` (the headless TypeScript simulation, trace-verified against the original
binary) and `sim/fixtures/` (JSON build specs + JSONL per-tick reference traces
captured from the original binary under emulation). The Go simulation in
`internal/sim` is a port of `sim/`, and `internal/sim/trace_test.go` replays the
fixtures. The copy is gitignored for size; `COMMIT` records the upstream commit.
Re-fetch with:

    git clone --depth 1 https://github.com/phulin/tower-together /tmp/tt
    mkdir -p reference/tower-together/sim
    cp -r /tmp/tt/specs /tmp/tt/LICENSE.md /tmp/tt/README.md reference/tower-together/
    cp -r /tmp/tt/apps/worker/src/sim/. reference/tower-together/sim/
