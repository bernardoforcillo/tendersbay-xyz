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

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
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
	ID        string `json:"id"`
	Title     string `json:"title"`
	BuyerName string `json:"buyer_name"`
	Country   string `json:"country"`
	CPV       string `json:"cpv"`
	// Named value_minor, not value, because the model reads this key and will
	// quote it back to the user: a bare "value": 629724005 gets reported as
	// €629,724,005 rather than €6,297,240.05. The suffix is the same unit
	// marker the dossier tools already use (turnover_minor, value_minor), and
	// it is the only thing standing between minor units and a figure a bidder
	// might act on.
	ValueMinor     *int64  `json:"value_minor,omitempty"`
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
			ValueMinor: r.Value, Deadline: deadline, RelevanceScore: r.RelevanceScore,
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
	Ref   string `json:"ref"`
	Title string `json:"title,omitempty"`
	// Minor units, and named for it — same reason as
	// searchTendersResultItem.ValueMinor: the model quotes this figure.
	ValueMinor    *int64 `json:"value_minor,omitempty"`
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
			Ref: l.Ref, Title: l.Title, ValueMinor: l.Value, Currency: l.Currency,
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

// ── Company dossier & eligibility ──
//
// Four tools, and the shape of what they return is the whole design. Every one
// of them exists to keep three answers distinguishable that a naive result
// would collapse into two:
//
//   - a BLOCKING gap — the gara demands something the dossier says we do not
//     have;
//   - a NON-BLOCKING gap — unmet, but a criterio premiante rather than a
//     requisito di partecipazione, so it costs points and not admission;
//   - UNKNOWN — nobody has captured this gara's requisiti at all, or the
//     dossier holds no fact either way. Italian selection criteria live in the
//     disciplinare, and this product persists AWARD criteria only, so unknown
//     is the honest majority case rather than an edge one.
//
// Collapsing the third into "no gaps found" is the failure this whole phase
// exists to prevent, and it is exactly what an empty gap list plus a bare
// "verdict" field would produce.
//
// Like get_tender_criteria and read_tender_documents, these tools persist
// nothing and push no stream event: the payload travels in the tool's own
// string return and nowhere else. That is a privacy property here and not only
// a scope choice — a dossier carries a partita IVA and a codice fiscale, and no
// chat_messages row should ever hold one.

// companyProfileFinancialYearLimit caps how many esercizi the model sees. Three
// is not arbitrary: "gli ultimi tre esercizi" is the window Italian selection
// criteria actually name, so a fourth year buys tokens and no answer.
const companyProfileFinancialYearLimit = 3

// companyProfilePastContractLimit caps the referenze listed. A dossier's
// contract list is unbounded and each entry is verbose; the model needs enough
// to judge a "lavori analoghi" requirement, not the whole track record.
const companyProfilePastContractLimit = 10

const (
	companyProfileNoticeEmpty = "The company dossier is empty. Do NOT invent company facts and do not " +
		"answer \"we qualify\" or \"we do not\". Ask only for what a specific gara's requirements demand, " +
		"one question at a time, via ask_choice, and write each answer with record_company_fact."
	companyProfileNoticeTruncatedFmt = "Only the %d most recent of %d referenze are shown; ask the user " +
		"about a specific sector if you need more."
)

// companyIdentityItem is the registry identity as the model sees it. The
// per-field provenance map is deliberately included: "the user typed this" and
// "the agent asked and the user confirmed it" are different claims, and only a
// human-sourced fact may support a blocking answer.
type companyIdentityItem struct {
	LegalName   string            `json:"legal_name,omitempty"`
	VATNumber   string            `json:"vat_number,omitempty"`
	FiscalCode  string            `json:"fiscal_code,omitempty"`
	LegalForm   string            `json:"legal_form,omitempty"`
	CCIAA       string            `json:"cciaa,omitempty"`
	Country     string            `json:"country,omitempty"`
	NUTS        string            `json:"nuts,omitempty"`
	FoundedYear *int32            `json:"founded_year,omitempty"`
	Provenance  map[string]string `json:"provenance,omitempty"`
}

type companySOAItem struct {
	Category   string `json:"category"`
	Classifica string `json:"classifica"`
	ValidUntil string `json:"valid_until,omitempty"`
	// VerifiedUntil is the triennial verification deadline, which lapses three
	// years BEFORE valid_until and invalidates the attestation on its own. It is
	// carried separately so the model can never read a lapsed attestation as
	// valid for two more years.
	VerifiedUntil string `json:"verified_until,omitempty"`
	Provenance    string `json:"provenance"`
}

type companyCertificationItem struct {
	Standard   string `json:"standard"`
	Scope      string `json:"scope,omitempty"`
	ValidUntil string `json:"valid_until,omitempty"`
	Provenance string `json:"provenance"`
}

type companyFinancialYearItem struct {
	Year                  int32  `json:"year"`
	TurnoverMinor         *int64 `json:"turnover_minor"`
	SpecificTurnoverMinor *int64 `json:"specific_turnover_minor"`
	Currency              string `json:"currency,omitempty"`
	Headcount             *int32 `json:"headcount"`
	Provenance            string `json:"provenance"`
}

