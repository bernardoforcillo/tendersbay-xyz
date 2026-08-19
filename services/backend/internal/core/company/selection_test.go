package company

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// TestNoticePublishedCannotDecide is the load-bearing test of this whole path:
// a requirement Spain published as prose must never, by itself, fail a bidder.
// If this ever goes green in the other direction, the engine has started
// deciding on a threshold nobody parsed.
func TestNoticePublishedCannotDecide(t *testing.T) {
	if RequirementNoticePublished.Authoritative() {
		t.Error("notice_published is authoritative — published prose is not a checked fact")
	}
	r := Requirement{Source: RequirementNoticePublished, Kind: RequirementOther, Blocking: true}
	if r.CanBlock() {
		t.Error("a notice_published requirement can block — it must inform a human, not decide")
	}
	// Even a human confirming it must not promote it: confirmation attests that
	// the QUOTE is right, and the quote is prose either way.
	r.ConfirmedBy = "user-1"
	if r.CanBlock() {
		t.Error("confirming a notice_published requirement made it blocking")
	}
	if !validRequirementSources[RequirementNoticePublished] {
		t.Error("notice_published is not in validRequirementSources — it would be rejected on write")
	}
}

func TestRequirementsFromSelectionCriteria(t *testing.T) {
	criteria := []tender.SelectionCriterion{
		{Ordinal: 0, Category: "financial", Type: "5", Origin: "es-placsp", Lang: "spa",
			Description: "Volumen anual de negocios igual o superior a 309.552,00 EUR."},
		{Ordinal: 1, Category: "technical", Type: "OSR-TECH", Origin: "es-placsp", Name: "Equipo minimo"},
		// Neither description nor name: states nothing, must be dropped rather
		// than becoming an empty row on the scheda gara.
		{Ordinal: 2, Category: "declaration", Type: "1", Origin: "es-placsp"},
	}

	got := RequirementsFromSelectionCriteria("ws-1", 42, criteria)
	if len(got) != 2 {
		t.Fatalf("got %d requirements, want 2 (the empty one dropped): %+v", len(got), got)
	}

	// A "financial" criterion must NOT become RequirementTurnover: the category
	// says which shelf the buyer filed it on, not what the threshold is, and an
	// AmountRequirement with a zero MinMinor would be evaluated and passed
	// against a number nobody published.
	for i, r := range got {
		if r.Kind != RequirementOther {
			t.Errorf("requirement %d Kind = %q, want other — the payload was never parsed", i, r.Kind)
		}
		if r.Amount != nil || r.SOA != nil || r.Count != nil {
			t.Errorf("requirement %d carries a machine-comparable payload: %+v", i, r)
		}
		if r.Source != RequirementNoticePublished {
			t.Errorf("requirement %d Source = %q, want notice_published", i, r.Source)
		}
		if !r.Blocking {
			t.Errorf("requirement %d is not Blocking — an unclassified requirement must raise a question", i)
		}
		if r.Citation != nil {
			t.Errorf("requirement %d carries a citation, but it came from a feed, not a document", i)
		}
		if r.WorkspaceID != "ws-1" || r.TenderID != 42 {
			t.Errorf("requirement %d scoped to (%q, %d), want (ws-1, 42)", i, r.WorkspaceID, r.TenderID)
		}
	}

	if got[0].Text != "Volumen anual de negocios igual o superior a 309.552,00 EUR." {
		t.Errorf("Text = %q, want the prose verbatim", got[0].Text)
	}
	// Name is the fallback when a source titles instead of describing.
	if got[1].Text != "Equipo minimo" {
		t.Errorf("fallback Text = %q, want the Name", got[1].Text)
	}
}

func TestRequirementsFromSelectionCriteria_EmptyIsNil(t *testing.T) {
	if got := RequirementsFromSelectionCriteria("ws-1", 42, nil); got != nil {
		t.Errorf("got %+v, want nil for no criteria", got)
	}
	onlyEmpty := []tender.SelectionCriterion{{Ordinal: 0, Category: "declaration"}}
	if got := RequirementsFromSelectionCriteria("ws-1", 42, onlyEmpty); got != nil {
		t.Errorf("got %+v, want nil when every criterion states nothing", got)
	}
}
