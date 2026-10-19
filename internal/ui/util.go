package ui

import "strconv"

func itoa(n int) string { return strconv.Itoa(n) }

func pad2(n int) string {
	if n < 10 && n >= 0 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// money formats dollars like the info bar: $1,732,500 / -$40,000.
func money(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	out := make([]byte, 0, len(s)+len(s)/3+2)
	if neg {
		out = append(out, '-')
	}
	out = append(out, '$')
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// commas formats a plain count with thousands separators.
func commas(n int) string { return money(int64(n))[1:] }

// floorLabel: 0 -> "1F", 11 -> "12F", -1 -> "B1".
func floorLabel(f int) string {
	if f >= 0 {
		return strconv.Itoa(f+1) + "F"
	}
	return "B" + strconv.Itoa(-f)
}

func padLeft(s string, n int) string {
	for len(s) < n {
		s = " " + s
	}
	return s
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

func clip(s string, n int) string {
	if n < 0 {
		return ""
	}
	if len(s) > n {
		return s[:n]
	}
	return s
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
