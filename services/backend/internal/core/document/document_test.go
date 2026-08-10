package document_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// ── The invariant ──────────────────────────────────────────────────────────
//
// Consistent is the single property this package exists to protect, so it is
// tested against named, hand-written states rather than a generated
// cross-product: a generated one would restate the implementation's own
// predicate and pass no matter what that predicate said. Each case below names
// the real-world situation it stands for.

func TestAvailability_Consistent(t *testing.T) {
	cases := []struct {
		name string
		av   document.Availability
		want bool
	}{
		{
			name: "full coverage needs no reason",
			av: document.Availability{
				Coverage: document.CoverageFull, Reason: document.ReasonNone,
				NoticeRead: true, ExtractedDocuments: 2, ExtractedParts: 40,
			},
			want: true,
		},
		{
			// A gap with no explanation attached reads downstream as an
			// absence, which is precisely the conflation being blocked.
			name: "partial coverage with no reason is a silent gap",
			av: document.Availability{
				Coverage: document.CoverageNotice, Reason: document.ReasonNone,
				NoticeRead: true,
			},
			want: false,
		},
		{
			name: "no coverage with no reason is a silent gap",
			av:   document.Availability{Coverage: document.CoverageNone, Reason: document.ReasonNone},
			want: false,
		},
		{
			// Full coverage cannot also carry a cause: there is no gap to
			// explain, and a reason here would be read as one.
			name: "full coverage may not carry a cause",
			av: document.Availability{
				Coverage: document.CoverageFull, Reason: document.ReasonBodyNotRetrieved,
				NoticeRead: true, KnownDocumentLinks: 1,
			},
			want: false,
		},
		{
			name: "we read the notice and it named no link",
			av: document.Availability{
				Coverage: document.CoverageNotice, Reason: document.ReasonNoDocumentsPublished,
				NoticeRead: true, KnownDocumentLinks: 0, ExtractedParts: 3,
			},
			want: true,
		},
		{
			// THE trap: we never read the notice, so we cannot know whether
			// the buyer published anything. Claiming absence here is the
			// failure mode this whole phase exists to remove.
			name: "unread notice may not claim the buyer published nothing",
			av: document.Availability{
				Coverage: document.CoverageNone, Reason: document.ReasonNoDocumentsPublished,
				NoticeRead: false, KnownDocumentLinks: 0,
			},
			want: false,
		},
		{
			// The other half of the trap: we hold a link, so documents
			// demonstrably exist. "None published" is refuted by our own data.
			name: "a held link refutes the claim that none were published",
			av: document.Availability{
				Coverage: document.CoverageNotice, Reason: document.ReasonNoDocumentsPublished,
				NoticeRead: true, KnownDocumentLinks: 1,
			},
			want: false,
		},
		{
			name: "the specification sits behind a link we have not followed",
			av: document.Availability{
				Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
				NoticeRead: true, KnownDocumentLinks: 2, ExtractedParts: 5,
			},
			want: true,
		},
		{
			name: "we have not read the notice, so we do not know",
			av: document.Availability{
				Coverage: document.CoverageNone, Reason: document.ReasonNoticeNotRead,
				NoticeRead: false,
			},
			want: true,
		},
		{
			name: "a document row exists, the indexer has not reached it",
			av: document.Availability{
				Coverage: document.CoverageNone, Reason: document.ReasonNotYetExtracted,
				NoticeRead: true, ExtractedDocuments: 1,
			},
			want: true,
		},
		{
			name: "the indexer ran and produced no text",
			av: document.Availability{
				Coverage: document.CoverageNone, Reason: document.ReasonExtractionFailed,
				NoticeRead: true, ExtractedDocuments: 1,
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.av.Consistent(); got != tc.want {
				t.Errorf("Consistent() = %v, want %v (%+v)", got, tc.want, tc.av)
			}
		})
	}
}

