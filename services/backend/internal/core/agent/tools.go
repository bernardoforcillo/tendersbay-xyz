package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/buildwithgo/berrygem/providers"
	"github.com/buildwithgo/berrygem/tools"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// ChoiceOption is one option in a ChoicePrompt.
type ChoiceOption struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ChoicePrompt is a closed-ended question the agent asked, waiting on the
// user's answer via SubmitChoice. ID is the chat_messages.id of the
// persisted "choice_prompt" row — the same value the client must send back
// as SubmitChoiceRequest.choice_id.
type ChoicePrompt struct {
	ID          string
	Question    string
	Options     []ChoiceOption
	AllowCustom bool
}

// newAskChoiceTool builds the generic "ask the user a closed-ended
// question" tool. berrygem's providers.Property has no array/object
// nesting, so `options` is declared as a JSON-encoded string parameter —
// the model must emit a JSON array of {"key","label","description"}
// objects as a string, which this tool parses.
//
// askChoice is called synchronously from within Execute; the caller
// (Service.runTurn) is responsible for persisting the prompt and
// cancelling the run's context after this returns nil.
func newAskChoiceTool(askChoice func(question string, options []ChoiceOption, allowCustom bool) error) tools.Tool {
	return tools.NewFunc(
		"ask_choice",
		"Ask the user a closed-ended question with a small set of options and wait for their answer. "+
			"Use this whenever you need the user to confirm or pick among specific choices before proceeding "+
			"(for example, before calling create_workbench). This ends your turn immediately — do not produce "+
			"any further text or tool calls after invoking it; the conversation resumes automatically once the "+
			"user answers.",
		map[string]providers.Property{
			"question": {
				Type:        "string",
				Description: "The question to ask the user.",
			},
			"options": {
				Type: "string",
				Description: `A JSON array of options, e.g. ` +
					`[{"key":"A","label":"Yes","description":"optional detail"},{"key":"B","label":"No"}]. ` +
					`Each option needs "key" and "label"; "description" is optional. Must not be empty.`,
			},
			"allow_custom": {
				Type:        "boolean",
				Description: "Whether the user may type a free-form answer instead of picking an option. Defaults to false.",
			},
		},
		[]string{"question", "options"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				Question    string `json:"question"`
				Options     string `json:"options"`
				AllowCustom bool   `json:"allow_custom"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("ask_choice: invalid arguments: %w", err)
			}
			var options []ChoiceOption
			if err := json.Unmarshal([]byte(parsed.Options), &options); err != nil {
				return "", fmt.Errorf("ask_choice: options is not a valid JSON array: %w", err)
			}
			if len(options) == 0 {
				return "", fmt.Errorf("ask_choice: options must not be empty")
			}
			if err := askChoice(parsed.Question, options, parsed.AllowCustom); err != nil {
				return "", err
			}
			return "Question sent to the user. Waiting for their answer.", nil
		},
	)
}

// newCreateWorkbenchTool builds the "create a workbench" tool. It takes a
// plain callback rather than a WorkbenchCreator + userID/workspaceID
// directly: the tool is constructed once per Service.runTurn call, but
// Registry.GetOrCreateChat only actually uses a freshly-built agent (and
// its tools) on a session's FIRST turn — every later turn's chat.Chat
// reuses turn 1's agent, discarding whatever this task's tool construction
// built for that later call. If this tool closed over userID/workspaceID
// (or a context) directly, it would silently keep using turn 1's
// already-stale values forever. Service.runTurn's callback instead reads
// the current turn's identity from its turnState at call time — see
// service.go's turnState doc comment (added in this task's Step 4) for the
// full explanation. This mirrors ask_choice's existing callback shape
// exactly (Task 2) — same reasoning, same fix.
func newCreateWorkbenchTool(createWorkbench func(name, description string, visibility workbench.Visibility) (workbench.Workbench, error)) tools.Tool {
	return tools.NewFunc(
		"create_workbench",
		"Create a new workbench in the user's current workspace. Always confirm the name and visibility "+
			"with the user via ask_choice before calling this — never call it speculatively or without a prior "+
			"confirmed answer.",
		map[string]providers.Property{
			"name": {
				Type:        "string",
				Description: "The workbench name.",
			},
			"description": {
				Type:        "string",
				Description: "A short description of the workbench's purpose. Optional.",
			},
			"visibility": {
				Type:        "string",
				Description: "Who can see the workbench.",
				Enum:        []string{"private", "shared"},
			},
		},
		[]string{"name", "visibility"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Visibility  string `json:"visibility"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("create_workbench: invalid arguments: %w", err)
			}
			if parsed.Name == "" {
				return "", fmt.Errorf("create_workbench: name is required")
			}
			visibility := workbench.VisibilityPrivate
			if parsed.Visibility == string(workbench.VisibilityShared) {
				visibility = workbench.VisibilityShared
			}
			wb, err := createWorkbench(parsed.Name, parsed.Description, visibility)
			if err != nil {
				return "", fmt.Errorf("create_workbench: %w", err)
			}
			result, err := json.Marshal(map[string]string{
				"id": wb.ID, "name": wb.Name, "visibility": string(wb.Visibility),
			})
			if err != nil {
				return "", err
			}
			return string(result), nil
		},
	)
}

