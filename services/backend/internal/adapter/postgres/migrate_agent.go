package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateAgent is the 0004 schema migration for the agent/chat/credits feature.
// It creates five tables (chat_sessions, chat_messages, workspace_credits,
// agent_pricing, token_usage_log), adds FK indexes, and seeds the default
// allowance for every existing workspace plus the base-chat pricing row.
func migrateAgent() pg.Migration {
	tables := []*pg.Table{
		ChatSessions,
		ChatMessages,
		WorkspaceCredits,
		AgentPricing,
		TokenUsageLog,
	}
	return pg.Migration{
		Version: "0004",
		Name:    "agent",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			for _, idx := range agentIndexes() {
				if _, err := db.ExecExpr(ctx, pg.CreateIndexIfNotExists(idx)); err != nil {
					return err
				}
			}
			// Seed: one workspace_credits row per existing workspace (2M default).
			if _, err := db.Exec(ctx, `
				INSERT INTO workspace_credits (workspace_id)
				SELECT id FROM workspaces
				ON CONFLICT (workspace_id) DO NOTHING
			`); err != nil {
				return err
			}
			// Seed: default pricing for base-chat agent.
			if _, err := db.Exec(ctx, `
				INSERT INTO agent_pricing (agent_type, input_token_cost, output_token_cost)
				VALUES ('base-chat', 1, 1)
				ON CONFLICT (agent_type) DO NOTHING
			`); err != nil {
				return err
			}
			return nil
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			for i := len(tables) - 1; i >= 0; i-- {
				if _, err := db.ExecExpr(ctx, pg.DropTableIfExists(tables[i])); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func agentIndexes() []*pg.Index {
	return []*pg.Index{
		pg.NewIndex("idx_chat_sessions_member", ChatSessions, idxCol(ChatSessionMemberID)),
		pg.NewIndex("idx_chat_sessions_workspace", ChatSessions, idxCol(ChatSessionWorkspaceID)),
		pg.NewIndex("idx_chat_messages_session", ChatMessages, idxCol(ChatMessageSessionID)),
		pg.NewIndex("idx_token_usage_workspace", TokenUsageLog, idxCol(TUsageLogWorkspaceID)),
		pg.NewIndex("idx_token_usage_session", TokenUsageLog, idxCol(TUsageLogSessionID)),
		pg.NewIndex("idx_token_usage_created", TokenUsageLog, idxCol(TUsageLogCreatedAt)),
	}
}
