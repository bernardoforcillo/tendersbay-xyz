package connectapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/credits"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
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

// The billing fixture runs the REAL credits service over featurelayer's
// in-memory entitlement stores, so these tests exercise the metering path a
// deployed turn takes rather than a stand-in for it.

// countingUsage is featurelayer's in-memory usage store with two additions a
// test needs: it records every increment, and it fails on a cancelled context
// the way a real query does — which is what makes the detached-context test
// below mean anything.
type countingUsage struct {
	inner      *entitlement.MemUsage
	increments []int64
	err        error
}

func newCountingUsage() *countingUsage {
	return &countingUsage{inner: entitlement.NewMemUsage()}
}

func (u *countingUsage) Get(ctx context.Context, key entitlement.UsageKey) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return u.inner.Get(ctx, key)
}

func (u *countingUsage) Increment(ctx context.Context, key entitlement.UsageKey, delta, max int64) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if u.err != nil {
		return 0, false, u.err
	}
	u.increments = append(u.increments, delta)
	return u.inner.Increment(ctx, key, delta, max)
}

// fakeLedger stands in for the token ledger. It is the only collaborator that
// is still a stand-in: the ledger is a plain append with nothing to model.
type fakeLedger struct{ rows []credits.UsageLog }

func (f *fakeLedger) Insert(ctx context.Context, log credits.UsageLog) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.rows = append(f.rows, log)
	return nil
}

// noPricing has no row for any agent type, so credits.Service falls back to its
// 1:1 cost — the weighted deduction is then just input+output.
type noPricing struct{}

func (noPricing) FindByAgentType(context.Context, string) (credits.Pricing, error) {
	return credits.Pricing{}, errors.New("no pricing row")
}

// memSubscriptions is the writable half of the subscription port; Seed is the
// only thing that uses it and these tests never call Seed.
type memSubscriptions struct{ *entitlement.MemSubscriptions }

func (m memSubscriptions) Upsert(_ context.Context, sub entitlement.Subscription) error {
	m.Set(sub)
	return nil
}

// newBillingHandler builds an AgentHandler with only the credits collaborator
// wired: runAndFinish's error path touches nothing else, and the stream is
// never written to on it.
func newBillingHandler(t *testing.T, usage *countingUsage, ledger *fakeLedger) *AgentHandler {
	t.Helper()
	subs := entitlement.NewMemSubscriptions()
	subs.Set(entitlement.Subscription{
		TenantID:      "ws-1",
		Plan:          features.PlanFree,
		BillingAnchor: time.Now().UTC(),
	})
	engine, err := features.New(subs, usage)
	if err != nil {
		t.Fatalf("features.New: %v", err)
	}
	svc := credits.NewService(engine, memSubscriptions{subs}, noPricing{}, ledger)
	return NewAgentHandler(nil, svc, nil)
}

// oneTurnOfUsage is what runTurn reports for a turn that reached the provider.
var oneTurnOfUsage = credits.Usage{
	AgentType:    "base-chat",
	SessionID:    "session-1",
	Model:        "accounts/fireworks/models/deepseek-v4-flash-0731",
	InputTokens:  12,
	OutputTokens: 30,
	TotalTokens:  42,
}

func TestRunAndFinish_DeductsOnError(t *testing.T) {
	usage := newCountingUsage()
	ledger := &fakeLedger{}
	h := newBillingHandler(t, usage, ledger)

	// Any error with a code of its own: the point is that the caller still sees
	// the failure that actually happened, not a billing artefact.
	runErr := agent.ErrChoiceNotPending
	err := h.runAndFinish(context.Background(), "user-1", "ws-1", features.FreeMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			return runErr
		})

	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want %v; billing must not mask the turn's own failure", got, connect.CodeFailedPrecondition)
	}
	if len(usage.increments) != 1 || usage.increments[0] != 42 {
		t.Fatalf("increments = %v, want exactly one of 42 (12 in + 30 out at the fallback 1:1 cost)", usage.increments)
	}
	if len(ledger.rows) != 1 {
		t.Fatalf("ledger rows = %d, want 1", len(ledger.rows))
	}
	if ledger.rows[0].WorkspaceID != "ws-1" || ledger.rows[0].UserID != "user-1" {
		t.Fatalf("ledger row = %+v, want it attributed to user-1 in ws-1", ledger.rows[0])
	}
}

// TestRunAndFinish_NoUsageDoesNotBlock is the deadlock guard, and the reason
// the fix could not be "move the channel read below the error check". A turn
// that failed before it reached the provider sends nothing, so a blocking read
// would hang the handler — and with it the request goroutine — forever.
func TestRunAndFinish_NoUsageDoesNotBlock(t *testing.T) {
	usage := newCountingUsage()
	ledger := &fakeLedger{}
	h := newBillingHandler(t, usage, ledger)

	done := make(chan error, 1)
	go func() {
		done <- h.runAndFinish(context.Background(), "user-1", "ws-1", features.FreeMonthlyTokenAllowance, nil,
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

	if len(usage.increments) != 0 || len(ledger.rows) != 0 {
		t.Fatalf("increments = %v, ledger rows = %d; a turn that never reached the provider spent nothing", usage.increments, len(ledger.rows))
	}
}

// TestRunAndFinish_BillsAnAbortedTurnOnACancelledContext is the case the whole
// change exists for. The commonest abort is the client going away, which
// cancels the request context — so if billing ran on that context it would
// fail on every occurrence of exactly the scenario it was written for.
func TestRunAndFinish_BillsAnAbortedTurnOnACancelledContext(t *testing.T) {
	usage := newCountingUsage()
	ledger := &fakeLedger{}
	h := newBillingHandler(t, usage, ledger)

	ctx, cancel := context.WithCancel(context.Background())
	err := h.runAndFinish(ctx, "user-1", "ws-1", features.FreeMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			cancel() // the browser tab closes mid-stream
			return context.Canceled
		})
	defer cancel()

	if err == nil {
		t.Fatal("err = nil, want the aborted turn's failure")
	}
	if len(usage.increments) != 1 {
		t.Fatalf("increments = %v, want one; a disconnect must not make the turn free", usage.increments)
	}
	if len(ledger.rows) != 1 {
		t.Fatalf("ledger rows = %d, want 1", len(ledger.rows))
	}
}

// A failed deduction is logged and swallowed: the caller is already returning
// the turn's own error, and an accounting fault must not overwrite it.
func TestRunAndFinish_BillingFailureLeavesTheOriginalErrorIntact(t *testing.T) {
	usage := newCountingUsage()
	usage.err = errors.New("usage counter unreachable")
	ledger := &fakeLedger{}
	h := newBillingHandler(t, usage, ledger)

	err := h.runAndFinish(context.Background(), "user-1", "ws-1", features.FreeMonthlyTokenAllowance, nil,
		func(usageCh chan<- credits.Usage) error {
			usageCh <- oneTurnOfUsage
			return agent.ErrChoiceNotPending
		})

	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want %v", got, connect.CodeFailedPrecondition)
	}
	if len(ledger.rows) != 0 {
		t.Fatalf("ledger rows = %d, want 0 — the deduction never got that far", len(ledger.rows))
	}
}
