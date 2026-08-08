package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	bagent "github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/providers"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

var ErrChoiceNotPending = errors.New("agent: choice already answered or not found")

// ErrEmptyMessage rejects a turn that carries no text.
//
// It is enforced here rather than in the ConnectRPC handler because it is a
// property of the conversation, not of the wire: every entry point that can
// start a turn must uphold it, and the row it prevents is written by this
// package.
//
// The row is the point. conversationForTurn drops any message whose mapped
// content is empty — it has to, since an empty user or assistant message is
// exactly the shape berrygem's provider path forwards as a blank turn — so
// persisting an empty one produces a row that is durable, visible in
// GetMessages, and invisible to every subsequent turn's prompt. That
// asymmetry is the harm: the transcript the user sees and the transcript the
// model sees stop being the same conversation, silently and permanently.
//
// It is also wrong for the turn that sends it. With the new message dropped,
// the prompt is the prior history verbatim, and the model's most likely
// response to it is to answer the previous question a second time — a
// confident non-sequitur that reads as a broken agent rather than as a
// rejected request. Failing the call with an argument error says the true
// thing to the client instead.
var ErrEmptyMessage = errors.New("agent: a chat message cannot be empty")

// chatRole is the chat_messages.role vocabulary this package persists. These
// five values are the WHOLE vocabulary — "system" is never written, and the
// column has no CHECK constraint or enum backing it (schema.go's role is a
// bare NOT NULL text), so this type is the only place the set is stated.
//
// Naming them is not cosmetics. conversationForTurn switches on this type to
// decide how a persisted row re-enters the model's context, and the five
// InsertMessage call sites write through the same constants. A sixth role
// added later therefore has to be added here to be written at all, at which
// point the missing switch case is one grep away instead of silently falling
// into conversationForTurn's drop-and-warn branch. The type is unexported
// because the vocabulary is this package's business: the transport layer and
// the frontend consume the persisted strings, not this type, so widening it to
// the public API would buy nothing and would leak a domain detail through the
// port.
type chatRole string

const (
	roleUser           chatRole = "user"
	roleAssistant      chatRole = "assistant"
	roleChoicePrompt   chatRole = "choice_prompt"
	roleChoiceResponse chatRole = "choice_response"
	roleTenderResults  chatRole = "tender_results"
)

// ChatRepository is the port agent.Service uses to persist chat sessions and
// messages. Satisfied by *postgres.ChatRepo without any changes there —
// defined here, the consumer, per this codebase's existing pattern for
// workspace/workbench (narrow interfaces owned by the package that needs
// them, not the package that implements them).
type ChatRepository interface {
	CreateSession(ctx context.Context, memberID, workspaceID, workbenchID, agentType, title string) (postgres.DBChatSession, error)
	FindSessionByID(ctx context.Context, id string) (postgres.DBChatSession, error)
	ListSessionsByWorkspace(ctx context.Context, workspaceID string) ([]postgres.DBChatSession, error)
	UpdateSession(ctx context.Context, id, title, workbenchID string) (postgres.DBChatSession, error)
	DeleteSession(ctx context.Context, id string) error
	InsertMessage(ctx context.Context, sessionID, role, content string, choices, metadata, tenders json.RawMessage) (postgres.DBChatMessage, error)
	ListMessagesBySession(ctx context.Context, sessionID string) ([]postgres.DBChatMessage, error)
	FindMessageByID(ctx context.Context, id string) (postgres.DBChatMessage, error)
}

// MemberRepository is the minimal membership-check port agent.Service needs
// — satisfied by *postgres.MemberRepo (which already implements the full
// workspace.MemberRepository interface, a superset of this one).
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

// Workbenches is the narrow port the agent service needs from the workbench
// domain — satisfied by *workbench.Service unchanged. The create_workbench tool
// creates one; chat-access scoping asks whether a caller may see a chat bound
// to a workbench, so a private workbench's chats stay private to its members.
type Workbenches interface {
	CreateWorkbench(ctx context.Context, userID, workspaceID, name, description string, visibility workbench.Visibility) (workbench.Workbench, error)
	// CanAccessWorkbench returns nil if userID may view workbenchID — used to
	// gate a chat bound to that workbench.
	CanAccessWorkbench(ctx context.Context, userID, workbenchID string) error
	// AccessibleWorkbenchIDs returns the workbench IDs in workspaceID the user
	// may view — used to filter workbench-tied chats in a workspace listing.
	AccessibleWorkbenchIDs(ctx context.Context, userID, workspaceID string) (map[string]struct{}, error)
}

// TenderSearcher is the narrow port the search_tenders tool needs —
// satisfied by *tender.Service unchanged.
type TenderSearcher interface {
	Search(ctx context.Context, p tender.SearchParams) (tender.SearchOutput, error)
}

// TenderInspector is the narrow port the get_tender_criteria tool needs —
// satisfied by *tender.Service unchanged. GetTender already rate-limits on its
// own dedicated tier, so the tool inherits a limit without this package
// growing one.
type TenderInspector interface {
	GetTender(ctx context.Context, p tender.GetTenderParams) (tender.TenderDetail, error)
}

// Tenders is the combined tender-domain port this service holds — kept as two
// declared halves rather than one flat interface so each tool's dependency
// stays legible on its own, mirroring how core/bid splits then combines its
// ports. *tender.Service satisfies it unchanged.
type Tenders interface {
	TenderSearcher
	TenderInspector
}

// Documents is the narrow port the read_tender_documents tool needs —
// satisfied by *document.Service unchanged.
//
// Expressed in core/document's own types and never in an adapter's: the
// postgres import at the top of this file (for the chat row types) is existing
// debt, and this addition deliberately does not extend it. Because the port
// speaks only domain vocabulary, the Phase 3 buyer-portal fetcher can be
// composed behind it without anything in this package changing.
type Documents interface {
	Excerpts(ctx context.Context, q document.ExcerptQuery) (document.ExcerptResult, error)
}

// ProfileSource is the subset of clientprofile.Service the agent needs to
// enrich the model's context with the client's bid profile — defined here,
// the consumer, mirroring tender.Service's own ProfileSource. Satisfied by
// *clientprofile.Service unchanged. Get is membership-checked, but the agent
// has already authorized the workspace at GetChat time, so an
// ErrProfileNotFound (or any error) is treated as a silent no-op: the turn
// proceeds with the base instructions.
type ProfileSource interface {
	Get(ctx context.Context, userID, workspaceID string) (clientprofile.Profile, error)
}

// SendChoice is called when the agent asks a closed-ended question — the
// ConnectRPC handler wires it to stream.Send, mirroring StreamToken.
type SendChoice func(ChoicePrompt) error

