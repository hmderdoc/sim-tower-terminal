package sim

// Port of reference/tower-together/sim/families/state-tables/office.ts.
// Binary tables cs:2005 (refresh), cs:2aac (dispatch), cs:1c51
// (dispatch_sim_behavior family-7 branch). The handler table itself
// (OfficeRefreshHandlerTable) lives in sims_office.go.

// OfficePrologueTable TS: OFFICE_PROLOGUE_TABLE (cs:1c51). Documentation
// table: state code -> binary prologue handler address. Lookup-only.
var OfficePrologueTable = map[int]string{
	0x40: "1228:1989",
	0x41: "1228:1989",
	0x42: "1228:1989",
	0x45: "1228:193d",
	0x60: "1228:193d",
	0x61: "1228:193d",
	0x62: "1228:193d",
	0x63: "1228:193d",
}
