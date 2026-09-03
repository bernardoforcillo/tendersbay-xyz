package features_test

import (
	"context"
	"testing"
	"time"

	featurelayer "github.com/bernardoforcillo/featurelayer"
	"github.com/bernardoforcillo/featurelayer/catalog"
	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
)

const ws = "ws-1"

func engineFor(t *testing.T, sub *entitlement.Subscription) (*features.Engine, *entitlement.MemUsage) {
	t.Helper()
	subs := entitlement.NewMemSubscriptions()
	if sub != nil {
		subs.Set(*sub)
	}
	usage := entitlement.NewMemUsage()
	e, err := features.New(subs, usage)
	if err != nil {
		t.Fatalf("features.New: %v", err)
	}
	return e, usage
}

func freePlan() *entitlement.Subscription {
	return &entitlement.Subscription{TenantID: ws, Plan: features.PlanFree, BillingAnchor: time.Now().UTC()}
}

// The definitions have to be valid — an invalid catalog, flag or plan is a
// startup failure in main.go, so it is worth failing here first.
func TestConfigIsValid(t *testing.T) {
	if _, err := featurelayer.NewSnapshot(features.Config()); err != nil {
		t.Fatalf("definitions are invalid: %v", err)
	}
}

func TestFreePlan_CarriesTheAgent(t *testing.T) {
	e, _ := engineFor(t, freePlan())
	if !e.Evaluate(context.Background(), features.AgentChat, ws, "u-1").Enabled {
		t.Fatal("the free plan must carry the agent")
	}
}

// Every declared feature has to be reachable on some plan, or it is a gate that
// can never open. This is the check that keeps the catalog and the plans from
// drifting apart as either grows.
func TestEveryFeatureIsReachable(t *testing.T) {
	cfg := features.Config()
	entitled := map[catalog.Key]bool{}
	for _, p := range cfg.Plans {
		for _, ent := range p.Entitlements {
			entitled[ent.Feature] = true
		}
	}
	for _, a := range cfg.AddOns {
		for _, ent := range a.Entitlements {
			entitled[ent.Feature] = true
		}
	}
	for _, f := range cfg.Features {
		if f.Free {
			continue // free features carry no entitlement check at all
		}
		if !entitled[f.Key] {
			t.Errorf("feature %q is declared but no plan or add-on grants it, so nothing can ever use it", f.Key)
		}
	}
}

// Search is served to anonymous callers, so it must not depend on a
// subscription existing at all.
func TestTenderSearch_IsFreeWithoutASubscription(t *testing.T) {
	e, _ := engineFor(t, nil)
	if !e.Evaluate(context.Background(), features.TenderSearch, "", "").Enabled {
		t.Fatal("tender search must be available with no tenant and no subscription")
	}
}

// Everything ambiguous resolves to off: an unseeded workspace is entitled to
// nothing, which is what keeps an unmetered agent from being the default.
func TestNoSubscription_FailsClosed(t *testing.T) {
	e, _ := engineFor(t, nil)
	if e.Evaluate(context.Background(), features.AgentChat, ws, "u-1").Enabled {
		t.Fatal("a workspace with no subscription must not reach the agent")
	}
	d := e.Evaluate(context.Background(), features.AgentChat, ws, "u-1")
	if d.Reason != featurelayer.ReasonNotEntitled {
		t.Fatalf("reason = %q, want not_entitled", d.Reason)
	}
}

func TestConsume_DrawsDownTheFreeAllowance(t *testing.T) {
	e, _ := engineFor(t, freePlan())
	ctx := context.Background()

	d, err := e.Consume(ctx, features.AgentTokens, ws, "u-1", 1_000)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if !d.Enabled || d.Usage == nil {
		t.Fatalf("decision = %+v", d)
	}
	if d.Usage.Max != features.FreeMonthlyTokenAllowance {
		t.Fatalf("max = %d, want the free allowance", d.Usage.Max)
	}
	if d.Usage.Remaining != features.FreeMonthlyTokenAllowance-1_000 {
		t.Fatalf("remaining = %d", d.Usage.Remaining)
	}
}

// The ceiling is not advisory: an overrun spends nothing at all, so a workspace
// can never end a period past its allowance.
func TestConsume_OverTheCeilingAppliesNothing(t *testing.T) {
	e, _ := engineFor(t, freePlan())
	ctx := context.Background()

	if _, err := e.Consume(ctx, features.AgentTokens, ws, "u-1", features.FreeMonthlyTokenAllowance); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	d, err := e.Consume(ctx, features.AgentTokens, ws, "u-1", 1)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if d.Enabled {
		t.Fatal("a turn past the allowance must not be enabled")
	}
	if d.Reason != featurelayer.ReasonLimitReached {
		t.Fatalf("reason = %q, want limit_reached", d.Reason)
	}
	if d.Usage.Used != features.FreeMonthlyTokenAllowance {
		t.Fatalf("used = %d, want the ceiling exactly", d.Usage.Used)
	}
}

