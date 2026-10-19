package sim

// Port of reference/tower-together/sim/route-scoring/delay-table.ts.
//
// 1288:e62c g_per_stop_even_parity_delay
// 1288:e62e g_per_stop_odd_parity_delay
//
// Indexed by segment.flags & 1 (bit 0 = stairs-cost parity). Values
// confirmed by emulator capture: even (escalator) = 16, odd (stairs) = 35.

// PerStopParityDelay TS: route-scoring/delay-table.ts perStopParityDelay
var PerStopParityDelay = [2]int{16, 35}
