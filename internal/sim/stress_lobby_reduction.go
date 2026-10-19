package sim

// Port of reference/tower-together/sim/stress/lobby-reduction.ts.

// ReduceElapsedForLobbyBoarding discounts accumulated elapsed when boarding
// from the lobby floor, keyed on lobby height (2: -25, >=3: -50, min 0).
// The TS takes a structural `{ elapsedTicks }` object (sims and ad-hoc
// records alike), so the Go port takes a pointer to that field: callers
// pass &sim.ElapsedTicks.
// TS: stress/lobby-reduction.ts reduceElapsedForLobbyBoarding (binary 11e0:0423)
func ReduceElapsedForLobbyBoarding(elapsedTicks *int, sourceFloor int, world *WorldState) {
	if sourceFloor != LobbyFloor {
		return
	}
	lobbyHeight := max(1, world.LobbyHeight)
	discount := 0
	if lobbyHeight >= 3 {
		discount = 50
	} else if lobbyHeight == 2 {
		discount = 25
	}
	if discount == 0 {
		return
	}
	*elapsedTicks = max(0, *elapsedTicks-discount)
}
