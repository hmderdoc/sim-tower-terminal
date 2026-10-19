package sim

import (
	"strconv"
	"strings"
)

// Shared helpers for JS idioms that recur across the TS sources. Not a port
// of a TS file; use these instead of re-declaring them per file.
//
// Integer division reminder (PORT.md rule 2): Go `/` and `%` truncate
// toward zero exactly like Math.trunc(a / b) and JS `%`. Only Math.floor on
// a possibly-negative quotient needs floorDiv.

// floorDiv is Math.floor(a / b) for integers (b != 0).
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// absInt is Math.abs for ints.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// cellKey formats the TS `${x},${y}` cell / placed-object key.
func cellKey(x, y int) string {
	return strconv.Itoa(x) + "," + strconv.Itoa(y)
}

// parseKeyXY is the TS `key.split(",").map(Number)` for an "x,y" key.
func parseKeyXY(key string) (int, int) {
	xs, ys, _ := strings.Cut(key, ",")
	x, _ := strconv.Atoi(xs)
	y, _ := strconv.Atoi(ys)
	return x, y
}

// intPtr returns &v (for optional TS number fields).
func intPtr(v int) *int { return &v }

// filledInts is `new Array(n).fill(v)`.
func filledInts(n, v int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = v
	}
	return out
}
