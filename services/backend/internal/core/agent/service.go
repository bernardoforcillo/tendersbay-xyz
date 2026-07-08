package agent

import (
	"context"
	"encoding/json"
	"errors"

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

	berrygemChat := s.registry.GetOrCreateChat(sessionID, ag)

	if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "user", message, nil, nil); err != nil {
		return err
	}

	result, err := berrygemChat.SendStream(ctx, message)
	if err != nil {
		return err
	}
	defer result.Close()

	var fullContent string

	for {
		select {
		case chunk, ok := <-result.C:
			if !ok {
				continue
			}
			fullContent += chunk
			if err := sendToken(chunk); err != nil {
				return err
			}

		case err, ok := <-result.Err:
			if !ok {
				continue
			}
			return err

		case done, ok := <-result.Done:
			if !ok {
				continue
			}

			if _, err := s.chatRepo.InsertMessage(ctx, sessionID, "assistant", fullContent, nil, nil); err != nil {
				return err
			}

			if usageCh != nil && done != nil {
				usageCh <- credits.Usage{
					AgentType:    agentType,
					SessionID:    sessionID,
					Model:        cfg.Model,
					InputTokens:  int32(done.Usage.PromptTokens),
					OutputTokens: int32(done.Usage.CompletionTokens),
					TotalTokens:  int32(done.Usage.TotalTokens),
				}
			}
			return nil
		}
	}
}