// No reason other than ReasonNoDocumentsPublished may assert absence, so no
// other reason is allowed to depend on the evidence fields for its validity.
// This pins the asymmetry: only the absence claim is gated.
func TestAvailability_OnlyAbsenceClaimIsGatedOnEvidence(t *testing.T) {
	gapReasons := []document.Reason{
		document.ReasonNoticeNotRead,
		document.ReasonBodyNotRetrieved,
		document.ReasonNotYetExtracted,
		document.ReasonExtractionFailed,
	}
	for _, r := range gapReasons {
		for _, noticeRead := range []bool{true, false} {
			for _, links := range []int{0, 3} {
				av := document.Availability{
					Coverage: document.CoverageNone, Reason: r,
					NoticeRead: noticeRead, KnownDocumentLinks: links,
				}
				if !av.Consistent() {
					t.Errorf("Consistent() = false for gap reason %q (noticeRead=%v links=%d), want true", r, noticeRead, links)
				}
			}
		}
	}
}

// ── Service ────────────────────────────────────────────────────────────────

// fakeReader lets a test drive Service without a database. It records what it
// was asked for, because the clamping and limit contracts are about what the
// service sends DOWN as much as what it returns up.
type fakeReader struct {
	av          document.Availability
	avErr       error
	excerpts    []document.Excerpt
	searchErr   error
	gotQuestion string
	gotLimit    int
	searchCalls int
}

func (f *fakeReader) Availability(context.Context, int64) (document.Availability, error) {
	return f.av, f.avErr
}

func (f *fakeReader) SearchParts(_ context.Context, _ int64, question string, limit int) ([]document.Excerpt, error) {
	f.searchCalls++
	f.gotQuestion, f.gotLimit = question, limit
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	out := make([]document.Excerpt, len(f.excerpts))
	copy(out, f.excerpts)
	return out, nil
}

var _ document.Reader = (*fakeReader)(nil)

func TestService_Excerpts_ClampsLimitIntoRange(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"zero means the default budget", 0, document.MaxExcerpts},
		{"negative means the default budget", -4, document.MaxExcerpts},
		{"one is honoured", 1, 1},
		{"under the cap is honoured", document.MaxExcerpts - 1, document.MaxExcerpts - 1},
		{"over the cap is capped", document.MaxExcerpts + 50, document.MaxExcerpts},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeReader{}
			svc := document.NewService(r)
			if _, err := svc.Excerpts(context.Background(), document.ExcerptQuery{
				TenderID: 7, Question: "penali per ritardo", Limit: tc.in,
			}); err != nil {
				t.Fatalf("Excerpts: %v", err)
			}
			if r.gotLimit != tc.want {
				t.Errorf("limit passed to reader = %d, want %d", r.gotLimit, tc.want)
			}
		})
	}
}

// The bound belongs to core precisely so that an adapter returning a whole
// 40-page part cannot leak it into a model's context.
func TestService_Excerpts_ClampsTextAtRuneBoundary(t *testing.T) {
	// Multi-byte throughout: a byte-wise clamp would split a rune here and
	// produce a replacement glyph mid-passage.
	long := strings.Repeat("à", document.MaxExcerptRunes+500)
	short := strings.Repeat("è", 10)
	r := &fakeReader{excerpts: []document.Excerpt{{Text: long}, {Text: short}}}
	svc := document.NewService(r)

	res, err := svc.Excerpts(context.Background(), document.ExcerptQuery{TenderID: 1, Question: "requisiti"})
	if err != nil {
		t.Fatalf("Excerpts: %v", err)
	}
	if len(res.Excerpts) != 2 {
		t.Fatalf("excerpts = %d, want 2", len(res.Excerpts))
	}
	got := []rune(res.Excerpts[0].Text)
	if len(got) != document.MaxExcerptRunes {
		t.Errorf("clamped length = %d runes, want %d", len(got), document.MaxExcerptRunes)
	}
	if !strings.HasPrefix(res.Excerpts[0].Text, "à") || strings.ContainsRune(res.Excerpts[0].Text, '�') {
		t.Errorf("clamp split a rune: %q…", res.Excerpts[0].Text[:8])
	}
	if !res.Excerpts[0].Truncated {
		t.Error("Truncated = false on a clamped excerpt, want true")
	}
	if res.Excerpts[1].Truncated {
		t.Error("Truncated = true on a short excerpt, want false")
	}
	if res.Excerpts[1].Text != short {
		t.Errorf("short excerpt was altered: %q", res.Excerpts[1].Text)
	}
}

