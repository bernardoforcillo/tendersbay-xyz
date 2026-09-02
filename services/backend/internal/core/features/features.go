// Package features is tendersbay's feature-management layer: what the product
// can do, who is entitled to it, and how much of it they may use.
//
// The engine is github.com/bernardoforcillo/featurelayer. This package owns the
// definitions it evaluates — the feature catalog, the plans and their metered
// limits, the flags — and the small amount of glue that turns "workspace" into
// featurelayer's "tenant".
//
// Everything here is code, not configuration. featurelayer's Config unmarshals
// straight from JSON, so moving these definitions into a file (or a database) is
// a change of source, not of shape; until there is a reason to deploy a plan
// change without a deploy, code review is the better gate.
//
// A feature is declared here when something actually asks about it. The catalog
// is a control surface, not an inventory of the product: an entry nobody checks
// reads like a gate that exists and isn't one, and the first person to move it
// out of the free plan would find they had changed nothing. The workbench, the
// dossier and the client profile are gated by workspace RBAC today and are not
// listed for that reason — add them here at the same commit that adds the
// check, not before.
package features

import (
	"context"
	"fmt"
	"log/slog"

	featurelayer "github.com/bernardoforcillo/featurelayer"
	"github.com/bernardoforcillo/featurelayer/catalog"
	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/featurelayer/flags"
)

// Feature keys. A key is a stable identifier: renaming one re-keys stored usage
// counters and any per-tenant grant that names it, so add rather than rename.
const (
	// AgentTokens is the metered LLM budget every workspace draws down when it
	// talks to an agent. It is the one feature with a limit today, and it is
	// what used to be the workspace_credits row.
	//
	// It DEPENDS ON AgentChat, so the kill switch below reaches the meter as
	// well: one evaluation answers both "is the agent on" and "is there budget
	// left", off one subscription read, instead of the caller asking twice.
	AgentTokens catalog.Key = "agent.tokens"
	// AgentChat is the agent surface itself, entitlement-gated and flagged so
	// it has a kill switch that needs no deploy.
	AgentChat catalog.Key = "agent.chat"
	// TenderSearch is free: it is served to anonymous callers, so it must not
	// depend on a subscription existing.
	TenderSearch catalog.Key = "tender.search"
)

// Plans.
const (
	PlanFree entitlement.PlanID = "free"
	PlanPro  entitlement.PlanID = "pro"
)

// AddOnExtraTokens tops up the monthly agent budget. Add-on entitlements are
// added to the plan's, so a pro workspace holding it gets Pro + Extra tokens.
const AddOnExtraTokens entitlement.AddOnID = "extra-tokens"

// Monthly agent-token allowances, in weighted tokens (see credits.Service for
// the weighting). FreeMonthlyTokenAllowance is the number the
// workspace_credits.monthly_token_allowance column used to default to; it moved
// here when the allowance became a property of the plan rather than of the row.
const (
	FreeMonthlyTokenAllowance int64 = 2_000_000
	ProMonthlyTokenAllowance  int64 = 20_000_000
	ExtraTokensAddOnAllowance int64 = 10_000_000
)

