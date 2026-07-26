package tender

import "testing"

func TestComputeFitTierFromSignals(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	cases := []struct {
		name   string
		reason ReasonSignals
		want   FitTier
	}{
		{"sector+country, in_band, no deadline -> strong", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "in_band"}, FitStrong},
		{"sector+country, unknown value, no deadline -> strong", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "unknown"}, FitStrong},
		{"sector+country, deadline exactly MinDeadlineDays -> strong", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "in_band", DeadlineDays: days(10)}, FitStrong},
		{"sector+country, deadline too soon for strong -> possible", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "in_band", DeadlineDays: days(8)}, FitPossible},
		{"sector+country but value below -> long_shot", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "below"}, FitLongShot},
		{"sector+country but value above -> long_shot", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "above"}, FitLongShot},
		{"sector+country but urgent deadline -> long_shot", ReasonSignals{SectorMatch: true, CountryMatch: true, ValueFit: "in_band", DeadlineDays: days(3)}, FitLongShot},
		{"sector only -> possible", ReasonSignals{SectorMatch: true, ValueFit: "in_band"}, FitPossible},
		{"country only -> possible", ReasonSignals{CountryMatch: true, ValueFit: "in_band"}, FitPossible},
		{"neither sector nor country -> long_shot", ReasonSignals{ValueFit: "in_band"}, FitLongShot},
		{"neither, unknown value -> long_shot", ReasonSignals{ValueFit: "unknown"}, FitLongShot},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeFitTierFromSignals(tc.reason, cfg); got != tc.want {
				t.Fatalf("computeFitTierFromSignals(%+v) = %q, want %q", tc.reason, got, tc.want)
			}
		})
	}
}

// TestComputeFitTierFromSignalsIgnoresRegionAndProcedure mirrors the search
// classifier's honesty guardrail: RegionMatch/ProcedureMatch are enrichment
// only and must never move the tier. Holds the deciding signals fixed at a
// strong / possible / long_shot baseline and varies both booleans.
func TestComputeFitTierFromSignalsIgnoresRegionAndProcedure(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	bases := []ReasonSignals{
		{SectorMatch: true, CountryMatch: true, ValueFit: "in_band", DeadlineDays: days(20)}, // strong
		{SectorMatch: true, ValueFit: "in_band"},                                             // possible
		{ValueFit: "below"},                                                                  // long_shot
	}
	combos := []struct{ region, procedure bool }{{false, false}, {true, false}, {false, true}, {true, true}}

	for _, base := range bases {
		want := computeFitTierFromSignals(base, cfg)
		for _, c := range combos {
			v := base
			v.RegionMatch = c.region
			v.ProcedureMatch = c.procedure
			if got := computeFitTierFromSignals(v, cfg); got != want {
				t.Fatalf("computeFitTierFromSignals(%+v) = %q, want %q — Region/ProcedureMatch must not move the tier", v, got, want)
			}
		}
	}
}