// A text whose rune count fits but whose byte length does not must survive
// intact — the byte pre-check is a fast path, not the decision.
func TestService_Excerpts_MultibyteUnderTheRuneCapIsUntouched(t *testing.T) {
	text := strings.Repeat("à", document.MaxExcerptRunes) // 2x MaxExcerptRunes bytes
	r := &fakeReader{excerpts: []document.Excerpt{{Text: text}}}
	res, err := document.NewService(r).Excerpts(context.Background(), document.ExcerptQuery{TenderID: 1, Question: "q"})
	if err != nil {
		t.Fatalf("Excerpts: %v", err)
	}
	if res.Excerpts[0].Text != text || res.Excerpts[0].Truncated {
		t.Errorf("a text of exactly MaxExcerptRunes runes was clamped (truncated=%v)", res.Excerpts[0].Truncated)
	}
}

// Availability travels with every result, including the empty one. A caller
// that only ever sees an empty slice cannot tell "no passage matched" from
// "we hold nothing".
func TestService_Excerpts_AlwaysCarriesAvailability(t *testing.T) {
	want := document.Availability{
		TenderID: 42, Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
		NoticeRead: true, KnownDocumentLinks: 1, ExtractedDocuments: 1, ExtractedParts: 4,
	}
	r := &fakeReader{av: want}
	res, err := document.NewService(r).Excerpts(context.Background(), document.ExcerptQuery{TenderID: 42, Question: "capacità tecnica"})
	if err != nil {
		t.Fatalf("Excerpts: %v", err)
	}
	if len(res.Excerpts) != 0 {
		t.Fatalf("excerpts = %d, want 0", len(res.Excerpts))
	}
	if res.Availability != want {
		t.Errorf("availability = %+v, want %+v", res.Availability, want)
	}
}

func TestService_Excerpts_BlankQuestionSkipsRetrieval(t *testing.T) {
	for _, q := range []string{"", "   ", "\n\t "} {
		r := &fakeReader{av: document.Availability{TenderID: 3, Coverage: document.CoverageFull}}
		res, err := document.NewService(r).Excerpts(context.Background(), document.ExcerptQuery{TenderID: 3, Question: q})
		if err != nil {
			t.Fatalf("Excerpts(%q): %v", q, err)
		}
		if r.searchCalls != 0 {
			t.Errorf("Excerpts(%q) called SearchParts %d times, want 0", q, r.searchCalls)
		}
		if res.Availability.TenderID != 3 {
			t.Errorf("Excerpts(%q) dropped the availability answer", q)
		}
	}
}

func TestService_Excerpts_TrimsTheQuestion(t *testing.T) {
	r := &fakeReader{}
	if _, err := document.NewService(r).Excerpts(context.Background(), document.ExcerptQuery{
		TenderID: 1, Question: "  penali per ritardo\n",
	}); err != nil {
		t.Fatalf("Excerpts: %v", err)
	}
	if r.gotQuestion != "penali per ritardo" {
		t.Errorf("question passed to reader = %q, want trimmed", r.gotQuestion)
	}
}

// A coverage lookup that fails must fail the call rather than degrade to a
// confident-looking empty answer.
func TestService_Excerpts_PropagatesReaderErrors(t *testing.T) {
	boom := errors.New("boom")

	if _, err := document.NewService(&fakeReader{avErr: boom}).
		Excerpts(context.Background(), document.ExcerptQuery{TenderID: 1, Question: "q"}); !errors.Is(err, boom) {
		t.Errorf("availability error = %v, want it to wrap boom", err)
	}
	if _, err := document.NewService(&fakeReader{searchErr: boom}).
		Excerpts(context.Background(), document.ExcerptQuery{TenderID: 1, Question: "q"}); !errors.Is(err, boom) {
		t.Errorf("search error = %v, want it to wrap boom", err)
	}
	if _, err := document.NewService(&fakeReader{avErr: document.ErrTenderNotFound}).
		Availability(context.Background(), 1); !errors.Is(err, document.ErrTenderNotFound) {
		t.Errorf("Availability error = %v, want it to wrap ErrTenderNotFound", err)
	}
}
