package connectapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func TestToProtoTenderResults_ConvertsEachTenderWithoutEuThreshold(t *testing.T) {
	value := int64(250000)
	tr := agent.TenderResults{Tenders: []tender.ScoredTender{
		{Tender: tender.Tender{ID: "t-1", Title: "Cestini intelligenti", Country: "IT", CPV: "34928480", Value: &value}},
	}}

	got := toProtoTenderResults(tr)

	if len(got.Tenders) != 1 {
		t.Fatalf("len(Tenders) = %d, want 1", len(got.Tenders))
	}
	tr0 := got.Tenders[0]
	if tr0.Id != "t-1" || tr0.Title != "Cestini intelligenti" || tr0.Country != "IT" || tr0.Value != 250000 {
		t.Fatalf("got = %+v", tr0)
	}
	if tr0.EuThreshold != "" {
		t.Fatalf("EuThreshold = %q, want empty (this path has no *tender.Service to compute a band from)", tr0.EuThreshold)
	}
}

func TestToProtoToolCall_MapsNameAndStatus(t *testing.T) {
	got := toProtoToolCall(agent.ToolCall{Name: "search_tenders", Status: "running"})
	if got.Name != "search_tenders" || got.Status != "running" {
		t.Fatalf("got = %+v, want {search_tenders running}", got)
	}
}

// ── billing an aborted turn ─────────────────────────────────────────────────
//
// runAndFinish used to return the moment the turn errored, without ever
// reading usageCh, so a turn that had already been paid for upstream cost the
// user nothing. These cover both halves of the fix: the drain-and-deduct, and
// the non-blocking read that keeps it from being a deadlock.

// fakeCreditRepo records every deduction. Each method fails on a cancelled
// context, the way a real query does — that is what makes the detached-context
// test below mean something.
type fakeCreditRepo struct {
	deducted []int64
	err      error
}

func (f *fakeCreditRepo) FindByWorkspace(ctx context.Context, _ string) (postgres.DBWorkspaceCredits, error) {
	if err := ctx.Err(); err != nil {
		return postgres.DBWorkspaceCredits{}, err
	}
	return postgres.DBWorkspaceCredits{MonthlyAllowance: credits.DefaultMonthlyTokenAllowance}, nil
}

func (f *fakeCreditRepo) Deduct(ctx context.Context, _ string, tokens int64) (postgres.DBWorkspaceCredits, bool, error) {
	if err := ctx.Err(); err != nil {
		return postgres.DBWorkspaceCredits{}, false, err
	}
	if f.err != nil {
		return postgres.DBWorkspaceCredits{}, false, f.err
	}
	f.deducted = append(f.deducted, tokens)
	return postgres.DBWorkspaceCredits{
		MonthlyAllowance:   credits.DefaultMonthlyTokenAllowance,
		CurrentCycleTokens: tokens,
	}, true, nil
}

func (f *fakeCreditRepo) ResetCycle(context.Context, string) (postgres.DBWorkspaceCredits, error) {
	return postgres.DBWorkspaceCredits{}, nil
}

func (f *fakeCreditRepo) Upsert(context.Context, string, int64) (postgres.DBWorkspaceCredits, error) {
	return postgres.DBWorkspaceCredits{}, nil
}

// fakePricingRepo has no row for any agent type, so credits.Service falls back
// to its 1:1 cost — the weighted deduction is then just input+output.
type fakePricingRepo struct{}

func (fakePricingRepo) FindByAgentType(context.Context, string) (postgres.DBAgentPricing, error) {
	return postgres.DBAgentPricing{}, pg.ErrNoRows
}

type fakeUsageRepo struct {
	rows []postgres.DBTokenUsage
}

func (f *fakeUsageRepo) Insert(ctx context.Context, log postgres.DBTokenUsage) (postgres.DBTokenUsage, error) {
	if err := ctx.Err(); err != nil {
		return postgres.DBTokenUsage{}, err
	}
	f.rows = append(f.rows, log)
	return log, nil
}

// newBillingHandler builds an AgentHandler with only the credits collaborator
// wired: runAndFinish's error path touches nothing else, and the stream is
// never written to on it.
func newBillingHandler(creditRepo *fakeCreditRepo, usageRepo *fakeUsageRepo) *AgentHandler {
	return NewAgentHandler(nil, credits.NewService(creditRepo, fakePricingRepo{}, usageRepo), nil)
}

// oneTurnOfUsage is what runTurn reports for a turn that reached the provider.
var oneTurnOfUsage = credits.Usage{
	AgentType:    "base-chat",
	SessionID:    "session-1",
	Model:        "accounts/fireworks/models/deepseek-v4-flash",
	InputTokens:  12,
	OutputTokens: 30,
	TotalTokens:  42,
}