// SendTenderResults is called whenever search_tenders returns at least one
// result — the ConnectRPC handler wires it to stream.Send, mirroring
// SendChoice/StreamToken. Unlike SendChoice, sending this does NOT end the
// turn: token streaming continues afterward.
type SendTenderResults func(TenderResults) error

// pendingChoice is set by the ask_choice tool, synchronously, before it
// cancels the turn's context — runTurn reads it afterward to tell a
// deliberate pause (waiting on the user) apart from a genuine failure or
// client disconnect, both of which surface identically as ctx.Done().
type pendingChoice struct {
	mu     sync.Mutex
	prompt *ChoicePrompt
}

func (p *pendingChoice) set(cp ChoicePrompt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prompt = &cp
}

func (p *pendingChoice) get() *ChoicePrompt {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prompt
}

// Service holds only its collaborators — no per-session state of any kind.
// That is deliberate and load-bearing: the conversation lives in Postgres,
// runTurn rebuilds it from there on every turn, and every field below is safe
// to share across concurrent requests. Anything added here that is keyed by
// session id reintroduces the per-pod divergence this design removed.
type Service struct {
	registry    *Registry
	chatRepo    ChatRepository
	creditSvc   *credits.Service
	members     MemberRepository
	workbenches Workbenches
	tenders     Tenders
	documents   Documents
	profiles    ProfileSource
	// pod is the process identity stamped into every assistant turn's
	// metadata. It is passed in rather than read here on purpose: core owns
	// what a turn records, and where the name of the machine comes from is a
	// deployment fact, so os.Hostname is called once in main.go and this
	// package never reaches for process identity itself.
	//
	// An empty value is a valid state, not a misconfiguration — a test builds
	// a Service without one — and it renders as an empty string in the JSON
	// rather than being omitted, so a row can never be mistaken for one
	// written before the field existed.
	pod string
	// agentOpts are extra berrygem options appended AFTER the ones
	// Registry.BuildAgent supplies. Empty in production; set only from
	// _test.go. It exists because BuildAgent hardcodes fireworks.New, which
	// leaves no way to drive a turn without a live provider — and runTurn's
	// whole contract is now "what exactly gets sent to the provider", which is
	// only assertable if a test can observe it. berrygem's agent.New applies
	// options in declaration order and WithProvider plainly assigns, so an
	// option appended here overrides the registry's provider without the
	// registry knowing. Unexported and never set by NewService: production
	// behaviour is byte-identical whether or not this field exists.
	agentOpts []bagent.Option
}

// NewService wires the agent domain. pod is the process identity recorded on
// every assistant turn (see Service.pod); pass os.Hostname's result, or "" in a
// context where the answer is meaningless.
func NewService(registry *Registry, chatRepo ChatRepository, creditSvc *credits.Service, members MemberRepository, workbenches Workbenches, tenders Tenders, documents Documents, profiles ProfileSource, pod string) *Service {
	return &Service{
		registry:    registry,
		chatRepo:    chatRepo,
		creditSvc:   creditSvc,
		members:     members,
		workbenches: workbenches,
		tenders:     tenders,
		documents:   documents,
		profiles:    profiles,
		pod:         pod,
	}
}

// requireMember returns workspace.ErrNotMember if userID is not a member of
// workspaceID (translated from the "no rows" case by the concrete
// MemberRepository implementation), or the underlying error on any other
// failure.
func (s *Service) requireMember(ctx context.Context, workspaceID, userID string) error {
	_, err := s.members.LoadMembership(ctx, workspaceID, userID)
	return err
}

// canAccessChat authorizes a caller for one chat session. A workspace-level
// chat (no workbench) is visible to every workspace member; a chat bound to a
// workbench is visible only to callers who may access that workbench, so a
// private workbench's chats stay private to its members. CanAccessWorkbench
// already implies workspace membership, so no separate member check is needed
// on that branch.
func (s *Service) canAccessChat(ctx context.Context, session postgres.DBChatSession, userID string) error {
	if session.WorkbenchID == nil || *session.WorkbenchID == "" {
		return s.requireMember(ctx, session.WorkspaceID, userID)
	}
	return s.workbenches.CanAccessWorkbench(ctx, userID, *session.WorkbenchID)
}

func (s *Service) CreateChat(ctx context.Context, userID, workspaceID, workbenchID, agentType, title string) (postgres.DBChatSession, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	// A chat pinned to a workbench requires access to that workbench, so a
	// caller can't seed a private workbench with a chat they couldn't read back.
	if workbenchID != "" {
		if err := s.workbenches.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
			return postgres.DBChatSession{}, err
		}
	}
	if title == "" {
		title = "Nuova chat"
	}
	return s.chatRepo.CreateSession(ctx, userID, workspaceID, workbenchID, agentType, title)
}

func (s *Service) ListChats(ctx context.Context, userID, workspaceID string) ([]postgres.DBChatSession, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	all, err := s.chatRepo.ListSessionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	accessible, err := s.workbenches.AccessibleWorkbenchIDs(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]postgres.DBChatSession, 0, len(all))
	for _, sess := range all {
		// Workspace-level chats stay visible to every member; a workbench-
		// scoped chat only appears if the caller can see its workbench.
		if sess.WorkbenchID == nil || *sess.WorkbenchID == "" {
			out = append(out, sess)
			continue
		}
		if _, ok := accessible[*sess.WorkbenchID]; ok {
			out = append(out, sess)
		}
	}
	return out, nil
}

