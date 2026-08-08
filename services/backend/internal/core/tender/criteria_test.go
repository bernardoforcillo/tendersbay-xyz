package tender

import "testing"

func f64p(v float64) *float64 { return &v }

// The notice-level fallback is the whole point of the helper: a multi-lot
// notice usually publishes one grid once, so filtering on LotRef alone would
// report "this lot has no criteria" for the majority of real notices.
func TestCriteriaForLot_FallsBackToNoticeLevel(t *testing.T) {
	d := TenderDetail{Criteria: []AwardCriterion{
		{LotRef: "", Ordinal: 0, Name: "Offerta tecnica", Weight: f64p(70)},
		{LotRef: "", Ordinal: 1, Name: "Offerta economica", Weight: f64p(30)},
	}}

	got := d.CriteriaForLot("LOT-0007")
	if len(got) != 2 {
		t.Fatalf("criteria = %d, want the 2 notice-level entries", len(got))
	}
	if got[0].Name != "Offerta tecnica" || got[1].Name != "Offerta economica" {
		t.Errorf("criteria = %v, want published order preserved", got)
	}
}

// A lot that states its own grid must not also inherit the notice-level one:
// the two would be summed by any consumer that trusts the weights.
func TestCriteriaForLot_OwnCriteriaSuppressTheNoticeLevelSet(t *testing.T) {
	d := TenderDetail{Criteria: []AwardCriterion{
		{LotRef: "", Ordinal: 0, Name: "Notice-level"},
		{LotRef: "LOT-0001", Ordinal: 0, Name: "Lot 1 quality"},
		{LotRef: "LOT-0001", Ordinal: 1, Name: "Lot 1 price"},
		{LotRef: "LOT-0002", Ordinal: 0, Name: "Lot 2 quality"},
	}}

	got := d.CriteriaForLot("LOT-0001")
	if len(got) != 2 {
		t.Fatalf("criteria = %d, want 2 lot-specific entries", len(got))
	}
	for _, c := range got {
		if c.LotRef != "LOT-0001" {
			t.Errorf("criterion %q has LotRef %q, want only LOT-0001", c.Name, c.LotRef)
		}
	}
}

// A lot the notice never mentioned still inherits the notice-level grid — the
// helper answers "what scores this lot", not "what did the notice say about
// this lot's ref".
func TestCriteriaForLot_UnknownLotInheritsNoticeLevel(t *testing.T) {
	d := TenderDetail{Criteria: []AwardCriterion{
		{LotRef: "", Ordinal: 0, Name: "Notice-level"},
		{LotRef: "LOT-0001", Ordinal: 0, Name: "Lot 1 quality"},
	}}

	got := d.CriteriaForLot("LOT-9999")
	if len(got) != 1 || got[0].Name != "Notice-level" {
		t.Errorf("criteria = %v, want the notice-level entry", got)
	}
}

// The empty ref is how a caller asks for the notice-level set itself; it must
// not return every lot's criteria as well.
func TestCriteriaForLot_EmptyRefReturnsTheNoticeLevelSetOnly(t *testing.T) {
	d := TenderDetail{Criteria: []AwardCriterion{
		{LotRef: "", Ordinal: 0, Name: "Notice-level"},
		{LotRef: "LOT-0001", Ordinal: 0, Name: "Lot 1 quality"},
	}}

	got := d.CriteriaForLot("")
	if len(got) != 1 || got[0].Name != "Notice-level" {
		t.Errorf("criteria = %v, want only the notice-level entry", got)
	}
}

// A notice that published nothing yields nothing, for any ref — the helper
// must not invent an entry to fill the gap.
func TestCriteriaForLot_NoCriteriaYieldsNone(t *testing.T) {
	d := TenderDetail{}
	if got := d.CriteriaForLot("LOT-0001"); got != nil {
		t.Errorf("criteria = %v, want nil", got)
	}
	if got := d.CriteriaForLot(""); got != nil {
		t.Errorf("criteria = %v, want nil", got)
	}
}

// Lot-specific criteria with no notice-level set at all must still be found —
// the fallback is a fallback, not a precondition.
func TestCriteriaForLot_LotOnlyGridNeedsNoNoticeLevelSet(t *testing.T) {
	d := TenderDetail{Criteria: []AwardCriterion{
		{LotRef: "LOT-0001", Ordinal: 0, Name: "Lot 1 quality", Weight: f64p(60)},
	}}
	got := d.CriteriaForLot("LOT-0001")
	if len(got) != 1 || got[0].Name != "Lot 1 quality" {
		t.Errorf("criteria = %v, want the lot's own entry", got)
	}
	// …and a sibling lot in the same notice correctly finds nothing, rather
	// than inheriting a grid that was never stated for it.
	if got := d.CriteriaForLot("LOT-0002"); got != nil {
		t.Errorf("criteria for LOT-0002 = %v, want nil", got)
	}
}

// HasWeight exists so that call sites read the invariant instead of
// re-deriving it; a zero weight is a published fact and must not be reported
// as an absent one.
func TestAwardCriterion_HasWeight(t *testing.T) {
	if (AwardCriterion{}).HasWeight() {
		t.Error("HasWeight() = true for a criterion with no published weight")
	}
	if !(AwardCriterion{Weight: f64p(0)}).HasWeight() {
		t.Error("HasWeight() = false for an explicitly published zero weight")
	}
	if !(AwardCriterion{Weight: f64p(30)}).HasWeight() {
		t.Error("HasWeight() = false for a weighted criterion")
	}
}
