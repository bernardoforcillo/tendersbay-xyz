package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/buildwithgo/berrygem/providers"
	"github.com/buildwithgo/berrygem/tools"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
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

// TenderResults is a batch of live search_tenders results the agent should
// show the user as structured cards, not prose — pushed to the client via
// SendTenderResults the moment search_tenders returns at least one result.
type TenderResults struct {
	Tenders []tender.ScoredTender
}

// ToolCall is a breadcrumb the agent emits when it starts ("running") or
// finishes ("done") a tool — pushed to the client via SendToolCall so the UI
// can show a "proof of work" chip. Emitted from the tool's own Execute, not
// inferred from model output.
type ToolCall struct {
	Name   string
	Status string // "running" | "done"
}

// SendToolCall is wired by the ConnectRPC handler to stream.Send, mirroring
// SendTenderResults. Sending it never ends the turn.
type SendToolCall func(ToolCall) error

// emitToolCall best-effort pushes a breadcrumb using this turn's sendToolCall,
// read from turnState under its mutex because berrygem executes tools on its
// own goroutines. Nil-safe and error-swallowing: a dropped breadcrumb must
// never fail a turn.
func emitToolCall(ts *turnState, name, status string) {
	if send := ts.currentSendToolCall(); send != nil {
		_ = send(ToolCall{Name: name, Status: status})
	}
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
// plain callback rather than a WorkbenchCreator plus userID/workspaceID: the
// caller's identity, the turn's context and the stream callbacks all belong to
// one turn, and Service.runTurn keeps them together in that turn's turnState
// rather than baking them into the tool. The tool and its turnState are both
// built and discarded per turn, so this is now a plain "pass the turn's
// collaborators through one struct" arrangement — it mirrors ask_choice's
// callback shape for the same reason.
func newCreateWorkbenchTool(ts *turnState, createWorkbench func(name, description string, visibility workbench.Visibility) (workbench.Workbench, error)) tools.Tool {
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
			emitToolCall(ts, "create_workbench", "running")
			defer emitToolCall(ts, "create_workbench", "done")
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
// why). Same reads-the-turn's-turnState shape as newCreateWorkbenchTool — see
// that function's doc comment for why the turn's identity, context and
// callbacks travel through ts rather than being baked into the closure.
//
// The consecutive-empty-search streak is tracked on ts (turnState) rather than
// in a closure variable so the count is well-defined when berrygem executes
// several search_tenders calls concurrently within one turn —
// turnState.recordSearchResult (service.go) does the increment under a mutex.
// The streak is per-turn because the turnState is: runTurn allocates a fresh
// one for every turn.
func newSearchTendersTool(ts *turnState, search func(query, country, cpv, status string) ([]tender.ScoredTender, error)) tools.Tool {
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
			emitToolCall(ts, "search_tenders", "running")
			defer emitToolCall(ts, "search_tenders", "done")
			results, err := search(parsed.Query, parsed.Country, parsed.CPV, parsed.Status)
			if err != nil {
				return "", fmt.Errorf("search_tenders: %w", err)
			}
			streak := ts.recordSearchResult(len(results) == 0)
			notice := ""
			if streak >= searchTendersEmptyStreakLimit {
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

// tenderResultsCardItem is the JSON shape persisted in chat_messages.tenders
// for a "tender_results" row — camelCase to match the frontend's TenderResult
// consumption directly (contrast searchTendersResultItem's snake_case, which
// is for the LLM's tool-result JSON, a different consumer with different
// conventions).
type tenderResultsCardItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	BuyerName string `json:"buyerName"`
	Status    string `json:"status"`
	Country   string `json:"country"`
	CPV       string `json:"cpv"`
	Value     int64  `json:"value"`
	Currency  string `json:"currency"`
	Deadline  string `json:"deadline,omitempty"`
	Source    string `json:"source"`
}

func marshalTenderResultsForHistory(results []tender.ScoredTender) (json.RawMessage, error) {
	items := make([]tenderResultsCardItem, len(results))
	for i, r := range results {
		var value int64
		if r.Value != nil {
			value = *r.Value
		}
		var deadline string
		if r.Deadline != nil {
			deadline = r.Deadline.Format(time.RFC3339)
		}
		items[i] = tenderResultsCardItem{
			ID: r.ID, Title: r.Title, BuyerName: r.BuyerName, Status: r.Status,
			Country: r.Country, CPV: r.CPV, Value: value, Currency: r.Currency,
			Deadline: deadline, Source: r.Source,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// ── get_tender_criteria ──
//
// Everything below returns its payload as the Execute callback's own string
// return value and nothing else: berrygem places that string into the turn's
// message list as a tool result, and nothing here is persisted.
//
// The consequence, stated rather than hidden: a criteria grid the model read
// on one turn is not in the next turn's context, because that context is
// assembled from the persisted rows (conversationForTurn, service.go) and no
// row was written. Unlike a search, the loss is self-healing —
// get_tender_criteria is deterministic, idempotent, keyed by a stable id and
// generously rate-limited, so the model simply calls it again.
//
// Persisting these results is a real option now that conversationForTurn can
// carry a role whose payload lives outside `content` (tender_results already
// does). It is Phase 1b's decision, not a workaround being preserved: it needs
// a new chatRole plus a mapper case, and a judgement about how much of a
// criteria grid is worth re-sending on every subsequent turn.

// tenderCriteriaToolLimit caps how many criterion entries the model sees in
// one result. A 13-lot notice that restates a 5-criterion grid once per lot
// yields 65 entries; collapseIdenticalCriteria normally folds those back to 5,
// and this is the backstop for the notice that varies one field per lot and so
// does not collapse at all.
const tenderCriteriaToolLimit = 40

// tenderCriteriaToolLotLimit caps the lot list carried alongside the criteria.
// A lot entry is cheap on its own (~150 characters) but unbounded in count,
// and the model needs the list only to map a criterion's lot_refs onto
// something nameable — not to enumerate a 200-lot framework agreement.
const tenderCriteriaToolLotLimit = 25

// The get_tender_criteria notices are the prompt-based steering channel, the
// same mechanism searchTendersEmptyStreakNotice uses and for the same reason:
// berrygem gives a tool no way to constrain the model except through what it
// returns. They are what carries "inaccessible is not the same as absent" all
// the way to the model, per-call, at the moment it would otherwise guess.
const (
	tenderCriteriaNoticeNotRead = "This tender's structured notice has NOT been read yet. An empty " +
		"criteria list here means we have not looked, NOT that the buyer published none. Say exactly " +
		"that to the user; do not state that this tender has no award criteria."
	tenderCriteriaNoticeNoGrid = "This notice publishes no weighted scoring grid — at most an award " +
		"method. The actual grid is in the disciplinare at documents_url, which tendersbay cannot read " +
		"yet. Tell the user where the grid lives and that we cannot read it; do not estimate weights."
	tenderCriteriaNoticeSuperseded = "These criteria come from an EARLIER notice for this procurement; " +
		"a newer one has been published and has not been read yet, so they may be out of date. Say that " +
		"before quoting any of them, and do not present them as the current award grid."
	tenderCriteriaNoticeTruncatedFmt = "Only the first %d of %d criterion entries are shown; ask about a " +
		"specific lot if you need the rest."
	tenderCriteriaNoticeLotsTruncatedFmt = "Only the first %d of %d lots are shown."
)

// newGetTenderCriteriaTool builds the "read one tender's published scoring
// grid" tool. Same reads-the-turn's-turnState shape as newSearchTendersTool —
// see newCreateWorkbenchTool's doc comment for why the turn's identity,
// context and callbacks travel through ts rather than being baked into the
// closure.
//
// This exists because search_tenders returns eight scalars and none of them is
// a criterion, so a model asked "how is this scored?" after a search has
// nothing to answer from and will happily invent a plausible 70/30 split. The
// tool's job is as much to make the ABSENCE of a grid legible as to deliver
// one when it exists.
func newGetTenderCriteriaTool(ts *turnState, getCriteria func(tenderID string) (tender.TenderDetail, error)) tools.Tool {
	return tools.NewFunc(
		"get_tender_criteria",
		"Get one tender's award criteria with their published weights, plus the buyer's tender-documents "+
			"and submission links. Call this before saying anything about how a tender is scored, and use "+
			"ONLY the weights it returns — never estimate, infer or invent a weight. A criterion whose "+
			"\"weight\" is null carries NO published weight: that is the common case, it does not mean "+
			"zero, and it must be reported as \"no weight published\". Read the \"notice\" field if "+
			"present and follow it exactly: it distinguishes 'this notice publishes no scoring grid' from "+
			"'we have not read this notice yet', which are different answers and must be reported "+
			"differently. The tender id comes from search_tenders' results.",
		map[string]providers.Property{
			"tender_id": {
				Type:        "string",
				Description: "The tender's id, exactly as returned by search_tenders.",
			},
		},
		[]string{"tender_id"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				TenderID string `json:"tender_id"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("get_tender_criteria: invalid arguments: %w", err)
			}
			if parsed.TenderID == "" {
				return "", fmt.Errorf("get_tender_criteria: tender_id is required")
			}
			emitToolCall(ts, "get_tender_criteria", "running")
			defer emitToolCall(ts, "get_tender_criteria", "done")
			detail, err := getCriteria(parsed.TenderID)
			if err != nil {
				return "", fmt.Errorf("get_tender_criteria: %w", err)
			}
			return marshalGetTenderCriteriaResult(detail)
		},
	)
}

// tenderCriterionItem is the compact JSON shape the model sees per criterion —
// snake_case, matching searchTendersResultItem's convention (the camelCase in
// tenderResultsCardItem is for the frontend, a different consumer).
type tenderCriterionItem struct {
	// LotRefs is empty for a notice-level criterion, which applies to every
	// lot; it carries several refs when collapseIdenticalCriteria folded an
	// entry the notice restated identically once per lot.
	LotRefs     []string `json:"lot_refs,omitempty"`
	Ordinal     int      `json:"ordinal"`
	Type        string   `json:"type,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	// Weight deliberately carries NO omitempty, unlike every other optional
	// field here: a criterion with no published weight must render as an
	// explicit "weight": null rather than vanish from the object. An omitted
	// key is a fact the model has to infer, and the inference it reaches for is
	// "the field did not apply" — whereas a stated null is a fact it can
	// report. nil is the common case (roughly a quarter of published entries),
	// and being able to say "this tender publishes criteria but no weights, so
	// the real grid is in the disciplinare" is the entire point of this tool.
	Weight *float64 `json:"weight"`
	// WeightRaw is the weight exactly as published ("30", "30%", "30,5"), so a
	// formatting oddity or a failed numeric parse stays visible to the model
	// rather than silently becoming a null.
	WeightRaw string `json:"weight_raw,omitempty"`
}

// tenderCriteriaLotItem is one lot, carried so the model can name what a
// criterion's lot_refs point at. Deliberately thin: no CPV list, no per-lot
// document links — those repeat the notice-level values on most real notices
// and would pay tokens for a duplicate.
type tenderCriteriaLotItem struct {
	Ref           string `json:"ref"`
	Title         string `json:"title,omitempty"`
	Value         *int64 `json:"value,omitempty"`
	Currency      string `json:"currency,omitempty"`
	Deadline      string `json:"deadline,omitempty"` // RFC3339
	DocumentsURL  string `json:"documents_url,omitempty"`
	SubmissionURL string `json:"submission_url,omitempty"`
}

type getTenderCriteriaResult struct {
	TenderID  string `json:"tender_id"`
	Title     string `json:"title"`
	BuyerName string `json:"buyer_name"`
	Deadline  string `json:"deadline,omitempty"` // RFC3339
	// GridUsable is a *bool on the wire too, and carries no omitempty for the
	// same reason Weight does not: JSON null means "the notice has not been
	// read (or the read is out of date)", which is neither true nor false.
	// Collapsing it to false is exactly the conflation that makes "criteria
	// published" read as "criteria usable".
	GridUsable    *bool                   `json:"grid_usable"`
	Criteria      []tenderCriterionItem   `json:"criteria"`
	DocumentsURL  string                  `json:"documents_url,omitempty"`
	SubmissionURL string                  `json:"submission_url,omitempty"`
	Lots          []tenderCriteriaLotItem `json:"lots,omitempty"`
	Notice        string                  `json:"notice,omitempty"`
}

func marshalGetTenderCriteriaResult(d tender.TenderDetail) (string, error) {
	criteria := collapseIdenticalCriteria(d.Criteria)

	// EnrichedAt, not GridUsable, is what says the stored detail describes the
	// notice that is current. The two come apart in a real window: a call for
	// tenders and its later award notice collapse onto ONE row (source_ref is
	// the procedure identifier), and re-ingesting the award notice updates the
	// publication number and re-queues the row while leaving the criteria,
	// grid_usable and the old xml_status exactly as the superseded notice left
	// them. EnrichedAt is nil for the whole of that window because it is derived
	// from the pair (xml_fetched_at, xml_status = ok), and the re-queue clears
	// the timestamp.
	//
	// Branching on GridUsable alone would emit that stale grid with
	// grid_usable: true and no notice at all — presented to the model as read
	// and current, which is precisely the "inaccessible is not the same as
	// absent" failure in its most expensive direction: not a gap, a wrong
	// answer stated confidently. So grid_usable is nulled whenever the detail
	// is not from a completed read, and the criteria that survive are labelled
	// as superseded rather than silently dropped — an out-of-date grid the
	// model is told is out of date is still useful.
	gridUsable := d.GridUsable
	if d.EnrichedAt == nil {
		gridUsable = nil
	}

	var notices []string
	switch {
	case d.EnrichedAt == nil && len(criteria) > 0:
		notices = append(notices, tenderCriteriaNoticeSuperseded)
	case gridUsable == nil:
		notices = append(notices, tenderCriteriaNoticeNotRead)
	case !*gridUsable:
		notices = append(notices, tenderCriteriaNoticeNoGrid)
	}
	if len(criteria) > tenderCriteriaToolLimit {
		notices = append(notices, fmt.Sprintf(tenderCriteriaNoticeTruncatedFmt, tenderCriteriaToolLimit, len(criteria)))
		criteria = criteria[:tenderCriteriaToolLimit]
	}

	lots := make([]tenderCriteriaLotItem, 0, len(d.Lots))
	for _, l := range d.Lots {
		if len(lots) == tenderCriteriaToolLotLimit {
			notices = append(notices, fmt.Sprintf(tenderCriteriaNoticeLotsTruncatedFmt, tenderCriteriaToolLotLimit, len(d.Lots)))
			break
		}
		var deadline string
		if l.Deadline != nil {
			deadline = l.Deadline.Format(time.RFC3339)
		}
		lots = append(lots, tenderCriteriaLotItem{
			Ref: l.Ref, Title: l.Title, Value: l.Value, Currency: l.Currency,
			Deadline: deadline, DocumentsURL: l.DocumentsURL, SubmissionURL: l.SubmissionURL,
		})
	}

	var deadline string
	if d.Deadline != nil {
		deadline = d.Deadline.Format(time.RFC3339)
	}
	b, err := json.Marshal(getTenderCriteriaResult{
		TenderID:      d.ID,
		Title:         d.Title,
		BuyerName:     d.BuyerName,
		Deadline:      deadline,
		GridUsable:    gridUsable,
		Criteria:      criteria,
		DocumentsURL:  d.DocumentsURL,
		SubmissionURL: d.SubmissionURL,
		Lots:          lots,
		Notice:        strings.Join(notices, " "),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// collapseIdenticalCriteria folds criteria that are identical apart from the
// lot they name into one entry carrying every lot_ref it covers.
//
// This is a PRESENTATION-time dedupe, not a parse-time one. A notice that
// restates the same five-criterion grid once per lot really did publish 65
// entries, and the ingestion side keeps all of them because that is the fact;
// showing the model 13 identical copies would cost 13x the tokens for zero
// information. Doing the same collapse at parse time — as one upstream tool
// does — would instead destroy the ability to tell a genuinely per-lot grid
// from a repeated one.
//
// A notice-level entry (LotRef "") applies to every lot, which is strictly
// more general than any list of refs, so a group containing one emits no
// lot_refs at all and stops collecting them.
//
// First-appearance order is preserved, which — since the repository returns
// criteria sorted by (lot_ref, ordinal) — puts the notice-level grid first and
// keeps each lot's entries in published order.
func collapseIdenticalCriteria(in []tender.AwardCriterion) []tenderCriterionItem {
	// The key is every field EXCEPT LotRef: two entries that agree on all of
	// them are the same criterion restated, and nothing else about them can
	// differ. Weight is keyed on its formatted text so that an absent weight
	// ("") and a published zero ("0") never collapse into each other.
	type key struct {
		ordinal                int
		typ, name, description string
		weight, weightRaw      string
	}
	out := make([]tenderCriterionItem, 0, len(in))
	at := make(map[key]int, len(in))
	noticeLevel := make(map[int]bool, len(in))
	for _, c := range in {
		var weight string
		if c.HasWeight() {
			weight = strconv.FormatFloat(*c.Weight, 'f', -1, 64)
		}
		k := key{c.Ordinal, c.Type, c.Name, c.Description, weight, c.WeightRaw}
		i, seen := at[k]
		if !seen {
			i = len(out)
			at[k] = i
			out = append(out, tenderCriterionItem{
				Ordinal: c.Ordinal, Type: c.Type, Name: c.Name,
				Description: c.Description, Weight: c.Weight, WeightRaw: c.WeightRaw,
			})
		}
		if c.LotRef == "" {
			noticeLevel[i] = true
			out[i].LotRefs = nil
			continue
		}
		if noticeLevel[i] {
			continue
		}
		out[i].LotRefs = append(out[i].LotRefs, c.LotRef)
	}
	return out
}

// ── read_tender_documents ──

// The read_tender_documents notices name the gap rather than letting an empty
// excerpt list read as an answer. In this phase most Italian tenders will hit
// the body_not_retrieved branch: the buyer publishes a documents_url and
// nothing follows it yet. That is the honest state of the corpus, and saying
// so is more useful than answering the question from the notice PDF as if it
// were the specification.
const (
	documentNoticeBodyNotRetrieved = "The specification documents exist and are published at the tender's " +
		"documents_url, but tendersbay has not retrieved them yet. Say this explicitly — do NOT say the " +
		"tender has no documents, and do not answer the question from the notice alone as if it were the " +
		"specification."
	documentNoticeNoDocumentsPublished = "We read this notice and it publishes no document link at all."
	documentNoticeNoticeNotRead        = "We have not read this tender's structured notice, so we do not " +
		"know what it publishes. Do not tell the user the tender has no documents."
	documentNoticeNotYetExtracted = "This tender's documents are queued for text extraction and have not " +
		"been processed yet. Say 'not yet', never 'not available' — the next indexing pass resolves it."
	documentNoticeExtractionFailed = "We hold this tender's documents but extracted no text from them; a " +
		"scanned PDF with no text layer is the usual cause. Point the user at documents_url rather than " +
		"answering from nothing."
	documentNoticeNoMatch = "We hold this tender's extracted text but no passage in it matches that " +
		"question. That is not the same as the answer not existing — try a narrower or differently " +
		"worded question, or say we could not find it."
)

// newReadTenderDocumentsTool builds the "read passages out of one tender's
// extracted documents" tool. Same callback-reads-current-turnState shape as
// newSearchTendersTool.
//
// The bound on what comes back lives in core/document (MaxExcerpts x
// MaxExcerptRunes), not here: this tool renders what the domain hands it and
// cannot widen it. That placement is the point — a 50-150 page disciplinare is
// 35k-100k tokens, and resent on every one of an 8-turn loop that is a quarter
// of a workspace-month for a single question.
func newReadTenderDocumentsTool(ts *turnState, readDocs func(tenderID, question string) (document.ExcerptResult, error)) tools.Tool {
	return tools.NewFunc(
		"read_tender_documents",
		"Search the text tendersbay has extracted from one tender's documents and return the passages "+
			"most relevant to a question, each with the document it came from. Use it for anything the "+
			"notice itself does not state — requirements, penalties, technical specifications. Quote or "+
			"paraphrase ONLY the passages it returns and cite their document_url; never fill a gap from "+
			"general knowledge of how such tenders usually read. Always read \"coverage\" and \"reason\" "+
			"before answering: they say how much of this tender we can actually read and why not more, "+
			"and an empty \"excerpts\" list with a reason is a statement about OUR coverage, not about "+
			"the tender. Follow the \"notice\" field exactly when it is present.",
		map[string]providers.Property{
			"tender_id": {
				Type:        "string",
				Description: "The tender's id, exactly as returned by search_tenders.",
			},
			"question": {
				Type: "string",
				Description: "The specific question to answer, e.g. 'requisiti di capacità tecnica' or " +
					"'penali per ritardo'. Ask one narrow question per call; this tool returns short " +
					"passages, never a whole document.",
			},
		},
		[]string{"tender_id", "question"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				TenderID string `json:"tender_id"`
				Question string `json:"question"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("read_tender_documents: invalid arguments: %w", err)
			}
			if parsed.TenderID == "" {
				return "", fmt.Errorf("read_tender_documents: tender_id is required")
			}
			if strings.TrimSpace(parsed.Question) == "" {
				return "", fmt.Errorf("read_tender_documents: question is required")
			}
			emitToolCall(ts, "read_tender_documents", "running")
			defer emitToolCall(ts, "read_tender_documents", "done")
			res, err := readDocs(parsed.TenderID, parsed.Question)
			if err != nil {
				return "", fmt.Errorf("read_tender_documents: %w", err)
			}
			return marshalReadTenderDocumentsResult(parsed.TenderID, res)
		},
	)
}

// documentExcerptItem is one retrieved passage as the model sees it. The
// provenance fields below document_url are all optional because the ingestion
// pass populates page and section metadata going forward only — every document
// extracted before those columns existed keeps NULL — so this shape has to
// degrade all the way to the document URL alone.
type documentExcerptItem struct {
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
	// DocumentURL is what the model must cite. It is the one piece of
	// provenance that is always present.
	DocumentURL  string `json:"document_url"`
	DocumentType string `json:"document_type,omitempty"`
	Page         string `json:"page,omitempty"`    // "14" or "14-16"
	Section      string `json:"section,omitempty"` // "7.2 Requisiti di capacità tecnica"
	HasTable     bool   `json:"has_table,omitempty"`
}

// readTenderDocumentsResult carries Coverage and Reason as two separate
// fields, for the same reason the domain keeps them as two: the model must be
// able to say "we hold only the notice, because the specification sits behind
// a link we have not fetched" as one sentence with two facts in it. A single
// merged enum would force it to choose which half to report.
type readTenderDocumentsResult struct {
	TenderID string                `json:"tender_id"`
	Coverage string                `json:"coverage"` // "full" | "notice_only" | "none"
	Reason   string                `json:"reason,omitempty"`
	Excerpts []documentExcerptItem `json:"excerpts"`
	Notice   string                `json:"notice,omitempty"`
}

func marshalReadTenderDocumentsResult(tenderID string, res document.ExcerptResult) (string, error) {
	items := make([]documentExcerptItem, len(res.Excerpts))
	for i, e := range res.Excerpts {
		items[i] = documentExcerptItem{
			Text:         e.Text,
			Truncated:    e.Truncated,
			DocumentURL:  e.Citation.DocumentURL,
			DocumentType: e.Citation.DocumentType,
			Page:         formatCitationPage(e.Citation),
			Section:      formatCitationSection(e.Citation),
			HasTable:     e.Citation.HasTable,
		}
	}
	b, err := json.Marshal(readTenderDocumentsResult{
		TenderID: tenderID,
		Coverage: string(res.Availability.Coverage),
		Reason:   string(res.Availability.Reason),
		Excerpts: items,
		Notice:   strings.Join(documentResultNotices(res), " "),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// documentResultNotices picks the steering text for one result. The
// no-match notice can stack on top of a coverage one: "we hold the notice PDF
// only" and "nothing in what we hold mentions penalties" are both true at once
// and the model needs both to answer honestly.
func documentResultNotices(res document.ExcerptResult) []string {
	var out []string
	switch res.Availability.Reason {
	case document.ReasonBodyNotRetrieved:
		out = append(out, documentNoticeBodyNotRetrieved)
	case document.ReasonNoDocumentsPublished:
		out = append(out, documentNoticeNoDocumentsPublished)
	case document.ReasonNoticeNotRead:
		out = append(out, documentNoticeNoticeNotRead)
	case document.ReasonNotYetExtracted:
		out = append(out, documentNoticeNotYetExtracted)
	case document.ReasonExtractionFailed:
		out = append(out, documentNoticeExtractionFailed)
	}
	// Only worth saying when there was something to search: with no extracted
	// text at all, the reason above has already explained the empty list, and
	// telling the model to reword its question would send it round a loop that
	// cannot succeed.
	if len(res.Excerpts) == 0 && res.Availability.Coverage != document.CoverageNone {
		out = append(out, documentNoticeNoMatch)
	}
	return out
}

// formatCitationPage renders a page range as "14" or "14-16", and "" when the
// document was extracted before page metadata was captured. A range whose end
// does not exceed its start is one page, not a degenerate span.
func formatCitationPage(c document.Citation) string {
	if c.PageStart == nil {
		return ""
	}
	if c.PageEnd == nil || *c.PageEnd <= *c.PageStart {
		return strconv.Itoa(*c.PageStart)
	}
	return fmt.Sprintf("%d-%d", *c.PageStart, *c.PageEnd)
}

// formatCitationSection renders "7.2 Requisiti di capacità tecnica" from the
// most specific element of the section path plus the section title, degrading
// to whichever of the two is present and to "" when neither is. The last path
// element is used rather than the whole chain because it already carries the
// full dotted number in the corpus's numbering ("7", "7.2"), so joining the
// chain would repeat it.
func formatCitationSection(c document.Citation) string {
	var number string
	if n := len(c.SectionPath); n > 0 {
		number = c.SectionPath[n-1]
	}
	return strings.TrimSpace(number + " " + c.SectionTitle)
}