type companyPastContractItem struct {
	Description string `json:"description,omitempty"`
	CPV         string `json:"cpv,omitempty"`
	ValueMinor  *int64 `json:"value_minor"`
	// EffectiveValueMinor is the slice of the contract that actually counts
	// toward a requirement — a 20% mandante on a €5M contract carries €1M of
	// reference, not €5M. It is null when the role is a shared one and no share
	// was stated, because assuming 100% there is the classic way an SME's
	// dossier overstates its own track record.
	EffectiveValueMinor *int64 `json:"effective_value_minor"`
	Currency            string `json:"currency,omitempty"`
	EndedOn             string `json:"ended_on,omitempty"`
	Role                string `json:"role,omitempty"`
	Provenance          string `json:"provenance"`
}

type companyRegistrationItem struct {
	Kind    string `json:"kind"`
	Section string `json:"section,omitempty"`
	// The registration NUMBER is deliberately absent. It answers no eligibility
	// question, and it is on the never-leaves-the-backend list — omitting the
	// field is the only enforcement that cannot be forgotten later.
	ValidUntil string `json:"valid_until,omitempty"`
	Provenance string `json:"provenance"`
}

type getCompanyProfileResult struct {
	Empty          bool                       `json:"empty"`
	Identity       companyIdentityItem        `json:"identity"`
	SOA            []companySOAItem           `json:"soa"`
	Certifications []companyCertificationItem `json:"certifications"`
	FinancialYears []companyFinancialYearItem `json:"financial_years"`
	PastContracts  []companyPastContractItem  `json:"past_contracts"`
	Registrations  []companyRegistrationItem  `json:"registrations"`
	Notice         string                     `json:"notice,omitempty"`
}

// newGetCompanyProfileTool builds the "read the company's own dossier" tool.
// Same reads-the-turn's-turnState shape as newSearchTendersTool — see
// newCreateWorkbenchTool's doc comment for why the turn's identity and context
// travel through ts rather than being baked into the closure.
func newGetCompanyProfileTool(ts *turnState, getDossier func() (company.Dossier, error)) tools.Tool {
	return tools.NewFunc(
		"get_company_profile",
		"Read the user's own company dossier: registry identity, SOA attestations, certifications, the "+
			"last esercizi (fatturato and organico), referenze, and iscrizioni. Call this before saying "+
			"anything about what the company holds or whether it qualifies for a gara. Report ONLY what it "+
			"returns — never infer a company fact from the sector, the company name or how such firms "+
			"usually look. Every record carries a \"provenance\": a fact the user stated or confirmed is "+
			"binding, anything else is not. An empty dossier means we have never asked, NOT that the "+
			"company holds nothing.",
		map[string]providers.Property{},
		nil,
		func(_ context.Context, args string) (string, error) {
			emitToolCall(ts, "get_company_profile", "running")
			defer emitToolCall(ts, "get_company_profile", "done")
			d, err := getDossier()
			if err != nil {
				return "", fmt.Errorf("get_company_profile: %w", err)
			}
			return marshalGetCompanyProfileResult(d)
		},
	)
}

