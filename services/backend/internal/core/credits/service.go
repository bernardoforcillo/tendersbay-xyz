package credits

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

type Usage struct {
	WorkspaceID  string
	UserID       string
	AgentType    string
	SessionID    string
	Model        string
	InputTokens  int32
	OutputTokens int32
	TotalTokens  int32
}

type Service struct {
	creditRepo  *postgres.WorkspaceCreditRepo
	pricingRepo *postgres.AgentPricingRepo
	usageRepo   *postgres.TokenUsageRepo
}

func NewService(
	creditRepo *postgres.WorkspaceCreditRepo,
	pricingRepo *postgres.AgentPricingRepo,
	usageRepo *postgres.TokenUsageRepo,
) *Service {
	return &Service{creditRepo: creditRepo, pricingRepo: pricingRepo, usageRepo: usageRepo}
}

type CheckResult struct {
	Remaining         int64
	Allowance         int64
	OK                bool
	CurrentCycleStart time.Time
}

func (s *Service) Check(ctx context.Context, workspaceID string) (CheckResult, error) {
	row, err := s.creditRepo.FindByWorkspace(ctx, workspaceID)
	if errors.Is(err, pg.ErrNoRows) {
		return CheckResult{}, nil
	}
	if err != nil {
		return CheckResult{}, err
	}
	remaining := row.MonthlyAllowance - row.CurrentCycleTokens
	if remaining < 0 {
		remaining = 0
	}
	return CheckResult{
		Remaining:         remaining,
		Allowance:         row.MonthlyAllowance,
		OK:                remaining > 0,
		CurrentCycleStart: row.CurrentCycleStart,
	}, nil
}

func (s *Service) Deduct(ctx context.Context, usage Usage) (int64, error) {
	multiplier := int64(1)
	pricing, err := s.pricingRepo.FindByAgentType(ctx, usage.AgentType)
	if err == nil {
		multiplier = pricing.InputCost + pricing.OutputCost
	}

	weighted := int64(usage.TotalTokens) * multiplier
	if weighted < 1 {
		weighted = 1
	}

	row, err := s.creditRepo.Deduct(ctx, usage.WorkspaceID, weighted)
	if err != nil {
		return 0, err
	}

	log := postgres.DBTokenUsage{
		WorkspaceID:    usage.WorkspaceID,
		UserID:         usage.UserID,
		AgentType:      usage.AgentType,
		SessionID:      usage.SessionID,
		Model:          usage.Model,
		InputTokens:    usage.InputTokens,
		OutputTokens:   usage.OutputTokens,
		TotalTokens:    usage.TotalTokens,
		CostMultiplier: multiplier,
	}
	if _, err := s.usageRepo.Insert(ctx, log); err != nil {
		return 0, err
	}

	return row.MonthlyAllowance - row.CurrentCycleTokens, nil
}

func (s *Service) ResetMonthly(ctx context.Context, workspaceID string) error {
	_, err := s.creditRepo.ResetCycle(ctx, workspaceID)
	return err
}
