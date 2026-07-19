package tender

import "testing"

func TestEUThreshold(t *testing.T) {
	cfg := EUThreshold{WorksMinor: 540400000, SuppliesCentralMinor: 14000000, SuppliesSubCentralMinor: 21600000}
	p := func(v int64) *int64 { return &v }
	cases := []struct {
		name, cpv string
		value     *int64
		want      string
	}{
		{"nil value", "45000000", nil, ""},
		{"works below", "45210000", p(100000_00), "below_eu"},                // €100k < €5.404M
		{"works above", "45210000", p(6000000_00), "above_eu"},               // €6M > €5.404M
		{"works exactly on the line", "45210000", p(540400000), ""},          // == works threshold → no claim
		{"supplies below central", "72000000", p(100000_00), "below_eu"},     // €100k < €140k
		{"supplies above sub-central", "72000000", p(300000_00), "above_eu"}, // €300k > €216k
		{"supplies ambiguous band", "72000000", p(180000_00), ""},            // €180k between €140k and €216k → NO badge
		{"supplies at central boundary", "72000000", p(14000000), ""},        // == central: not < lower → ambiguous, no badge
		{"supplies at sub-central boundary", "72000000", p(21600000), ""},    // == upper: not > upper → ambiguous, no badge
		{"empty cpv falls to supplies", "", p(100000_00), "below_eu"},
	}
	for _, c := range cases {
		if got := euThreshold(c.value, c.cpv, cfg); got != c.want {
			t.Fatalf("%s: euThreshold(%v,%q)=%q want %q", c.name, c.value, c.cpv, got, c.want)
		}
	}
}