func (s *Service) GetChat(ctx context.Context, userID, chatID string) (postgres.DBChatSession, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.canAccessChat(ctx, session, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	return session, nil
}

// GetChatForChoice resolves choiceID to its session, verifying it is a
// choice_prompt that hasn't already been answered, and checks userID is a
// member of the session's workspace. Mirrors GetChat's membership-checked
// lookup so SubmitChoice's handler can credit-check before running, the same
// way ChatStream's handler does via GetChat.
//
// "Not answered" is asked as "no ANSWERING row follows it" rather than as "it
// is the strictly last row", and that distinction is a correctness fix, not a
// generalisation. ListMessagesBySession orders by (created_at, id); created_at
// is now(), i.e. transaction-start time at microsecond resolution, and the id
// is a random uuid — so two rows written microseconds apart in separate
// transactions can share a timestamp and be ordered by a coin flip. A
// tender_results row that ties with the choice_prompt of the same turn and
// wins that flip used to make the prompt permanently unanswerable: every
// SubmitChoice on it returns ErrChoiceNotPending, and nothing the user can do
// changes the row order. Rare, and unrecoverable when it fires.
//
// Only a user turn can answer a choice, so only roleChoiceResponse (the
// answer) and roleUser (the user ignored the prompt and typed something else
// instead, which retires it just as definitively) disqualify it. Rows the
// SAME turn wrote — tender_results, an assistant reply — are not answers and
// no longer decide the question, whichever way the tiebreaker happened to sort
// them. The honest key is still a bigserial sequence column; that needs a
// migration and a backfill and is deferred.
func (s *Service) GetChatForChoice(ctx context.Context, userID, choiceID string) (postgres.DBChatSession, error) {
	promptMsg, err := s.chatRepo.FindMessageByID(ctx, choiceID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if chatRole(promptMsg.Role) != roleChoicePrompt {
		return postgres.DBChatSession{}, ErrChoiceNotPending
	}
	msgs, err := s.chatRepo.ListMessagesBySession(ctx, promptMsg.SessionID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if !choiceStillPending(msgs, promptMsg.ID) {
		return postgres.DBChatSession{}, ErrChoiceNotPending
	}
	session, err := s.chatRepo.FindSessionByID(ctx, promptMsg.SessionID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.canAccessChat(ctx, session, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	return session, nil
}

// choiceStillPending reports whether the choice_prompt with id promptID is
// unanswered in the session transcript msgs, which must be in conversation
// order. A prompt that is not in msgs at all is not pending — it belongs to
// another session, or to none.
//
// See GetChatForChoice for why this asks about the roles that follow rather
// than about the last row.
func choiceStillPending(msgs []postgres.DBChatMessage, promptID string) bool {
	found := false
	for _, m := range msgs {
		if m.ID == promptID {
			found = true
			continue
		}
		if !found {
			continue
		}
		switch chatRole(m.Role) {
		case roleChoiceResponse, roleUser:
			return false
		}
	}
	return found
}

// formatChoiceAnswer turns the user's SubmitChoice selection into the text
// fed back to the agent as the next turn's message.
func formatChoiceAnswer(promptMsg postgres.DBChatMessage, selectedKey, customValue string) (string, error) {
	if selectedKey == "custom" {
		if customValue == "" {
			return "", fmt.Errorf("agent: custom_value is required when selected_key is \"custom\"")
		}
		return customValue, nil
	}
	if promptMsg.Choices == nil {
		return "", fmt.Errorf("agent: choice prompt has no options")
	}
	var options []ChoiceOption
	if err := json.Unmarshal(*promptMsg.Choices, &options); err != nil {
		return "", err
	}
	for _, o := range options {
		if o.Key == selectedKey {
			return fmt.Sprintf("%s) %s", o.Key, o.Label), nil
		}
	}
	return "", fmt.Errorf("agent: %q is not a valid option key", selectedKey)
}

// SubmitChoice resumes sessionID's conversation with the user's answer to
// the choice_prompt at choiceID. It trusts that the caller has already
// authorized session (via GetChatForChoice) — like ChatStream, it does not
// re-check membership itself. It does re-fetch the prompt message (a
// second, cheap indexed lookup after GetChatForChoice's) to read its
// persisted options — accepted duplication rather than threading the row
// through two service methods.
func (s *Service) SubmitChoice(
	ctx context.Context,
	session postgres.DBChatSession,
	userID, choiceID, selectedKey, customValue string,
	sendToken StreamToken,
	sendChoice SendChoice,
	sendTenderResults SendTenderResults,
	sendToolCall SendToolCall,
	usageCh chan<- credits.Usage,
) error {
	promptMsg, err := s.chatRepo.FindMessageByID(ctx, choiceID)
	if err != nil {
		return err
	}
	answerText, err := formatChoiceAnswer(promptMsg, selectedKey, customValue)
	if err != nil {
		return err
	}
	if _, err := s.chatRepo.InsertMessage(ctx, session.ID, string(roleChoiceResponse), answerText, nil, nil, nil); err != nil {
		return err
	}
	return s.runTurn(ctx, session.ID, userID, session.WorkspaceID, session.AgentType, answerText, sendToken, sendChoice, sendTenderResults, sendToolCall, usageCh)
}

func (s *Service) UpdateChat(ctx context.Context, userID, chatID, title, workbenchID string) (postgres.DBChatSession, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.canAccessChat(ctx, session, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	// Moving a chat into a workbench requires access to the destination too, so
	// a caller can't push a chat into a workbench they can't see.
	if workbenchID != "" {
		if err := s.workbenches.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
			return postgres.DBChatSession{}, err
		}
	}
	return s.chatRepo.UpdateSession(ctx, chatID, title, workbenchID)
}

func (s *Service) DeleteChat(ctx context.Context, userID, chatID string) error {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return err
	}
	if err := s.canAccessChat(ctx, session, userID); err != nil {
		return err
	}
	// Nothing to evict: deleting the rows deletes the conversation, because
	// the rows ARE the conversation. There is no in-memory copy to keep in
	// step with them.
	return s.chatRepo.DeleteSession(ctx, chatID)
}

func (s *Service) GetMessages(ctx context.Context, userID, sessionID string) ([]postgres.DBChatMessage, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.canAccessChat(ctx, session, userID); err != nil {
		return nil, err
	}
	return s.chatRepo.ListMessagesBySession(ctx, sessionID)
}

// StreamToken is called by ChatStream for each token.
type StreamToken func(string) error

// turnState holds ONE turn's callbacks, context and per-turn counters, shared
// between runTurn and the tool closures it builds. runTurn allocates a fresh
// one per turn and nothing outlives the turn, so two concurrent turns on the
// same session can no longer see each other's stream, callbacks or cancel
// function.
//
// It still carries a mutex because the values are written by the request
// goroutine and read by berrygem's tool-execution goroutines within the same
// turn — a within-turn handoff, not a cross-turn one.
type turnState struct {
	mu                sync.Mutex
	userID            string
	workspaceID       string
	ctx               context.Context
	sendChoice        SendChoice
	sendTenderResults SendTenderResults
	sendToolCall      SendToolCall
	cancel            context.CancelFunc
	pending           *pendingChoice
	emptyStreak       int
}

func (t *turnState) snapshot() (userID, workspaceID string, ctx context.Context, sendChoice SendChoice, sendTenderResults SendTenderResults, cancel context.CancelFunc, pending *pendingChoice) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.userID, t.workspaceID, t.ctx, t.sendChoice, t.sendTenderResults, t.cancel, t.pending
}

// currentSendToolCall returns this turn's sendToolCall under the mutex — read
// by emitToolCall from whichever goroutine berrygem executes the tool on.
func (t *turnState) currentSendToolCall() SendToolCall {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sendToolCall
}

// recordSearchResult updates this turn's consecutive-empty-search streak —
// incrementing on an empty result, resetting on a non-empty one — and returns
// the streak's new value. It lives on turnState rather than in a
// newSearchTendersTool closure variable because the streak is what drives a
// prompt-level "stop searching" notice, and berrygem may execute several
// search_tenders calls concurrently within one turn; the mutex here is what
// makes the count well-defined. Being per-turn is now automatic — the whole
// turnState is.
func (t *turnState) recordSearchResult(empty bool) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if empty {
		t.emptyStreak++
	} else {
		t.emptyStreak = 0
	}
	return t.emptyStreak
}

// runTurn drives one berrygem turn-loop invocation: builds the agent with this
// session's tools, hands it the WHOLE conversation assembled from Postgres,
// and either persists the assistant's reply and reports full usage, or — if
// the agent invoked ask_choice — returns without a reply row and reports the
// partial turn's (estimated) usage, leaving the persisted choice_prompt as the
// resume point for a later SubmitChoice.
//
// Every path that reached the provider reports usage, including the failing
// one. Once the request is out the door the turn has cost money, and whether
// the user stayed to read the answer does not change that.
//
// Callers persist turnMessage themselves BEFORE calling (ChatStream as a
// "user" row, SubmitChoice's resume as a "choice_response" one), so by the
// time the history is loaded below, turnMessage is already its last element.
// That invariant is what lets this function hand berrygem the transcript and
// nothing else: passing turnMessage separately as well is what used to append
// the current turn twice. runTurn only persists what happens *after*.
//
// There is deliberately no in-memory chat here. Postgres is the single copy of
// the conversation; a second, process-local copy is what made a session's
// context depend on which replica served it and silently dropped every
// assistant reply (berrygem's chat.Chat records the reply only via
// CompleteStream, which this service never called).
func (s *Service) runTurn(
	ctx context.Context,
	sessionID, userID, workspaceID, agentType, turnMessage string,
	sendToken StreamToken,
	sendChoice SendChoice,
	sendTenderResults SendTenderResults,
	sendToolCall SendToolCall,
	usageCh chan<- credits.Usage,
) error {
	cfg, ok := s.registry.GetConfig(AgentType(agentType))
	if !ok {
		// Through GetConfig, not the configs map directly: the map is guarded
		// by the registry's RWMutex and reading it from here bypassed that
		// lock. Benign while configs are only written by RegisterDefaults at
		// startup, but it is a race the moment they are not, and go test -race
		// would then flag it in whatever code happened to be running.
		cfg, _ = s.registry.GetConfig(AgentTypeBaseChat)
	}

	// Fold the client's bid profile into this build's instructions (context,
	// not filter injection). The agent is rebuilt from scratch every turn, so
	// unlike before a mid-session profile edit takes effect on the next turn
	// rather than never. ErrProfileNotFound (the common case) and any error
	// fall through to the base instructions unchanged.
	if prof, err := s.profiles.Get(ctx, userID, workspaceID); err == nil {
		if block := buildProfileContext(prof); block != "" {
			cfg.Instructions = cfg.Instructions + "\n\n" + block
		}
	}

	streamCtx, cancelForChoice := context.WithCancel(ctx)
	defer cancelForChoice()
	pending := &pendingChoice{}

	// Fresh per turn, not looked up per session: the tool closures below are
	// built for this turn and discarded with it, so there is nothing for a
	// later turn to inherit. Two overlapping turns on one session now each get
	// their own, instead of the second one overwriting the first's live stream
	// context and cancel function mid-flight.
	ts := &turnState{
		userID:            userID,
		workspaceID:       workspaceID,
		ctx:               streamCtx,
		sendChoice:        sendChoice,
		sendTenderResults: sendTenderResults,
		sendToolCall:      sendToolCall,
		cancel:            cancelForChoice,
		pending:           pending,
	}

	askChoice := func(question string, options []ChoiceOption, allowCustom bool) error {
		_, _, curCtx, curSendChoice, _, curCancel, curPending := ts.snapshot()
		choicesJSON, err := json.Marshal(options)
		if err != nil {
			return err
		}
		msg, err := s.chatRepo.InsertMessage(curCtx, sessionID, string(roleChoicePrompt), question, choicesJSON, nil, nil)
		if err != nil {
			return err
		}
		cp := ChoicePrompt{ID: msg.ID, Question: question, Options: options, AllowCustom: allowCustom}
		if err := curSendChoice(cp); err != nil {
			return err
		}
		// Set pending only AFTER sendChoice succeeds — if the push to the
		// client fails (e.g. mid-disconnect), this turn must still be
		// classified as a genuine failure below, not a delivered pause.
		curPending.set(cp)
		curCancel()
		return nil
	}

	createWorkbench := func(name, description string, visibility workbench.Visibility) (workbench.Workbench, error) {
		curUserID, curWorkspaceID, curCtx, _, _, _, _ := ts.snapshot()
		return s.workbenches.CreateWorkbench(curCtx, curUserID, curWorkspaceID, name, description, visibility)
	}

	searchTenders := func(query, country, cpv, status string) ([]tender.ScoredTender, error) {
		curUserID, _, curCtx, _, curSendTenderResults, _, _ := ts.snapshot()
		out, err := s.tenders.Search(curCtx, tender.SearchParams{
			Query: query,
			// The tool exposes one value per facet; Filters takes lists, so
			// each is wrapped. SingleFilter drops the empty ones — an unset
			// tool argument must contribute no predicate at all.
			Filters: tender.Filters{
				Countries:   tender.SingleFilter(country),
				CPVPrefixes: tender.SingleFilter(cpv),
				Statuses:    tender.SingleFilter(status),
			},
			// The model builds this query from what the user asked for, so it
			// carries the same buried constraints a typed search would.
			ParseQuery:    true,
			Limit:         searchTendersToolLimit,
			Authenticated: true,
			RateLimitKey:  curUserID,
		})
		if err != nil {
			return nil, err
		}
		if len(out.Results) > 0 {
			if err := s.persistAndNotifyTenderResults(curCtx, sessionID, out.Results, curSendTenderResults); err != nil {
				return nil, err
			}
		}
		return out.Results, nil
	}

	// getTenderCriteria deliberately persists nothing and pushes no stream
	// event: the detail travels back to the model inside the tool's own string
	// result and nowhere else. That was originally a workaround for the mapper
	// dropping any payload held outside `content`; it is now a plain
	// cost/scope choice, since conversationForTurn could carry such a row if
	// one existed. Adding a "criteria_results" role is Phase 1b's call — it
	// needs a mapper case here and a decision about how much of a criteria
	// grid is worth re-sending every turn. Until then the data is in Postgres,
	// keyed by tender id, and the model re-reads it by calling the tool again.
	getTenderCriteria := func(tenderID string) (tender.TenderDetail, error) {
		curUserID, _, curCtx, _, _, _, _ := ts.snapshot()
		return s.tenders.GetTender(curCtx, tender.GetTenderParams{ID: tenderID, RateLimitKey: curUserID})
	}

	// readTenderDocuments parses the model-supplied id here rather than pushing
	// a string id into the document domain: core/document keys on the numeric
	// tender id its Reader queries by, and an id the model garbled is a
	// not-found, not a malformed query for Postgres to reject.
	//
	// Limit is left at MaxExcerpts because the tool exposes no knob for it —
	// the model cannot ask for more text than the domain's budget allows, which
	// is the whole reason that budget lives in core/document.
	readTenderDocuments := func(tenderID, question string) (document.ExcerptResult, error) {
		_, _, curCtx, _, _, _, _ := ts.snapshot()
		id, err := strconv.ParseInt(tenderID, 10, 64)
		if err != nil || id <= 0 {
			return document.ExcerptResult{}, document.ErrTenderNotFound
		}
		return s.documents.Excerpts(curCtx, document.ExcerptQuery{
			TenderID: id,
			Question: question,
			Limit:    document.MaxExcerpts,
		})
	}

	opts := append([]bagent.Option{
		bagent.WithTools(
			newAskChoiceTool(askChoice),
			newCreateWorkbenchTool(ts, createWorkbench),
			newSearchTendersTool(ts, searchTenders),
			newGetTenderCriteriaTool(ts, getTenderCriteria),
			newReadTenderDocumentsTool(ts, readTenderDocuments),
		),
	}, s.agentOpts...)
	ag, err := s.registry.BuildAgent(cfg, opts...)
	if err != nil {
		return err
	}

	// One indexed read per turn against idx_chat_messages_session — the same
	// query GetMessages already runs on every page load. turnMessage is
	// already the last row here (the caller persisted it), so it must NOT be
	// passed to the provider path a second time.
	history, err := s.chatRepo.ListMessagesBySession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Kept in a variable rather than inlined into the call because it is also
	// what gets measured: buildTurnMetadata sizes this exact slice, so the
	// number stamped on the reply is the prompt that was actually sent and not
	// a second, drifting reconstruction of it.
	conversation := conversationForTurn(ctx, history)

	result, err := ag.StreamWithMessages(streamCtx, conversation)
	if err != nil {
		return err
	}
	defer result.Close()

	fullContent, done, err := s.consumeStream(streamCtx, result.C, result.Err, result.Done, sendToken)
	if err != nil {
		if pending.get() != nil {
			// Deliberate pause: ask_choice cancelled streamCtx on purpose.
			// The choice_prompt message is already persisted (inside
			// askChoice, above), which is the entire resume state
			// SubmitChoice needs — it rebuilds the conversation from the rows
			// like any other turn. Report this partial turn's usage the same
			// estimated way a turn with no real provider usage already is
			// (see estimateUsage).
			sendUsage(usageCh, estimateUsage(sessionID, agentType, cfg.Model, turnMessage, fullContent, nil))
			return nil
		}
		// A genuine failure — client disconnect, a sendToken that could not
		// reach the client, or a provider error mid-stream. Report usage
		// anyway, BEFORE returning: by the time control reaches here the
		// provider call has been made and any tools have run, so this turn cost
		// real money whether or not the user ever saw its output. It used to
		// return silently, which made an aborted turn free — repeatable for
		// free by closing the tab mid-stream.
		//
		// This half is load-bearing for the other one. The handler cannot
		// simply move its usage read after the error: nothing sent on the
		// channel here, so that read would block forever. Both sides change
		// together (the handler drains non-blockingly; see billAbortedTurn).
		//
		// fullContent holds whatever was streamed before the failure, so a turn
		// that produced nothing bills the estimate floor and one that produced
		// 2KB bills for it.
		//
		// Nothing to clean up otherwise. A failed turn wrote no assistant row,
		// and the rows that WERE written before it failed (the user message,
		// any tender_results) are all genuine facts about the conversation, so
		// the next turn should see them. The eviction that used to happen here
		// existed only to discard a half-built in-memory transcript that no
		// longer exists.
		sendUsage(usageCh, estimateUsage(sessionID, agentType, cfg.Model, turnMessage, fullContent, nil))
		return err
	}

	usage := estimateUsage(sessionID, agentType, cfg.Model, turnMessage, fullContent, done)

	// The reply row carries this turn's own measurement of the billing gap —
	// see buildTurnMetadata. Usage is reported whether or not the insert
	// succeeds: the reply was already streamed and already cost money, so a
	// failure to persist it is not a reason to hand the turn out for free. A
	// non-nil insertErr takes the handler's error path, which drains this same
	// channel and deducts there.
	_, insertErr := s.chatRepo.InsertMessage(ctx, sessionID, string(roleAssistant), fullContent, nil, s.buildTurnMetadata(ctx, sessionID, cfg.Instructions, conversation, usage), nil)
	sendUsage(usageCh, usage)
	return insertErr
}

// persistAndNotifyTenderResults is called by the search_tenders closure
// whenever a search returns at least one result: it persists a
// "tender_results" row (so a page reload reconstructs the cards, mirroring
// how askChoice persists "choice_prompt") and pushes the live
// SendTenderResults event.
func (s *Service) persistAndNotifyTenderResults(
	ctx context.Context, sessionID string, results []tender.ScoredTender, sendTenderResults SendTenderResults,
) error {
	tendersJSON, err := marshalTenderResultsForHistory(results)
	if err != nil {
		return err
	}
	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, string(roleTenderResults), "", nil, nil, tendersJSON); err != nil {
		return err
	}
	return sendTenderResults(TenderResults{Tenders: results})
}

// estimateUsage computes what a turn is billed. When real is nil (a paused or
// failed turn) or carries zero usage (berrygem's streaming client never sets
// stream_options.include_usage on the OpenAI-compatible request — verified
// against its vendored source — so Fireworks never returns usage in streaming
// mode, which means this fallback is taken on every turn today), it falls back
// to a character-length estimate rather than silently billing nothing.
//
// inputText is the user's own message for this turn and nothing else. That is
// deliberately NOT the prompt: the prompt also carries the system
// instructions, the profile block, the whole transcript, the tool schemas and
// every tool result, so the billed input is a fraction of the real one — see
// buildTurnMetadata, which measures the gap instead of closing it.
//
// Split from sendUsage so the same value can be both stamped on the reply row
// and pushed to the handler. Computing it twice from the same inputs would
// work today and drift the first time either side grows a term.
func estimateUsage(sessionID, agentType, model, inputText, outputText string, real *bagent.RunResult) credits.Usage {
	var inputTokens, outputTokens, totalTokens int32
	if real != nil {
		inputTokens = int32(real.Usage.PromptTokens)
		outputTokens = int32(real.Usage.CompletionTokens)
		totalTokens = int32(real.Usage.TotalTokens)
	}
	if totalTokens == 0 {
		inputTokens = estimateTokens(inputText)
		outputTokens = estimateTokens(outputText)
		totalTokens = inputTokens + outputTokens
	}
	return credits.Usage{
		AgentType:    agentType,
		SessionID:    sessionID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}
}

// sendUsage hands one turn's usage to the handler, which deducts it. The
// channel is buffered with room for exactly one turn and every path through
// runTurn sends at most once, so this never blocks and never overwrites a
// value the handler has not read yet.
func sendUsage(usageCh chan<- credits.Usage, usage credits.Usage) {
	if usageCh == nil {
		return
	}
	usageCh <- usage
}

// turnMetadata is the JSON stamped into the assistant reply's metadata column
// (chat_messages.metadata, a jsonb that was written NULL at every call site
// until now). It is pure instrumentation: the frontend does not read the
// column, no balance depends on it, and a row without it is simply a turn from
// before this shipped.
//
// Two of the numbers exist to be divided by each other. CtxChars/4
// approximates what the turn's prompt really cost in tokens; EstBilledTokens
// is what the user was actually charged for it. The ratio is the size of the
// gap, per session and per day, straight out of Postgres.
//
// The other two are the proof that the defect this phase fixed is gone.
// CtxMsgs lets one query recompute, from the rows themselves, how much of its
// own session each turn actually saw — an assertion the turn writes about
// itself in the same transaction that records it — and Pod is what makes the
// unverified inference about cross-pod sessions answerable from the database
// instead of from a cluster nobody can query. Neither is derivable after the
// fact: nothing recorded what any pre-fix turn was shown, which is why the
// before state is permanently unmeasurable and this is the only evidence
// there will be.
//
// Recording it is NOT the same as fixing it, and that separation is the whole
// point of doing it this way. The billing basis is unchanged by this commit —
// changing what every customer is charged, by a multiple nobody has measured
// yet, is a pricing decision, and pricing decisions on this project get made
// on data (see the 0009 CPV-label revert). This is the instrument that makes
// that data exist; the decision itself comes later and separately, and until
// it is made anyone reading token_usage_log must know it undercounts.
type turnMetadata struct {
	// CtxMsgs is how many messages conversationForTurn emitted for this turn,
	// i.e. how many rows of its own session the model was shown. It counts the
	// EMITTED messages and not the rows read, because the two differ by
	// exactly the rows the mapper drops (an unknown role, a payload-less
	// tender_results) and it is the emitted number that the prompt contains.
	// The system instructions are not one of them.
	CtxMsgs int `json:"ctx_msgs"`
	// CtxChars is the character length of the prompt that was actually sent:
	// the agent's instructions (with the client-profile block already folded
	// in) plus every message conversationForTurn emitted. Still an
	// undercount — the tool schemas berrygem serializes and the tool results
	// exchanged inside the turn loop are not visible from here — so the ratio
	// it feeds is a floor on the gap, never an exaggeration of it.
	CtxChars int `json:"ctx_chars"`
	// EstBilledTokens is what this turn charged the workspace, i.e.
	// estimateUsage's total. Stamped next to CtxChars so a single row answers
	// the pricing question without a join to token_usage_log.
	EstBilledTokens int32 `json:"est_billed_tokens"`
	// Pod is the process that served the turn. With two replicas behind
	// Traefik's default round-robin and no session affinity anywhere in the
	// manifests, consecutive turns of one session routinely land on different
	// processes — how often was an inference nobody could observe, and this
	// makes it a GROUP BY. Post-fix it is informational rather than a defect:
	// a session spanning both pods is now correct by construction, because
	// nothing about a turn depends on which process ran the previous one.
	Pod string `json:"pod"`
}

// ctxCharsWarnThreshold is the prompt size, in characters, past which a turn
// logs a warning. Roughly 40k tokens at this file's ~4-chars-per-token
// approximation — well inside any model this service targets, and far outside
// what a healthy bid conversation reaches, so it fires on the shape that is
// actually worrying (a session whose transcript has grown without bound)
// rather than on a long question.
//
// It is deliberately a LOG LINE AND NOTHING ELSE. Every turn now sends the
// whole transcript, which is the point — that is what makes the model see its
// own session — so real provider cost per session went from roughly linear to
// quadratic while the credits ledger, which bills the user's typed string,
// stayed flat. That gap is invisible on this team's own dashboard, and a
// silent one is how a month of it reaches the invoice before anyone looks.
// Capping the context instead would trade a cost surprise for a correctness
// one: a long session would silently start losing its own history again,
// which is the exact defect this phase removed.
const ctxCharsWarnThreshold = 160_000

// buildTurnMetadata sizes the prompt that was just sent and pairs it with what
// the turn billed. Returning nil on a marshalling failure leaves the column
// NULL, which is exactly the pre-instrumentation state: a measurement must
// never be able to fail a turn whose reply the user has already read. (These
// fields cannot in fact fail to marshal; the branch is there so that stays
// true if the struct grows.)
func (s *Service) buildTurnMetadata(ctx context.Context, sessionID, instructions string, conversation []providers.Message, usage credits.Usage) json.RawMessage {
	chars := len(instructions)
	for _, m := range conversation {
		chars += len(m.Content)
	}

	// Warned from here rather than from runTurn so the number that is logged
	// is the same one that is stored, computed once. A session over the line
	// is not failed and not truncated — see ctxCharsWarnThreshold.
	if chars > ctxCharsWarnThreshold {
		slog.WarnContext(ctx, "agent: turn context is unusually large",
			"session_id", sessionID, "ctx_chars", chars, "ctx_msgs", len(conversation),
			"threshold", ctxCharsWarnThreshold)
	}

	b, err := json.Marshal(turnMetadata{
		CtxMsgs:         len(conversation),
		CtxChars:        chars,
		EstBilledTokens: usage.TotalTokens,
		Pod:             s.pod,
	})
	if err != nil {
		return nil
	}
	return b
}

// ChatStream runs the Berrygem agent streaming loop for a fresh user
// message. It trusts that the caller has already authorized sessionID's
// workspace (the ConnectRPC handler does this by calling GetChat, which is
// membership-checked, before ChatStream) — it does not re-check membership
// itself, to avoid a redundant FindSessionByID round trip on the hot path.
func (s *Service) ChatStream(
	ctx context.Context,
	sessionID, userID, workspaceID, message, agentType string,
	sendToken StreamToken,
	sendChoice SendChoice,
	sendTenderResults SendTenderResults,
	sendToolCall SendToolCall,
	usageCh chan<- credits.Usage,
) error {
	// Checked before the insert, so a rejected turn leaves no trace at all —
	// see ErrEmptyMessage for why a persisted empty row is worse than a
	// refused call. Whitespace counts as empty: " " survives a != "" test,
	// reaches the provider as a blank turn, and is the trivial way to walk
	// past the guard.
	if strings.TrimSpace(message) == "" {
		return ErrEmptyMessage
	}
	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, string(roleUser), message, nil, nil, nil); err != nil {
		return err
	}
	return s.runTurn(ctx, sessionID, userID, workspaceID, agentType, message, sendToken, sendChoice, sendTenderResults, sendToolCall, usageCh)
}