func TestRunAndFinish_DeductsOnError(t *testing.T) {
	creditRepo := &fakeCreditRepo{}
	usageRepo := &fakeUsageRepo{}
	h := newBillingHandler(creditRepo, usageRepo)

	// Any error with a code of its own: the point is that the caller still sees
	// the failure that actually happened, not a billing artefact.
	runErr := agent.ErrChoiceNotPending
	err := h.runAndFinish(context.Background(), "user-1", "ws-1", credits.DefaultMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			return runErr
		})

	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want %v; billing must not mask the turn's own failure", got, connect.CodeFailedPrecondition)
	}
	if len(creditRepo.deducted) != 1 || creditRepo.deducted[0] != 42 {
		t.Fatalf("deductions = %v, want exactly one of 42 (12 in + 30 out at the fallback 1:1 cost)", creditRepo.deducted)
	}
	if len(usageRepo.rows) != 1 {
		t.Fatalf("usage log rows = %d, want 1", len(usageRepo.rows))
	}
	if usageRepo.rows[0].WorkspaceID != "ws-1" || usageRepo.rows[0].UserID != "user-1" {
		t.Fatalf("usage row = %+v, want it attributed to user-1 in ws-1", usageRepo.rows[0])
	}
}

// TestRunAndFinish_NoUsageDoesNotBlock is the deadlock guard, and the reason
// the fix could not be "move the channel read below the error check". A turn
// that failed before it reached the provider sends nothing, so a blocking read
// would hang the handler — and with it the request goroutine — forever.
func TestRunAndFinish_NoUsageDoesNotBlock(t *testing.T) {
	creditRepo := &fakeCreditRepo{}
	usageRepo := &fakeUsageRepo{}
	h := newBillingHandler(creditRepo, usageRepo)

	done := make(chan error, 1)
	go func() {
		done <- h.runAndFinish(context.Background(), "user-1", "ws-1", credits.DefaultMonthlyTokenAllowance, nil,
			func(chan<- credits.Usage) error { return agent.ErrChoiceNotPending })
	}()

	select {
	case err := <-done:
		if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
			t.Fatalf("code = %v, want %v", got, connect.CodeFailedPrecondition)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runAndFinish blocked waiting for usage that was never reported")
	}

	if len(creditRepo.deducted) != 0 || len(usageRepo.rows) != 0 {
		t.Fatalf("deductions = %v, usage rows = %d; a turn that never reached the provider spent nothing", creditRepo.deducted, len(usageRepo.rows))
	}
}

// TestRunAndFinish_BillsAnAbortedTurnOnACancelledContext is the case the whole
// change exists for. The commonest abort is the client going away, which
// cancels the request context — so if billing ran on that context it would
// fail on every occurrence of exactly the scenario it was written for.
func TestRunAndFinish_BillsAnAbortedTurnOnACancelledContext(t *testing.T) {
	creditRepo := &fakeCreditRepo{}
	usageRepo := &fakeUsageRepo{}
	h := newBillingHandler(creditRepo, usageRepo)

	ctx, cancel := context.WithCancel(context.Background())
	err := h.runAndFinish(ctx, "user-1", "ws-1", credits.DefaultMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			cancel() // the browser tab closes mid-stream
			return context.Canceled
		})
	defer cancel()

	if err == nil {
		t.Fatal("err = nil, want the aborted turn's failure")
	}
	if len(creditRepo.deducted) != 1 {
		t.Fatalf("deductions = %v, want one; a disconnect must not make the turn free", creditRepo.deducted)
	}
	if len(usageRepo.rows) != 1 {
		t.Fatalf("usage log rows = %d, want 1", len(usageRepo.rows))
	}
}

// A failed deduction is logged and swallowed: the caller is already returning
// the turn's own error, and an accounting fault must not overwrite it.
func TestRunAndFinish_BillingFailureLeavesTheOriginalErrorIntact(t *testing.T) {
	creditRepo := &fakeCreditRepo{err: errors.New("credits table unreachable")}
	usageRepo := &fakeUsageRepo{}
	h := newBillingHandler(creditRepo, usageRepo)

	err := h.runAndFinish(context.Background(), "user-1", "ws-1", credits.DefaultMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			return agent.ErrChoiceNotPending
		})

	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want %v", got, connect.CodeFailedPrecondition)
	}
	if len(usageRepo.rows) != 0 {
		t.Fatalf("usage log rows = %d, want 0 — the deduction never got that far", len(usageRepo.rows))
	}
}
