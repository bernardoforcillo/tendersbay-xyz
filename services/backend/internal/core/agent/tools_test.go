package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

func TestAskChoiceTool_ParsesOptionsAndInvokesCallback(t *testing.T) {
	var gotQuestion string
	var gotOptions []ChoiceOption
	var gotAllowCustom bool
	tool := newAskChoiceTool(func(question string, options []ChoiceOption, allowCustom bool) error {
		gotQuestion = question
		gotOptions = options
		gotAllowCustom = allowCustom
		return nil
	})

	args, _ := json.Marshal(map[string]any{
		"question":     "Private or shared?",
		"options":      `[{"key":"A","label":"Private"},{"key":"B","label":"Shared","description":"Visible to the workspace"}]`,
		"allow_custom": true,
	})

	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result == "" {
		t.Fatal("Execute returned an empty result")
	}
	if gotQuestion != "Private or shared?" {
		t.Fatalf("question = %q", gotQuestion)
	}
	if len(gotOptions) != 2 || gotOptions[0].Key != "A" || gotOptions[1].Description != "Visible to the workspace" {
		t.Fatalf("options = %+v", gotOptions)
	}
	if !gotAllowCustom {
		t.Fatal("allowCustom = false, want true")
	}
}

func TestAskChoiceTool_RejectsInvalidOptionsJSON(t *testing.T) {
	tool := newAskChoiceTool(func(string, []ChoiceOption, bool) error {
		t.Fatal("callback should not run when options is invalid")
		return nil
	})
	args, _ := json.Marshal(map[string]any{"question": "Q?", "options": "not json"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for invalid options JSON, got nil")
	}
}

func TestAskChoiceTool_RejectsEmptyOptions(t *testing.T) {
	tool := newAskChoiceTool(func(string, []ChoiceOption, bool) error {
		t.Fatal("callback should not run for empty options")
		return nil
	})
	args, _ := json.Marshal(map[string]any{"question": "Q?", "options": "[]"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for empty options, got nil")
	}
}

func TestPendingChoice_GetReturnsNilUntilSet(t *testing.T) {
	p := &pendingChoice{}
	if p.get() != nil {
		t.Fatal("get() on a fresh pendingChoice should be nil")
	}
	p.set(ChoicePrompt{ID: "abc", Question: "Q?"})
	got := p.get()
	if got == nil || got.ID != "abc" {
		t.Fatalf("get() = %+v, want ID=abc", got)
	}
}

func TestCreateWorkbenchTool_CallsCallbackWithParsedArgs(t *testing.T) {
	var gotName, gotDescription string
	var gotVisibility workbench.Visibility
	tool := newCreateWorkbenchTool(&turnState{}, func(name, description string, visibility workbench.Visibility) (workbench.Workbench, error) {
		gotName, gotDescription, gotVisibility = name, description, visibility
		return workbench.Workbench{ID: "wb-1", Name: name, Visibility: visibility}, nil
	})

	args, _ := json.Marshal(map[string]any{
		"name": "Mense in Piemonte", "description": "Bandi FEASR/FSE+", "visibility": "shared",
	})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotName != "Mense in Piemonte" || gotDescription != "Bandi FEASR/FSE+" || gotVisibility != workbench.VisibilityShared {
		t.Fatalf("name=%q description=%q visibility=%q", gotName, gotDescription, gotVisibility)
	}
	if result == "" {
		t.Fatal("Execute returned an empty result")
	}
}

func TestCreateWorkbenchTool_DefaultsUnknownVisibilityToPrivate(t *testing.T) {
	var gotVisibility workbench.Visibility
	tool := newCreateWorkbenchTool(&turnState{}, func(_, _ string, visibility workbench.Visibility) (workbench.Workbench, error) {
		gotVisibility = visibility
		return workbench.Workbench{}, nil
	})

	args, _ := json.Marshal(map[string]any{"name": "X", "visibility": "public"})
	if _, err := tool.Execute(context.Background(), string(args)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotVisibility != workbench.VisibilityPrivate {
		t.Fatalf("visibility = %q, want private for an unrecognized value", gotVisibility)
	}
}

func TestCreateWorkbenchTool_RejectsMissingName(t *testing.T) {
	tool := newCreateWorkbenchTool(&turnState{}, func(string, string, workbench.Visibility) (workbench.Workbench, error) {
		t.Fatal("callback should not run without a name")
		return workbench.Workbench{}, nil
	})
	args, _ := json.Marshal(map[string]any{"visibility": "private"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for missing name, got nil")
	}
}

func TestSearchTendersTool_CallsCallbackWithParsedArgs(t *testing.T) {
	var gotQuery, gotCountry, gotCPV, gotStatus string
	tool := newSearchTendersTool(&turnState{}, func(query, country, cpv, status string) ([]tender.ScoredTender, error) {
		gotQuery, gotCountry, gotCPV, gotStatus = query, country, cpv, status
		return []tender.ScoredTender{{Tender: tender.Tender{ID: "1", Title: "Lavori stradali"}, RelevanceScore: 0.9}}, nil
	})

	args, _ := json.Marshal(map[string]any{
		"query": "road construction", "country": "IT", "cpv": "45", "status": "open",
	})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotQuery != "road construction" || gotCountry != "IT" || gotCPV != "45" || gotStatus != "open" {
		t.Fatalf("query=%q country=%q cpv=%q status=%q", gotQuery, gotCountry, gotCPV, gotStatus)
	}
	if !strings.Contains(result, `"id":"1"`) {
		t.Fatalf("result = %q, want it to contain the tender id", result)
	}
}

func TestSearchTendersTool_RejectsMissingQuery(t *testing.T) {
	tool := newSearchTendersTool(&turnState{}, func(string, string, string, string) ([]tender.ScoredTender, error) {
		t.Fatal("callback should not run without a query")
		return nil, nil
	})
	args, _ := json.Marshal(map[string]any{"country": "IT"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for missing query, got nil")
	}
}

func TestSearchTendersTool_PropagatesSearchError(t *testing.T) {
	tool := newSearchTendersTool(&turnState{}, func(string, string, string, string) ([]tender.ScoredTender, error) {
		return nil, errors.New("boom")
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want the search callback's error propagated")
	}
}

func TestSearchTendersTool_AddsNoticeAfterFiveConsecutiveEmptyResults(t *testing.T) {
	ts := &turnState{}
	tool := newSearchTendersTool(ts, func(string, string, string, string) ([]tender.ScoredTender, error) {
		return nil, nil
	})
	args, _ := json.Marshal(map[string]any{"query": "cestini intelligenti"})

	var lastResult string
	for i := 0; i < 5; i++ {
		result, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("Execute call %d: %v", i+1, err)
		}
		lastResult = result
		if i < 4 && strings.Contains(result, "STOP calling search_tenders") {
			t.Fatalf("call %d: notice appeared too early: %q", i+1, result)
		}
	}
	if !strings.Contains(lastResult, "STOP calling search_tenders") {
		t.Fatalf("5th call result = %q, want the broaden-search notice", lastResult)
	}
}

func TestSearchTendersTool_NonEmptyResultResetsEmptyStreak(t *testing.T) {
	empty := true
	ts := &turnState{}
	tool := newSearchTendersTool(ts, func(string, string, string, string) ([]tender.ScoredTender, error) {
		if empty {
			return nil, nil
		}
		return []tender.ScoredTender{{Tender: tender.Tender{ID: "1", Title: "Found one"}}}, nil
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})

	for i := 0; i < 4; i++ {
		if _, err := tool.Execute(context.Background(), string(args)); err != nil {
			t.Fatalf("Execute call %d: %v", i+1, err)
		}
	}
	empty = false
	if _, err := tool.Execute(context.Background(), string(args)); err != nil {
		t.Fatalf("Execute (reset call): %v", err)
	}
	empty = true
	for i := 0; i < 4; i++ {
		result, err := tool.Execute(context.Background(), string(args))
		if err != nil {
			t.Fatalf("Execute post-reset call %d: %v", i+1, err)
		}
		if strings.Contains(result, "STOP calling search_tenders") {
			t.Fatalf("post-reset call %d: notice appeared, streak should have reset: %q", i+1, result)
		}
	}
}

func TestSearchTendersTool_SearchErrorDoesNotAffectEmptyStreak(t *testing.T) {
	callCount := 0
	ts := &turnState{}
	tool := newSearchTendersTool(ts, func(string, string, string, string) ([]tender.ScoredTender, error) {
		callCount++
		if callCount == 3 {
			return nil, errors.New("boom")
		}
		return nil, nil
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})

	// Calls 1, 2: empty (streak -> 2). Call 3: errors — the streak must stay
	// untouched at 2, neither incremented to 3 nor reset to 0. Calls 4, 5: empty
	// (streak -> 3, 4 if the error correctly left it at 2 after call 2; only 4 by
	// call 5, one short of searchTendersEmptyStreakLimit=5, so no notice yet).
	// Call 6 must be the one that finally reaches 5 and triggers the notice —
	// proving the errored call (call 3) contributed nothing to the count either
	// way. If the error had instead reset the streak, call 6 would only reach a
	// streak of 3 (calls 4,5,6), and the notice would NOT appear — this test
	// would then fail, which is the point.
	for i := 0; i < 5; i++ {
		result, err := tool.Execute(context.Background(), string(args))
		if i == 2 {
			if err == nil {
				t.Fatalf("call %d: want the search error propagated, got nil", i+1)
			}
			continue
		}
		if err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
		if strings.Contains(result, "STOP calling search_tenders") {
			t.Fatalf("call %d: notice appeared too early: %q", i+1, result)
		}
	}
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("call 6: %v", err)
	}
	if !strings.Contains(result, "STOP calling search_tenders") {
		t.Fatalf("call 6 result = %q, want the notice (5 empty calls: 1,2,4,5,6 — call 3 errored and must not have contributed)", result)
	}
}

func TestSearchTendersTool_EmitsRunningThenDoneBreadcrumbs(t *testing.T) {
	ts := &turnState{}
	var got []ToolCall
	ts.sendToolCall = func(tc ToolCall) error { got = append(got, tc); return nil }

	tool := newSearchTendersTool(ts, func(string, string, string, string) ([]tender.ScoredTender, error) {
		return []tender.ScoredTender{{Tender: tender.Tender{ID: "1"}}}, nil
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})
	if _, err := tool.Execute(context.Background(), string(args)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []ToolCall{{Name: "search_tenders", Status: "running"}, {Name: "search_tenders", Status: "done"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("breadcrumbs = %+v, want %+v", got, want)
	}
}

func TestSearchTendersTool_EmitsDoneEvenOnSearchError(t *testing.T) {
	ts := &turnState{}
	var got []ToolCall
	ts.sendToolCall = func(tc ToolCall) error { got = append(got, tc); return nil }

	tool := newSearchTendersTool(ts, func(string, string, string, string) ([]tender.ScoredTender, error) {
		return nil, errors.New("boom")
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want the search error propagated")
	}
	if len(got) != 2 || got[1].Status != "done" {
		t.Fatalf("breadcrumbs = %+v, want a trailing done even on error", got)
	}
}

// ── get_tender_criteria ──

func ptrFloat(v float64) *float64 { return &v }

func ptrBool(v bool) *bool { return &v }

// readNotice is a completed, current read: EnrichedAt set means the stored
// detail describes the notice that is live now. Detail with GridUsable set but
// EnrichedAt nil is a real and different state — a notice superseded by a
// republication that has not been read yet — so a fixture that means "we read
// this" has to say so rather than leave the timestamp zero.
func readNotice() *time.Time {
	at := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	return &at
}

func TestGetTenderCriteriaTool_CallsCallbackWithParsedArgs(t *testing.T) {
	var gotTenderID string
	tool := newGetTenderCriteriaTool(&turnState{}, func(tenderID string) (tender.TenderDetail, error) {
		gotTenderID = tenderID
		return tender.TenderDetail{
			ID: "42", Title: "Servizio di ristorazione", BuyerName: "Comune di Torino",
			GridUsable:   ptrBool(true),
			EnrichedAt:   readNotice(),
			DocumentsURL: "https://example.test/docs",
			Criteria: []tender.AwardCriterion{
				{Ordinal: 1, Type: "quality", Name: "Offerta tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
				{Ordinal: 2, Type: "price", Name: "Offerta economica", Weight: ptrFloat(30), WeightRaw: "30"},
			},
		}, nil
	})

	args, _ := json.Marshal(map[string]any{"tender_id": "42"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTenderID != "42" {
		t.Fatalf("tender_id = %q", gotTenderID)
	}
	if !strings.Contains(result, `"tender_id":"42"`) || !strings.Contains(result, `"weight":70`) {
		t.Fatalf("result = %q, want the tender id and the published weights", result)
	}
	// A fully usable grid has nothing to warn about, so no notice may be added
	// — an unconditional one would train the model to ignore the field.
	if strings.Contains(result, `"notice"`) {
		t.Fatalf("result = %q, want no notice for a usable grid", result)
	}
}

func TestGetTenderCriteriaTool_RejectsMissingTenderID(t *testing.T) {
	tool := newGetTenderCriteriaTool(&turnState{}, func(string) (tender.TenderDetail, error) {
		t.Fatal("callback should not run without a tender_id")
		return tender.TenderDetail{}, nil
	})
	args, _ := json.Marshal(map[string]any{})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for missing tender_id, got nil")
	}
}

func TestGetTenderCriteriaTool_PropagatesLookupError(t *testing.T) {
	tool := newGetTenderCriteriaTool(&turnState{}, func(string) (tender.TenderDetail, error) {
		return tender.TenderDetail{}, tender.ErrTenderNotFound
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "999"})
	_, err := tool.Execute(context.Background(), string(args))
	if !errors.Is(err, tender.ErrTenderNotFound) {
		t.Fatalf("Execute error = %v, want it to wrap ErrTenderNotFound", err)
	}
}

// TestGetTenderCriteriaTool_RendersAbsentWeightAsExplicitNull guards the single
// most important property of this tool: a criterion with no published weight
// must reach the model as a stated null, never as an omitted key and never as
// zero. Absence is something the model has to infer, and the inference it
// reaches for is "the field did not apply"; a null it can report.
func TestGetTenderCriteriaTool_RendersAbsentWeightAsExplicitNull(t *testing.T) {
	tool := newGetTenderCriteriaTool(&turnState{}, func(string) (tender.TenderDetail, error) {
		return tender.TenderDetail{
			ID: "7", GridUsable: ptrBool(false), EnrichedAt: readNotice(),
			Criteria: []tender.AwardCriterion{
				{Ordinal: 1, Type: "quality", Name: "Offerta economicamente più vantaggiosa"},
			},
		}, nil
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "7"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, `"weight":null`) {
		t.Fatalf("result = %q, want an explicit \"weight\":null for an unweighted criterion", result)
	}
	if strings.Contains(result, `"weight":0`) {
		t.Fatalf("result = %q, an absent weight must never render as zero", result)
	}
	if !strings.Contains(result, "no weighted scoring grid") {
		t.Fatalf("result = %q, want the no-grid notice when grid_usable is false", result)
	}
}

// TestGetTenderCriteriaTool_UnreadNoticeIsNotAnAbsentGrid is the
// inaccessible-versus-absent invariant at the tool boundary: grid_usable nil
// must survive as JSON null and carry the "we have not looked" notice, because
// an empty criteria list with no explanation reads as "the buyer published
// none".
func TestGetTenderCriteriaTool_UnreadNoticeIsNotAnAbsentGrid(t *testing.T) {
	tool := newGetTenderCriteriaTool(&turnState{}, func(string) (tender.TenderDetail, error) {
		return tender.TenderDetail{ID: "8"}, nil
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "8"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, `"grid_usable":null`) {
		t.Fatalf("result = %q, want grid_usable to stay null when the notice was never read", result)
	}
	if !strings.Contains(result, "has NOT been read yet") {
		t.Fatalf("result = %q, want the notice-not-read warning", result)
	}
	if !strings.Contains(result, `"criteria":[]`) {
		t.Fatalf("result = %q, want an empty array rather than null for criteria", result)
	}
}

// TestGetTenderCriteriaTool_SupersededDetailIsNotReportedAsCurrent covers the
// window that the grid_usable flag alone cannot describe.
//
// A call for tenders and its later award notice collapse onto ONE tender row,
// because source_ref is the procedure identifier. Re-ingesting the award notice
// bumps the publication number and re-queues the row for enrichment, but leaves
// the criteria, grid_usable and xml_status exactly as the superseded notice
// wrote them. For the whole of that window — at minimum until the next
// enrichment pass — the stored grid describes a notice that is no longer
// current, and EnrichedAt is the only field that says so.
//
// Branching on grid_usable alone would emit that grid as `true` with no notice
// at all: not a gap in the answer, a wrong answer stated confidently, on the
// one tool whose entire purpose is to keep "we cannot read it" distinct from
// "there is nothing to read".
func TestGetTenderCriteriaTool_SupersededDetailIsNotReportedAsCurrent(t *testing.T) {
	tool := newGetTenderCriteriaTool(&turnState{}, func(string) (tender.TenderDetail, error) {
		return tender.TenderDetail{
			ID:         "9",
			GridUsable: ptrBool(true), // written by the notice that has been superseded
			EnrichedAt: nil,           // ...and cleared when the newer one was ingested
			Criteria: []tender.AwardCriterion{
				{Ordinal: 1, Type: "quality", Name: "Offerta tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
			},
		}, nil
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "9"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, `"grid_usable":null`) {
		t.Fatalf("result = %q, want grid_usable null: the stored flag describes a superseded notice", result)
	}
	if !strings.Contains(result, "come from an EARLIER notice") {
		t.Fatalf("result = %q, want the superseded warning", result)
	}
	// The criteria themselves are still worth showing — an out-of-date grid the
	// model is TOLD is out of date is useful, unlike one it believes is current.
	if !strings.Contains(result, `"weight":70`) {
		t.Fatalf("result = %q, want the superseded criteria retained alongside the warning", result)
	}
	if strings.Contains(result, "has NOT been read yet") {
		t.Fatalf("result = %q, the not-read notice is wrong here — we have read a notice, just not this one", result)
	}
}

func TestGetTenderCriteriaTool_EmitsRunningThenDoneBreadcrumbs(t *testing.T) {
	ts := &turnState{}
	var got []ToolCall
	ts.sendToolCall = func(tc ToolCall) error { got = append(got, tc); return nil }

	tool := newGetTenderCriteriaTool(ts, func(string) (tender.TenderDetail, error) {
		return tender.TenderDetail{ID: "1", GridUsable: ptrBool(true)}, nil
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "1"})
	if _, err := tool.Execute(context.Background(), string(args)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []ToolCall{{Name: "get_tender_criteria", Status: "running"}, {Name: "get_tender_criteria", Status: "done"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("breadcrumbs = %+v, want %+v", got, want)
	}
}

// TestCollapseIdenticalCriteria_FoldsRepeatedPerLotGrid is the token-budget
// property: the 13-lot notice that restates one grid per lot must reach the
// model as the grid, once, listing the lots it covers.
func TestCollapseIdenticalCriteria_FoldsRepeatedPerLotGrid(t *testing.T) {
	var in []tender.AwardCriterion
	for _, lot := range []string{"LOT-0001", "LOT-0002", "LOT-0003"} {
		in = append(in,
			tender.AwardCriterion{LotRef: lot, Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
			tender.AwardCriterion{LotRef: lot, Ordinal: 2, Type: "price", Name: "Economica", Weight: ptrFloat(30), WeightRaw: "30"},
		)
	}
	out := collapseIdenticalCriteria(in)
	if len(out) != 2 {
		t.Fatalf("collapsed to %d entries, want 2", len(out))
	}
	if len(out[0].LotRefs) != 3 || out[0].LotRefs[0] != "LOT-0001" || out[0].LotRefs[2] != "LOT-0003" {
		t.Fatalf("lot refs = %v, want all three lots on the folded entry", out[0].LotRefs)
	}
	if out[0].Ordinal != 1 || out[1].Ordinal != 2 {
		t.Fatalf("published order not preserved: %+v", out)
	}
}

// TestCollapseIdenticalCriteria_KeepsGenuinelyPerLotWeights proves the fold is
// keyed on the criterion's content, so a grid that really does differ per lot
// survives intact — the fact a parse-time dedupe would destroy.
func TestCollapseIdenticalCriteria_KeepsGenuinelyPerLotWeights(t *testing.T) {
	in := []tender.AwardCriterion{
		{LotRef: "LOT-0001", Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
		{LotRef: "LOT-0002", Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(60), WeightRaw: "60"},
	}
	out := collapseIdenticalCriteria(in)
	if len(out) != 2 {
		t.Fatalf("collapsed to %d entries, want 2 — differing weights are different criteria", len(out))
	}
}

// TestCollapseIdenticalCriteria_AbsentWeightNeverFoldsIntoZero is the same
// nil-is-not-zero invariant, one layer down: if the fold keyed on a coalesced
// weight, an unweighted criterion and a zero-weighted one would merge and the
// distinction would be gone before the model ever saw it.
func TestCollapseIdenticalCriteria_AbsentWeightNeverFoldsIntoZero(t *testing.T) {
	in := []tender.AwardCriterion{
		{LotRef: "LOT-0001", Ordinal: 1, Type: "quality", Name: "Tecnica"},
		{LotRef: "LOT-0002", Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(0), WeightRaw: "0"},
	}
	out := collapseIdenticalCriteria(in)
	if len(out) != 2 {
		t.Fatalf("collapsed to %d entries, want 2 — an absent weight is not a zero weight", len(out))
	}
}

// TestCollapseIdenticalCriteria_NoticeLevelEntryCarriesNoLotRefs: a
// notice-level criterion applies to every lot, which is strictly more general
// than any list of refs, so a group containing one must emit none — including
// when a per-lot restatement of the same criterion follows it.
func TestCollapseIdenticalCriteria_NoticeLevelEntryCarriesNoLotRefs(t *testing.T) {
	in := []tender.AwardCriterion{
		{LotRef: "", Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
		{LotRef: "LOT-0001", Ordinal: 1, Type: "quality", Name: "Tecnica", Weight: ptrFloat(70), WeightRaw: "70"},
	}
	out := collapseIdenticalCriteria(in)
	if len(out) != 1 {
		t.Fatalf("collapsed to %d entries, want 1", len(out))
	}
	if len(out[0].LotRefs) != 0 {
		t.Fatalf("lot refs = %v, want none on a notice-level criterion", out[0].LotRefs)
	}
}

func TestMarshalGetTenderCriteriaResult_CapsCriteriaAndSaysSo(t *testing.T) {
	var in []tender.AwardCriterion
	for i := 0; i < tenderCriteriaToolLimit+5; i++ {
		in = append(in, tender.AwardCriterion{LotRef: "LOT-0001", Ordinal: i, Type: "quality", Name: "C"})
	}
	result, err := marshalGetTenderCriteriaResult(tender.TenderDetail{ID: "1", GridUsable: ptrBool(true), Criteria: in})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var parsed getTenderCriteriaResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed.Criteria) != tenderCriteriaToolLimit {
		t.Fatalf("criteria = %d, want capped at %d", len(parsed.Criteria), tenderCriteriaToolLimit)
	}
	if !strings.Contains(parsed.Notice, "criterion entries are shown") {
		t.Fatalf("notice = %q, want the truncation warning", parsed.Notice)
	}
}

// ── read_tender_documents ──

func TestReadTenderDocumentsTool_CallsCallbackWithParsedArgs(t *testing.T) {
	var gotTenderID, gotQuestion string
	pageStart, pageEnd := 14, 16
	tool := newReadTenderDocumentsTool(&turnState{}, func(tenderID, question string) (document.ExcerptResult, error) {
		gotTenderID, gotQuestion = tenderID, question
		return document.ExcerptResult{
			Availability: document.Availability{
				TenderID: 42, Coverage: document.CoverageFull, Reason: document.ReasonNone,
				NoticeRead: true, ExtractedDocuments: 2, ExtractedParts: 180,
			},
			Excerpts: []document.Excerpt{{
				Text: "Il concorrente deve dimostrare...", RelevanceScore: 0.8,
				Citation: document.Citation{
					DocumentURL: "https://example.test/disciplinare.pdf", DocumentType: "spec",
					PartIndex: 12, PageStart: &pageStart, PageEnd: &pageEnd,
					SectionPath: []string{"7", "7.2"}, SectionTitle: "Requisiti di capacità tecnica",
					HasTable: true,
				},
			}},
		}, nil
	})

	args, _ := json.Marshal(map[string]any{"tender_id": "42", "question": "requisiti di capacità tecnica"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotTenderID != "42" || gotQuestion != "requisiti di capacità tecnica" {
		t.Fatalf("tender_id=%q question=%q", gotTenderID, gotQuestion)
	}
	for _, want := range []string{
		`"coverage":"full"`,
		`"document_url":"https://example.test/disciplinare.pdf"`,
		`"page":"14-16"`,
		`"section":"7.2 Requisiti di capacità tecnica"`,
		`"has_table":true`,
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("result = %q, want it to contain %s", result, want)
		}
	}
	// Full coverage with matching passages is the one state with nothing to
	// explain, so no notice may be attached.
	if strings.Contains(result, `"notice"`) {
		t.Fatalf("result = %q, want no notice for full coverage with hits", result)
	}
}

func TestReadTenderDocumentsTool_RejectsMissingArguments(t *testing.T) {
	tool := newReadTenderDocumentsTool(&turnState{}, func(string, string) (document.ExcerptResult, error) {
		t.Fatal("callback should not run with incomplete arguments")
		return document.ExcerptResult{}, nil
	})
	for _, args := range []map[string]any{
		{"question": "penali"},
		{"tender_id": "42"},
		{"tender_id": "42", "question": "   "},
	} {
		encoded, _ := json.Marshal(args)
		if _, err := tool.Execute(context.Background(), string(encoded)); err == nil {
			t.Fatalf("Execute(%v): want an error, got nil", args)
		}
	}
}

func TestReadTenderDocumentsTool_PropagatesReadError(t *testing.T) {
	tool := newReadTenderDocumentsTool(&turnState{}, func(string, string) (document.ExcerptResult, error) {
		return document.ExcerptResult{}, document.ErrTenderNotFound
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "999", "question": "penali"})
	_, err := tool.Execute(context.Background(), string(args))
	if !errors.Is(err, document.ErrTenderNotFound) {
		t.Fatalf("Execute error = %v, want it to wrap ErrTenderNotFound", err)
	}
}

// TestReadTenderDocumentsTool_UnretrievedBodyIsNotAnAbsentDocument is the
// invariant this whole feature exists for, at the tool boundary: a tender
// whose specification sits behind a link we have not fetched must come back
// carrying both halves of the answer AND the instruction not to report it as
// "this tender has no documents".
func TestReadTenderDocumentsTool_UnretrievedBodyIsNotAnAbsentDocument(t *testing.T) {
	tool := newReadTenderDocumentsTool(&turnState{}, func(string, string) (document.ExcerptResult, error) {
		return document.ExcerptResult{
			Availability: document.Availability{
				TenderID: 42, Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
				NoticeRead: true, KnownDocumentLinks: 1, ExtractedDocuments: 1, ExtractedParts: 4,
			},
		}, nil
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "42", "question": "penali per ritardo"})
	result, err := tool.Execute(context.Background(), string(args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, `"coverage":"notice_only"`) || !strings.Contains(result, `"reason":"body_not_retrieved"`) {
		t.Fatalf("result = %q, want coverage and reason as two separate fields", result)
	}
	if !strings.Contains(result, "has not retrieved them yet") {
		t.Fatalf("result = %q, want the body-not-retrieved notice", result)
	}
	if !strings.Contains(result, `"excerpts":[]`) {
		t.Fatalf("result = %q, want an empty array rather than null for excerpts", result)
	}
}

// TestDocumentResultNotices_NoMatchStacksOnCoverage: "we hold only the notice
// PDF" and "nothing in what we hold mentions this" are both true at once, and
// the model needs both to answer honestly.
func TestDocumentResultNotices_NoMatchStacksOnCoverage(t *testing.T) {
	got := documentResultNotices(document.ExcerptResult{
		Availability: document.Availability{
			Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
			NoticeRead: true, KnownDocumentLinks: 1,
		},
	})
	if len(got) != 2 {
		t.Fatalf("notices = %v, want both the coverage cause and the no-match hint", got)
	}
}

// TestDocumentResultNotices_NoMatchSuppressedWithoutText: with no extracted
// text at all, the cause has already explained the empty list, and telling the
// model to reword its question would send it round a loop that cannot succeed.
func TestDocumentResultNotices_NoMatchSuppressedWithoutText(t *testing.T) {
	got := documentResultNotices(document.ExcerptResult{
		Availability: document.Availability{
			Coverage: document.CoverageNone, Reason: document.ReasonNotYetExtracted,
		},
	})
	if len(got) != 1 || !strings.Contains(got[0], "queued for text extraction") {
		t.Fatalf("notices = %v, want only the not-yet-extracted cause", got)
	}
}

func TestFormatCitation_DegradesWhenMetadataIsAbsent(t *testing.T) {
	// The corpus extracted before page/section columns existed keeps NULL for
	// all of them, so a citation must render down to the document URL alone.
	if page := formatCitationPage(document.Citation{}); page != "" {
		t.Fatalf("page = %q, want empty for a citation with no page metadata", page)
	}
	if section := formatCitationSection(document.Citation{}); section != "" {
		t.Fatalf("section = %q, want empty for a citation with no section metadata", section)
	}
	single := 9
	if page := formatCitationPage(document.Citation{PageStart: &single, PageEnd: &single}); page != "9" {
		t.Fatalf("page = %q, want a single page rather than a degenerate range", page)
	}
	if section := formatCitationSection(document.Citation{SectionTitle: "Penali"}); section != "Penali" {
		t.Fatalf("section = %q, want the title alone when the path is unknown", section)
	}
}

func TestReadTenderDocumentsTool_EmitsRunningThenDoneBreadcrumbs(t *testing.T) {
	ts := &turnState{}
	var got []ToolCall
	ts.sendToolCall = func(tc ToolCall) error { got = append(got, tc); return nil }

	tool := newReadTenderDocumentsTool(ts, func(string, string) (document.ExcerptResult, error) {
		return document.ExcerptResult{}, errors.New("boom")
	})
	args, _ := json.Marshal(map[string]any{"tender_id": "1", "question": "penali"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want the read error propagated")
	}
	want := []ToolCall{{Name: "read_tender_documents", Status: "running"}, {Name: "read_tender_documents", Status: "done"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("breadcrumbs = %+v, want a trailing done even on error", got)
	}
}

func TestCreateWorkbenchTool_EmitsRunningThenDoneBreadcrumbs(t *testing.T) {
	ts := &turnState{}
	var got []ToolCall
	ts.sendToolCall = func(tc ToolCall) error { got = append(got, tc); return nil }

	tool := newCreateWorkbenchTool(ts, func(name, _ string, v workbench.Visibility) (workbench.Workbench, error) {
		return workbench.Workbench{ID: "wb-1", Name: name, Visibility: v}, nil
	})
	args, _ := json.Marshal(map[string]any{"name": "X", "visibility": "private"})
	if _, err := tool.Execute(context.Background(), string(args)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := []ToolCall{{Name: "create_workbench", Status: "running"}, {Name: "create_workbench", Status: "done"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("breadcrumbs = %+v, want %+v", got, want)
	}
}