// estimateTokens roughly approximates a token count from text length for the
// stream_options.include_usage fallback above — ~4 characters per token is a
// standard rough approximation for GPT-style tokenizers. Never zero, so even
// a short real exchange still costs at least one token.
func estimateTokens(s string) int32 {
	n := int32(len(s) / 4)
	if n < 1 {
		return 1
	}
	return n
}

// buildProfileContext renders the client's bid profile into a compact Italian
// instruction block appended to the agent's base instructions, so the model
// knows the client's criteria when it chooses search_tenders arguments. Only
// present fields produce a line; an all-empty profile yields "". This never
// forces filter values — search_tenders stays client-agnostic (the model, and
// the user's explicit request, still decide the actual arguments).
func buildProfileContext(p clientprofile.Profile) string {
	var lines []string
	if len(p.Sectors) > 0 {
		lines = append(lines, "- Settori (CPV): "+strings.Join(p.Sectors, ", "))
	}
	if len(p.Countries) > 0 {
		lines = append(lines, "- Paesi: "+strings.Join(p.Countries, ", "))
	}
	if len(p.Regions) > 0 {
		lines = append(lines, "- Regioni (NUTS): "+strings.Join(p.Regions, ", "))
	}
	if len(p.ProcedureTypes) > 0 {
		lines = append(lines, "- Tipi di procedura: "+strings.Join(p.ProcedureTypes, ", "))
	}
	if p.ValueMin != nil || p.ValueMax != nil {
		var v string
		switch {
		case p.ValueMin != nil && p.ValueMax != nil:
			v = fmt.Sprintf("da %d a %d EUR", *p.ValueMin, *p.ValueMax)
		case p.ValueMin != nil:
			v = fmt.Sprintf("da %d EUR", *p.ValueMin)
		default:
			v = fmt.Sprintf("fino a %d EUR", *p.ValueMax)
		}
		lines = append(lines, "- Valore stimato: "+v)
	}
	if strings.TrimSpace(p.Notes) != "" {
		lines = append(lines, "- Note: "+p.Notes)
	}
	if len(lines) == 0 {
		return ""
	}
	return "Profilo del cliente (usa questi criteri quando cerchi bandi, ma rispetta " +
		"sempre eventuali richieste esplicite dell'utente):\n" + strings.Join(lines, "\n")
}

