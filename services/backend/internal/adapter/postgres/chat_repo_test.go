package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

func testChatRepo(t *testing.T) (*postgres.ChatRepo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return postgres.NewChatRepo(db), sqlDB
}

// TestCreateSession_EmptyWorkbenchIDStoresNull guards against the bug where a
// workspace-level chat (not tied to a specific workbench, the common case —
// see workspace.Service's shared-chat model) sent workbenchID == "" straight
// into the nullable uuid column, which Postgres rejects with SQLSTATE 22P02
// ("invalid input syntax for type uuid"). CreateChat must never fail for the
// no-workbench case.
func TestCreateSession_EmptyWorkbenchIDStoresNull(t *testing.T) {
	repo, sqlDB := testChatRepo(t)
	ctx := context.Background()

	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ('chat-repo-test@example.com', 'x', 'Chat Repo Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Chat Repo Test WS', 'chat-repo-test-ws-1', $1) RETURNING id`,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })

	session, err := repo.CreateSession(ctx, ownerID, workspaceID, "", "base-chat", "Test chat")
	if err != nil {
		t.Fatalf("CreateSession with empty workbenchID: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM chat_sessions WHERE id = $1`, session.ID) })

	if session.WorkbenchID != nil {
		t.Fatalf("WorkbenchID = %q, want nil (NULL)", *session.WorkbenchID)
	}
}

func TestFindMessageByID_RoundTrips(t *testing.T) {
	repo, sqlDB := testChatRepo(t)
	ctx := context.Background()

	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ('chat-repo-msg-test@example.com', 'x', 'Chat Repo Msg Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Chat Repo Msg Test WS', 'chat-repo-msg-test-ws-1', $1) RETURNING id`,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })

	session, err := repo.CreateSession(ctx, ownerID, workspaceID, "", "base-chat", "Test chat")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM chat_sessions WHERE id = $1`, session.ID) })

	inserted, err := repo.InsertMessage(ctx, session.ID, "choice_prompt", "Private or shared?", []byte(`[{"key":"A","label":"Private"}]`), nil, nil)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	got, err := repo.FindMessageByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("FindMessageByID: %v", err)
	}
	if got.ID != inserted.ID || got.Role != "choice_prompt" || got.Content != "Private or shared?" {
		t.Fatalf("got = %+v", got)
	}
	if got.Choices == nil {
		t.Fatal("Choices = nil, want the persisted JSON")
	}
}

func TestInsertMessage_PersistsTenders(t *testing.T) {
	repo, sqlDB := testChatRepo(t)
	ctx := context.Background()

	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ('chat-repo-tenders-test@example.com', 'x', 'Chat Repo Tenders Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Chat Repo Tenders Test WS', 'chat-repo-tenders-test-ws-1', $1) RETURNING id`,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })

	session, err := repo.CreateSession(ctx, ownerID, workspaceID, "", "base-chat", "Test chat")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM chat_sessions WHERE id = $1`, session.ID) })

	inserted, err := repo.InsertMessage(ctx, session.ID, "tender_results", "", nil, nil, []byte(`[{"id":"t-1","title":"Cestini intelligenti"}]`))
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	got, err := repo.FindMessageByID(ctx, inserted.ID)
	if err != nil {
		t.Fatalf("FindMessageByID: %v", err)
	}
	if got.Role != "tender_results" {
		t.Fatalf("Role = %q, want tender_results", got.Role)
	}
	if got.Tenders == nil {
		t.Fatal("Tenders = nil, want the persisted JSON")
	}
}
