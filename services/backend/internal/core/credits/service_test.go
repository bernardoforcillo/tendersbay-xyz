package credits_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
)

const (
	workspaceID = "ws-1"
	userID      = "u-1"
)

type stubPricing struct {
	pricing credits.Pricing
	missing bool
}

func (s stubPricing) FindByAgentType(context.Context, string) (credits.Pricing, error) {
	if s.missing {
		return credits.Pricing{}, errors.New("no pricing row")
	}
	return s.pricing, nil
}

type recordingLedger struct{ rows []credits.UsageLog }

func (r *recordingLedger) Insert(_ context.Context, log credits.UsageLog) error {
	r.rows = append(r.rows, log)
	return nil
}

// memSubscriptions is featurelayer's in-memory subscription store plus the
// write half credits.Seed needs.
type memSubscriptions struct{ *entitlement.MemSubscriptions }

func (m memSubscriptions) Upsert(_ context.Context, sub entitlement.Subscription) error {
	// Mirrors the real repository: an existing billing anchor is preserved, so
	// re-seeding cannot move the period boundary.
	if existing, err := m.Subscription(context.Background(), sub.TenantID); err == nil {
		sub.BillingAnchor = existing.BillingAnchor
	}
	m.Set(sub)
	return nil
}

type fixture struct {
	svc    *credits.Service
	subs   memSubscriptions
	ledger *recordingLedger
}

// newFixture builds the service over featurelayer's in-memory stores. seeded
// says whether the workspace already holds the free plan; an unseeded one is
// how a workspace that was never given an allowance behaves.
func newFixture(t *testing.T, pricing credits.PricingSource, seeded bool) *fixture {
	t.Helper()
	subs := memSubscriptions{entitlement.NewMemSubscriptions()}
	if seeded {
		subs.Set(entitlement.Subscription{
			TenantID:      workspaceID,
			Plan:          features.PlanFree,
			BillingAnchor: time.Now().UTC(),
		})
	}
	engine, err := features.New(subs, entitlement.NewMemUsage())
	if err != nil {
		t.Fatalf("features.New: %v", err)
	}
	ledger := &recordingLedger{}
	return &fixture{svc: credits.NewService(engine, subs, pricing, ledger), subs: subs, ledger: ledger}
}

// Input and output tokens are weighed by their OWN per-token cost. Summing the
// two into one flat multiplier is the bug this replaced: at 1 in / 3 out it
// billed every token at 4.
func TestDeduct_WeighsInputAndOutputSeparately(t *testing.T) {
	f := newFixture(t, stubPricing{pricing: credits.Pricing{InputCost: 1, OutputCost: 3}}, true)
	ctx := context.Background()

	remaining, err := f.svc.Deduct(ctx, credits.Usage{
		WorkspaceID: workspaceID, UserID: "u-1", AgentType: "base-chat",
		InputTokens: 100, OutputTokens: 10, TotalTokens: 110,
	})
	if err != nil {
		t.Fatalf("Deduct: %v", err)
	}
	const want = 100*1 + 10*3
	if got := features.FreeMonthlyTokenAllowance - remaining; got != want {
		t.Fatalf("spent %d, want %d", got, want)
	}
}

func TestDeduct_FallsBackToOneToOneWithoutPricing(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, true)
	remaining, err := f.svc.Deduct(context.Background(), credits.Usage{
		WorkspaceID: workspaceID, InputTokens: 12, OutputTokens: 30, TotalTokens: 42,
	})
	if err != nil {
		t.Fatalf("Deduct: %v", err)
	}
	if got := features.FreeMonthlyTokenAllowance - remaining; got != 42 {
		t.Fatalf("spent %d, want 42", got)
	}
}

// Every turn is logged, including one whose weighted cost rounds to nothing:
// the ledger is the record of what the provider was paid for, not of what the
// budget allowed.
func TestDeduct_AlwaysWritesTheLedger(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, true)
	if _, err := f.svc.Deduct(context.Background(), credits.Usage{
		WorkspaceID: workspaceID, UserID: "u-1", AgentType: "base-chat", SessionID: "s-1",
		Model: "m", InputTokens: 0, OutputTokens: 0, TotalTokens: 0,
	}); err != nil {
		t.Fatalf("Deduct: %v", err)
	}
	if len(f.ledger.rows) != 1 {
		t.Fatalf("ledger rows = %d, want 1", len(f.ledger.rows))
	}
	row := f.ledger.rows[0]
	if row.WorkspaceID != workspaceID || row.SessionID != "s-1" || row.CostMultiplier != 2 {
		t.Fatalf("ledger row = %+v", row)
	}
}

