package eforms

import (
	"strconv"
	"strings"
)

// parseMinorUnits converts a decimal string (e.g. "22134549.01",
// "17694115.2", "125000000" — verified live to have inconsistent
// fractional-digit counts) into minor units (cents), without the
// precision loss float64 multiplication would introduce. Returns nil for
// an empty or malformed string — a bad/absent value is "unknown", not a
// fatal error for the whole notice.
func parseMinorUnits(s string) *int64 {
	if s == "" {
		return nil
	}
	whole, frac, _ := strings.Cut(s, ".")
	switch {
	case len(frac) == 0:
		frac = "00"
	case len(frac) == 1:
		frac += "0"
	case len(frac) > 2:
		frac = frac[:2]
	}
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
