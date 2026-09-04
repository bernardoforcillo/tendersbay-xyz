package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// The 0014 tables are created from the schema handles, and two shapes in that
// DDL are load-bearing rather than cosmetic — both are what the repositories'
// ON CONFLICT clauses key on. A migration that cannot be run in a unit test can
// at least have its generated statements read.
func TestFeatureTablesRenderTheConflictTargets(t *testing.T) {
	subs, err := drops.String(pg.CreateTableIfNotExists(WorkspaceSubscriptions))
	if err != nil {
		t.Fatalf("render workspace_subscriptions: %v", err)
	}
	// SubscriptionRepo.Upsert says ON CONFLICT (workspace_id); without the
	// primary key that clause has no unique index to match and every upsert
	// fails at runtime.
	if !strings.Contains(subs, "PRIMARY KEY") {
		t.Fatalf("workspace_subscriptions has no primary key:\n%s", subs)
	}
	for _, col := range []string{"plan", "add_ons", "grants", "billing_anchor"} {
		if !strings.Contains(subs, `"`+col+`"`) {
			t.Fatalf("workspace_subscriptions is missing %q:\n%s", col, subs)
		}
	}

	usage, err := drops.String(pg.CreateTableIfNotExists(FeatureUsage))
	if err != nil {
		t.Fatalf("render feature_usage: %v", err)
	}
	// The counter's identity is the whole triple, and drops emits
	// single-column constraints only — which is exactly why the migration adds
	// pk_feature_usage by hand. If drops ever started emitting one here, the
	// migration's ALTER would fail on a duplicate primary key.
	if strings.Contains(usage, "PRIMARY KEY") {
		t.Fatalf("feature_usage now carries an inline primary key; the migration's explicit one would collide:\n%s", usage)
	}
	for _, col := range []string{"workspace_id", "feature", "period", "used"} {
		if !strings.Contains(usage, `"`+col+`"`) {
			t.Fatalf("feature_usage is missing %q:\n%s", col, usage)
		}
	}
}
