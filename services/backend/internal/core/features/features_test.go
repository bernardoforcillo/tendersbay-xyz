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

func TestFreePlan_CarriesTheProductFeatures(t *testing.T) {
	e, _ := engineFor(t, freePlan())
	ctx := context.Background()
	for _, key := range []struct {
		key  catalog.Key
		want bool
	}{
		{features.AgentChat, true},
		{features.BidWorkbench, true},
		{features.CompanyDossier, true},
		{features.ClientProfile, true},
	} {
		if got := e.Enabled(ctx, key.key, ws, "u-1"); got != key.want {
			t.Fatalf("%s enabled = %v, want %v", key.key, got, key.want)
		}
	}
}

// Search is served to anonymous callers, so it must not depend on a
// subscription existing at all.
func TestTenderSearch_IsFreeWithoutASubscription(t *testing.T) {
	e, _ := engineFor(t, nil)
	if !e.Enabled(context.Background(), features.TenderSearch, "", "") {
		t.Fatal("tender search must be available with no tenant and no subscription")
	}
}

// Everything ambiguous resolves to off: an unseeded workspace is entitled to
// nothing, which is what keeps an unmetered agent from being the default.
func TestNoSubscription_FailsClosed(t *testing.T) {
	e, _ := engineFor(t, nil)
	if e.Enabled(context.Background(), features.AgentChat, ws, "u-1") {
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
	d, err := e.Usage(context.Background(), features.AgentTokens, ws)
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
	d, err := e.Usage(ctx, features.AgentTokens, ws)
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
	d, err = free.Usage(ctx, features.AgentTokens, ws)
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
	d, err := e.Usage(context.Background(), features.AgentTokens, ws)
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	if d.Usage.Max != custom {
		t.Fatalf("max = %d, want the overridden %d", d.Usage.Max, custom)
	}
}
