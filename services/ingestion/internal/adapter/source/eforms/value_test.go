package eforms

import "testing"

func TestParseMinorUnits(t *testing.T) {
	cases := []struct {
		in   string
		want *int64
	}{
		{"22134549.01", int64Ptr(2213454901)},
		{"17694115.2", int64Ptr(1769411520)}, // single fractional digit
		{"125000000", int64Ptr(12500000000)}, // no fractional part
		{"", nil},                            // field absent
		{"not-a-number", nil},                // malformed input is "unknown", not a crash
	}
	for _, c := range cases {
		got := parseMinorUnits(c.in)
		if (got == nil) != (c.want == nil) {
			t.Errorf("parseMinorUnits(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		if got != nil && *got != *c.want {
			t.Errorf("parseMinorUnits(%q) = %d, want %d", c.in, *got, *c.want)
		}
	}
}

func int64Ptr(v int64) *int64 { return &v }
