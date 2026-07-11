package eforms

import (
	"testing"
	"time"
)

func TestParseDeadline_DateAndTime(t *testing.T) {
	got := parseDeadline("2026-08-11+03:00", "15:00:00+03:00")
	want := time.Date(2026, 8, 11, 15, 0, 0, 0, time.FixedZone("", 3*3600))
	if got == nil || !got.Equal(want) {
		t.Errorf("parseDeadline = %v, want %v", got, want)
	}
}

func TestParseDeadline_DateOnly(t *testing.T) {
	got := parseDeadline("2026-07-09+02:00", "")
	want := time.Date(2026, 7, 9, 0, 0, 0, 0, time.FixedZone("", 2*3600))
	if got == nil || !got.Equal(want) {
		t.Errorf("parseDeadline = %v, want %v", got, want)
	}
}

func TestParseDeadline_Empty(t *testing.T) {
	if got := parseDeadline("", ""); got != nil {
		t.Errorf("parseDeadline(\"\", \"\") = %v, want nil", got)
	}
}

func TestParseDeadline_Malformed(t *testing.T) {
	if got := parseDeadline("not-a-date", "also-not-a-time"); got != nil {
		t.Errorf("parseDeadline with malformed input = %v, want nil", got)
	}
}
