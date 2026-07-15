package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
)

type ChatRepo struct{ db *pg.DB }

func NewChatRepo(db *pg.DB) *ChatRepo { return &ChatRepo{db: db} }

func (r *ChatRepo) CreateSession(ctx context.Context, memberID, workspaceID, workbenchID, agentType, title string) (DBChatSession, error) {
	// A workspace-level chat (not tied to a specific workbench) arrives with
	// workbenchID == "" — bind SQL NULL rather than the empty string, which
	// Postgres rejects for a uuid column (SQLSTATE 22P02).
	workbenchVal := ChatSessionWorkbenchID.SetDefault()
	if workbenchID != "" {
		workbenchVal = ChatSessionWorkbenchID.Val(workbenchID)
	}

	var row DBChatSession
	err := r.db.Insert(ChatSessions).
		Row(
			ChatSessionMemberID.Val(memberID),
			ChatSessionWorkspaceID.Val(workspaceID),
			workbenchVal,
			ChatSessionAgentType.Val(agentType),
			ChatSessionTitle.Val(title),
		).
		Returning(ChatSessionID, ChatSessionMemberID, ChatSessionWorkspaceID, ChatSessionWorkbenchID,
			ChatSessionAgentType, ChatSessionTitle, ChatSessionCreatedAt, ChatSessionUpdatedAt).
		One(ctx, &row)
	return row, err
}

func (r *ChatRepo) FindSessionByID(ctx context.Context, id string) (DBChatSession, error) {
	var row DBChatSession
	err := r.db.Select().From(ChatSessions).Where(ChatSessionID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return row, err
	}
	return row, err
}

func (r *ChatRepo) ListSessionsByWorkspace(ctx context.Context, workspaceID string) ([]DBChatSession, error) {
	var rows []DBChatSession
	err := r.db.Select().From(ChatSessions).
		Where(ChatSessionWorkspaceID.Eq(workspaceID)).
		OrderBy(ChatSessionUpdatedAt.Desc()).
		All(ctx, &rows)
	return rows, err
}

func (r *ChatRepo) UpdateSession(ctx context.Context, id, title, workbenchID string) (DBChatSession, error) {
	var row DBChatSession
	q := r.db.Update(ChatSessions).
		Set(ChatSessionUpdatedAt.Val(time.Now())).
		Where(ChatSessionID.Eq(id))
	if title != "" {
		_ = q.Set(ChatSessionTitle.Val(title))
	}
	if workbenchID != "" {
		_ = q.Set(ChatSessionWorkbenchID.Val(workbenchID))
	}
	err := q.
		Returning(ChatSessionID, ChatSessionMemberID, ChatSessionWorkspaceID, ChatSessionWorkbenchID,
			ChatSessionAgentType, ChatSessionTitle, ChatSessionCreatedAt, ChatSessionUpdatedAt).
		One(ctx, &row)
	return row, err
}

func (r *ChatRepo) DeleteSession(ctx context.Context, id string) error {
	_, err := r.db.Delete(ChatSessions).Where(ChatSessionID.Eq(id)).Exec(ctx)
	return err
}

// ── Messages ────────────────────────────────────────────────────────────────

func (r *ChatRepo) InsertMessage(ctx context.Context, sessionID, role, content string, choices, metadata json.RawMessage) (DBChatMessage, error) {
	var row DBChatMessage
	err := r.db.Insert(ChatMessages).
		Row(
			ChatMessageSessionID.Val(sessionID),
			ChatMessageRole.Val(role),
			ChatMessageContent.Val(content),
			ChatMessageChoices.Val(choices),
			ChatMessageMetadata.Val(metadata),
		).
		Returning(ChatMessageID, ChatMessageSessionID, ChatMessageRole, ChatMessageContent,
			ChatMessageChoices, ChatMessageMetadata, ChatMessageCreatedAt).
		One(ctx, &row)
	return row, err
}

func (r *ChatRepo) ListMessagesBySession(ctx context.Context, sessionID string) ([]DBChatMessage, error) {
	var rows []DBChatMessage
	err := r.db.Select().From(ChatMessages).
		Where(ChatMessageSessionID.Eq(sessionID)).
		OrderBy(ChatMessageCreatedAt.Asc()).
		All(ctx, &rows)
	return rows, err
}

func (r *ChatRepo) FindMessageByID(ctx context.Context, id string) (DBChatMessage, error) {
	var row DBChatMessage
	err := r.db.Select().From(ChatMessages).Where(ChatMessageID.Eq(id)).One(ctx, &row)
	return row, err
}
