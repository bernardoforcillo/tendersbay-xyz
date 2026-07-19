package tender_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func detailConfig() tender.Config {
	c := testConfig()
	c.GetTenderTier = tender.Tier{MaxResults: 20, RateLimit: 600, RateWindow: time.Minute}
	return c
}

func TestGetTender_ReturnsDetailAndResolvesSourceURL(t *testing.T) {
	repo := &fakeRepo{detail: &tender.TenderDetail{ID: "5", Title: "Road works", Source: "ted", SourceRef: "PROC-1"}}
	kb := &fakeKnowledgeBase{}
	svc := tender.NewService(repo, kb, &fakeRateLimiter{allow: true}, &fakeProfiles{}, detailConfig())

	got, err := svc.GetTender(context.Background(), tender.GetTenderParams{ID: "5", RateLimitKey: "1.2.3.4"})
	if err != nil {
		t.Fatalf("GetTender: %v", err)
	}
	if got.ID != "5" || got.Title != "Road works" {
		t.Errorf("got = %+v, want id 5 / Road works", got)
	}
}

func TestGetTender_NotFoundPropagates(t *testing.T) {
	repo := &fakeRepo{detailErr: tender.ErrTenderNotFound}
	svc := tender.NewService(repo, &fakeKnowledgeBase{}, &fakeRateLimiter{allow: true}, &fakeProfiles{}, detailConfig())
	_, err := svc.GetTender(context.Background(), tender.GetTenderParams{ID: "9", RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrTenderNotFound) {
		t.Errorf("err = %v, want ErrTenderNotFound", err)
	}
}

func TestGetTender_RejectsNonNumericID(t *testing.T) {
	svc := tender.NewService(&fakeRepo{}, &fakeKnowledgeBase{}, &fakeRateLimiter{allow: true}, &fakeProfiles{}, detailConfig())
	_, err := svc.GetTender(context.Background(), tender.GetTenderParams{ID: "abc", RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrTenderNotFound) {
		t.Errorf("err = %v, want ErrTenderNotFound for a non-numeric id", err)
	}
}

func TestGetTender_RejectsOverRateLimit(t *testing.T) {
	repo := &fakeRepo{detail: &tender.TenderDetail{ID: "5"}}
	svc := tender.NewService(repo, &fakeKnowledgeBase{}, &fakeRateLimiter{allow: false}, &fakeProfiles{}, detailConfig())
	_, err := svc.GetTender(context.Background(), tender.GetTenderParams{ID: "5", RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestGetRelatedTenders_OrdersByScore(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{"7": {ID: "7", Title: "A"}, "9": {ID: "9", Title: "B"}}}
	kb := &fakeKnowledgeBase{related: []tender.ScoredChunk{{DocID: "9", Score: 0.6}, {DocID: "7", Score: 0.9}}}
	svc := tender.NewService(repo, kb, &fakeRateLimiter{allow: true}, &fakeProfiles{}, detailConfig())

	out, err := svc.GetRelatedTenders(context.Background(), tender.RelatedParams{ID: "5", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("GetRelatedTenders: %v", err)
	}
	if len(out) != 2 || out[0].ID != "7" || out[1].ID != "9" {
		t.Errorf("out = %+v, want [7, 9] ordered by score", out)
	}
}

func TestGetRelatedTenders_DegradesOnKBError(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{"7": {ID: "7"}}}
	kb := &fakeKnowledgeBase{relatedErr: errors.New("qdrant down")}
	svc := tender.NewService(repo, kb, &fakeRateLimiter{allow: true}, &fakeProfiles{}, detailConfig())

	out, err := svc.GetRelatedTenders(context.Background(), tender.RelatedParams{ID: "5", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("GetRelatedTenders should degrade, got %v", err)
	}
	if len(out) != 0 {
		t.Errorf("out = %+v, want empty on kb error", out)
	}
}