// A pro workspace gets the pro limit, not free's — Extends replaces the limit
// rather than adding to it.
func TestProPlan_RaisesTheCeiling(t *testing.T) {
	e, _ := engineFor(t, &entitlement.Subscription{
		TenantID: ws, Plan: features.PlanPro, BillingAnchor: time.Now().UTC(),
	})
	d, err := e.Usage(context.Background(), features.AgentTokens, ws, "u-1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if d.Usage.Max != features.ProMonthlyTokenAllowance {
		t.Fatalf("max = %d, want the pro allowance", d.Usage.Max)
	}
}

// An add-on ADDS to the plan's limit, and requires the plan it names.
func TestExtraTokensAddOn(t *testing.T) {
	ctx := context.Background()

	e, _ := engineFor(t, &entitlement.Subscription{
		TenantID: ws, Plan: features.PlanPro,
		AddOns:        []entitlement.AddOnID{features.AddOnExtraTokens},
		BillingAnchor: time.Now().UTC(),
	})
	d, err := e.Usage(ctx, features.AgentTokens, ws, "u-1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if want := features.ProMonthlyTokenAllowance + features.ExtraTokensAddOnAllowance; d.Usage.Max != want {
		t.Fatalf("max = %d, want %d", d.Usage.Max, want)
	}

	// The same add-on on the free plan does not apply: it requires pro.
	free, _ := engineFor(t, &entitlement.Subscription{
		TenantID: ws, Plan: features.PlanFree,
		AddOns:        []entitlement.AddOnID{features.AddOnExtraTokens},
		BillingAnchor: time.Now().UTC(),
	})
	d, err = free.Usage(ctx, features.AgentTokens, ws, "u-1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if d.Usage.Max != features.FreeMonthlyTokenAllowance {
		t.Fatalf("max = %d, want the free allowance — the add-on requires pro", d.Usage.Max)
	}
}

// A per-tenant grant overrides the plan's limit. This is what the 0014
// migration writes for a workspace whose allowance had been tuned by hand.
func TestGrantOverridesThePlanLimit(t *testing.T) {
	const custom int64 = 500_000
	e, _ := engineFor(t, &entitlement.Subscription{
		TenantID: ws, Plan: features.PlanFree,
		Grants: []entitlement.Grant{entitlement.Override(
			features.AgentTokens,
			&entitlement.Limit{Max: custom, Period: entitlement.Month},
			"carried over",
		)},
		BillingAnchor: time.Now().UTC(),
	})
	d, err := e.Usage(context.Background(), features.AgentTokens, ws, "u-1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if d.Usage.Max != custom {
		t.Fatalf("max = %d, want the overridden %d", d.Usage.Max, custom)
	}
}

// The kill switch on the agent surface reaches the METER, because the meter
// depends on the surface. That dependency is what lets one credits.Check answer
// both "is the agent on" and "is there budget", off one subscription read.
func TestKillSwitchReachesTheMeter(t *testing.T) {
	cfg := features.Config()
	for i := range cfg.Flags {
		if cfg.Flags[i].Feature == features.AgentChat {
			cfg.Flags[i].Enabled = false
		}
	}
	snap, err := featurelayer.NewSnapshot(cfg)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	subs := entitlement.NewMemSubscriptions()
	subs.Set(*freePlan())
	engine := featurelayer.New(snap,
		featurelayer.WithSubscriptions(subs),
		featurelayer.WithUsage(entitlement.NewMemUsage()),
	)

	d, err := engine.Usage(context.Background(), features.AgentTokens, featurelayer.EvalContext{TenantID: ws})
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if d.Enabled {
		t.Fatal("the meter is still open with the agent switched off")
	}
	if d.Reason != featurelayer.ReasonPrerequisite {
		t.Fatalf("reason = %q, want prerequisite — that is what tells a caller it is a kill switch and not an empty budget", d.Reason)
	}
	if d.Usage != nil {
		t.Fatal("a refusal before the counter must report no usage, or it reads as a budget answer")
	}
}

// The 0014 backfill keys its carried counters with this, so a catalog that
// stopped agreeing on the window would have it writing rows the engine reads
// back under a different key.
func TestMeterPeriod_AgreesAcrossEveryDefinition(t *testing.T) {
	got, ok := features.MeterPeriod(features.AgentTokens)
	if !ok {
		t.Fatal("the agent token budget declares no single metered period")
	}
	if got != entitlement.Month {
		t.Fatalf("MeterPeriod = %q, want %q", got, entitlement.Month)
	}
}

// A feature nothing meters has no window, and saying so is the point: a caller
// that needs one has to notice rather than be handed the zero value.
func TestMeterPeriod_UnmeteredFeature(t *testing.T) {
	for _, key := range []catalog.Key{features.AgentChat, features.TenderSearch, "nothing.at.all"} {
		if _, ok := features.MeterPeriod(key); ok {
			t.Fatalf("%s reported a metered period", key)
		}
	}
}