func marshalGetCompanyProfileResult(d company.Dossier) (string, error) {
	out := getCompanyProfileResult{
		Empty:          d.Empty(),
		Identity:       companyIdentityToItem(d.Identity),
		SOA:            make([]companySOAItem, 0, len(d.SOA)),
		Certifications: make([]companyCertificationItem, 0, len(d.Certifications)),
		FinancialYears: make([]companyFinancialYearItem, 0, len(d.FinancialYears)),
		PastContracts:  make([]companyPastContractItem, 0, len(d.PastContracts)),
		Registrations:  make([]companyRegistrationItem, 0, len(d.Registrations)),
	}
	for _, c := range d.SOA {
		out.SOA = append(out.SOA, companySOAItem{
			Category:      c.Category,
			Classifica:    c.Classifica.String(),
			ValidUntil:    formatOptionalDate(c.ValidUntil),
			VerifiedUntil: formatOptionalDate(c.VerifiedUntil),
			Provenance:    string(c.Provenance),
		})
	}
	for _, c := range d.Certifications {
		name := string(c.Standard)
		if c.Standard == company.CertOtherStd && strings.TrimSpace(c.StandardRaw) != "" {
			name = strings.TrimSpace(c.StandardRaw)
		}
		out.Certifications = append(out.Certifications, companyCertificationItem{
			Standard:   name,
			Scope:      c.Scope,
			ValidUntil: formatOptionalDate(c.ValidUntil),
			Provenance: string(c.Provenance),
		})
	}
	// FinancialYears arrive newest first, so the head of the slice is the window
	// every "ultimi tre esercizi" requirement asks about.
	for i, f := range d.FinancialYears {
		if i == companyProfileFinancialYearLimit {
			break
		}
		out.FinancialYears = append(out.FinancialYears, companyFinancialYearItem{
			Year:                  f.Year,
			TurnoverMinor:         f.TurnoverMinor,
			SpecificTurnoverMinor: f.SpecificTurnoverMinor,
			Currency:              f.Currency,
			Headcount:             f.Headcount,
			Provenance:            string(f.Provenance),
		})
	}
	var notices []string
	for i, p := range d.PastContracts {
		if i == companyProfilePastContractLimit {
			notices = append(notices, fmt.Sprintf(companyProfileNoticeTruncatedFmt,
				companyProfilePastContractLimit, len(d.PastContracts)))
			break
		}
		out.PastContracts = append(out.PastContracts, companyPastContractItem{
			Description:         p.Description,
			CPV:                 p.CPV,
			ValueMinor:          p.ValueMinor,
			EffectiveValueMinor: p.EffectiveValueMinor(),
			Currency:            p.Currency,
			EndedOn:             formatOptionalDate(p.EndedOn),
			Role:                string(p.Role),
			Provenance:          string(p.Provenance),
		})
	}
	for _, r := range d.Registrations {
		out.Registrations = append(out.Registrations, companyRegistrationItem{
			Kind:       string(r.Kind),
			Section:    r.Section,
			ValidUntil: formatOptionalDate(r.ValidUntil),
			Provenance: string(r.Provenance),
		})
	}
	if d.Empty() {
		notices = append([]string{companyProfileNoticeEmpty}, notices...)
	}
	out.Notice = strings.Join(notices, " ")
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func companyIdentityToItem(id company.Identity) companyIdentityItem {
	item := companyIdentityItem{
		LegalName:   id.LegalName,
		VATNumber:   id.VATNumber,
		FiscalCode:  id.FiscalCode,
		LegalForm:   string(id.LegalForm),
		Country:     id.Country,
		NUTS:        id.NUTS,
		FoundedYear: id.FoundedYear,
	}
	// The CCIAA office and number are one answer to one question — a REA number
	// without its provincia identifies nothing — so they render as one string,
	// the same way the domain keys their attribution under a single field.
	if id.CCIAAOffice != "" || id.CCIAANumber != "" {
		item.CCIAA = strings.TrimPrefix(id.CCIAAOffice+"/"+id.CCIAANumber, "/")
	}
	if len(id.Attribution) > 0 {
		item.Provenance = make(map[string]string, len(id.Attribution))
		for k, a := range id.Attribution {
			item.Provenance[string(k)] = string(a.Provenance)
		}
	}
	return item
}

// formatOptionalDate renders a stated date as a plain calendar day. The model
// reasons about expiry against a submission deadline, and a full RFC3339
// timestamp on an attestation whose stored precision is a day would invite it to
// quote an hour the document never stated.
func formatOptionalDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// ── check_eligibility ──

const (
	eligibilityNoticeLeadWithFact = "Report the blocking FACT first, quoting required vs held " +
		"(\"richiede SOA OG1 classifica III; il profilo aziendale indica OG1 I\"). Only then state the " +
		"recommendation, and always say what would change it (avvalimento, RTI, subappalto, upgrade). " +
		"Never open with \"non partecipare\": the user carries the liability and decides."
	eligibilityNoticeInsufficient = "No participation requirement has been captured for this tender, so " +
		"NO eligibility judgement is possible. Say exactly that. Do NOT infer requirements from the CPV " +
		"code, the value or how such gare usually read, and do NOT report an empty gap list as if the " +
		"company qualified."
	eligibilityNoticeAwardGridOnly = "What this tender publishes is the AWARD grid — how an admitted bid " +
		"is scored. It is NOT the requisiti di partecipazione, which live in the disciplinare. Do not " +
		"present the award grid as an eligibility check."
	eligibilityNoticeUnknown = "One or more requirements cannot be checked because the company dossier " +
		"has no answer. Ask for exactly those, one at a time, via ask_choice, and write each answer with " +
		"record_company_fact. Do not guess, and do not ask for anything not listed here."
	eligibilityNoticeUnconfirmed = "One or more requirements were extracted from a document and are " +
		"UNCONFIRMED. Show the user the quote and ask them to confirm before treating it as binding."
	eligibilityNoticeDocumentsUnread = "We do not hold the text of this tender's own documents, so nobody " +
		"and nothing has read its requisiti section. Say that before any judgement; the structured notice " +
		"data cannot substitute for it."
)

type eligibilityGapItem struct {
	RequirementText string `json:"requirement_text"`
	Kind            string `json:"kind"`
	// Status distinguishes the three answers this tool exists to keep apart:
	// "missing"/"shortfall"/"expired" (we asked and the answer is no),
	// "unknown" (nobody has told us either way — ask), and "met".
	Status   string `json:"status"` // met|missing|shortfall|expiring|expired|unknown
	Blocking bool   `json:"blocking"`
	// Held carries NO omitempty, for the same reason tenderCriterionItem.Weight
	// does not: an omitted key is a fact the model has to infer, and the
	// inference it reaches for is "the field did not apply". An empty held must
	// render as an empty string the model can report.
	Held        string `json:"held"`
	Remedy      string `json:"remedy,omitempty"`
	Question    string `json:"question,omitempty"`
	Source      string `json:"source"`
	Confirmed   bool   `json:"confirmed"`
	DocumentURL string `json:"document_url,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
}

// checkEligibilityResult's field order is the design. gaps come FIRST and
// verdict LAST, because the model reads the object top-down and the product
// rule is "never lead with the verdict". This costs nothing and is the cheapest
// enforcement available.
type checkEligibilityResult struct {
	TenderID                  string               `json:"tender_id"`
	Gaps                      []eligibilityGapItem `json:"gaps"`
	RequirementCount          int                  `json:"requirement_count"`
	AuthoritativeRequirements int                  `json:"authoritative_requirement_count"`
	DocumentCoverage          string               `json:"document_coverage"`
	DocumentReason            string               `json:"document_reason,omitempty"`
	Verdict                   string               `json:"verdict"`
	Notice                    string               `json:"notice,omitempty"`
}

// newCheckEligibilityTool builds the "check this gara's captured requirements
// against the company dossier" tool.
func newCheckEligibilityTool(ts *turnState, check func(tenderID int64, lotRef string) (company.Assessment, error)) tools.Tool {
	return tools.NewFunc(
		"check_eligibility",
		"Check one tender's captured participation requirements (requisiti di partecipazione) against the "+
			"user's company dossier, and get the gaps plus what would close each one. Read \"gaps\" before "+
			"\"verdict\" and report the FACT before the recommendation. A gap's \"status\" is the answer: "+
			"\"unknown\" means nobody has told us either way — ask the user, never guess; "+
			"\"missing\"/\"shortfall\"/\"expired\" with \"blocking\": true is a real obstacle; the same "+
			"statuses with \"blocking\": false cost points, not admission. An empty \"gaps\" list means no "+
			"requirement has been captured for this gara — it does NOT mean the company qualifies. Follow "+
			"the \"notice\" field exactly when present.",
		map[string]providers.Property{
			"tender_id": {
				Type:        "string",
				Description: "The tender's id, exactly as returned by search_tenders.",
			},
			"lot_ref": {
				Type: "string",
				Description: "Optional lot identifier, exactly as returned by get_tender_criteria. Omit " +
					"for the notice level; notice-level requirements apply to every lot either way.",
			},
		},
		[]string{"tender_id"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				TenderID string `json:"tender_id"`
				LotRef   string `json:"lot_ref"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("check_eligibility: invalid arguments: %w", err)
			}
			id, err := parseToolTenderID(parsed.TenderID)
			if err != nil {
				return "", fmt.Errorf("check_eligibility: %w", err)
			}
			emitToolCall(ts, "check_eligibility", "running")
			defer emitToolCall(ts, "check_eligibility", "done")
			a, err := check(id, parsed.LotRef)
			if err != nil {
				return "", fmt.Errorf("check_eligibility: %w", err)
			}
			return marshalCheckEligibilityResult(parsed.TenderID, a)
		},
	)
}

