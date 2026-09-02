// Package credits is the agent's token budget: how much LLM spend a workspace
// has left this period, and the ledger of what it actually spent.
//
// The budget itself is no longer this package's own bookkeeping. Entitlement,
// the monthly limit and the counter behind it belong to featurelayer through
// internal/core/features, where the allowance is a property of the workspace's
// PLAN rather than a column on its row. What stays here is what featurelayer
// has no opinion about: turning a turn's input and output tokens into a single
// weighted number using this product's per-agent pricing, and recording every
// turn in the token ledger — including the ones the budget refused, because the
// provider was already paid for them.
package credits

import (
	"context"
	"time"

	featurelayer "github.com/bernardoforcillo/featurelayer"

	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
)

// Usage is one agent turn's token consumption, as reported by the agent domain.
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

// UsageLog is one row of the token ledger: what a turn consumed and what it was
// weighted at. It is an audit record, kept whether or not the budget allowed
// the spend.
type UsageLog struct {
	WorkspaceID    string
	UserID         string
	AgentType      string
	SessionID      string
	Model          string
	InputTokens    int32
	OutputTokens   int32
	TotalTokens    int32
	CostMultiplier int64
}

// Pricing is an agent type's per-token cost, in budget units.
type Pricing struct {
	InputCost  int64
	OutputCost int64
}

// Ports. Each is satisfied by a postgres repository unchanged; the domain
// declares the shapes so this package does not depend on the adapter layer.
type (
	// PricingSource resolves an agent type's per-token cost. A miss is not an
	// error the caller has to handle — Deduct falls back to 1:1.
	PricingSource interface {
		FindByAgentType(ctx context.Context, agentType string) (Pricing, error)
	}
	// UsageLogger appends to the token ledger.
	UsageLogger interface {
		Insert(ctx context.Context, log UsageLog) error
	}
	// SubscriptionWriter creates or updates a workspace's subscription. Only
	// Seed uses it; reading subscriptions is featurelayer's job.
	SubscriptionWriter interface {
		Upsert(ctx context.Context, sub entitlement.Subscription) error
	}
)

type Service struct {
	features *features.Engine
	subs     SubscriptionWriter
	pricing  PricingSource
	ledger   UsageLogger
}

func NewService(engine *features.Engine, subs SubscriptionWriter, pricing PricingSource, ledger UsageLogger) *Service {
	return &Service{features: engine, subs: subs, pricing: pricing, ledger: ledger}
}

// CheckResult is a workspace's standing against its agent-token budget.
//
// Unlimited is possible in principle — featurelayer allows an entitlement with
// no limit — and is reported as Allowance and Remaining of -1 with OK true. No
// plan defined today is unlimited, but a caller that renders these numbers has
// to know which value means "no ceiling" rather than "no budget".
type CheckResult struct {
	Remaining int64
	Allowance int64
	// OK is whether a turn may run: entitled, switched on, and with budget
	// left.
	OK bool
	// Unavailable distinguishes the two ways OK can be false. It means the
	// agent surface itself is off — the kill switch, a rollout, a retired
	// feature — rather than this workspace being out of budget, and the two
	// need different answers: "try again later" is not "top up".
	Unavailable       bool
	CurrentCycleStart time.Time
	// ResetsAt is when the current period ends and the counter starts again.
	// It is derived from the workspace's billing anchor, so it is the real
	// date rather than the first of next month.
	ResetsAt time.Time
}

