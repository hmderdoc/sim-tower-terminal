package sim

import "testing"

// Phase B2 test helpers (no TS counterpart).

// b2World returns a fresh perfect-parity world without hydration.
func b2World(t *testing.T) *WorldState {
	t.Helper()
	return newBareSim(LobbyModePerfectParity).World()
}

// b2PlaceObject registers a placed object anchored at (x, floor).
func b2PlaceObject(w *WorldState, x, floor int, rec *PlacedObjectRecord) string {
	key := cellKey(x, FloorToY(floor))
	w.PlacedObjects.Set(key, rec)
	return key
}

// b2Sim builds a sim owned by the object anchored at (homeColumn, floor).
func b2Sim(floor, homeColumn, family, state int) *SimRecord {
	return &SimRecord{
		FloorAnchor: floor, HomeColumn: homeColumn, FamilyCode: family, StateCode: state,
		Route: RouteIdle, SelectedFloor: floor, DestinationFloor: -1, CommercialVenueSlot: -1,
	}
}

// b2SeedFor returns an RNG state whose next SampleRng() % mod == want.
func b2SeedFor(t *testing.T, mod, want int) int32 {
	t.Helper()
	for seed := int32(1); seed < 1_000_000; seed++ {
		next := int32(uint32(seed)*0x15a4e35 + 1)
		if int((uint32(next)>>16)&0x7fff)%mod == want {
			return seed
		}
	}
	t.Fatalf("no seed for %d mod %d", want, mod)
	return 0
}
