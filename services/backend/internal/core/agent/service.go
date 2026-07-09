package agent

import (
	"context"
	"encoding/json"
	"errors"

	bagent "github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/providers"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

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
	InsertMessage(ctx context.Context, sessionID, role, content string, choices, metadata json.RawMessage) (postgres.DBChatMessage, error)
	ListMessagesBySession(ctx context.Context, sessionID string) ([]postgres.DBChatMessage, error)
}

// MemberRepository is the minimal membership-check port agent.Service needs
// — satisfied by *postgres.MemberRepo (which already implements the full
// workspace.MemberRepository interface, a superset of this one).
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

type Service struct {
	registry  *Registry
	chatRepo  ChatRepository
	creditSvc *credits.Service
	members   MemberRepository
}

func NewService(registry *Registry, chatRepo ChatRepository, creditSvc *credits.Service, members MemberRepository) *Service {
	return &Service{registry: registry, chatRepo: chatRepo, creditSvc: creditSvc, members: members}
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
	s.registry.RemoveChat(chatID)
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

// ChatStream runs the Berrygem agent streaming loop. It trusts that the
// caller has already authorized sessionID's workspace (the ConnectRPC
// handler does this by calling GetChat, which is membership-checked, before
// ChatStream) — it does not re-check membership itself, to avoid a redundant
// FindSessionByID round trip on the hot path.
func (s *Service) ChatStream(
	ctx context.Context,
	sessionID, message, agentType string,
	sendToken StreamToken,
	usageCh chan<- credits.Usage,
) error {
	cfg, ok := s.registry.GetConfig(AgentType(agentType))
	if !ok {
		cfg = s.registry.configs[AgentTypeBaseChat]
	}

	ag, err := s.registry.BuildAgent(cfg)
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

	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "user", message, nil, nil); err != nil {
		return err
	}

	result, err := berrygemChat.SendStream(ctx, message)
	if err != nil {
		return err
	}
	defer result.Close()

	fullContent, done, err := s.consumeStream(ctx, result.C, result.Err, result.Done, sendToken)
	if err != nil {
		// The in-memory chat's message list now has a dangling user turn
		// with no assistant reply (berrygem's SendStream appends the user
		// message before streaming and doesn't roll it back on error/
		// cancellation — verified against its source). Evict it so the
		// next message for this session rebuilds fresh via rehydration
		// (see Task 4) instead of corrupting the next turn's context.
		s.registry.RemoveChat(sessionID)
		return err
	}

	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "assistant", fullContent, nil, nil); err != nil {
		return err
	}

	if usageCh != nil && done != nil {
		inputTokens := int32(done.Usage.PromptTokens)
		outputTokens := int32(done.Usage.CompletionTokens)
		totalTokens := int32(done.Usage.TotalTokens)
		if totalTokens == 0 {
			// berrygem's streaming client never sets stream_options.include_usage
			// on the OpenAI-compatible request (verified against its vendored
			// source), so Fireworks never returns usage in streaming mode and
			// RunResult.Usage comes back all-zero. Falling through with zeros
			// would hit credits.Service.Deduct's floor and silently bill a flat
			// 1 token regardless of the real exchange size — estimate instead.
			inputTokens = estimateTokens(message)
			outputTokens = estimateTokens(fullContent)
			totalTokens = inputTokens + outputTokens
		}
		usageCh <- credits.Usage{
			AgentType:    agentType,
			SessionID:    sessionID,
			Model:        cfg.Model,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  totalTokens,
		}
	}
	return nil
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
