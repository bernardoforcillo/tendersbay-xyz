package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

type TokenUsageRepo struct{ db *pg.DB }

func NewTokenUsageRepo(db *pg.DB) *TokenUsageRepo { return &TokenUsageRepo{db: db} }

func (r *TokenUsageRepo) Insert(ctx context.Context, u DBTokenUsage) (DBTokenUsage, error) {
	var row DBTokenUsage
	err := r.db.Insert(TokenUsageLog).
		Row(
			TUsageLogWorkspaceID.Val(u.WorkspaceID),
			TUsageLogUserID.Val(u.UserID),
			TUsageLogAgentType.Val(u.AgentType),
			TUsageLogSessionID.Val(u.SessionID),
			TUsageLogModel.Val(u.Model),
			TUsageLogInputTokens.Val(u.InputTokens),
			TUsageLogOutputTokens.Val(u.OutputTokens),
			TUsageLogTotalTokens.Val(u.TotalTokens),
			TUsageLogCostMultiplier.Val(u.CostMultiplier),
		).
		Returning(TUsageLogID, TUsageLogWorkspaceID, TUsageLogUserID,
			TUsageLogAgentType, TUsageLogSessionID, TUsageLogModel,
			TUsageLogInputTokens, TUsageLogOutputTokens, TUsageLogTotalTokens,
			TUsageLogCostMultiplier, TUsageLogCreatedAt).
		One(ctx, &row)
	return row, err
}
