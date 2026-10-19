package sim

// Phase A2 helpers for JS idioms in the transport files (carriers/, queue/,
// reachability/, route-scoring/). Not a port of a TS file. Prefixed "tr"
// (transport) so they cannot collide with helpers other phases add.

// trAt is the JS `arr[i] ?? def` for an int array: def when i is out of
// range (JS reads undefined past either end).
func trAt(s []int, i, def int) int {
	if i < 0 || i >= len(s) {
		return def
	}
	return s[i]
}

// trSet is the JS `arr[i] = v` for an int array. Writing past the end grows
// the array (JS leaves holes; the holes are later read through `?? 0` or
// re-filled, so zero-padding is equivalent). Negative i is a no-op in
// effect (JS would create a "-1" property nobody reads).
func trSet(s *[]int, i, v int) {
	if i < 0 {
		return
	}
	for len(*s) <= i {
		*s = append(*s, 0)
	}
	(*s)[i] = v
}

// trBit is the JS `1 << n`: the shift count is taken mod 32 and the result
// is a signed 32-bit integer (1 << 31 is negative). Every mask built from
// trBit values with |, & and ^ stays a sign-extended int32, matching the
// JS bitwise results.
func trBit(n int) int {
	return int(int32(uint32(1) << (uint(n) & 31)))
}

// trFill is the JS `arr.fill(v)` (in place, keeps the length).
func trFill(s []int, v int) {
	for i := range s {
		s[i] = v
	}
}

// trBoolPtr returns &v (for optional TS boolean options).
func trBoolPtr(v bool) *bool { return &v }