func marshalCheckEligibilityResult(tenderID string, a company.Assessment) (string, error) {
	// Gaps arrive pre-sorted by the domain (blocking, then unknown, then
	// expiring, then other unmet, then met) and are emitted in that order
	// unchanged: re-sorting here would let this consumer disagree with the panel
	// and the proto message about what the user must see first.
	gaps := make([]eligibilityGapItem, len(a.Gaps))
	for i, g := range a.Gaps {
		item := eligibilityGapItem{
			RequirementText: g.Requirement.Text,
			Kind:            string(g.Requirement.Kind),
			Status:          string(g.Status),
			Blocking:        g.Blocking,
			Held:            g.Held,
			Remedy:          string(g.Remedy.Kind),
			Source:          string(g.Requirement.Source),
			Confirmed:       g.Requirement.Confirmed(),
			Excerpt:         g.Requirement.Excerpt,
		}
		if g.Question != nil {
			item.Question = g.Question.Prompt
		}
		if g.Requirement.Citation != nil {
			item.DocumentURL = g.Requirement.Citation.DocumentURL
		}
		gaps[i] = item
	}
	b, err := json.Marshal(checkEligibilityResult{
		TenderID:                  tenderID,
		Gaps:                      gaps,
		RequirementCount:          a.RequirementCount,
		AuthoritativeRequirements: a.AuthoritativeRequirements,
		DocumentCoverage:          string(a.DocumentCoverage),
		DocumentReason:            string(a.DocumentReason),
		Verdict:                   string(a.Verdict),
		Notice:                    strings.Join(eligibilityNotices(a), " "),
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// eligibilityNotices stacks the steering text, the same way
// documentResultNotices does and for the same reason: "we captured only the
// award grid" and "the dossier cannot answer two of these" are both true at
// once, and the model needs both to answer honestly.
//
// It is driven off the domain's own Caveat list rather than off a second
// reading of the assessment's counters, so the tool cannot come to disagree with
// the eligibility panel about what is worth saying.
func eligibilityNotices(a company.Assessment) []string {
	var out []string
	for _, c := range a.Recommendation().Caveats {
		switch c {
		case company.CaveatNoRequirements:
			out = append(out, eligibilityNoticeInsufficient)
		case company.CaveatAwardGridOnly:
			out = append(out, eligibilityNoticeAwardGridOnly)
		case company.CaveatUnconfirmedExtraction:
			out = append(out, eligibilityNoticeUnconfirmed)
		case company.CaveatOpenQuestions:
			out = append(out, eligibilityNoticeUnknown)
		case company.CaveatDocumentsUnread:
			out = append(out, eligibilityNoticeDocumentsUnread)
		}
	}
	// The lead-with-the-fact rule applies whenever there is a fact to lead with.
	// It goes last in the notice string but first in what it asks for, because
	// the caveats above are what a reader has to know before framing anything.
	if a.BlockingGapCount > 0 {
		out = append(out, eligibilityNoticeLeadWithFact)
	}
	return out
}

// ── record_tender_requirement ──

const requirementNoticeUnconfirmed = "You recorded a requirement extracted from a document. It is " +
	"UNCONFIRMED until the user checks it against the quoted passage, and an unconfirmed requirement can " +
	"never produce a \"do not bid\". Show the user the quote and the document you took it from."

// recordRequirementResult tells the model what was actually stored, including
// the fact that it is not binding yet. Returning a bare "ok" would let the model
// treat its own extraction as established.
type recordRequirementResult struct {
	Kind      string `json:"kind"`
	Blocking  bool   `json:"blocking"`
	Confirmed bool   `json:"confirmed"`
	Notice    string `json:"notice"`
}

// requirementDetailArgs is the flat union the model fills for a requirement's
// machine-comparable payload. It is flat, and passed as a JSON-encoded STRING,
// because providers.Property has no array or object nesting — the same
// constraint that makes ask_choice's options a JSON string. Keeping it flat also
// keeps the model's job simple: fill the fields your kind names, ignore the rest.
type requirementDetailArgs struct {
	// soa
	Category   string `json:"category"`
	Classifica string `json:"classifica"`
	// Prevalente is a *bool and defaults to TRUE when absent. Prevalente decides
	// the REMEDY (a scorporabile category may be covered by subappalto, a
	// prevalente one may not), and defaulting to prevalente is the conservative
	// direction: it offers avvalimento rather than telling the user they can
	// simply subcontract a category they cannot.
	Prevalente *bool `json:"prevalente"`
	// certification
	Standard    string `json:"standard"`
	StandardRaw string `json:"standard_raw"`
	Scope       string `json:"scope"`
	// turnover / past_contracts / headcount window
	MinMinor int64  `json:"min_minor"`
	Currency string `json:"currency"`
	Years    int32  `json:"years"`
	Specific bool   `json:"specific"`
	CPV      string `json:"cpv"`
	MinCount int32  `json:"min_count"`
	// registration
	Kind    string `json:"kind"`
	Section string `json:"section"`
}

// newRecordTenderRequirementTool builds the "write down one requisito this gara
// publishes" tool.
func newRecordTenderRequirementTool(ts *turnState, record func(tenderID int64, r company.Requirement) error) tools.Tool {
	return tools.NewFunc(
		"record_tender_requirement",
		"Record ONE participation requirement (requisito di partecipazione) you read in this tender's "+
			"documents, so it can be checked against the company dossier. Only call this for a passage you "+
			"actually quoted from read_tender_documents — never for a requirement you inferred from the "+
			"CPV code, the value, or how such gare usually read, and never for an award criterion "+
			"(get_tender_criteria's grid is how an ADMITTED bid is scored, not who may be admitted). What "+
			"you record is stored UNCONFIRMED: it can raise a question but can never produce a \"do not "+
			"bid\" until the user checks it against your quote. Recording the same requirement again "+
			"updates it rather than adding a second one, so correct a mistake by re-recording it.",
		map[string]providers.Property{
			"tender_id": {
				Type:        "string",
				Description: "The tender's id, exactly as returned by search_tenders.",
			},
			"lot_ref": {
				Type: "string",
				Description: "Optional lot identifier this requirement applies to. Omit when the " +
					"disciplinare states it once for the whole gara — a notice-level requirement applies " +
					"to every lot.",
			},
			"kind": {
				Type: "string",
				Description: "Which requirement this is. Use \"other\" for anything the list does not " +
					"cover: it is stored verbatim and reported as un-modelled, which is better than " +
					"forcing it into the wrong kind.",
				Enum: []string{"soa", "certification", "turnover", "past_contracts", "headcount", "registration", "other"},
			},
			"requirement_text": {
				Type:        "string",
				Description: "The requirement as published, in the document's own words. Always required.",
			},
			"excerpt": {
				Type:        "string",
				Description: "The passage you took it from, verbatim, exactly as read_tender_documents returned it.",
			},
			"document_url": {
				Type:        "string",
				Description: "The document_url read_tender_documents reported for that passage.",
			},
			"blocking": {
				Type: "boolean",
				Description: "Whether failing it excludes the bidder (a requisito di partecipazione) " +
					"rather than only costing points (a criterio premiante). Defaults to true when omitted.",
			},
			"detail": {
				Type: "string",
				Description: `A JSON object with the machine-comparable numbers for this kind, as a string. ` +
					`soa: {"category":"OG1","classifica":"III","prevalente":true}. ` +
					`certification: {"standard":"iso_9001","scope":""} (standard is one of iso_9001, ` +
					`iso_14001, iso_45001, iso_27001, iso_37001, iso_50001, sa_8000, uni_pdr_125, ` +
					`iso_20121, emas, other; with "other" also send {"standard_raw":"the published name"}). ` +
					`turnover: {"min_minor":15000000,"currency":"EUR","years":3,"specific":false} — ` +
					`min_minor is in MINOR units (cents), and "specific":true means fatturato nel settore. ` +
					`past_contracts: {"min_count":2,"min_minor":50000000,"years":5,"cpv":"45"}. ` +
					`headcount: {"min_count":15,"years":3}. ` +
					`registration: {"kind":"white_list","section":""} (kind is one of white_list, cciaa, ` +
					`albo_gestori_ambientali, mepa, sistema_qualificazione, albo_professionale, anac, other). ` +
					`Omit entirely for kind "other".`,
			},
		},
		[]string{"tender_id", "kind", "requirement_text"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				TenderID        string `json:"tender_id"`
				LotRef          string `json:"lot_ref"`
				Kind            string `json:"kind"`
				RequirementText string `json:"requirement_text"`
				Excerpt         string `json:"excerpt"`
				DocumentURL     string `json:"document_url"`
				Blocking        *bool  `json:"blocking"`
				Detail          string `json:"detail"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("record_tender_requirement: invalid arguments: %w", err)
			}
			id, err := parseToolTenderID(parsed.TenderID)
			if err != nil {
				return "", fmt.Errorf("record_tender_requirement: %w", err)
			}
			if strings.TrimSpace(parsed.RequirementText) == "" {
				return "", fmt.Errorf("record_tender_requirement: requirement_text is required")
			}
			req, err := buildRequirement(parsed.Kind, parsed.RequirementText, parsed.LotRef,
				parsed.Excerpt, parsed.DocumentURL, parsed.Blocking, parsed.Detail)
			if err != nil {
				return "", fmt.Errorf("record_tender_requirement: %w", err)
			}
			emitToolCall(ts, "record_tender_requirement", "running")
			defer emitToolCall(ts, "record_tender_requirement", "done")
			if err := record(id, req); err != nil {
				return "", fmt.Errorf("record_tender_requirement: %w", err)
			}
			b, err := json.Marshal(recordRequirementResult{
				Kind:      string(req.Kind),
				Blocking:  req.Blocking,
				Confirmed: false,
				Notice:    requirementNoticeUnconfirmed,
			})
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	)
}

// buildRequirement assembles the domain value from the tool's flat arguments.
// It sets Source to RequirementDocumentExcerpt and leaves ConfirmedBy empty
// unconditionally: an agent has no way to state a requirement on its own
// authority, and company.Service clears ConfirmedBy for a non-user source
// anyway, so the two layers agree rather than one trusting the other.
func buildRequirement(kind, text, lotRef, excerpt, documentURL string, blocking *bool, detail string) (company.Requirement, error) {
	k := company.RequirementKind(strings.TrimSpace(kind))
	if !company.ValidRequirementKind(k) {
		return company.Requirement{}, fmt.Errorf("unknown kind %q", kind)
	}
	// Blocking defaults to TRUE: an unclassified requirement treated as blocking
	// produces a question, treated as non-blocking produces a silent pass.
	isBlocking := true
	if blocking != nil {
		isBlocking = *blocking
	}
	r := company.Requirement{
		LotRef:   strings.TrimSpace(lotRef),
		Kind:     k,
		Text:     text,
		Blocking: isBlocking,
		Source:   company.RequirementDocumentExcerpt,
		Excerpt:  excerpt,
	}
	if u := strings.TrimSpace(documentURL); u != "" {
		r.Citation = &document.Citation{DocumentURL: u}
	}

	if k == company.RequirementOther {
		return r, nil
	}
	var d requirementDetailArgs
	if s := strings.TrimSpace(detail); s != "" {
		if err := json.Unmarshal([]byte(s), &d); err != nil {
			return company.Requirement{}, fmt.Errorf("detail is not a valid JSON object: %w", err)
		}
	}
	switch k {
	case company.RequirementSOA:
		cl, ok := company.ParseClassifica(d.Classifica)
		if !ok {
			return company.Requirement{}, fmt.Errorf("detail.classifica %q names no SOA tier (I..VIII, incl. III-bis, IV-bis)", d.Classifica)
		}
		prevalente := true
		if d.Prevalente != nil {
			prevalente = *d.Prevalente
		}
		r.SOA = &company.SOARequirement{Category: d.Category, Classifica: cl, Prevalente: prevalente}
	case company.RequirementCertification:
		r.Certification = &company.CertificationRequirement{
			Standard:    company.CertificationStandard(strings.TrimSpace(d.Standard)),
			StandardRaw: d.StandardRaw,
			Scope:       d.Scope,
		}
	case company.RequirementTurnover:
		r.Amount = &company.AmountRequirement{
			MinMinor: d.MinMinor, Currency: d.Currency, Years: d.Years, Specific: d.Specific, CPV: d.CPV,
		}
	case company.RequirementPastContracts:
		// Both payloads are carried when both were stated: the count is "almeno
		// 2 servizi analoghi" and the amount is what makes a contract count as
		// one. Dropping either would silently widen the requirement.
		if d.MinCount > 0 {
			r.Count = &company.CountRequirement{Min: d.MinCount}
		}
		if d.MinMinor > 0 || d.Years > 0 || strings.TrimSpace(d.CPV) != "" {
			r.Amount = &company.AmountRequirement{
				MinMinor: d.MinMinor, Currency: d.Currency, Years: d.Years, CPV: d.CPV,
			}
		}
	case company.RequirementHeadcount:
		r.Count = &company.CountRequirement{Min: d.MinCount}
		if d.Years > 1 {
			r.Amount = &company.AmountRequirement{Years: d.Years, Currency: d.Currency}
		}
	case company.RequirementRegistration:
		r.Registration = &company.RegistrationRequirement{
			Kind: company.RegistrationKind(strings.TrimSpace(d.Kind)), Section: d.Section,
		}
	}
	// Validated here so a malformed payload comes back to the MODEL as a tool
	// error it can correct on the next call, rather than as an opaque failure
	// from three layers down.
	if err := r.Validate(); err != nil {
		return company.Requirement{}, err
	}
	return r, nil
}

// ── record_company_fact ──

const companyFactNoticeStored = "Stored against the company dossier as a fact the USER stated, prompted " +
	"by this gara. Only write what the user actually answered — never your own reading of it — and if " +
	"they were unsure, ask again rather than recording a guess."

type recordCompanyFactResult struct {
	Field  string `json:"field"`
	Notice string `json:"notice"`
}

// companyFactValueArgs is the flat union the model fills for one dossier fact,
// passed as a JSON-encoded string for the same reason requirementDetailArgs is.
type companyFactValueArgs struct {
	// identity scalars
	Value string `json:"value"`
	// soa
	Category      string `json:"category"`
	Classifica    string `json:"classifica"`
	VerifiedUntil string `json:"verified_until"`
	// soa + certification + registration
	IssuedBy   string `json:"issued_by"`
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
	// certification
	Standard    string `json:"standard"`
	StandardRaw string `json:"standard_raw"`
	Scope       string `json:"scope"`
	// financial year
	Year                  int32  `json:"year"`
	TurnoverMinor         *int64 `json:"turnover_minor"`
	SpecificTurnoverMinor *int64 `json:"specific_turnover_minor"`
	Headcount             *int32 `json:"headcount"`
	// past contract
	Description string   `json:"description"`
	BuyerName   string   `json:"buyer_name"`
	CPV         string   `json:"cpv"`
	ValueMinor  *int64   `json:"value_minor"`
	StartedOn   string   `json:"started_on"`
	EndedOn     string   `json:"ended_on"`
	Role        string   `json:"role"`
	SharePct    *float64 `json:"share_pct"`
	// past contract + financial year
	Currency string `json:"currency"`
	// registration
	Kind       string `json:"kind"`
	Authority  string `json:"authority"`
	Identifier string `json:"identifier"`
	Section    string `json:"section"`
}

// newRecordCompanyFactTool builds the "write the user's answer into the company
// dossier" tool.
//
// The "a human answered this" claim is enforced at the PROMPT level, not in
// code, and that is stated rather than hidden: this tool's description forbids
// calling it without a preceding ask_choice, exactly the accepted prompt-based
// control newCreateWorkbenchTool already relies on and for the same reason —
// berrygem gives a tool no lever except what it returns. The residual risk is
// the model mis-transcribing an answer, and the mitigations are real: the answer
// physically exists as a choice_response row, every agent-written fact carries a
// source note naming the gara that prompted it, and the dossier UI shows
// provenance per field, so a wrong value is visible and correctable.
func newRecordCompanyFactTool(ts *turnState, record func(field string, v companyFactValueArgs, tenderID *int64) error) tools.Tool {
	return tools.NewFunc(
		"record_company_fact",
		"Write ONE fact the user just told you about their own company into the company dossier, so it "+
			"never has to be asked again. Call this ONLY immediately after the user answered an ask_choice "+
			"question with this value — never for something you inferred, assumed, or read in a tender "+
			"document, and never to \"fill in\" a field the user did not answer. If the user was unsure, "+
			"ask again instead of recording a guess. One fact per call.",
		map[string]providers.Property{
			"field": {
				Type:        "string",
				Description: "Which dossier fact the user answered.",
				Enum: []string{
					"soa", "certification", "turnover", "headcount", "past_contract", "registration",
					"legal_name", "vat_number", "fiscal_code", "legal_form", "cciaa", "country", "nuts", "founded_year",
				},
			},
			"value": {
				Type: "string",
				Description: `A JSON object with the answer, as a string. ` +
					`soa: {"category":"OG1","classifica":"III","issued_by":"","valid_until":"2029-04-30","verified_until":"2026-04-30"}. ` +
					`certification: {"standard":"iso_9001","scope":"","valid_until":"2027-06-30"}. ` +
					`turnover: {"year":2024,"turnover_minor":210000000,"specific_turnover_minor":90000000,"currency":"EUR"} — MINOR units (cents). ` +
					`headcount: {"year":2024,"headcount":18}. ` +
					`past_contract: {"description":"","buyer_name":"","cpv":"45233","value_minor":50000000,"currency":"EUR","started_on":"2023-01-10","ended_on":"2024-05-30","role":"principal","share_pct":100} ` +
					`(role is one of principal, subcontractor, rti_mandataria, rti_mandante; send share_pct for the RTI and subcontractor roles — the company's OWN share is what counts). ` +
					`registration: {"kind":"white_list","authority":"Prefettura di Milano","section":"","valid_until":"2027-01-31"}. ` +
					`Any identity field (legal_name, vat_number, fiscal_code, legal_form, cciaa, country, nuts, founded_year): {"value":"…"}, ` +
					`with cciaa written as "<sigla>/<numero REA>" and country as an alpha-2 code. Dates are "YYYY-MM-DD".`,
			},
			"prompted_by_tender_id": {
				Type: "string",
				Description: "The tender whose requirement made you ask, exactly as returned by " +
					"search_tenders. Send it whenever a specific gara prompted the question — it is what " +
					"records WHY this was asked.",
			},
		},
		[]string{"field", "value"},
		func(_ context.Context, args string) (string, error) {
			var parsed struct {
				Field              string `json:"field"`
				Value              string `json:"value"`
				PromptedByTenderID string `json:"prompted_by_tender_id"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err != nil {
				return "", fmt.Errorf("record_company_fact: invalid arguments: %w", err)
			}
			var v companyFactValueArgs
			if err := json.Unmarshal([]byte(parsed.Value), &v); err != nil {
				return "", fmt.Errorf("record_company_fact: value is not a valid JSON object: %w", err)
			}
			var tenderID *int64
			if s := strings.TrimSpace(parsed.PromptedByTenderID); s != "" {
				id, err := parseToolTenderID(s)
				if err != nil {
					return "", fmt.Errorf("record_company_fact: prompted_by_tender_id: %w", err)
				}
				tenderID = &id
			}
			emitToolCall(ts, "record_company_fact", "running")
			defer emitToolCall(ts, "record_company_fact", "done")
			if err := record(parsed.Field, v, tenderID); err != nil {
				return "", fmt.Errorf("record_company_fact: %w", err)
			}
			b, err := json.Marshal(recordCompanyFactResult{
				Field:  parsed.Field,
				Notice: companyFactNoticeStored,
			})
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	)
}

// buildCompanyFact turns the tool's flat arguments into exactly one
// company.Fact. It never stamps provenance — company.Service derives that from
// how the call arrived, which is the only reason the claim "a human said this"
// is worth anything.
func buildCompanyFact(field string, v companyFactValueArgs) (company.Fact, error) {
	switch strings.TrimSpace(field) {
	case "soa":
		cl, ok := company.ParseClassifica(v.Classifica)
		if !ok {
			return company.Fact{}, fmt.Errorf("value.classifica %q names no SOA tier (I..VIII, incl. III-bis, IV-bis)", v.Classifica)
		}
		validUntil, err := parseToolDate("value.valid_until", v.ValidUntil)
		if err != nil {
			return company.Fact{}, err
		}
		verifiedUntil, err := parseToolDate("value.verified_until", v.VerifiedUntil)
		if err != nil {
			return company.Fact{}, err
		}
		return company.Fact{SOA: &company.SOACategory{
			Category: v.Category, Classifica: cl, IssuedBy: v.IssuedBy,
			ValidUntil: validUntil, VerifiedUntil: verifiedUntil,
		}}, nil

	case "certification":
		validFrom, err := parseToolDate("value.valid_from", v.ValidFrom)
		if err != nil {
			return company.Fact{}, err
		}
		validUntil, err := parseToolDate("value.valid_until", v.ValidUntil)
		if err != nil {
			return company.Fact{}, err
		}
		return company.Fact{Certification: &company.Certification{
			Standard:    company.CertificationStandard(strings.TrimSpace(v.Standard)),
			StandardRaw: v.StandardRaw, Scope: v.Scope, IssuedBy: v.IssuedBy,
			ValidFrom: validFrom, ValidUntil: validUntil,
		}}, nil

	// turnover and headcount are the same row — an esercizio carries both,
	// because Italian selection criteria ask for both per year ("fatturato
	// globale degli ultimi tre esercizi", "organico medio annuo del triennio").
	// They stay two tool fields because they are two questions to a user.
	case "turnover", "headcount":
		if v.Year == 0 {
			return company.Fact{}, fmt.Errorf("value.year is required for a turnover or headcount answer — an esercizio without its year answers no requirement")
		}
		return company.Fact{FinancialYear: &company.FinancialYear{
			Year: v.Year, TurnoverMinor: v.TurnoverMinor,
			SpecificTurnoverMinor: v.SpecificTurnoverMinor,
			Currency:              v.Currency, Headcount: v.Headcount,
		}}, nil

	case "past_contract":
		startedOn, err := parseToolDate("value.started_on", v.StartedOn)
		if err != nil {
			return company.Fact{}, err
		}
		endedOn, err := parseToolDate("value.ended_on", v.EndedOn)
		if err != nil {
			return company.Fact{}, err
		}
		return company.Fact{PastContract: &company.PastContract{
			Description: v.Description, BuyerName: v.BuyerName, CPV: v.CPV,
			ValueMinor: v.ValueMinor, Currency: v.Currency,
			StartedOn: startedOn, EndedOn: endedOn,
			Role: company.ContractRole(strings.TrimSpace(v.Role)), SharePct: v.SharePct,
		}}, nil

	case "registration":
		validFrom, err := parseToolDate("value.valid_from", v.ValidFrom)
		if err != nil {
			return company.Fact{}, err
		}
		validUntil, err := parseToolDate("value.valid_until", v.ValidUntil)
		if err != nil {
			return company.Fact{}, err
		}
		return company.Fact{Registration: &company.Registration{
			Kind: company.RegistrationKind(strings.TrimSpace(v.Kind)), Authority: v.Authority,
			Identifier: v.Identifier, Section: v.Section,
			ValidFrom: validFrom, ValidUntil: validUntil,
		}}, nil
	}

	// Everything else is an identity scalar, checked against the domain's closed
	// set so a typo cannot create a provenance entry nothing will ever read.
	key := company.FieldKey(strings.TrimSpace(field))
	if !company.ValidFieldKey(key) {
		return company.Fact{}, fmt.Errorf("unknown field %q", field)
	}
	return company.Fact{Identity: &company.IdentityFact{Key: key, Value: v.Value}}, nil
}

// ── shared tool-argument parsing ──

// parseToolTenderID reads the tender id the model echoes back from
// search_tenders. It is parsed here rather than deeper down for the same reason
// read_tender_documents parses its own: an id the model garbled is a bad
// argument the model can fix, not a malformed query for Postgres to reject.
func parseToolTenderID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("tender_id %q is not a valid tender id", s)
	}
	return id, nil
}

// parseToolDate accepts both a plain calendar day and a full RFC3339 timestamp.
// The documents these dates come from state a day, and the model emits a day
// more often than not; rejecting "2027-06-30" would turn a correct answer into a
// tool error and cost a turn. An unparseable non-empty value is still an error,
// because dropping a stated expiry silently turns a lapsed certificate into one
// that never expires.
func parseToolDate(field, s string) (*time.Time, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("%s %q must be a date (YYYY-MM-DD) or an RFC3339 timestamp", field, s)
}