// Config is the complete set of definitions the engine evaluates.
func Config() featurelayer.Config {
	return featurelayer.Config{
		Features: []catalog.Feature{
			{Key: AgentTokens, Name: "Agent tokens", Lifecycle: catalog.GA,
				Description: "Metered LLM budget drawn down by every agent turn.",
				DependsOn:   []catalog.Key{AgentChat}},
			{Key: AgentChat, Name: "Agent chat", Lifecycle: catalog.GA},
			{Key: TenderSearch, Name: "Tender search", Lifecycle: catalog.GA, Free: true,
				Description: "Served to anonymous callers, so it carries no entitlement check."},
		},
		// One flag, and it earns its place: a kill switch on the only surface
		// that spends real money per request. Enabled:true with an On default
		// is a no-op today — flip Enabled to false and every agent turn is
		// refused with reason flag_off, with no deploy and no schema change.
		// Percentage rollouts and segment targeting hang off the same Flag; see
		// featurelayer/flags for the shape.
		Flags: []flags.Flag{
			{Feature: AgentChat, Enabled: true, Default: flags.Serve{On: true}},
		},
		Plans: []entitlement.Plan{
			{
				ID:   PlanFree,
				Name: "Free",
				Entitlements: []entitlement.Entitlement{
					{Feature: AgentChat},
					entitlement.Limited(AgentTokens, FreeMonthlyTokenAllowance, entitlement.Month),
				},
			},
			{
				ID:      PlanPro,
				Name:    "Pro",
				Extends: PlanFree,
				Entitlements: []entitlement.Entitlement{
					entitlement.Limited(AgentTokens, ProMonthlyTokenAllowance, entitlement.Month),
				},
			},
		},
		AddOns: []entitlement.AddOn{
			{
				ID:       AddOnExtraTokens,
				Name:     "Extra tokens",
				Requires: []entitlement.PlanID{PlanPro},
				Entitlements: []entitlement.Entitlement{
					entitlement.Limited(AgentTokens, ExtraTokensAddOnAllowance, entitlement.Month),
				},
			},
		},
	}
}

// Engine answers "may this workspace use this feature right now, and how much
// of it is left". It is safe for concurrent use and caches nothing beyond the
// immutable snapshot, so build one at startup and share it.
type Engine struct{ e *featurelayer.Engine }

// New builds the engine over the two per-tenant stores. Both are required in
// production: without a SubscriptionStore featurelayer runs in flags-only mode
// and every gated feature skips its commercial check, which would hand every
// workspace an unmetered agent.
func New(subs entitlement.SubscriptionStore, usage entitlement.UsageStore) (*Engine, error) {
	snap, err := featurelayer.NewSnapshot(Config())
	if err != nil {
		return nil, fmt.Errorf("features: invalid definitions: %w", err)
	}
	return &Engine{e: featurelayer.New(snap,
		featurelayer.WithSubscriptions(subs),
		featurelayer.WithUsage(usage),
		featurelayer.WithDecisionHook(logDenied),
	)}, nil
}

// logDenied records the decisions worth looking at: a refusal, or a store
// failure behind one. An allowed decision is the overwhelming majority and says
// nothing, so logging it would only bury the ones that do.
func logDenied(ev featurelayer.DecisionEvent) {
	if ev.Decision.Enabled && ev.Err == nil {
		return
	}
	slog.Debug("feature denied",
		"op", ev.Op,
		"feature", string(ev.Decision.Feature),
		"workspace_id", ev.Context.TenantID,
		"reason", string(ev.Decision.Reason),
		"detail", ev.Decision.Detail,
		"error", ev.Err,
	)
}

// evalContext maps this product's identifiers onto featurelayer's. A workspace
// is the tenant: entitlements, limits and usage counters are all per workspace,
// never per user.
func evalContext(workspaceID, userID string) featurelayer.EvalContext {
	return featurelayer.EvalContext{TenantID: workspaceID, UserID: userID}
}

// Evaluate reports whether a workspace may use a feature, with the reasoning
// attached so a caller can tell "not on your plan" from "switched off". It
// consumes nothing.
func (e *Engine) Evaluate(ctx context.Context, key catalog.Key, workspaceID, userID string) featurelayer.Decision {
	return e.e.Evaluate(ctx, key, evalContext(workspaceID, userID))
}

// Usage reads a metered feature's counter without spending any of it.
func (e *Engine) Usage(ctx context.Context, key catalog.Key, workspaceID string) (featurelayer.Decision, error) {
	return e.e.Usage(ctx, key, evalContext(workspaceID, ""))
}

// Consume spends n units of a metered feature. A refusal — the limit is
// reached, the plan does not carry the feature, the flag is off — comes back as
// a decision with Enabled false and a nil error; only a store or configuration
// failure is an error. Nothing is spent on a refusal.
func (e *Engine) Consume(ctx context.Context, key catalog.Key, workspaceID, userID string, n int64) (featurelayer.Decision, error) {
	return e.e.Consume(ctx, key, evalContext(workspaceID, userID), n)
}
