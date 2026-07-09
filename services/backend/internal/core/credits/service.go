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

// CreditRepo, PricingRepo, and UsageRepo are the ports credits.Service uses —
// each satisfied by *postgres.WorkspaceCreditRepo, *postgres.AgentPricingRepo,
// and *postgres.TokenUsageRepo respectively, without changes there.
type CreditRepo interface {
	FindByWorkspace(ctx context.Context, workspaceID string) (postgres.DBWorkspaceCredits, error)
	Deduct(ctx context.Context, workspaceID string, tokens int64) (postgres.DBWorkspaceCredits, bool, error)
	ResetCycle(ctx context.Context, workspaceID string) (postgres.DBWorkspaceCredits, error)
}

type PricingRepo interface {
	FindByAgentType(ctx context.Context, agentType string) (postgres.DBAgentPricing, error)
}

type UsageRepo interface {
	Insert(ctx context.Context, log postgres.DBTokenUsage) (postgres.DBTokenUsage, error)
}

type Service struct {
	creditRepo  CreditRepo
	pricingRepo PricingRepo
	usageRepo   UsageRepo
}

func NewService(creditRepo CreditRepo, pricingRepo PricingRepo, usageRepo UsageRepo) *Service {
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

// Deduct weighs input and output tokens by their own per-token cost (not a
// summed flat multiplier — see the design doc for the bug this replaces) and
// applies the result through CreditRepo.Deduct's atomic ceiling. A deduct
// that the repo rejects (workspace already at/over its monthly cap) is NOT
// an error: the response was already streamed to the user and already cost
// real money with the LLM provider by the time this runs, so failing the
// whole ConnectRPC call here would be a UX regression, not a safety win. The
// usage log is written either way, as the accurate record of what happened.
func (s *Service) Deduct(ctx context.Context, usage Usage) (int64, error) {
	var inputCost, outputCost int64 = 1, 1
	pricing, err := s.pricingRepo.FindByAgentType(ctx, usage.AgentType)
	if err == nil {
		inputCost = pricing.InputCost
		outputCost = pricing.OutputCost
	}

	weighted := int64(usage.InputTokens)*inputCost + int64(usage.OutputTokens)*outputCost
	if weighted < 1 {
		weighted = 1
	}

	row, _, err := s.creditRepo.Deduct(ctx, usage.WorkspaceID, weighted)
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
		CostMultiplier: inputCost + outputCost,
	}
	if _, err := s.usageRepo.Insert(ctx, log); err != nil {
		return 0, err
	}

	remaining := row.MonthlyAllowance - row.CurrentCycleTokens
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

func (s *Service) ResetMonthly(ctx context.Context, workspaceID string) error {
	_, err := s.creditRepo.ResetCycle(ctx, workspaceID)
	return err
}