// searchTendersToolLimit caps how many results the model sees per call —
// enough to reason about, small enough to stay cheap in the model's context.
const searchTendersToolLimit = 5

// searchTendersEmptyStreakLimit is how many consecutive zero-result
// search_tenders calls within one turn trigger the broaden-the-search
// notice below.
const searchTendersEmptyStreakLimit = 5

// searchTendersEmptyStreakNotice is appended to the tool's JSON result once
// searchTendersEmptyStreakLimit consecutive empty searches have happened in
// this turn. This relies on prompt-based control (the same accepted pattern
// already used to enforce ask_choice before create_workbench — see
// newCreateWorkbenchTool's doc comment) rather than a hard code-level stop:
// berrygem gives this tool no way to end the turn itself the way
// ask_choice's callback does.
const searchTendersEmptyStreakNotice = "You have searched 5 times with zero results. STOP calling " +
	"search_tenders. Call ask_choice now, offering 3-4 broader alternative search terms or CPV " +
	"categories as clickable options, and briefly explain that no exact matches were found."

// newSearchTendersTool builds the "search live EU tenders" tool — a plain,
// client-agnostic search on model-provided terms (v1.0 does not auto-scope
// this to a ClientProfile; see the design spec's architecture section for
// why). Same callback-reads-current-turnState shape as
// newCreateWorkbenchTool — see that function's doc comment for the full
// stale-closure rationale.
//
// emptyStreak is captured by the closure below, not passed in: newSearchTendersTool
// is called once per Service.runTurn (service.go), so it naturally resets to 0 every
// turn and persists correctly across that turn's repeated tool calls.
func newSearchTendersTool(search func(query, country, cpv, status string) ([]tender.ScoredTender, error)) tools.Tool {
	emptyStreak := 0
	return tools.NewFunc(
		"search_tenders",
		"Search live EU public tenders by free-text query and optional filters. Use this whenever "+
			"the user asks about tenders, procurement opportunities, or a specific sector/country. Only "+
			"report tenders this tool actually returns — never invent tender details.",
		map[string]providers.Property{
			"query": {
				Type:        "string",
				Description: "Free-text search query, e.g. 'road construction Milan'.",
			},
			"country": {
				Type:        "string",
				Description: "Optional alpha-2 country filter, e.g. 'IT'.",
			},
			"cpv": {
				Type:        "string",
				Description: "Optional CPV code prefix filter, e.g. '45' for construction.",
			},
			"status": {
				Type:        "string",
				Description: "Optional status filter.",
				Enum:        []string{"open", "awarded", "cancelled", "closed"},
			},
		},
		[]string{"query"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				Query   string `json:"query"`
				Country string `json:"country"`
				CPV     string `json:"cpv"`
				Status  string `json:"status"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("search_tenders: invalid arguments: %w", err)
			}
			if parsed.Query == "" {
				return "", fmt.Errorf("search_tenders: query is required")
			}
			results, err := search(parsed.Query, parsed.Country, parsed.CPV, parsed.Status)
			if err != nil {
				return "", fmt.Errorf("search_tenders: %w", err)
			}
			if len(results) == 0 {
				emptyStreak++
			} else {
				emptyStreak = 0
			}
			notice := ""
			if emptyStreak >= searchTendersEmptyStreakLimit {
				notice = searchTendersEmptyStreakNotice
			}
			return marshalSearchTendersResult(results, notice)
		},
	)
}

// searchTendersResultItem is the compact JSON shape the model sees per
// result — raw fields only, no fit tier or reason (that's the deterministic
// RecommendTendersForClient RPC's job, not this tool's).
type searchTendersResultItem struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	BuyerName      string  `json:"buyer_name"`
	Country        string  `json:"country"`
	CPV            string  `json:"cpv"`
	Value          *int64  `json:"value,omitempty"`
	Deadline       string  `json:"deadline,omitempty"`
	RelevanceScore float64 `json:"relevance_score"`
}

// searchTendersResult is the tool's full JSON payload — Notice is only set
// once searchTendersEmptyStreakLimit consecutive empty searches have
// happened in this turn (see newSearchTendersTool).
type searchTendersResult struct {
	Results []searchTendersResultItem `json:"results"`
	Notice  string                    `json:"notice,omitempty"`
}

func marshalSearchTendersResult(results []tender.ScoredTender, notice string) (string, error) {
	items := make([]searchTendersResultItem, len(results))
	for i, r := range results {
		var deadline string
		if r.Deadline != nil {
			deadline = r.Deadline.Format(time.RFC3339)
		}
		items[i] = searchTendersResultItem{
			ID: r.ID, Title: r.Title, BuyerName: r.BuyerName, Country: r.Country, CPV: r.CPV,
			Value: r.Value, Deadline: deadline, RelevanceScore: r.RelevanceScore,
		}
	}
	b, err := json.Marshal(searchTendersResult{Results: items, Notice: notice})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