// A turn that overruns the budget is refused by the counter — nothing partial
// is applied — but it still reaches the ledger, and it is not an error: the
// answer was already streamed and already paid for.
func TestDeduct_OverBudgetIsLoggedNotFailed(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, true)
	ctx := context.Background()

	// Spend the whole allowance, then one more token.
	if _, err := f.svc.Deduct(ctx, credits.Usage{
		WorkspaceID: workspaceID,
		InputTokens: int32(features.FreeMonthlyTokenAllowance),
	}); err != nil {
		t.Fatalf("first Deduct: %v", err)
	}
	remaining, err := f.svc.Deduct(ctx, credits.Usage{WorkspaceID: workspaceID, InputTokens: 1})
	if err != nil {
		t.Fatalf("over-budget Deduct must not be an error: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("remaining = %d, want 0", remaining)
	}
	if len(f.ledger.rows) != 2 {
		t.Fatalf("ledger rows = %d, want 2 — the refused turn is still a real cost", len(f.ledger.rows))
	}

	check, err := f.svc.Check(ctx, workspaceID, userID)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if check.Allowance-check.Remaining != features.FreeMonthlyTokenAllowance {
		t.Fatalf("counter moved past the ceiling: %+v", check)
	}
	if check.OK {
		t.Fatal("an exhausted budget must not report OK")
	}
	if check.Unavailable {
		t.Fatal("an exhausted budget is not the agent being switched off")
	}
}

func TestCheck_SeededWorkspace(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, true)
	check, err := f.svc.Check(context.Background(), workspaceID, userID)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !check.OK {
		t.Fatalf("check = %+v, want OK", check)
	}
	if check.Allowance != features.FreeMonthlyTokenAllowance || check.Remaining != features.FreeMonthlyTokenAllowance {
		t.Fatalf("check = %+v, want the free plan's whole allowance", check)
	}
	if !check.ResetsAt.After(check.CurrentCycleStart) {
		t.Fatalf("period = %v..%v, want a forward-running window", check.CurrentCycleStart, check.ResetsAt)
	}
}

// A workspace nobody seeded is entitled to nothing, which is what blocks its
// agent turns upstream. This is the same answer a workspace with no credits row
// gave before featurelayer.
func TestCheck_UnseededWorkspaceIsNotOK(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, false)
	check, err := f.svc.Check(context.Background(), workspaceID, userID)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if check.OK || check.Allowance != 0 {
		t.Fatalf("check = %+v, want the zero result", check)
	}
	// Not entitled is not the same as switched off: the caller renders one as
	// "top up" and the other as "try again later".
	if check.Unavailable {
		t.Fatal("a workspace with no subscription must not read as the agent being switched off")
	}
}

func TestSeed_GivesTheFreePlanAndIsIdempotent(t *testing.T) {
	f := newFixture(t, stubPricing{missing: true}, false)
	ctx := context.Background()

	if err := f.svc.Seed(ctx, workspaceID); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	first, err := f.svc.Check(ctx, workspaceID, userID)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !first.OK || first.Allowance != features.FreeMonthlyTokenAllowance {
		t.Fatalf("check = %+v", first)
	}

	if _, err := f.svc.Deduct(ctx, credits.Usage{WorkspaceID: workspaceID, InputTokens: 100}); err != nil {
		t.Fatalf("Deduct: %v", err)
	}
	// Re-seeding must not hand the workspace a fresh budget.
	if err := f.svc.Seed(ctx, workspaceID); err != nil {
		t.Fatalf("re-Seed: %v", err)
	}
	after, err := f.svc.Check(ctx, workspaceID, userID)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if after.Remaining != features.FreeMonthlyTokenAllowance-100 {
		t.Fatalf("remaining = %d, want the spend to survive re-seeding", after.Remaining)
	}
	if !after.CurrentCycleStart.Equal(first.CurrentCycleStart) {
		t.Fatalf("re-seeding moved the period boundary: %v -> %v", first.CurrentCycleStart, after.CurrentCycleStart)
	}
}
