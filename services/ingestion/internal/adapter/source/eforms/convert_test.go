package eforms

import "testing"

func TestAlpha3ToAlpha2(t *testing.T) {
	cases := map[string]string{
		"ROU": "RO",
		"DEU": "DE",
		"ITA": "IT",
		"IRL": "IE",
		"XYZ": "", // unrecognized code
		"":    "",
	}
	for in, want := range cases {
		if got := alpha3ToAlpha2(in); got != want {
			t.Errorf("alpha3ToAlpha2(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLang3To1(t *testing.T) {
	cases := map[string]string{
		"RON": "ro",
		"DEU": "de",
		"ENG": "en",
		"GLE": "ga",
		"XYZ": "",
		"":    "",
	}
	for in, want := range cases {
		if got := lang3To1(in); got != want {
			t.Errorf("lang3To1(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDedupCPV(t *testing.T) {
	primary, secondary := dedupCPV([]string{"45233220", "45233220", "45213352", "45213352", "45223320"})
	if primary != "45233220" {
		t.Errorf("primary = %q, want %q", primary, "45233220")
	}
	want := []string{"45213352", "45223320"}
	if len(secondary) != len(want) {
		t.Fatalf("secondary = %v, want %v", secondary, want)
	}
	for i := range want {
		if secondary[i] != want[i] {
			t.Errorf("secondary[%d] = %q, want %q", i, secondary[i], want[i])
		}
	}
}

func TestDedupCPV_Empty(t *testing.T) {
	primary, secondary := dedupCPV(nil)
	if primary != "" || secondary != nil {
		t.Errorf("dedupCPV(nil) = (%q, %v), want (\"\", nil)", primary, secondary)
	}
}

func TestFirst(t *testing.T) {
	if got := first([]string{"a", "b"}); got != "a" {
		t.Errorf("first([a,b]) = %q, want %q", got, "a")
	}
	if got := first(nil); got != "" {
		t.Errorf("first(nil) = %q, want \"\"", got)
	}
}
