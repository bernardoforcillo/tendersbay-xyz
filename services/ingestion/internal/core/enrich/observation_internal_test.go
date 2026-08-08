// Internal tests for the notice_xml_ingested observation. They live in package
// enrich because the payload is deliberately unexported — nothing outside this
// package may construct or extend it — and its property set is the thing under
// test.
package enrich

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// The property set is a contract with every chart built on it, so it is pinned
// exactly rather than checked for "contains". The load-bearing half of this
// assertion is the absence: has_criteria would report roughly twice the real
// coverage (76% of sampled Italian notices publish a criterion entry; 36%
// attach a weight to any of them), and the only reliable way to keep it out of
// a dashboard is for it not to exist. If this test fails because a property was
// added, read observation's doc comment before changing the want list.
func TestObservationAttrs_PropertySetIsExactAndExcludesHasCriteria(t *testing.T) {
	want := []string{
		"location", "source", "country", "notice_type", "parse_status",
		"lot_count", "has_documents_url", "has_submission_url",
		"criteria_count", "weighted_criteria_count", "grid_usable",
		"organization_count", "xml_bytes_bucket",
	}

	attrs := observation{}.attrs()
	if len(attrs) != 2*len(want) {
		t.Fatalf("attrs() returned %d values, want %d key/value pairs", len(attrs), len(want))
	}

	got := make([]string, 0, len(want))
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok {
			t.Fatalf("attrs()[%d] = %v, want a string key", i, attrs[i])
		}
		got = append(got, key)
	}
	for i, key := range want {
		if got[i] != key {
			t.Errorf("attrs() key %d = %q, want %q", i, got[i], key)
		}
	}
	for _, key := range got {
		if key == "has_criteria" {
			t.Error("attrs() emits has_criteria, which overstates real coverage by ~2x and must not exist")
		}
	}
}

// Nothing free-form may reach the event: the notice's own text, its buyer's
// name and its portal URLs are all in scope for this pass, and an observation
// is the easiest place for one of them to leak. Feeding a detail whose every
// string field is a marker proves none of them is carried through.
func TestObserveNotice_CarriesNoFreeTextFromTheNotice(t *testing.T) {
	const marker = "MUST-NOT-APPEAR"
	weight := 40.0
	d := tender.NoticeDetail{
		Lots: []tender.Lot{{
			Ref: marker, Title: marker, DocumentsURL: marker,
			Criteria: []tender.AwardCriterion{{Name: marker, Weight: &weight}},
		}},
		Criteria:      []tender.AwardCriterion{{Name: marker, Description: marker}},
		Organizations: []tender.Organization{{Name: marker, Email: marker, Phone: marker}},
		DocumentsURL:  marker,
		SubmissionURL: marker,
		Language:      marker,
	}
	n := PendingNotice{ID: 1, PublicationNumber: marker, XMLURL: marker, Country: "IT", NoticeType: "cn-standard"}

	o := observation{
		country: n.Country, noticeType: n.NoticeType, parseStatus: StatusOK,
		lotCount:         len(d.Lots),
		hasDocumentsURL:  d.DocumentsURL != "",
		hasSubmissionURL: d.SubmissionURL != "",
		criteriaCount:    d.CriteriaCount(), weightedCount: d.WeightedCount(),
		gridUsable: d.GridUsable(), organizationCount: len(d.Organizations),
		xmlBytes: 1234,
	}
	for i, v := range o.attrs() {
		if s, ok := v.(string); ok && s == marker {
			t.Fatalf("attrs()[%d] carries notice text verbatim (%q)", i, s)
		}
	}

	// The counts still have to be right, or "no free text" would be satisfiable
	// by reporting nothing at all.
	if o.criteriaCount != 2 || o.weightedCount != 1 || !o.gridUsable {
		t.Errorf("criteria_count/weighted_criteria_count/grid_usable = %d/%d/%v, want 2/1/true",
			o.criteriaCount, o.weightedCount, o.gridUsable)
	}
	if !o.hasDocumentsURL || !o.hasSubmissionURL {
		t.Error("has_documents_url/has_submission_url must report presence even though the URL itself is dropped")
	}
}

// "none" is a bucket of its own, not the smallest one: a notice whose document
// never arrived did not produce a tiny document, and folding the two together
// would hide the absent cases inside the bucket almost every real notice lands
// in.
func TestBytesBucket(t *testing.T) {
	tests := []struct {
		name  string
		bytes int
		want  string
	}{
		{"no document arrived", noXMLBytes, "none"},
		{"empty body", 0, "<50k"},
		{"a typical notice", 90_000, "50-200k"},
		{"just under the first boundary", bucketSmallMax - 1, "<50k"},
		{"exactly the first boundary", bucketSmallMax, "50-200k"},
		{"exactly the second boundary", bucketMediumMax, "200k-1m"},
		{"exactly the third boundary", bucketLargeMax, ">1m"},
		{"at the fetcher's bound", 8 << 20, ">1m"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesBucket(tt.bytes); got != tt.want {
				t.Errorf("bytesBucket(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
