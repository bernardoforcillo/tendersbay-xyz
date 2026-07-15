package connectapi

import (
	"context"
	"time"

	"connectrpc.com/connect"
	agentv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1/agentv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
)

type AgentHandler struct {
	svc       *agent.Service
	creditSvc *credits.Service
	members   agent.MemberRepository
}

func NewAgentHandler(svc *agent.Service, creditSvc *credits.Service, members agent.MemberRepository) *AgentHandler {
	return &AgentHandler{svc: svc, creditSvc: creditSvc, members: members}
}

var _ agentv1connect.AgentServiceHandler = (*AgentHandler)(nil)

func (h *AgentHandler) CreateChat(ctx context.Context, req *connect.Request[agentv1.CreateChatRequest]) (*connect.Response[agentv1.CreateChatResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}

	session, err := h.svc.CreateChat(ctx, uid, req.Msg.WorkspaceId, req.Msg.WorkbenchId, req.Msg.AgentType, req.Msg.Title)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&agentv1.CreateChatResponse{
		Chat: toProtoChatSession(session),
	}), nil
}

func (h *AgentHandler) ListChats(ctx context.Context, req *connect.Request[agentv1.ListChatsRequest]) (*connect.Response[agentv1.ListChatsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	sessions, err := h.svc.ListChats(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}

	out := make([]*agentv1.ChatSession, len(sessions))
	for i, s := range sessions {
		out[i] = toProtoChatSession(s)
	}

	return connect.NewResponse(&agentv1.ListChatsResponse{Chats: out}), nil
}

func (h *AgentHandler) GetChat(ctx context.Context, req *connect.Request[agentv1.GetChatRequest]) (*connect.Response[agentv1.GetChatResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	session, err := h.svc.GetChat(ctx, uid, req.Msg.ChatId)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&agentv1.GetChatResponse{
		Chat: toProtoChatSession(session),
	}), nil
}

func (h *AgentHandler) UpdateChat(ctx context.Context, req *connect.Request[agentv1.UpdateChatRequest]) (*connect.Response[agentv1.UpdateChatResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	session, err := h.svc.UpdateChat(ctx, uid, req.Msg.ChatId, req.Msg.Title, req.Msg.WorkbenchId)
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&agentv1.UpdateChatResponse{
		Chat: toProtoChatSession(session),
	}), nil
}

func (h *AgentHandler) DeleteChat(ctx context.Context, req *connect.Request[agentv1.DeleteChatRequest]) (*connect.Response[agentv1.DeleteChatResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteChat(ctx, uid, req.Msg.ChatId); err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&agentv1.DeleteChatResponse{}), nil
}

func (h *AgentHandler) GetMessages(ctx context.Context, req *connect.Request[agentv1.GetMessagesRequest]) (*connect.Response[agentv1.GetMessagesResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := h.svc.GetMessages(ctx, uid, req.Msg.ChatId)
	if err != nil {
		return nil, toConnectError(err)
	}

	out := make([]*agentv1.ChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = toProtoChatMessage(m)
	}

	return connect.NewResponse(&agentv1.GetMessagesResponse{Messages: out}), nil
}

func newStreamCallbacks(stream *connect.ServerStream[agentv1.ChatStreamResponse]) (agent.StreamToken, agent.SendChoice) {
	sendToken := func(token string) error {
		return stream.Send(&agentv1.ChatStreamResponse{
			Event: &agentv1.ChatStreamResponse_Token{Token: token},
		})
	}
	sendChoice := func(cp agent.ChoicePrompt) error {
		return stream.Send(&agentv1.ChatStreamResponse{
			Event: &agentv1.ChatStreamResponse_Choice{Choice: toProtoChoicePrompt(cp)},
		})
	}
	return sendToken, sendChoice
}

// runAndFinish runs a service-layer turn (ChatStream or SubmitChoice) and,
// on success, drains its usage, deducts credits, and sends the terminal
// Done event — the tail shared by both RPCs.
func (h *AgentHandler) runAndFinish(
	ctx context.Context,
	uid, workspaceID string,
	allowance int64,
	stream *connect.ServerStream[agentv1.ChatStreamResponse],
	run func(usageCh chan<- credits.Usage) error,
) error {
	usageCh := make(chan credits.Usage, 1)
	if err := run(usageCh); err != nil {
		return toConnectError(err)
	}
	usage := <-usageCh
	usage.WorkspaceID = workspaceID
	usage.UserID = uid

	remaining, err := h.creditSvc.Deduct(ctx, usage)
	if err != nil {
		return toConnectError(err)
	}

	return stream.Send(&agentv1.ChatStreamResponse{
		Event: &agentv1.ChatStreamResponse_Done{Done: &agentv1.StreamDone{
			Usage: &agentv1.AgentUsage{
				InputTokens:  usage.InputTokens,
				OutputTokens: usage.OutputTokens,
				TotalTokens:  usage.TotalTokens,
			},
			CreditsRemaining:  remaining,
			CreditsMonthlyMax: allowance,
		}},
	})
}

func (h *AgentHandler) ChatStream(ctx context.Context, req *connect.Request[agentv1.ChatStreamRequest], stream *connect.ServerStream[agentv1.ChatStreamResponse]) error {
	uid, err := requireUser(ctx)
	if err != nil {
		return err
	}

	session, err := h.svc.GetChat(ctx, uid, req.Msg.ChatId)
	if err != nil {
		return toConnectError(err)
	}

	check, err := h.creditSvc.Check(ctx, session.WorkspaceID)
	if err != nil {
		return toConnectError(err)
	}
	if !check.OK {
		return connect.NewError(connect.CodeResourceExhausted, agent.ErrInsufficientCredits)
	}

	sendToken, sendChoice := newStreamCallbacks(stream)

	return h.runAndFinish(ctx, uid, session.WorkspaceID, check.Allowance, stream, func(usageCh chan<- credits.Usage) error {
		return h.svc.ChatStream(ctx, session.ID, uid, session.WorkspaceID, req.Msg.Message, session.AgentType, sendToken, sendChoice, usageCh)
	})
}

func (h *AgentHandler) SubmitChoice(ctx context.Context, req *connect.Request[agentv1.SubmitChoiceRequest], stream *connect.ServerStream[agentv1.ChatStreamResponse]) error {
	uid, err := requireUser(ctx)
	if err != nil {
		return err
	}

	session, err := h.svc.GetChatForChoice(ctx, uid, req.Msg.ChoiceId)
	if err != nil {
		return toConnectError(err)
	}

	check, err := h.creditSvc.Check(ctx, session.WorkspaceID)
	if err != nil {
		return toConnectError(err)
	}
	if !check.OK {
		return connect.NewError(connect.CodeResourceExhausted, agent.ErrInsufficientCredits)
	}

	sendToken, sendChoice := newStreamCallbacks(stream)

	return h.runAndFinish(ctx, uid, session.WorkspaceID, check.Allowance, stream, func(usageCh chan<- credits.Usage) error {
		return h.svc.SubmitChoice(ctx, session, uid, req.Msg.ChoiceId, req.Msg.SelectedKey, req.Msg.CustomValue, sendToken, sendChoice, usageCh)
	})
}

func (h *AgentHandler) GetCredits(ctx context.Context, req *connect.Request[agentv1.GetCreditsRequest]) (*connect.Response[agentv1.GetCreditsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.members.LoadMembership(ctx, req.Msg.WorkspaceId, uid); err != nil {
		return nil, toConnectError(err)
	}

	check, err := h.creditSvc.Check(ctx, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}

	resetDate := nextMonthStart(check.CurrentCycleStart)

	return connect.NewResponse(&agentv1.GetCreditsResponse{
		Remaining:  check.Remaining,
		MonthlyMax: check.Allowance,
		Used:       check.Allowance - check.Remaining,
		ResetDate:  resetDate,
	}), nil
}

// ── proto mappers ─────────────────────────────────────────────────────────

func toProtoChatSession(s postgres.DBChatSession) *agentv1.ChatSession {
	p := &agentv1.ChatSession{
		Id:          s.ID,
		UserId:      s.MemberID,
		WorkspaceId: s.WorkspaceID,
		AgentType:   s.AgentType,
		Title:       s.Title,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
	if s.WorkbenchID != nil {
		p.WorkbenchId = *s.WorkbenchID
	}
	return p
}

func toProtoChatMessage(m postgres.DBChatMessage) *agentv1.ChatMessage {
	p := &agentv1.ChatMessage{
		Id:        m.ID,
		SessionId: m.SessionID,
		Role:      m.Role,
		Content:   m.Content,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
	if m.Choices != nil {
		p.Choices = []byte(*m.Choices)
	}
	if m.Metadata != nil {
		p.Metadata = []byte(*m.Metadata)
	}
	return p
}

func toProtoChoicePrompt(cp agent.ChoicePrompt) *agentv1.ChoicePrompt {
	options := make([]*agentv1.ChoiceOption, len(cp.Options))
	for i, o := range cp.Options {
		options[i] = &agentv1.ChoiceOption{Key: o.Key, Label: o.Label, Description: o.Description}
	}
	return &agentv1.ChoicePrompt{
		Id:          cp.ID,
		Question:    cp.Question,
		Options:     options,
		AllowCustom: cp.AllowCustom,
	}
}

func nextMonthStart(t time.Time) string {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 1, 0, 0, 0, 0, t.Location()).Format("2006-01-02")
}
