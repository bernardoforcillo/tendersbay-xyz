package agent

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

type Service struct {
	registry  *Registry
	chatRepo  *postgres.ChatRepo
	creditSvc *credits.Service
}

func NewService(registry *Registry, chatRepo *postgres.ChatRepo, creditSvc *credits.Service) *Service {
	return &Service{registry: registry, chatRepo: chatRepo, creditSvc: creditSvc}
}

func (s *Service) CreateChat(ctx context.Context, memberID, workspaceID, workbenchID, agentType, title string) (postgres.DBChatSession, error) {
	if title == "" {
		title = "Nuova chat"
	}
	return s.chatRepo.CreateSession(ctx, memberID, workspaceID, workbenchID, agentType, title)
}

func (s *Service) ListChats(ctx context.Context, workspaceID string) ([]postgres.DBChatSession, error) {
	return s.chatRepo.ListSessionsByWorkspace(ctx, workspaceID)
}

func (s *Service) GetChat(ctx context.Context, chatID string) (postgres.DBChatSession, error) {
	return s.chatRepo.FindSessionByID(ctx, chatID)
}

func (s *Service) UpdateChat(ctx context.Context, chatID, title, workbenchID string) (postgres.DBChatSession, error) {
	return s.chatRepo.UpdateSession(ctx, chatID, title, workbenchID)
}

func (s *Service) DeleteChat(ctx context.Context, chatID string) error {
	s.registry.RemoveChat(chatID)
	return s.chatRepo.DeleteSession(ctx, chatID)
}

func (s *Service) GetMessages(ctx context.Context, sessionID string) ([]postgres.DBChatMessage, error) {
	return s.chatRepo.ListMessagesBySession(ctx, sessionID)
}

// StreamToken is called by ChatStream for each token.
type StreamToken func(string) error

// ChatStream runs the Berrygem agent streaming loop.
// It sends tokens via sendToken, saves messages to DB, and reports
// the final usage through usageCh (which the caller reads for credit
// deduction before sending StreamDone to the client).
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
