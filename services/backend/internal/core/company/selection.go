// Package company — this file maps the selection criteria a source published
// into requirements the engine can display.
//
// Display, not decide. Everything here lands as RequirementOther under
// RequirementNoticePublished, which is non-authoritative, so none of it can
// contribute to a no_go. That is the whole design: a published requirement the
// engine cannot model must be VISIBLE as un-modelled rather than silently
// absent, and it must not pretend to a precision the source never offered.
package company

import (
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// RequirementsFromSelectionCriteria turns a tender's published selection
// criteria into requirements scoped to one workspace.
//
// Every entry becomes RequirementOther regardless of its Category, and that is
// deliberate rather than lazy. Category tells us the source filed a requirement
// under "financial"; it does not tell us the threshold, the currency, or the
// window of esercizi it applies over, which is what RequirementTurnover exists
// to hold. Mapping "financial" onto RequirementTurnover would produce an
// AmountRequirement with a zero MinMinor — a requirement the engine would
// evaluate, and pass, against a threshold nobody published. RequirementOther is
// the honest destination: captured verbatim, gap always GapUnknown, remedy
// always "read it yourself".
//
// The criteria carry no ID and no timestamps: they are derived on read from
// tender-owned rows, not persisted per workspace. Writing them into the
// workspace requirement table would duplicate identical rows per tenant and
// mis-model ownership — the notice's requirement is a fact about the tender,
// true for everyone.
func RequirementsFromSelectionCriteria(workspaceID string, tenderID int64, criteria []tender.SelectionCriterion) []Requirement {
	if len(criteria) == 0 {
		return nil
	}
	out := make([]Requirement, 0, len(criteria))
	for _, c := range criteria {
		text := selectionText(c)
		if text == "" {
			// A criterion with neither description nor name states nothing a
			// human could act on. Showing it as an empty requirement would add
			// a row to the scheda gara that answers no question.
			continue
		}
		out = append(out, Requirement{
			WorkspaceID: workspaceID,
			TenderID:    tenderID,
			LotRef:      c.LotRef,
			Kind:        RequirementOther,
			Text:        text,
			// Blocking is TRUE for the reason the field documents: an
			// unclassified requirement treated as blocking produces a QUESTION,
			// while treating it as non-blocking produces a silent pass. Note
			// this does not make it decide anything — CanBlock is false for this
			// source, so Blocking here means "ask the user", not "fail them".
			Blocking: true,
			Source:   RequirementNoticePublished,
			// No Citation: the verification affordance points at a passage in a
			// document the user can open, and this requirement did not come from
			// one — it came from the source's feed. An empty citation is honest;
			// a fabricated one pointing at the notice would be the kind of
			// unverifiable claim the Citation field exists to prevent.
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// selectionText picks the text a human reads. Description first because that is
// where the substance is — PLACSP puts the whole requirement, threshold and
// legal basis included, in it — falling back to Name for a source that titles a
// criterion instead of describing it.
func selectionText(c tender.SelectionCriterion) string {
	if d := strings.TrimSpace(c.Description); d != "" {
		return d
	}
	return strings.TrimSpace(c.Name)
}
