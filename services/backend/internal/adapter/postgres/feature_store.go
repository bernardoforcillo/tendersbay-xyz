package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/featurelayer/entitlement"
)

// SubscriptionRepo is featurelayer's entitlement.SubscriptionStore over
// workspace_subscriptions: what a workspace is entitled to. The DEFINITIONS
// those ids resolve against (plans, add-ons, their limits) live in
// internal/core/features, not here — this table stores which of them apply.
type SubscriptionRepo struct{ db *pg.DB }

func NewSubscriptionRepo(db *pg.DB) *SubscriptionRepo { return &SubscriptionRepo{db: db} }

var _ entitlement.SubscriptionStore = (*SubscriptionRepo)(nil)

// Subscription returns entitlement.ErrNoSubscription for a workspace with no
// row, which featurelayer treats as "entitled to nothing". That is the correct
// fail-closed answer and matches what the credits ledger did before: a
// workspace with no allowance row could not run an agent turn.
func (r *SubscriptionRepo) Subscription(ctx context.Context, workspaceID string) (*entitlement.Subscription, error) {
	var row DBWorkspaceSubscription
	err := r.db.Select().From(WorkspaceSubscriptions).Where(WSubWorkspaceID.Eq(workspaceID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return nil, entitlement.ErrNoSubscription
	}
	if err != nil {
		return nil, err
	}

	sub := entitlement.Subscription{
		TenantID:      row.WorkspaceID,
		Plan:          entitlement.PlanID(row.Plan),
		BillingAnchor: row.BillingAnchor,
	}
	if len(row.AddOns) > 0 {
		if err := json.Unmarshal(row.AddOns, &sub.AddOns); err != nil {
			return nil, err
		}
	}
	if len(row.Grants) > 0 {
		if err := json.Unmarshal(row.Grants, &sub.Grants); err != nil {
			return nil, err
		}
	}
	if row.Trial != nil && len(*row.Trial) > 0 && string(*row.Trial) != "null" {
		var trial entitlement.PlanTrial
		if err := json.Unmarshal(*row.Trial, &trial); err != nil {
			return nil, err
		}
		sub.Trial = &trial
	}
	return &sub, nil
}

// Upsert writes a workspace's subscription. It is idempotent so the workspace
// creation path can call it on every attempt, including a retry after a partial
// failure, and it preserves the existing billing anchor: moving the anchor
// would move the current period's boundary and hand the workspace a fresh
// budget mid-month.
func (r *SubscriptionRepo) Upsert(ctx context.Context, sub entitlement.Subscription) error {
	addOns, err := json.Marshal(orEmpty(sub.AddOns))
	if err != nil {
		return err
	}
	grants, err := json.Marshal(orEmpty(sub.Grants))
	if err != nil {
		return err
	}
	var trial any
	if sub.Trial != nil {
		b, err := json.Marshal(sub.Trial)
		if err != nil {
			return err
		}
		trial = string(b)
	}
	anchor := sub.BillingAnchor
	if anchor.IsZero() {
		anchor = time.Now().UTC()
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO workspace_subscriptions (workspace_id, plan, add_ons, trial, grants, billing_anchor)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, $5::jsonb, $6)
		ON CONFLICT (workspace_id) DO UPDATE SET
			plan = EXCLUDED.plan,
			add_ons = EXCLUDED.add_ons,
			trial = EXCLUDED.trial,
			grants = EXCLUDED.grants,
			updated_at = now()`,
		sub.TenantID, string(sub.Plan), string(addOns), trial, string(grants), anchor)
	return err
}

// orEmpty renders a nil slice as [] rather than null, so the column's shape
// never depends on whether anything was set.
func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// UsageRepo is featurelayer's entitlement.UsageStore over feature_usage: how
// much of a metered feature a workspace has spent in the current period.
type UsageRepo struct{ db *pg.DB }

func NewFeatureUsageRepo(db *pg.DB) *UsageRepo { return &UsageRepo{db: db} }

var _ entitlement.UsageStore = (*UsageRepo)(nil)

func (r *UsageRepo) Get(ctx context.Context, key entitlement.UsageKey) (int64, error) {
	var used int64
	rows, err := r.db.Query(ctx,
		`SELECT used FROM feature_usage WHERE workspace_id = $1 AND feature = $2 AND period = $3`,
		key.Tenant, string(key.Feature), key.Period)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&used); err != nil {
			return 0, err
		}
	}
	return used, rows.Err()
}

// Increment adds delta to the counter, but only if the result stays within max
// (max < 0 means unlimited). It reports the counter's value and whether the
// increment was applied — never a partial one, matching featurelayer's own
// in-memory store.
//
// The ceiling is enforced by the statement, not by a read followed by a write:
// two concurrent agent turns on the same workspace must not both see room for
// the last of a budget and both spend it. That is the same guarantee the
// workspace_credits deduct gave, kept.
func (r *UsageRepo) Increment(ctx context.Context, key entitlement.UsageKey, delta, max int64) (int64, bool, error) {
	// A single request larger than the whole allowance can never fit, and the
	// INSERT below would not catch it: with no row yet there is no conflict, so
	// DO UPDATE's guard never runs. Answering from the current counter is both
	// correct and what the in-memory store does.
	if max >= 0 && delta > max {
		used, err := r.Get(ctx, key)
		return used, false, err
	}

	rows, err := r.db.Query(ctx, `
		INSERT INTO feature_usage (workspace_id, feature, period, used)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, feature, period) DO UPDATE
			SET used = feature_usage.used + $4, updated_at = now()
			WHERE $5 < 0 OR feature_usage.used + $4 <= $5
		RETURNING used`,
		key.Tenant, string(key.Feature), key.Period, delta, max)
	if err != nil {
		return 0, false, err
	}
	applied := false
	var used int64
	if rows.Next() {
		if err := rows.Scan(&used); err != nil {
			_ = rows.Close()
			return 0, false, err
		}
		applied = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, false, err
	}
	_ = rows.Close()

	if applied {
		return used, true, nil
	}
	// The guard refused the update: report where the counter actually stands.
	used, err = r.Get(ctx, key)
	return used, false, err
}
