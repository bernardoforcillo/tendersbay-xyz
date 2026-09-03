package features

import (
	"context"
	"testing"
	"time"

	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/featurelayer/flags"
)

// Usage gates an agent turn and Consume spends it, and both resolve the same
// dependency chain — AgentTokens through the flag on AgentChat. A flag is
// evaluated against the CALLER, so the two have to be told about the same one.
//
// Usage used to hardcode an empty user id. Nothing in the shipped catalog
// noticed, because the flag it carries today is a plain kill switch that reads
// no attributes. A rollout bucketed by user is the shape the Flags declaration
// explicitly invites, and under one the old code let a turn pass the gate,
// stream, cost provider money, and then be refused when it came to spend.
//
// So this builds that catalog rather than waiting for someone to ship it.
func TestUsageAndConsumeAgreeAboutTheCaller(t *testing.T) {
	cfg := Config()
	// 100% on for whoever is bucketed, so the only way to be refused is to
	// arrive without a user to bucket at all.
	cfg.Flags = []flags.Flag{{
		Feature: AgentChat,
		Enabled: true,
		Default: flags.Serve{On: true, Rollout: &flags.Rollout{
			BucketBy: "user",
			Split:    []flags.Portion{{Percent: 100}},
		}},
	}}

	subs := entitlement.NewMemSubscriptions()
	subs.Set(entitlement.Subscription{
		TenantID: "ws-1", Plan: PlanFree, BillingAnchor: time.Now().UTC(),
	})
	e, err := newEngine(cfg, subs, entitlement.NewMemUsage())
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}

	ctx := context.Background()
	gate, err := e.Usage(ctx, AgentTokens, "ws-1", "u-1")
	if err != nil {
		t.Fatalf("Usage: %v", err)
	}
	spend, err := e.Consume(ctx, AgentTokens, "ws-1", "u-1", 1)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if gate.Enabled != spend.Enabled {
		t.Fatalf("the gate said enabled=%v (%s) and the spend said enabled=%v (%s): "+
			"one of them is evaluating the flag without a caller",
			gate.Enabled, gate.Reason, spend.Enabled, spend.Reason)
	}
	if !gate.Enabled {
		t.Fatalf("both refused a 100%% rollout: reason %q, detail %q", gate.Reason, gate.Detail)
	}
}

// The catalog this product actually ships must not depend on the caller, or the
// test above would be the only thing standing between a rollout and the
// mismatch it describes. Stated as its own check so a flag that starts reading
// attributes is a deliberate decision.
func TestShippedFlagsReadNoCallerAttributes(t *testing.T) {
	for _, f := range Config().Flags {
		if len(f.Rules) != 0 || f.Default.Rollout != nil {
			t.Fatalf("flag on %s now targets the caller; every path that evaluates it "+
				"must pass a real user id", f.Feature)
		}
	}
}