// Check answers, in one read, whether an agent turn may run here and how much
// budget is left.
//
// It is one question, not two, because the answer comes from one evaluation:
// features.AgentTokens depends on features.AgentChat, so the kill switch on the
// surface reaches the meter, and featurelayer resolves the whole chain off a
// single subscription lookup. The caller used to ask twice — once through a
// feature gate, once here — for the same row.
//
// A workspace with no subscription, or one whose plan does not carry the agent,
// comes back as the zero CheckResult: OK false, and not Unavailable, because
// nothing is switched off — it simply has no entitlement. That is the same
// answer a workspace with no credits row got before featurelayer.
func (s *Service) Check(ctx context.Context, workspaceID string) (CheckResult, error) {
	d, err := s.features.Usage(ctx, features.AgentTokens, workspaceID)
	if err != nil {
		return CheckResult{}, err
	}
	if d.Usage == nil {
		return CheckResult{Unavailable: switchedOff(d.Reason)}, nil
	}
	return CheckResult{
		Remaining:         d.Usage.Remaining,
		Allowance:         d.Usage.Max,
		OK:                d.Enabled && d.Usage.Remaining != 0,
		CurrentCycleStart: d.Usage.PeriodStart,
		ResetsAt:          d.Usage.ResetsAt,
	}, nil
}

// switchedOff reports whether a refusal was the agent surface being off rather
// than this workspace being out of budget or off-plan. Everything a flag, a
// lifecycle or a prerequisite decides is about the feature; everything an
// entitlement decides is about the workspace.
func switchedOff(r featurelayer.Reason) bool {
	switch r {
	case featurelayer.ReasonFlagOff,
		featurelayer.ReasonFlagWindow,
		featurelayer.ReasonFlagRule,
		featurelayer.ReasonFlagRollout,
		featurelayer.ReasonFlagDefault,
		featurelayer.ReasonLifecycle,
		featurelayer.ReasonPrerequisite,
		featurelayer.ReasonUnknownFeature:
		return true
	default:
		return false
	}
}

// Deduct weighs input and output tokens by their own per-token cost (not a
// summed flat multiplier — see the design doc for the bug this replaces) and
// spends the result against the workspace's budget.
//
// A spend the budget refuses is NOT an error: the response was already streamed
// to the user and already cost real money with the LLM provider by the time
// this runs, so failing the whole ConnectRPC call here would be a UX
// regression, not a safety win. The ledger row is written either way, as the
// accurate record of what happened — it is the only place an over-budget turn
// is visible at all, since featurelayer applies no partial increment.
func (s *Service) Deduct(ctx context.Context, usage Usage) (int64, error) {
	var inputCost, outputCost int64 = 1, 1
	if p, err := s.pricing.FindByAgentType(ctx, usage.AgentType); err == nil {
		inputCost, outputCost = p.InputCost, p.OutputCost
	}

	weighted := int64(usage.InputTokens)*inputCost + int64(usage.OutputTokens)*outputCost
	if weighted < 1 {
		weighted = 1
	}

	d, err := s.features.Consume(ctx, features.AgentTokens, usage.WorkspaceID, usage.UserID, weighted)
	if err != nil {
		return 0, err
	}

	if err := s.ledger.Insert(ctx, UsageLog{
		WorkspaceID:    usage.WorkspaceID,
		UserID:         usage.UserID,
		AgentType:      usage.AgentType,
		SessionID:      usage.SessionID,
		Model:          usage.Model,
		InputTokens:    usage.InputTokens,
		OutputTokens:   usage.OutputTokens,
		TotalTokens:    usage.TotalTokens,
		CostMultiplier: inputCost + outputCost,
	}); err != nil {
		return 0, err
	}

	if d.Usage == nil {
		// Refused before the counter was consulted — the workspace is not
		// entitled to the agent at all. There is no budget to report.
		return 0, nil
	}
	// -1 when the entitlement carries no limit; see CheckResult.
	return d.Usage.Remaining, nil
}

// Seed gives a workspace the free plan. It is idempotent — safe to call on
// every workspace creation, including a retry after a partial failure — and
// leaves an existing subscription's billing anchor alone, so re-seeding cannot
// move the period boundary and hand out a fresh budget mid-month.
func (s *Service) Seed(ctx context.Context, workspaceID string) error {
	return s.subs.Upsert(ctx, entitlement.Subscription{
		TenantID:      workspaceID,
		Plan:          features.PlanFree,
		BillingAnchor: time.Now().UTC(),
	})
}
