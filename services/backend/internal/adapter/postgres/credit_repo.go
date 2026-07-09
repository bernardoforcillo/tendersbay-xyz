package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
)

type WorkspaceCreditRepo struct{ db *pg.DB }

func NewWorkspaceCreditRepo(db *pg.DB) *WorkspaceCreditRepo { return &WorkspaceCreditRepo{db: db} }

func (r *WorkspaceCreditRepo) FindByWorkspace(ctx context.Context, workspaceID string) (DBWorkspaceCredits, error) {
	var row DBWorkspaceCredits
	err := r.db.Select().From(WorkspaceCredits).Where(WCreditsWorkspaceID.Eq(workspaceID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return row, err
	}
	return row, err
}

func (r *WorkspaceCreditRepo) Deduct(ctx context.Context, workspaceID string, tokens int64) (DBWorkspaceCredits, bool, error) {
	res, err := r.db.Exec(ctx,
		`UPDATE workspace_credits
		 SET current_cycle_tokens = current_cycle_tokens + $1, updated_at = now()
		 WHERE workspace_id = $2 AND current_cycle_tokens + $1 <= monthly_token_allowance`,
		tokens, workspaceID)
	if err != nil {
		return DBWorkspaceCredits{}, false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return DBWorkspaceCredits{}, false, err
	}
	row, err := r.FindByWorkspace(ctx, workspaceID)
	if err != nil {
		return DBWorkspaceCredits{}, false, err
	}
	return row, affected > 0, nil
}

func (r *WorkspaceCreditRepo) ResetCycle(ctx context.Context, workspaceID string) (DBWorkspaceCredits, error) {
	var row DBWorkspaceCredits
	err := r.db.Update(WorkspaceCredits).
		Set(WCreditsCurrentCycleTokens.Val(0)).
		Set(WCreditsCurrentCycleStart.Val(time.Now())).
		Set(WCreditsUpdatedAt.Val(time.Now())).
		Where(WCreditsWorkspaceID.Eq(workspaceID)).
		Returning(WCreditsID, WCreditsWorkspaceID, WCreditsMonthlyTokenAllowance,
			WCreditsCurrentCycleStart, WCreditsCurrentCycleTokens, WCreditsCreatedAt, WCreditsUpdatedAt).
		One(ctx, &row)
	return row, err
}

func (r *WorkspaceCreditRepo) Upsert(ctx context.Context, workspaceID string, allowance int64) (DBWorkspaceCredits, error) {
	var row DBWorkspaceCredits
	err := r.db.Insert(WorkspaceCredits).
		Row(
			WCreditsWorkspaceID.Val(workspaceID),
			WCreditsMonthlyTokenAllowance.Val(allowance),
		).
		Returning(WCreditsID, WCreditsWorkspaceID, WCreditsMonthlyTokenAllowance,
			WCreditsCurrentCycleStart, WCreditsCurrentCycleTokens, WCreditsCreatedAt, WCreditsUpdatedAt).
		One(ctx, &row)
	if err != nil {
		// ON CONFLICT DO UPDATE
		err = r.db.Update(WorkspaceCredits).
			Set(WCreditsMonthlyTokenAllowance.Val(allowance)).
			Set(WCreditsUpdatedAt.Val(time.Now())).
			Where(WCreditsWorkspaceID.Eq(workspaceID)).
			Returning(WCreditsID, WCreditsWorkspaceID, WCreditsMonthlyTokenAllowance,
				WCreditsCurrentCycleStart, WCreditsCurrentCycleTokens, WCreditsCreatedAt, WCreditsUpdatedAt).
			One(ctx, &row)
		return row, err
	}
	return row, nil
}
