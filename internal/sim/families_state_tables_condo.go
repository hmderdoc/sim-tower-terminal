package sim

// Port of reference/tower-together/sim/families/state-tables/condo.ts.
// Binary table cs:1c2d. The handler table (CondoRefreshHandlerTable) lives
// in sims_condo.go.

// CondoPrologueTable TS: CONDO_PROLOGUE_TABLE (cs:1c2d). Lookup-only.
var CondoPrologueTable = map[int]string{
	0x40: "1228:1aba",
	0x41: "1228:1aba",
	0x60: "1228:1aba",
	0x61: "1228:1aba",
	0x62: "1228:1aba",
}
