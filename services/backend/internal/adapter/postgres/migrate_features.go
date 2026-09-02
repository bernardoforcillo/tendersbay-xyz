package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
)

// migrateFeatures is the 0014 schema migration that moves the agent's token
// budget from a column to a plan.
//
// workspace_credits held three things at once: which workspace it was about,
// how much it was allowed per month, and how much it had spent this cycle.
// featurelayer splits those: the allowance is a property of the PLAN
// (internal/core/features), the subscription says which plan a workspace holds,
// and feature_usage is one counter per (workspace, feature, period).
//
// The backfill preserves what each workspace actually had:
//
//   - Every workspace gets a free-plan subscription, including one that never
//     had a credits row — such a workspace could not run an agent turn before
//     and would still be unable to, which is a regression nobody asked for.
//   - Its billing anchor is the cycle start it was already on, so the period it
//     is in the middle of does not move and nobody gains or loses a month.
//   - A workspace whose allowance had been raised or lowered off the default
//     keeps that number as a per-tenant GRANT overriding the plan's limit.
//     Dropping it would silently re-price somebody's workspace.
//   - Its spend so far becomes the counter for the current period, so a
//     workspace that had already used half its month still has half a month.
//
// The period rolls over on its own from here: the counter is keyed by period
// start, so a new month writes to a new row and there is nothing left to reset.
// That is why credits.Service lost its ResetMonthly.
func migrateFeatures() pg.Migration {
	tables := []*pg.Table{WorkspaceSubscriptions, FeatureUsage}
	return pg.Migration{
		Version: "0014",
		Name:    "featurelayer_entitlements",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			// drops emits single-column constraints only, and the usage
			// counter's identity is the whole triple — it is also what makes
			// the ON CONFLICT in FeatureUsageRepo.Increment atomic.
			if _, err := db.Exec(ctx,
				`ALTER TABLE feature_usage ADD CONSTRAINT pk_feature_usage PRIMARY KEY (workspace_id, feature, period)`,
			); err != nil {
				return err
			}

			if err := backfillSubscriptions(ctx, db); err != nil {
				return err
			}
			_, err := db.Exec(ctx, `DROP TABLE IF EXISTS workspace_credits`)
			return err
		},
		// Down recreates workspace_credits and pours the counters back into it.
		// A per-tenant grant becomes the allowance column again; a workspace on
		// any plan but free comes back at that plan's limit, which is the
		// closest the old single-column shape can represent.
		Down: func(ctx context.Context, db *pg.DB) error {
			if _, err := db.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS workspace_credits (
				    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
				    workspace_id             UUID        NOT NULL UNIQUE REFERENCES workspaces(id) ON DELETE CASCADE,
				    monthly_token_allowance  BIGINT      NOT NULL DEFAULT 2000000,
				    current_cycle_start      TIMESTAMPTZ NOT NULL DEFAULT now(),
				    current_cycle_tokens     BIGINT      NOT NULL DEFAULT 0,
				    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
				    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
				)`); err != nil {
				return err
			}
			if _, err := db.Exec(ctx, `
				INSERT INTO workspace_credits (workspace_id, monthly_token_allowance, current_cycle_start, current_cycle_tokens)
				SELECT s.workspace_id, $1, s.billing_anchor,
				       COALESCE((SELECT u.used FROM feature_usage u
				                 WHERE u.workspace_id = s.workspace_id AND u.feature = $2
				                 ORDER BY u.period DESC LIMIT 1), 0)
				FROM workspace_subscriptions s
				ON CONFLICT (workspace_id) DO NOTHING`,
				features.FreeMonthlyTokenAllowance, string(features.AgentTokens),
			); err != nil {
				return err
			}
			for i := len(tables) - 1; i >= 0; i-- {
				if _, err := db.ExecExpr(ctx, pg.DropTableIfExists(tables[i])); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// backfillSubscriptions gives every workspace a subscription and carries its
// current cycle's spend across. It reads through a LEFT JOIN so a workspace
// with no credits row is included rather than left behind.
func backfillSubscriptions(ctx context.Context, db *pg.DB) error {
	rows, err := db.Query(ctx, `
		SELECT w.id, c.monthly_token_allowance, c.current_cycle_start, c.current_cycle_tokens
		FROM workspaces w
		LEFT JOIN workspace_credits c ON c.workspace_id = w.id`)
	if err != nil {
		return err
	}

	type carried struct {
		workspaceID string
		allowance   int64
		anchor      time.Time
		spent       int64
	}
	var all []carried
	now := time.Now().UTC()
	for rows.Next() {
		var (
			id        string
			allowance sql.NullInt64
			start     sql.NullTime
			spent     sql.NullInt64
		)
		if err := rows.Scan(&id, &allowance, &start, &spent); err != nil {
			_ = rows.Close()
			return err
		}
		c := carried{workspaceID: id, allowance: features.FreeMonthlyTokenAllowance, anchor: now}
		if allowance.Valid {
			c.allowance = allowance.Int64
		}
		if start.Valid {
			c.anchor = start.Time.UTC()
		}
		if spent.Valid {
			c.spent = spent.Int64
		}
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	for _, c := range all {
		grants := []entitlement.Grant{}
		if c.allowance != features.FreeMonthlyTokenAllowance {
			grants = append(grants, entitlement.Override(
				features.AgentTokens,
				&entitlement.Limit{Max: c.allowance, Period: entitlement.Month},
				"allowance carried over from workspace_credits",
			))
		}
		encoded, err := json.Marshal(grants)
		if err != nil {
			return err
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO workspace_subscriptions (workspace_id, plan, add_ons, grants, billing_anchor)
			VALUES ($1, $2, '[]'::jsonb, $3::jsonb, $4)
			ON CONFLICT (workspace_id) DO NOTHING`,
			c.workspaceID, string(features.PlanFree), string(encoded), c.anchor,
		); err != nil {
			return err
		}
		if c.spent <= 0 {
			continue
		}
		if _, err := db.Exec(ctx, `
			INSERT INTO feature_usage (workspace_id, feature, period, used)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (workspace_id, feature, period) DO NOTHING`,
			c.workspaceID, string(features.AgentTokens),
			entitlement.PeriodKey(entitlement.Month, c.anchor, now), c.spent,
		); err != nil {
			return err
		}
	}
	return nil
}
