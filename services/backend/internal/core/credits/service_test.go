package credits

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

type fakeCreditRepo struct {
	row     postgres.DBWorkspaceCredits
	applied bool
}

func (f *fakeCreditRepo) FindByWorkspace(_ context.Context, _ string) (postgres.DBWorkspaceCredits, error) {
	return f.row, nil
}

func (f *fakeCreditRepo) Deduct(_ context.Context, _ string, tokens int64) (postgres.DBWorkspaceCredits, bool, error) {
	if f.applied {
		f.row.CurrentCycleTokens += tokens
	}
	return f.row, f.applied, nil
}

func (f *fakeCreditRepo) ResetCycle(_ context.Context, _ string) (postgres.DBWorkspaceCredits, error) {
	f.row.CurrentCycleTokens = 0
	return f.row, nil
}

type fakePricingRepo struct {
	pricing postgres.DBAgentPricing
	err     error
}

func (f *fakePricingRepo) FindByAgentType(_ context.Context, _ string) (postgres.DBAgentPricing, error) {
	return f.pricing, f.err
}

type fakeUsageRepo struct {
	inserted []postgres.DBTokenUsage
}

func (f *fakeUsageRepo) Insert(_ context.Context, u postgres.DBTokenUsage) (postgres.DBTokenUsage, error) {
	f.inserted = append(f.inserted, u)
	return u, nil
}

func TestDeduct_WeighsInputAndOutputTokensSeparately(t *testing.T) {
	creditRepo := &fakeCreditRepo{
		row:     postgres.DBWorkspaceCredits{MonthlyAllowance: 1_000_000},
		applied: true,
	}
	pricingRepo := &fakePricingRepo{pricing: postgres.DBAgentPricing{InputCost: 1, OutputCost: 3}}
	usageRepo := &fakeUsageRepo{}
	svc := NewService(creditRepo, pricingRepo, usageRepo)

	_, err := svc.Deduct(context.Background(), Usage{
		WorkspaceID: "ws-1", InputTokens: 10, OutputTokens: 5, TotalTokens: 15,
	})
	if err != nil {
		t.Fatalf("Deduct: %v", err)
	}

	// Correct: 10*1 + 5*3 = 25. The old bug computed (1+3)*15 = 60.
	if got := creditRepo.row.CurrentCycleTokens; got != 25 {
		t.Fatalf("CurrentCycleTokens = %d, want 25 (10*1 + 5*3)", got)
	}
	if len(usageRepo.inserted) != 1 || usageRepo.inserted[0].CostMultiplier != 4 {
		t.Fatalf("usage log = %+v, want one entry with CostMultiplier=4 (InputCost+OutputCost)", usageRepo.inserted)
	}
}

func TestDeduct_NotAppliedIsNotAnError(t *testing.T) {
	creditRepo := &fakeCreditRepo{
		row:     postgres.DBWorkspaceCredits{MonthlyAllowance: 100, CurrentCycleTokens: 100},
		applied: false, // simulates the repo rejecting the deduct (over ceiling)
	}
	pricingRepo := &fakePricingRepo{pricing: postgres.DBAgentPricing{InputCost: 1, OutputCost: 1}}
	usageRepo := &fakeUsageRepo{}
	svc := NewService(creditRepo, pricingRepo, usageRepo)

	remaining, err := svc.Deduct(context.Background(), Usage{
		WorkspaceID: "ws-1", InputTokens: 10, OutputTokens: 10, TotalTokens: 20,
	})
	// A capped/rejected deduct is NOT an error — the response was already
	// streamed to the user and already cost real money with the provider;
	// erroring here would fail the ConnectRPC stream after content was
	// already sent. Only a genuine DB failure should return err != nil.
	if err != nil {
		t.Fatalf("Deduct with applied=false returned an error: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0 (workspace already at its cap)", remaining)
	}
	// The usage log still records what actually happened, even though the
	// ledger couldn't fully reflect it.
	if len(usageRepo.inserted) != 1 {
		t.Fatalf("usage log entries = %d, want 1 (audit trail even when capped)", len(usageRepo.inserted))
	}
}

func TestDeduct_MissingPricingDefaultsToOnePerToken(t *testing.T) {
	creditRepo := &fakeCreditRepo{row: postgres.DBWorkspaceCredits{MonthlyAllowance: 1_000_000}, applied: true}
	pricingRepo := &fakePricingRepo{err: errors.New("no pricing row")}
	usageRepo := &fakeUsageRepo{}
	svc := NewService(creditRepo, pricingRepo, usageRepo)

	if _, err := svc.Deduct(context.Background(), Usage{
		WorkspaceID: "ws-1", InputTokens: 3, OutputTokens: 4, TotalTokens: 7,
	}); err != nil {
		t.Fatalf("Deduct: %v", err)
	}
	if got := creditRepo.row.CurrentCycleTokens; got != 7 {
		t.Fatalf("CurrentCycleTokens = %d, want 7 (3*1 + 4*1, default costs)", got)
	}
}
