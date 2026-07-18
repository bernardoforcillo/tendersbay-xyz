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
	tool := newSearchTendersTool(func(query, country, cpv, status string) ([]tender.ScoredTender, error) {
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
	tool := newSearchTendersTool(func(string, string, string, string) ([]tender.ScoredTender, error) {
		t.Fatal("callback should not run without a query")
		return nil, nil
	})
	args, _ := json.Marshal(map[string]any{"country": "IT"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want error for missing query, got nil")
	}
}

func TestSearchTendersTool_PropagatesSearchError(t *testing.T) {
	tool := newSearchTendersTool(func(string, string, string, string) ([]tender.ScoredTender, error) {
		return nil, errors.New("boom")
	})
	args, _ := json.Marshal(map[string]any{"query": "x"})
	if _, err := tool.Execute(context.Background(), string(args)); err == nil {
		t.Fatal("Execute: want the search callback's error propagated")
	}
}
