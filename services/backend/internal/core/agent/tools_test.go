package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

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

// TestTurnStateFor_ReturnsSamePointerAcrossCalls proves the core invariant
// the turnState pause/resume fix depends on: turnStateFor must hand back the
// SAME *turnState for a given sessionID on every call (including a
// GetOrCreateChat cache hit on turn 2+), so runTurn's field writes on the
// most recent call are visible to whichever tool closure berrygem actually
// invokes — see the "Why turnState exists" note on runTurn in service.go.
// Without this, the whole fix is a no-op.
func TestTurnStateFor_ReturnsSamePointerAcrossCalls(t *testing.T) {
	svc := &Service{turnStates: make(map[string]*turnState)}
	first := svc.turnStateFor("session-1")
	second := svc.turnStateFor("session-1")
	if first != second {
		t.Fatal("turnStateFor returned different pointers for the same sessionID")
	}
	other := svc.turnStateFor("session-2")
	if other == first {
		t.Fatal("turnStateFor returned the same pointer for a different sessionID")
	}
}

func TestCreateWorkbenchTool_CallsCallbackWithParsedArgs(t *testing.T) {
	var gotName, gotDescription string
	var gotVisibility workbench.Visibility
	tool := newCreateWorkbenchTool(func(name, description string, visibility workbench.Visibility) (workbench.Workbench, error) {
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
	tool := newCreateWorkbenchTool(func(_, _ string, visibility workbench.Visibility) (workbench.Workbench, error) {
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
	tool := newCreateWorkbenchTool(func(string, string, workbench.Visibility) (workbench.Workbench, error) {
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