// The labels that wrap a persisted row's out-of-content payload when it is
// replayed to the model. Italian, matching buildProfileContext and the base
// instructions: these strings sit inside an Italian conversation as ordinary
// turns, and an English frame around Italian content reads to the model as a
// different speaker. (The tools' notices in tools.go are English on purpose —
// those are instructions ABOUT a tool result, not part of the transcript.)
const (
	// tenderResultsHeader introduces the JSON the user was actually shown as
	// cards, so "il secondo mi interessa" resolves to a specific tender
	// instead of to nothing.
	tenderResultsHeader = "[Risultati di ricerca mostrati all'utente come schede, in JSON. " +
		"Usa questi dati per rispondere a riferimenti tipo \"il secondo\"; non inventarne altri.]"
	// tenderResultsSuperseded stands in for an older search's payload. Only
	// the most recent tender_results row carries its data (see
	// conversationForTurn) — the rest collapse to this one line so a long
	// session's prompt stays bounded.
	tenderResultsSuperseded = "[Una ricerca precedente ha mostrato %d bandi all'utente; " +
		"i dettagli non sono riportati qui. Se servono, esegui di nuovo la ricerca.]"
	// choicePromptOptionsHeader precedes the option list flattened into the
	// assistant's own question.
	choicePromptOptionsHeader = "Opzioni proposte:"
)

