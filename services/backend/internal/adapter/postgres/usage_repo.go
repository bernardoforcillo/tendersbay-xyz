package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
)

type TokenUsageRepo struct{ db *pg.DB }

func NewTokenUsageRepo(db *pg.DB) *TokenUsageRepo { return &TokenUsageRepo{db: db} }

var _ credits.UsageLogger = (*TokenUsageRepo)(nil)

// Insert appends one turn to the token ledger. Nothing reads the row back on
// this path — it is an audit record, not a value the caller needs — so the
// insert returns no row.
func (r *TokenUsageRepo) Insert(ctx context.Context, u credits.UsageLog) error {
	_, err := r.db.Insert(TokenUsageLog).
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
		Exec(ctx)
	return err
}
