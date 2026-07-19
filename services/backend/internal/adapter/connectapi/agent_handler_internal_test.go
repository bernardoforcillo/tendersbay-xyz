package connectapi

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func TestToProtoTenderResults_ConvertsEachTenderWithoutEuThreshold(t *testing.T) {
	value := int64(250000)
	tr := agent.TenderResults{Tenders: []tender.ScoredTender{
		{Tender: tender.Tender{ID: "t-1", Title: "Cestini intelligenti", Country: "IT", CPV: "34928480", Value: &value}},
	}}

	got := toProtoTenderResults(tr)

	if len(got.Tenders) != 1 {
		t.Fatalf("len(Tenders) = %d, want 1", len(got.Tenders))
	}
	tr0 := got.Tenders[0]
	if tr0.Id != "t-1" || tr0.Title != "Cestini intelligenti" || tr0.Country != "IT" || tr0.Value != 250000 {
		t.Fatalf("got = %+v", tr0)
	}
	if tr0.EuThreshold != "" {
		t.Fatalf("EuThreshold = %q, want empty (this path has no *tender.Service to compute a band from)", tr0.EuThreshold)
	}
}
