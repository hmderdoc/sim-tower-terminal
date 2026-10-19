package sim

// Port of reference/tower-together/sim/families/state-tables/hotel.ts.
// Binary table cs:1c41. The handler table (HotelRefreshHandlerTable) lives
// in sims_hotel.go.

// HotelPrologueTable TS: HOTEL_PROLOGUE_TABLE (cs:1c41). Lookup-only;
// everything not listed maps to HotelPrologueDefault.
var HotelPrologueTable = map[int]string{
	0x41: "1228:1a4f",
	0x45: "1228:19f4",
	0x60: "1228:1a4f",
	0x62: "1228:1a4f",
}

// HotelPrologueDefault TS: HOTEL_PROLOGUE_DEFAULT
const HotelPrologueDefault = "1228:1c24"
