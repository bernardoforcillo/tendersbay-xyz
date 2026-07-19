package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	bagent "github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/providers"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

var ErrChoiceNotPending = errors.New("agent: choice already answered or not found")

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

// WorkbenchCreator is the narrow port the create_workbench tool needs —
// satisfied by *workbench.Service unchanged.
type WorkbenchCreator interface {
	CreateWorkbench(ctx context.Context, userID, workspaceID, name, description string, visibility workbench.Visibility) (workbench.Workbench, error)
}

// TenderSearcher is the narrow port the search_tenders tool needs —
// satisfied by *tender.Service unchanged.
type TenderSearcher interface {
	Search(ctx context.Context, p tender.SearchParams) (tender.SearchOutput, error)
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

type Service struct {
	registry     *Registry
	chatRepo     ChatRepository
	creditSvc    *credits.Service
	members      MemberRepository
	workbenches  WorkbenchCreator
	tenders      TenderSearcher
	turnStates   map[string]*turnState
	turnStatesMu sync.Mutex
}

func NewService(registry *Registry, chatRepo ChatRepository, creditSvc *credits.Service, members MemberRepository, workbenches WorkbenchCreator, tenders TenderSearcher) *Service {
	return &Service{
		registry:    registry,
		chatRepo:    chatRepo,
		creditSvc:   creditSvc,
		members:     members,
		workbenches: workbenches,
		tenders:     tenders,
		turnStates:  make(map[string]*turnState),
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

func (s *Service) CreateChat(ctx context.Context, userID, workspaceID, workbenchID, agentType, title string) (postgres.DBChatSession, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return postgres.DBChatSession{}, err
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
	return s.chatRepo.ListSessionsByWorkspace(ctx, workspaceID)
}

func (s *Service) GetChat(ctx context.Context, userID, chatID string) (postgres.DBChatSession, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.requireMember(ctx, session.WorkspaceID, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	return session, nil
}

// GetChatForChoice resolves choiceID to its session, verifying it is a
// choice_prompt that hasn't already been answered (i.e. it's still the
// last message in its session), and checks userID is a member of the
// session's workspace. Mirrors GetChat's membership-checked lookup so
// SubmitChoice's handler can credit-check before running, the same way
// ChatStream's handler does via GetChat.
func (s *Service) GetChatForChoice(ctx context.Context, userID, choiceID string) (postgres.DBChatSession, error) {
	promptMsg, err := s.chatRepo.FindMessageByID(ctx, choiceID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if promptMsg.Role != "choice_prompt" {
		return postgres.DBChatSession{}, ErrChoiceNotPending
	}
	msgs, err := s.chatRepo.ListMessagesBySession(ctx, promptMsg.SessionID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if len(msgs) == 0 || msgs[len(msgs)-1].ID != promptMsg.ID {
		return postgres.DBChatSession{}, ErrChoiceNotPending
	}
	session, err := s.chatRepo.FindSessionByID(ctx, promptMsg.SessionID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.requireMember(ctx, session.WorkspaceID, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	return session, nil
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
	if _, err := s.chatRepo.InsertMessage(ctx, session.ID, "choice_response", answerText, nil, nil, nil); err != nil {
		return err
	}
	return s.runTurn(ctx, session.ID, userID, session.WorkspaceID, session.AgentType, answerText, sendToken, sendChoice, sendTenderResults, usageCh)
}

func (s *Service) UpdateChat(ctx context.Context, userID, chatID, title, workbenchID string) (postgres.DBChatSession, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return postgres.DBChatSession{}, err
	}
	if err := s.requireMember(ctx, session.WorkspaceID, userID); err != nil {
		return postgres.DBChatSession{}, err
	}
	return s.chatRepo.UpdateSession(ctx, chatID, title, workbenchID)
}

func (s *Service) DeleteChat(ctx context.Context, userID, chatID string) error {
	session, err := s.chatRepo.FindSessionByID(ctx, chatID)
	if err != nil {
		return err
	}
	if err := s.requireMember(ctx, session.WorkspaceID, userID); err != nil {
		return err
	}
	s.evictChat(chatID)
	return s.chatRepo.DeleteSession(ctx, chatID)
}

func (s *Service) GetMessages(ctx context.Context, userID, sessionID string) ([]postgres.DBChatMessage, error) {
	session, err := s.chatRepo.FindSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.requireMember(ctx, session.WorkspaceID, userID); err != nil {
		return nil, err
	}
	return s.chatRepo.ListMessagesBySession(ctx, sessionID)
}

// StreamToken is called by ChatStream for each token.
type StreamToken func(string) error

// turnState holds one chat session's current-turn callbacks/context, kept
// alive and refreshed for the session's whole lifetime — see the "Why
// turnState exists" note above runTurn for why this indirection is
// necessary. The SAME *turnState pointer is returned by turnStateFor on
// every call for a given sessionID; runTurn overwrites its fields (under
// its mutex) at the start of every turn, including a GetOrCreateChat cache
// hit where the freshly-built agent/tools are otherwise discarded.
type turnState struct {
	mu                sync.Mutex
	userID            string
	workspaceID       string
	ctx               context.Context
	sendChoice        SendChoice
	sendTenderResults SendTenderResults
	cancel            context.CancelFunc
	pending           *pendingChoice
	emptyStreak       int
}

func (t *turnState) snapshot() (userID, workspaceID string, ctx context.Context, sendChoice SendChoice, sendTenderResults SendTenderResults, cancel context.CancelFunc, pending *pendingChoice) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.userID, t.workspaceID, t.ctx, t.sendChoice, t.sendTenderResults, t.cancel, t.pending
}

// recordSearchResult updates this turn's consecutive-empty-search streak —
// incrementing on an empty result, resetting on a non-empty one — and
// returns the streak's new value. This must live on turnState (not a
// closure-local variable in newSearchTendersTool) for the same reason
// sendTenderResults does (see the "Why turnState exists" note above
// runTurn): Registry.GetOrCreateChat reuses turn 1's tool closures for a
// session's whole lifetime, so a closure-local counter would count empty
// searches across the ENTIRE session instead of resetting every turn.
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

// turnStateFor returns sessionID's turnState, creating it on first use.
func (s *Service) turnStateFor(sessionID string) *turnState {
	s.turnStatesMu.Lock()
	defer s.turnStatesMu.Unlock()
	if ts, ok := s.turnStates[sessionID]; ok {
		return ts
	}
	ts := &turnState{}
	s.turnStates[sessionID] = ts
	return ts
}

// evictChat removes sessionID's in-memory chat AND its turnState together —
// use this instead of calling s.registry.RemoveChat directly anywhere in
// this package, so a rebuilt chat never inherits a stale turnState left
// over from before eviction.
func (s *Service) evictChat(sessionID string) {
	s.registry.RemoveChat(sessionID)
	s.turnStatesMu.Lock()
	delete(s.turnStates, sessionID)
	s.turnStatesMu.Unlock()
}

// runTurn drives one berrygem turn-loop invocation: builds the agent with
// this session's tools, feeds turnMessage into its (possibly rehydrated)
// conversation, and either persists the assistant's reply and reports full
// usage, or — if the agent invoked ask_choice — leaves the chat live for a
// later SubmitChoice to resume and reports the partial turn's (estimated)
// usage. Callers persist turnMessage themselves first (ChatStream as a
// "user" message, SubmitChoice's resume as a "choice_response" one) —
// runTurn only persists what happens *after*.
//
// Why turnState exists: Registry.GetOrCreateChat(sessionID, ag) discards
// the *agent.Agent (and its tools) passed on every call after a session's
// first — chat.Chat binds its Agent once, at chat.New(ag) time, with no way
// to swap it later. So the ask_choice/create_workbench tool closures built
// here on turn 2+ are silently unused; only turn 1's closures are ever
// actually invoked by berrygem. Reading everything through the session's
// single long-lived *turnState (refreshed at the start of every runTurn
// call, including a cache hit) means whichever closure berrygem ends up
// calling always sees the MOST RECENT turn's context/callbacks, not turn
// 1's stale, already-cancelled ones.
func (s *Service) runTurn(
	ctx context.Context,
	sessionID, userID, workspaceID, agentType, turnMessage string,
	sendToken StreamToken,
	sendChoice SendChoice,
	sendTenderResults SendTenderResults,
	usageCh chan<- credits.Usage,
) error {
	cfg, ok := s.registry.GetConfig(AgentType(agentType))
	if !ok {
		cfg = s.registry.configs[AgentTypeBaseChat]
	}

	streamCtx, cancelForChoice := context.WithCancel(ctx)
	defer cancelForChoice()
	pending := &pendingChoice{}

	ts := s.turnStateFor(sessionID)
	ts.mu.Lock()
	ts.userID, ts.workspaceID, ts.ctx, ts.sendChoice, ts.sendTenderResults, ts.cancel, ts.pending = userID, workspaceID, streamCtx, sendChoice, sendTenderResults, cancelForChoice, pending
	ts.emptyStreak = 0
	ts.mu.Unlock()

	askChoice := func(question string, options []ChoiceOption, allowCustom bool) error {
		_, _, curCtx, curSendChoice, _, curCancel, curPending := ts.snapshot()
		choicesJSON, err := json.Marshal(options)
		if err != nil {
			return err
		}
		msg, err := s.chatRepo.InsertMessage(curCtx, sessionID, "choice_prompt", question, choicesJSON, nil, nil)
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
			Query:         query,
			Filters:       tender.Filters{Country: country, CPV: cpv, Status: status},
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

	ag, err := s.registry.BuildAgent(cfg,
		bagent.WithTools(
			newAskChoiceTool(askChoice),
			newCreateWorkbenchTool(createWorkbench),
			newSearchTendersTool(ts, searchTenders),
		),
	)
	if err != nil {
		return err
	}

	berrygemChat, wasCreated := s.registry.GetOrCreateChat(sessionID, ag)
	if wasCreated {
		history, err := s.chatRepo.ListMessagesBySession(ctx, sessionID)
		if err != nil {
			return err
		}
		berrygemChat.SetMessages(dbMessagesToProviderMessages(history))
	}

	result, err := berrygemChat.SendStream(streamCtx, turnMessage)
	if err != nil {
		return err
	}
	defer result.Close()

	fullContent, done, err := s.consumeStream(streamCtx, result.C, result.Err, result.Done, sendToken)
	if err != nil {
		if pending.get() != nil {
			// Deliberate pause: ask_choice cancelled streamCtx on purpose.
			// The choice_prompt message is already persisted (inside
			// askChoice, above) — keep the in-memory chat alive so
			// SubmitChoice can resume it, and report this partial turn's
			// usage the same estimated way a turn with no real provider
			// usage already is (see sendUsage).
			s.sendUsage(usageCh, sessionID, agentType, cfg.Model, turnMessage, fullContent, nil)
			return nil
		}
		// The in-memory chat's message list now has a dangling turn with
		// no assistant reply (berrygem's SendStream appends the input
		// message before streaming and doesn't roll it back on error/
		// cancellation — verified against its source). Evict it (chat AND
		// turnState together) so the next message for this session rebuilds
		// fresh via rehydration instead of corrupting the next turn's
		// context.
		s.evictChat(sessionID)
		return err
	}

	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "assistant", fullContent, nil, nil, nil); err != nil {
		return err
	}
	s.sendUsage(usageCh, sessionID, agentType, cfg.Model, turnMessage, fullContent, done)
	return nil
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
	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "tender_results", "", nil, nil, tendersJSON); err != nil {
		return err
	}
	return sendTenderResults(TenderResults{Tenders: results})
}

// sendUsage reports usage for a turn. When real is nil (a paused turn) or
// carries zero usage (berrygem's streaming client never sets
// stream_options.include_usage on the OpenAI-compatible request — verified
// against its vendored source — so Fireworks never returns usage in
// streaming mode), it falls back to a character-length estimate rather
// than silently billing nothing.
func (s *Service) sendUsage(usageCh chan<- credits.Usage, sessionID, agentType, model, inputText, outputText string, real *bagent.RunResult) {
	if usageCh == nil {
		return
	}
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
	usageCh <- credits.Usage{
		AgentType:    agentType,
		SessionID:    sessionID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  totalTokens,
	}
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
	usageCh chan<- credits.Usage,
) error {
	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "user", message, nil, nil, nil); err != nil {
		return err
	}
	return s.runTurn(ctx, sessionID, userID, workspaceID, agentType, message, sendToken, sendChoice, sendTenderResults, usageCh)
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

// dbMessagesToProviderMessages converts persisted chat history into the
// shape berrygem's Chat.SetMessages expects. DBChatMessage.Role is already
// the plain strings "user"/"assistant"/"system" — identical to
// providers.RoleUser/RoleAssistant/RoleSystem's underlying values — so this
// is a direct conversion, not a lookup table.
func dbMessagesToProviderMessages(msgs []postgres.DBChatMessage) []providers.Message {
	out := make([]providers.Message, len(msgs))
	for i, m := range msgs {
		out[i] = providers.Message{Role: providers.Role(m.Role), Content: m.Content}
	}
	return out
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
