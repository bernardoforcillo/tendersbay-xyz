package tender_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

func weight(v float64) *float64 { return &v }

// crit builds a criterion carrying only what the derivations look at, so a
// table row reads as the fact it encodes rather than as a struct literal.
func crit(lotRef string, ordinal int, w *float64) tender.AwardCriterion {
	return tender.AwardCriterion{LotRef: lotRef, Ordinal: ordinal, Type: "quality", Weight: w}
}

// TestNoticeDetailDerivations pins the three aggregates the enricher persists
// and reports. The cases that matter are the weightless ones: a notice can
// publish criteria and still yield no grid, and every case below that has
// criteria but GridUsable false is one that a nil-flattened weight would have
// silently reported as usable.
func TestNoticeDetailDerivations(t *testing.T) {
	tests := []struct {
		name         string
		detail       tender.NoticeDetail
		wantCriteria int
		wantWeighted int
		wantUsable   bool
	}{
		{
			name:   "no criteria at all",
			detail: tender.NoticeDetail{Lots: []tender.Lot{{Ref: "LOT-0001"}}},
			// GridUsable is false, not unknown: we read the notice and there is
			// no grid in it.
			wantUsable: false,
		},
		{
			name: "notice-level criteria, none weighted",
			// The frequent trap: a lone entry naming the award method
			// ("offerta economicamente più vantaggiosa") with no weight attached.
			detail: tender.NoticeDetail{
				Criteria: []tender.AwardCriterion{crit("", 0, nil)},
			},
			wantCriteria: 1,
			wantUsable:   false,
		},
		{
			name: "notice-level criteria, one weighted",
			detail: tender.NoticeDetail{
				Criteria: []tender.AwardCriterion{
					crit("", 0, weight(70)),
					crit("", 1, nil),
				},
			},
			wantCriteria: 2,
			wantWeighted: 1,
			wantUsable:   true,
		},
		{
			name: "weight published as zero is a weight",
			// A buyer that publishes 0 has told us something; only nil means
			// nothing was published. This case is precisely what would break if
			// Weight were a float64.
			detail: tender.NoticeDetail{
				Criteria: []tender.AwardCriterion{crit("", 0, weight(0))},
			},
			wantCriteria: 1,
			wantWeighted: 1,
			wantUsable:   true,
		},
		{
			name: "weights on a lot only",
			detail: tender.NoticeDetail{
				Lots: []tender.Lot{
					{Ref: "LOT-0001"},
					{Ref: "LOT-0002", Criteria: []tender.AwardCriterion{crit("LOT-0002", 0, weight(30))}},
				},
			},
			wantCriteria: 1,
			wantWeighted: 1,
			wantUsable:   true,
		},
		{
			name: "notice-level and lot-level entries are counted once each",
			detail: tender.NoticeDetail{
				Criteria: []tender.AwardCriterion{crit("", 0, nil)},
				Lots: []tender.Lot{
					{Ref: "LOT-0001", Criteria: []tender.AwardCriterion{
						crit("LOT-0001", 0, weight(60)),
						crit("LOT-0001", 1, weight(40)),
					}},
					{Ref: "LOT-0002", Criteria: []tender.AwardCriterion{crit("LOT-0002", 0, nil)}},
				},
			},
			wantCriteria: 4,
			wantWeighted: 2,
			wantUsable:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.detail.CriteriaCount(); got != tt.wantCriteria {
				t.Errorf("CriteriaCount() = %d, want %d", got, tt.wantCriteria)
			}
			if got := tt.detail.WeightedCount(); got != tt.wantWeighted {
				t.Errorf("WeightedCount() = %d, want %d", got, tt.wantWeighted)
			}
			if got := tt.detail.GridUsable(); got != tt.wantUsable {
				t.Errorf("GridUsable() = %v, want %v", got, tt.wantUsable)
			}
			// The flat set is what gets persisted; if it disagreed with the
			// counts, the stored grid_usable column and its child rows would
			// drift apart with nothing to catch it.
			if got := len(tt.detail.AllCriteria()); got != tt.wantCriteria {
				t.Errorf("len(AllCriteria()) = %d, want %d", got, tt.wantCriteria)
			}
		})
	}
}

// TestAllCriteriaOrder pins the persistence order: notice-level first, then
// each lot in lot order, each keeping its published Ordinal. Ordinal is unique
// only within a LotRef, so the pair is what identifies an entry — a reordering
// here would rewrite which criterion a stored (lot_ref, ordinal) row means.
func TestAllCriteriaOrder(t *testing.T) {
	detail := tender.NoticeDetail{
		Criteria: []tender.AwardCriterion{crit("", 0, nil)},
		Lots: []tender.Lot{
			{Ref: "LOT-0002", Criteria: []tender.AwardCriterion{
				crit("LOT-0002", 0, weight(50)),
				crit("LOT-0002", 1, weight(50)),
			}},
			{Ref: "LOT-0001", Criteria: []tender.AwardCriterion{crit("LOT-0001", 0, nil)}},
		},
	}

	type key struct {
		lotRef  string
		ordinal int
	}
	want := []key{{"", 0}, {"LOT-0002", 0}, {"LOT-0002", 1}, {"LOT-0001", 0}}

	got := detail.AllCriteria()
	if len(got) != len(want) {
		t.Fatalf("AllCriteria() returned %d entries, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].LotRef != w.lotRef || got[i].Ordinal != w.ordinal {
			t.Errorf("entry %d = (%q, %d), want (%q, %d)", i, got[i].LotRef, got[i].Ordinal, w.lotRef, w.ordinal)
		}
	}
}

// TestAllCriteriaEmpty keeps "no criteria published" a nil slice rather than an
// empty non-nil one: a notice with no criteria is the majority outcome, and the
// write path distinguishes nothing-to-write from an empty set.
func TestAllCriteriaEmpty(t *testing.T) {
	detail := tender.NoticeDetail{Lots: []tender.Lot{{Ref: "LOT-0001"}, {Ref: "LOT-0002"}}}
	if got := detail.AllCriteria(); got != nil {
		t.Fatalf("AllCriteria() = %v, want nil", got)
	}
}

// TestHasWeight guards the one predicate every consumer routes through, so the
// nil-is-absent invariant has a test to break if it is ever "simplified".
func TestHasWeight(t *testing.T) {
	tests := []struct {
		name string
		w    *float64
		want bool
	}{
		{name: "absent", w: nil, want: false},
		{name: "published zero", w: weight(0), want: true},
		{name: "published value", w: weight(30.5), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (tender.AwardCriterion{Weight: tt.w}).HasWeight(); got != tt.want {
				t.Errorf("HasWeight() = %v, want %v", got, tt.want)
			}
		})
	}
}