// conversationForTurn assembles the messages sent to the provider for one turn
// from the session's persisted rows — the single source of truth for what the
// model sees.
//
// Every persisted role gets an explicit case. That is the whole point: the
// previous implementation copied {Role, Content} verbatim, which sent
// tender_results (whose payload lives in the tenders column, with content
// deliberately empty) as an empty message, and sent choice_prompt — the
// assistant's own question — under the role "choice_prompt", which berrygem's
// OpenAI-compatible provider silently coerces to "user" in a default: branch.
// The invariant this function now guarantees, and which its tests assert
// directly, is that no message it emits can reach that default: branch: every
// output role is one the provider knows.
//
// Roles are mapped, never passed through:
//
//   - user             -> RoleUser, verbatim
//   - assistant        -> RoleAssistant, verbatim
//   - choice_response  -> RoleUser, verbatim. Stated explicitly rather than
//     left to coercion, so a berrygem upgrade that tightens its default:
//     branch cannot turn this into an error.
//   - choice_prompt    -> RoleAssistant, question + the options flattened in.
//     The options must travel with it: formatChoiceAnswer emits answers shaped
//     "A) Label", so the following turn is uninterpretable without the key/
//     label list. Reconstructing it as an assistant tool_calls entry plus a
//     RoleTool result is not an option — no tool-call id was ever persisted,
//     and an assistant tool_calls with no matching tool response is rejected
//     outright by OpenAI-compatible backends.
//   - tender_results   -> RoleUser, payload for the LATEST such row only.
//     RoleUser and not RoleTool for the same missing-tool-call-id reason, and
//     not RoleSystem for two: the titles and buyer names are third-party
//     strings ingested from TED, and promoting attacker-influenceable text to
//     system authority is a prompt-injection downgrade; and berrygem's
//     ensureSystemMessage only prepends the instructions when messages[0] is
//     not already RoleSystem, so a tender_results row at index 0 would delete
//     the entire system prompt.
//   - anything else    -> dropped, with a warning.
//
// Dropping an unknown role rather than passing its content through is
// deliberate: a turn attributed to the wrong speaker is worse than an absent
// one. With the current five-role vocabulary the branch is unreachable, so any
// occurrence in the logs means a sixth role was added without a case here.
//
// Rows whose mapped content is empty are dropped too — hence append onto a
// zero-length slice rather than index assignment into a fixed-length one.
func conversationForTurn(ctx context.Context, msgs []postgres.DBChatMessage) []providers.Message {
	// Only the most recent tender_results row replays its payload. A session
	// that searched ten times would otherwise re-send ten result sets on every
	// subsequent turn, growing the prompt quadratically for data the user has
	// almost certainly moved on from — "il secondo" refers to the latest
	// search. Older rows still leave a trace (see tenderResultsSuperseded) so
	// the model knows a search happened rather than silently forgetting it.
	latestTenderResults := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if chatRole(msgs[i].Role) == roleTenderResults {
			latestTenderResults = i
			break
		}
	}

	out := make([]providers.Message, 0, len(msgs))
	for i, m := range msgs {
		var msg providers.Message
		switch chatRole(m.Role) {
		case roleUser:
			msg = providers.Message{Role: providers.RoleUser, Content: m.Content}
		case roleAssistant:
			msg = providers.Message{Role: providers.RoleAssistant, Content: m.Content}
		case roleChoiceResponse:
			msg = providers.Message{Role: providers.RoleUser, Content: m.Content}
		case roleChoicePrompt:
			msg = providers.Message{Role: providers.RoleAssistant, Content: renderChoicePrompt(m)}
		case roleTenderResults:
			msg = providers.Message{Role: providers.RoleUser, Content: renderTenderResults(m, i == latestTenderResults)}
		default:
			slog.WarnContext(ctx, "agent: unknown persisted chat role; dropped from rehydrated context", "role", m.Role)
			continue
		}
		if msg.Content == "" {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// renderChoicePrompt flattens a choice_prompt row back into the assistant
// prose it stood for: the question, then its options in the same "A) Label"
// shape formatChoiceAnswer uses for the answer, so the two read as one
// exchange. A row whose choices column is absent or unparseable still yields
// the question — a question without its options is degraded, but an empty
// message is a defect.
func renderChoicePrompt(m postgres.DBChatMessage) string {
	if m.Choices == nil {
		return m.Content
	}
	var options []ChoiceOption
	if err := json.Unmarshal(*m.Choices, &options); err != nil || len(options) == 0 {
		return m.Content
	}
	lines := make([]string, 0, len(options)+2)
	if m.Content != "" {
		lines = append(lines, m.Content)
	}
	lines = append(lines, choicePromptOptionsHeader)
	for _, o := range options {
		line := fmt.Sprintf("%s) %s", o.Key, o.Label)
		if o.Description != "" {
			line += " — " + o.Description
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderTenderResults turns a tender_results row into a labelled block around
// the JSON the user was shown as cards. Only the latest such row carries its
// payload; every earlier one collapses to a one-line placeholder naming how
// many results it held. A row with no tenders column renders empty and is
// dropped by the caller — that is a row that never carried a payload, not a
// payload that was lost. A superseded row whose JSON will not parse as an
// array is dropped the same way: the placeholder's only content is the count,
// so with nothing countable there is nothing to say.
func renderTenderResults(m postgres.DBChatMessage, latest bool) string {
	if m.Tenders == nil {
		return ""
	}
	if latest {
		return tenderResultsHeader + "\n" + string(*m.Tenders)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(*m.Tenders, &items); err != nil {
		return ""
	}
	return fmt.Sprintf(tenderResultsSuperseded, len(items))
}

// consumeStream drains a Berrygem stream's three channels, guarding against
// re-selecting an already-exhausted (closed) channel — which berrygem closes
// together, always, when its internal loop returns for any reason (success,
// error, or ctx cancellation; verified against its source). Without this
// guard, once all three close near-simultaneously, select's uniform-random
// case choice can burn several iterations re-picking exhausted cases before
// landing on the one real buffered value — wasteful, and in the (per
// berrygem's contract, unreachable in practice) case where NONE of them ever
// carried a value, it would hang forever instead of returning an error.
//
// Berrygem's producer goroutine sends every content chunk to `content` and
// THEN sends the terminal value to `done`/`errs`, sequentially, from the
// SAME goroutine — so by the time a `done`/`errs` value is observable,
// every content chunk for this turn is already either received or sitting
// in `content`'s buffer (capacity 64). But Go's `select` does NOT preserve
// that producer-side ordering across DIFFERENT channels: if `content` and
// `done` are simultaneously ready (reachable for any reply short enough to
// fully fill the buffer before this goroutine's first scheduling turn),
// `select` can pick `done` before `content` is fully drained, silently
// truncating `fullContent` while still reporting success. Fix: on the
// `errs`/`done` branches specifically, drain any remaining buffered
// `content` — non-blockingly, since nothing new can arrive on it after
// `errs`/`done` per berrygem's sequential-send contract — before returning.
func (s *Service) consumeStream(
	ctx context.Context,
	content <-chan string,
	errs <-chan error,
	done <-chan *bagent.RunResult,
	sendToken StreamToken,
) (fullContent string, result *bagent.RunResult, err error) {
	drainContent := func() {
		for {
			select {
			case chunk, ok := <-content:
				if !ok {
					return
				}
				fullContent += chunk
				_ = sendToken(chunk) // best-effort: the result is already decided either way
			default:
				return
			}
		}
	}

	for {
		select {
		case chunk, ok := <-content:
			if !ok {
				content = nil
				break
			}
			fullContent += chunk
			if err := sendToken(chunk); err != nil {
				return fullContent, nil, err
			}

		case e, ok := <-errs:
			if !ok {
				errs = nil
				break
			}
			drainContent()
			return fullContent, nil, e

		case r, ok := <-done:
			if !ok {
				done = nil
				break
			}
			drainContent()
			return fullContent, r, nil

		case <-ctx.Done():
			return fullContent, nil, ctx.Err()
		}

		if content == nil && errs == nil && done == nil {
			return fullContent, nil, errors.New("agent: stream ended without a result")
		}
	}
}
