package tender

import (
	"testing"
	"time"
)

func i64(v int64) *int64 { return &v }

func TestValueFit(t *testing.T) {
	cases := []struct {
		name            string
		value, min, max *int64
		want            string
	}{
		{"no value", nil, i64(100), i64(200), "unknown"},
		{"no band at all", i64(150), nil, nil, "unknown"},
		{"below min", i64(50), i64(100), i64(200), "below"},
		{"above max", i64(250), i64(100), i64(200), "above"},
		{"in band", i64(150), i64(100), i64(200), "in_band"},
		{"at min boundary", i64(100), i64(100), i64(200), "in_band"},
		{"at max boundary", i64(200), i64(100), i64(200), "in_band"},
		{"only min set, above it", i64(150), i64(100), nil, "in_band"},
		{"only min set, below it", i64(50), i64(100), nil, "below"},
		{"only max set, below it", i64(150), nil, i64(200), "in_band"},
		{"only max set, above it", i64(250), nil, i64(200), "above"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := valueFit(tc.value, tc.min, tc.max); got != tc.want {
				t.Fatalf("valueFit(%v,%v,%v) = %q, want %q", tc.value, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

func TestDeadlineDays(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	if got := deadlineDays(nil, now); got != nil {
		t.Fatalf("deadlineDays(nil) = %v, want nil", got)
	}

	past := now.Add(-24 * time.Hour)
	if got := deadlineDays(&past, now); got != nil {
		t.Fatalf("deadlineDays(past) = %v, want nil (already closed)", got)
	}

	in10Days := now.Add(10*24*time.Hour + time.Hour) // a hair over 10 whole days
	got := deadlineDays(&in10Days, now)
	if got == nil || *got != 10 {
		t.Fatalf("deadlineDays(+10d1h) = %v, want 10", got)
	}
}

func TestMatchesAnyPrefix(t *testing.T) {
	if !matchesAnyPrefix("45210000", []string{"45"}) {
		t.Fatal("want a match: 45210000 has prefix 45")
	}
	if matchesAnyPrefix("72000000", []string{"45", "80"}) {
		t.Fatal("want no match: 72000000 matches neither prefix")
	}
	if matchesAnyPrefix("45210000", nil) {
		t.Fatal("want no match against an empty prefix list")
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"open", "restricted"}, "open") {
		t.Fatal("want a match: list contains open")
	}
	if containsString([]string{"open", "restricted"}, "negotiated") {
		t.Fatal("want no match: list does not contain negotiated")
	}
	if containsString(nil, "open") {
		t.Fatal("want no match against a nil list")
	}
}

func TestComputeFitTier(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	cases := []struct {
		name      string
		relevance float64
		reason    ReasonSignals
		want      FitTier
	}{
		{"high relevance, in band, no deadline", 0.9, ReasonSignals{ValueFit: "in_band"}, FitStrong},
		{"high relevance, unknown value, no deadline", 0.9, ReasonSignals{ValueFit: "unknown"}, FitStrong},
		{"high relevance but value below band", 0.9, ReasonSignals{ValueFit: "below"}, FitLongShot},
		{"high relevance but value above band", 0.9, ReasonSignals{ValueFit: "above"}, FitLongShot},
		{"high relevance but deadline too soon for strong", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(8)}, FitPossible},
		{"high relevance but deadline urgent (long-shot floor)", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(3)}, FitLongShot},
		{"high relevance, deadline exactly at MinDeadlineDays", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(10)}, FitStrong},
		{"mid relevance, in band", 0.6, ReasonSignals{ValueFit: "in_band"}, FitPossible},
		{"low relevance", 0.2, ReasonSignals{ValueFit: "in_band"}, FitLongShot},
		{"relevance exactly at RelevanceLow", 0.4, ReasonSignals{ValueFit: "in_band"}, FitPossible},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeFitTier(tc.relevance, tc.reason, cfg); got != tc.want {
				t.Fatalf("computeFitTier(%v, %+v) = %q, want %q", tc.relevance, tc.reason, got, tc.want)
			}
		})
	}
}

