package sim

// Port of reference/tower-together/sim/families/state-tables/family-prologue.ts.
// Binary table cs:1c71: dispatch_sim_behavior family-prologue jump table,
// indexed by (family_code - 3).

// FamilyPrologueTable TS: FAMILY_PROLOGUE_TABLE (family_code - 3 -> binary
// prologue handler address). Lookup-only (membership gates dispatchSimBehavior).
var FamilyPrologueTable = map[int]string{
	0:  "1228:19d4", // family 3 - hotel single
	1:  "1228:19d4", // family 4 - hotel twin
	2:  "1228:19d4", // family 5 - hotel suite
	3:  "1228:1b05", // family 6 - restaurant
	4:  "1228:191d", // family 7 - office
	6:  "1228:1a9a", // family 9 - condo
	7:  "1228:1b05", // family 10 - retail
	9:  "1228:1b05", // family 12 - fast food
	12: "1228:1bd8", // family 15 - housekeeping
	15: "1228:1bb0", // family 18 - entertainment guest
	26: "1228:1bb0", // family 29 - entertainment variant
	30: "1228:1bb0", // family 33 - recycling center lower
}
