package tender

import (
	"context"
	"testing"
)

// TestCoverage_PassesThroughRepoCountries proves Service.Coverage is a thin
// pass-through over Repo.DistinctCountries — the anonymous landing marquee's
// only caller — with no filtering or reordering of its own.
func TestCoverage_PassesThroughRepoCountries(t *testing.T) {
	repo := &recommendFakeRepo{countries: []string{"IT", "PL"}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{}, testFitConfig())

	got, err := svc.Coverage(context.Background())
	if err != nil {
		t.Fatalf("Coverage: %v", err)
	}
	if len(got) != 2 || got[0] != "IT" || got[1] != "PL" {
		t.Fatalf("Coverage() = %v, want [IT PL] passed through unchanged", got)
	}
}
