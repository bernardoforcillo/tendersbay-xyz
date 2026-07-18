package connectapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

type fakeRepo struct{ results []tender.Tender }

func (f *fakeRepo) SearchTenders(context.Context, tender.Filters, int, int) ([]tender.Tender, error) {
	return f.results, nil
}
func (f *fakeRepo) EnrichTenders(context.Context, []string, tender.Filters) ([]tender.Tender, error) {
	return nil, nil
}

type fakeKB struct{}

func (fakeKB) SearchWithScores(context.Context, string, int) ([]tender.ScoredChunk, error) {
	return nil, nil
}

type fakeRL struct{}

func (fakeRL) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

type fakeProfileSource struct{}

func (fakeProfileSource) Get(context.Context, string, string) (clientprofile.Profile, error) {
	return clientprofile.Profile{}, nil
}

func testTenderHandler(t *testing.T) *connectapi.TenderHandler {
	t.Helper()
	repo := &fakeRepo{results: []tender.Tender{{ID: "1", Title: "Lavori stradali"}}}
	cfg := tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
	}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSource{}, cfg)
	return connectapi.NewTenderHandler(svc)
}

func TestSearchTenders_WorksWithoutAuth(t *testing.T) {
	h := testTenderHandler(t)
	// No UserIDFromContext value set on this context — simulates an
	// unauthenticated request. Must not error.
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Query: "", Limit: 5})
	resp, err := h.SearchTenders(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchTenders (anonymous): %v", err)
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(resp.Msg.Results) = %d, want 1", len(resp.Msg.Results))
	}
	if resp.Msg.Results[0].Id != "1" {
		t.Errorf("resp.Msg.Results[0].Id = %q, want %q", resp.Msg.Results[0].Id, "1")
	}
}

func TestSearchTenders_RejectsInvalidDeadlineRangeAsInvalidArgument(t *testing.T) {
	h := testTenderHandler(t)
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{
		Filters: &tenderv1.TenderFilters{DeadlineFrom: "2030-01-01T00:00:00Z", DeadlineTo: "2020-01-01T00:00:00Z"},
	})
	_, err := h.SearchTenders(context.Background(), req)
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want a connect.Error with CodeInvalidArgument", err)
	}
}