// TestComputeFitTierIgnoresRegionAndProcedureMatch is the delta's required
// invariance check: RegionMatch/ProcedureMatch are tie-breakers and reason
// enrichment only (see the design spec's honesty guardrail) — they must
// never move the tier. This proves it by holding relevance/ValueFit/
// DeadlineDays fixed across every combination of the two new booleans, at
// three different relevance bands (strong / possible / long_shot), and
// asserting the tier never changes.
func TestComputeFitTierIgnoresRegionAndProcedureMatch(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	base := ReasonSignals{ValueFit: "in_band", DeadlineDays: days(20)}
	regionProcedureCombos := []struct {
		region, procedure bool
	}{
		{false, false},
		{true, false},
		{false, true},
		{true, true},
	}

	for _, relevance := range []float64{0.9, 0.6, 0.2} {
		want := computeFitTier(relevance, base, cfg)
		for _, combo := range regionProcedureCombos {
			variant := base
			variant.RegionMatch = combo.region
			variant.ProcedureMatch = combo.procedure
			if got := computeFitTier(relevance, variant, cfg); got != want {
				t.Fatalf("computeFitTier(%v, %+v) = %q, want %q (relevance=%v baseline) — RegionMatch/ProcedureMatch must not move the tier",
					relevance, variant, got, want, relevance)
			}
		}
	}
}

func TestComputeReasonSignals(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(5 * 24 * time.Hour)
	// NOTE: no CPVSecondary field here — Tender does not have one (Task A0
	// confirmed CPVSecondary is unreachable through the Postgres access
	// layer and was never added to the struct). SectorMatch below is
	// primary-CPV only.
	tn := Tender{
		CPV:           "45210000",
		Country:       "ITA",
		NUTS:          "ITC4",
		ProcedureType: "open",
		Value:         i64(150),
		Deadline:      &deadline,
	}

	got := computeReasonSignals(tn, []string{"45"}, []string{"ITA"}, []string{"ITC"}, []string{"open"}, i64(100), i64(200), now)
	if !got.SectorMatch {
		t.Fatal("SectorMatch = false, want true")
	}
	if !got.CountryMatch {
		t.Fatal("CountryMatch = false, want true")
	}
	if !got.RegionMatch {
		t.Fatal("RegionMatch = false, want true")
	}
	if !got.ProcedureMatch {
		t.Fatal("ProcedureMatch = false, want true")
	}
	if got.ValueFit != "in_band" {
		t.Fatalf("ValueFit = %q, want in_band", got.ValueFit)
	}
	if got.DeadlineDays == nil || *got.DeadlineDays != 5 {
		t.Fatalf("DeadlineDays = %v, want 5", got.DeadlineDays)
	}
}

// TestComputeReasonSignalsRegionAndProcedureMatch is the delta's required
// table coverage for the two new signals: an empty regions/procedureTypes
// list is an honest "not claimed" (never a penalty, never a false match),
// and a non-empty list matches on NUTS prefix / exact ProcedureType.
func TestComputeReasonSignalsRegionAndProcedureMatch(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	tn := Tender{CPV: "45210000", Country: "ITA", NUTS: "ITC4", ProcedureType: "open"}

	cases := []struct {
		name           string
		regions        []string
		procedureTypes []string
		wantRegion     bool
		wantProcedure  bool
	}{
		{"nothing claimed by the client", nil, nil, false, false},
		{"region prefix matches", []string{"ITC"}, nil, true, false},
		{"region prefix does not match", []string{"FRB"}, nil, false, false},
		{"procedure type matches, no region claimed", nil, []string{"open"}, false, true},
		{"procedure type claimed but does not match", nil, []string{"restricted"}, false, false},
		{"region and procedure both match", []string{"ITC"}, []string{"open"}, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeReasonSignals(tn, nil, nil, tc.regions, tc.procedureTypes, nil, nil, now)
			if got.RegionMatch != tc.wantRegion {
				t.Fatalf("RegionMatch = %v, want %v", got.RegionMatch, tc.wantRegion)
			}
			if got.ProcedureMatch != tc.wantProcedure {
				t.Fatalf("ProcedureMatch = %v, want %v", got.ProcedureMatch, tc.wantProcedure)
			}
		})
	}
}
