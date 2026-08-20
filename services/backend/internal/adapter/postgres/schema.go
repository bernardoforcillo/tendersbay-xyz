package postgres

import (
	"encoding/json"
	"time"

	"github.com/bernardoforcillo/drops/pg"
)

// Table and column definitions — single source of truth for all repositories.
var (
	Users               = pg.NewTable("users")
	UserID              = pg.Add(Users, pg.Text("id").PrimaryKey())
	UserEmail           = pg.Add(Users, pg.Text("email").NotNull())
	UserPasswordHash    = pg.Add(Users, pg.Text("password_hash").NotNull())
	UserDisplayName     = pg.Add(Users, pg.Text("display_name").NotNull())
	UserEmailVerifiedAt = pg.Add(Users, pg.Timestamp("email_verified_at", true))
	UserCreatedAt       = pg.Add(Users, pg.Timestamp("created_at", true).NotNull())
	UserUpdatedAt       = pg.Add(Users, pg.Timestamp("updated_at", true).NotNull())

	Sessions         = pg.NewTable("sessions")
	SessionID        = pg.Add(Sessions, pg.Text("id").PrimaryKey())
	SessionUserID    = pg.Add(Sessions, pg.Text("user_id").NotNull())
	SessionTokenHash = pg.Add(Sessions, pg.Text("token_hash").NotNull())
	SessionExpiresAt = pg.Add(Sessions, pg.Timestamp("expires_at", true).NotNull())
	SessionCreatedAt = pg.Add(Sessions, pg.Timestamp("created_at", true).NotNull())

	EmailVerifications = pg.NewTable("email_verifications")
	EVID               = pg.Add(EmailVerifications, pg.Text("id").PrimaryKey())
	EVUserID           = pg.Add(EmailVerifications, pg.Text("user_id").NotNull())
	EVNewEmail         = pg.Add(EmailVerifications, pg.Text("new_email").NotNull())
	EVTokenHash        = pg.Add(EmailVerifications, pg.Text("token_hash").NotNull())
	EVExpiresAt        = pg.Add(EmailVerifications, pg.Timestamp("expires_at", true).NotNull())
	EVCreatedAt        = pg.Add(EmailVerifications, pg.Timestamp("created_at", true).NotNull())

	PasswordResets = pg.NewTable("password_resets")
	PRID           = pg.Add(PasswordResets, pg.Text("id").PrimaryKey())
	PRUserID       = pg.Add(PasswordResets, pg.Text("user_id").NotNull())
	PRTokenHash    = pg.Add(PasswordResets, pg.Text("token_hash").NotNull())
	PRExpiresAt    = pg.Add(PasswordResets, pg.Timestamp("expires_at", true).NotNull())
	PRCreatedAt    = pg.Add(PasswordResets, pg.Timestamp("created_at", true).NotNull())
)

// DB scan targets — drops maps fields by `drop` tag.

type DBUser struct {
	ID              string     `drop:"id"`
	Email           string     `drop:"email"`
	PasswordHash    string     `drop:"password_hash"`
	DisplayName     string     `drop:"display_name"`
	EmailVerifiedAt *time.Time `drop:"email_verified_at"`
	CreatedAt       time.Time  `drop:"created_at"`
	UpdatedAt       time.Time  `drop:"updated_at"`
}

type DBSession struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}

type DBEmailVerification struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	NewEmail  string    `drop:"new_email"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}

type DBPasswordReset struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}

// ── Workspace tables ────────────────────────────────────────────────────────
// These columns carry full DDL constraints (types, NOT NULL, UNIQUE, DEFAULT,
// FOREIGN KEY) so drops generates the CREATE TABLE for the 0002 migration from
// the same handles the repositories query with. Composite constraints (the
// members composite PK and the (workspace_id, name)/(workspace_id, email)
// uniques) are added as raw ALTER TABLE in migrate_workspaces.go — drops does
// not emit them inline.
var (
	Workspaces         = pg.NewTable("workspaces")
	WorkspaceID        = pg.Add(Workspaces, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WorkspaceName      = pg.Add(Workspaces, pg.Text("name").NotNull())
	WorkspaceSlug      = pg.Add(Workspaces, pg.Text("slug").NotNull().Unique())
	WorkspaceOwnerID   = pg.Add(Workspaces, pg.UUID("owner_id").NotNull().References(UserID, pg.OnDelete("RESTRICT")))
	WorkspaceCreatedAt = pg.Add(Workspaces, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	WorkspaceUpdatedAt = pg.Add(Workspaces, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	WorkspaceRoles   = pg.NewTable("workspace_roles")
	WRoleID          = pg.Add(WorkspaceRoles, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WRoleWorkspaceID = pg.Add(WorkspaceRoles, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WRoleName        = pg.Add(WorkspaceRoles, pg.Text("name").NotNull())
	WRolePermissions = pg.Add(WorkspaceRoles, pg.BigInt("permissions").NotNull().Default("0"))
	WRoleIsDefault   = pg.Add(WorkspaceRoles, pg.Boolean("is_default").NotNull().Default("false"))
	WRoleCreatedAt   = pg.Add(WorkspaceRoles, pg.Timestamp("created_at", true).NotNull().Default("now()"))

	WorkspaceMembers   = pg.NewTable("workspace_members")
	WMemberWorkspaceID = pg.Add(WorkspaceMembers, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WMemberUserID      = pg.Add(WorkspaceMembers, pg.UUID("user_id").NotNull().References(UserID, pg.OnDelete("CASCADE")))
	WMemberRoleID      = pg.Add(WorkspaceMembers, pg.UUID("role_id").NotNull().References(WRoleID, pg.OnDelete("RESTRICT")))
	WMemberJoinedAt    = pg.Add(WorkspaceMembers, pg.Timestamp("joined_at", true).NotNull().Default("now()"))

	WorkspaceEmailInvites = pg.NewTable("workspace_email_invitations")
	WEInviteID            = pg.Add(WorkspaceEmailInvites, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WEInviteWorkspaceID   = pg.Add(WorkspaceEmailInvites, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WEInviteEmail         = pg.Add(WorkspaceEmailInvites, pg.Text("email").NotNull())
	WEInviteRoleID        = pg.Add(WorkspaceEmailInvites, pg.UUID("role_id").NotNull().References(WRoleID, pg.OnDelete("CASCADE")))
	WEInviteTokenHash     = pg.Add(WorkspaceEmailInvites, pg.Text("token_hash").NotNull().Unique())
	WEInviteInvitedBy     = pg.Add(WorkspaceEmailInvites, pg.UUID("invited_by").NotNull().References(UserID, pg.OnDelete("CASCADE")))
	WEInviteExpiresAt     = pg.Add(WorkspaceEmailInvites, pg.Timestamp("expires_at", true).NotNull())
	WEInviteCreatedAt     = pg.Add(WorkspaceEmailInvites, pg.Timestamp("created_at", true).NotNull().Default("now()"))

	WorkspaceInviteLinks = pg.NewTable("workspace_invite_links")
	WLinkID              = pg.Add(WorkspaceInviteLinks, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WLinkWorkspaceID     = pg.Add(WorkspaceInviteLinks, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WLinkCode            = pg.Add(WorkspaceInviteLinks, pg.Text("code").NotNull().Unique())
	WLinkRoleID          = pg.Add(WorkspaceInviteLinks, pg.UUID("role_id").NotNull().References(WRoleID, pg.OnDelete("CASCADE")))
	WLinkCreatedBy       = pg.Add(WorkspaceInviteLinks, pg.UUID("created_by").NotNull().References(UserID, pg.OnDelete("CASCADE")))
	WLinkMaxUses         = pg.Add(WorkspaceInviteLinks, pg.Integer("max_uses").NotNull().Default("0"))
	WLinkUseCount        = pg.Add(WorkspaceInviteLinks, pg.Integer("use_count").NotNull().Default("0"))
	WLinkExpiresAt       = pg.Add(WorkspaceInviteLinks, pg.Timestamp("expires_at", true)) // nullable
	WLinkRevoked         = pg.Add(WorkspaceInviteLinks, pg.Boolean("revoked").NotNull().Default("false"))
	WLinkCreatedAt       = pg.Add(WorkspaceInviteLinks, pg.Timestamp("created_at", true).NotNull().Default("now()"))
)

type DBWorkspace struct {
	ID        string    `drop:"id"`
	Name      string    `drop:"name"`
	Slug      string    `drop:"slug"`
	OwnerID   string    `drop:"owner_id"`
	CreatedAt time.Time `drop:"created_at"`
	UpdatedAt time.Time `drop:"updated_at"`
}

type DBWorkspaceRole struct {
	ID          string    `drop:"id"`
	WorkspaceID string    `drop:"workspace_id"`
	Name        string    `drop:"name"`
	Permissions int64     `drop:"permissions"`
	IsDefault   bool      `drop:"is_default"`
	CreatedAt   time.Time `drop:"created_at"`
}

type DBWorkspaceMember struct {
	WorkspaceID string    `drop:"workspace_id"`
	UserID      string    `drop:"user_id"`
	RoleID      string    `drop:"role_id"`
	JoinedAt    time.Time `drop:"joined_at"`
}

type DBWorkspaceEmailInvite struct {
	ID          string    `drop:"id"`
	WorkspaceID string    `drop:"workspace_id"`
	Email       string    `drop:"email"`
	RoleID      string    `drop:"role_id"`
	TokenHash   string    `drop:"token_hash"`
	InvitedBy   string    `drop:"invited_by"`
	ExpiresAt   time.Time `drop:"expires_at"`
	CreatedAt   time.Time `drop:"created_at"`
}

type DBWorkspaceInviteLink struct {
	ID          string     `drop:"id"`
	WorkspaceID string     `drop:"workspace_id"`
	Code        string     `drop:"code"`
	RoleID      string     `drop:"role_id"`
	CreatedBy   string     `drop:"created_by"`
	MaxUses     int32      `drop:"max_uses"`
	UseCount    int32      `drop:"use_count"`
	ExpiresAt   *time.Time `drop:"expires_at"`
	Revoked     bool       `drop:"revoked"`
	CreatedAt   time.Time  `drop:"created_at"`
}

// DBMembership is the flat scan target for the workspace_members ⋈ workspace_roles
// join used to load a caller's membership + role in one query.
type DBMembership struct {
	WorkspaceID string    `drop:"workspace_id"`
	UserID      string    `drop:"user_id"`
	RoleID      string    `drop:"role_id"`
	RoleName    string    `drop:"name"`
	Permissions int64     `drop:"permissions"`
	JoinedAt    time.Time `drop:"joined_at"`
}

// ── Workbench tables ────────────────────────────────────────────────────────
// Mirrors the workspace tables above: full DDL constraints on the drops
// handles so the 0003 migration generates CREATE TABLE from the same columns
// the repositories query with. Composite constraints (the members composite
// PK and the (workbench_id, name) unique) are added as raw ALTER TABLE in
// migrate_workbenches.go — drops does not emit them inline.
var (
	Workbenches   = pg.NewTable("workbenches")
	WBID          = pg.Add(Workbenches, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WBWorkspaceID = pg.Add(Workbenches, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WBName        = pg.Add(Workbenches, pg.Text("name").NotNull())
	WBDescription = pg.Add(Workbenches, pg.Text("description").NotNull().Default("''"))
	WBVisibility  = pg.Add(Workbenches, pg.Text("visibility").NotNull().Default("'private'"))
	WBOwnerID     = pg.Add(Workbenches, pg.UUID("owner_id").NotNull().References(UserID, pg.OnDelete("RESTRICT")))
	WBCreatedAt   = pg.Add(Workbenches, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	WBUpdatedAt   = pg.Add(Workbenches, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	WorkbenchRoles    = pg.NewTable("workbench_roles")
	WBRoleID          = pg.Add(WorkbenchRoles, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	WBRoleWorkbenchID = pg.Add(WorkbenchRoles, pg.UUID("workbench_id").NotNull().References(WBID, pg.OnDelete("CASCADE")))
	WBRoleName        = pg.Add(WorkbenchRoles, pg.Text("name").NotNull())
	WBRolePermissions = pg.Add(WorkbenchRoles, pg.BigInt("permissions").NotNull().Default("0"))
	WBRoleIsDefault   = pg.Add(WorkbenchRoles, pg.Boolean("is_default").NotNull().Default("false"))
	WBRoleCreatedAt   = pg.Add(WorkbenchRoles, pg.Timestamp("created_at", true).NotNull().Default("now()"))

	WorkbenchMembers    = pg.NewTable("workbench_members")
	WBMemberWorkbenchID = pg.Add(WorkbenchMembers, pg.UUID("workbench_id").NotNull().References(WBID, pg.OnDelete("CASCADE")))
	WBMemberUserID      = pg.Add(WorkbenchMembers, pg.UUID("user_id").NotNull().References(UserID, pg.OnDelete("CASCADE")))
	WBMemberRoleID      = pg.Add(WorkbenchMembers, pg.UUID("role_id").NotNull().References(WBRoleID, pg.OnDelete("RESTRICT")))
	WBMemberAddedAt     = pg.Add(WorkbenchMembers, pg.Timestamp("added_at", true).NotNull().Default("now()"))
)

type DBWorkbench struct {
	ID          string    `drop:"id"`
	WorkspaceID string    `drop:"workspace_id"`
	Name        string    `drop:"name"`
	Description string    `drop:"description"`
	Visibility  string    `drop:"visibility"`
	OwnerID     string    `drop:"owner_id"`
	CreatedAt   time.Time `drop:"created_at"`
	UpdatedAt   time.Time `drop:"updated_at"`
}

type DBWorkbenchRole struct {
	ID          string    `drop:"id"`
	WorkbenchID string    `drop:"workbench_id"`
	Name        string    `drop:"name"`
	Permissions int64     `drop:"permissions"`
	IsDefault   bool      `drop:"is_default"`
	CreatedAt   time.Time `drop:"created_at"`
}

type DBWorkbenchMember struct {
	WorkbenchID string    `drop:"workbench_id"`
	UserID      string    `drop:"user_id"`
	RoleID      string    `drop:"role_id"`
	AddedAt     time.Time `drop:"added_at"`
}

// DBWorkbenchMembership is the flat scan target for the workbench_members ⋈
// workbench_roles join used to load a caller's membership + role in one query.
type DBWorkbenchMembership struct {
	WorkbenchID string    `drop:"workbench_id"`
	UserID      string    `drop:"user_id"`
	RoleID      string    `drop:"role_id"`
	RoleName    string    `drop:"name"`
	Permissions int64     `drop:"permissions"`
	AddedAt     time.Time `drop:"added_at"`
}

// ── Agent tables (chat, credits, pricing, usage) ────────────────────────────
var (
	ChatSessions           = pg.NewTable("chat_sessions")
	ChatSessionID          = pg.Add(ChatSessions, pg.UUID("id").Default("gen_random_uuid()").PrimaryKey())
	ChatSessionMemberID    = pg.Add(ChatSessions, pg.UUID("member_id").NotNull().References(UserID, pg.OnDelete("CASCADE")))
	ChatSessionWorkspaceID = pg.Add(ChatSessions, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	ChatSessionWorkbenchID = pg.Add(ChatSessions, pg.UUID("workbench_id").References(WBID, pg.OnDelete("SET NULL")))
	ChatSessionAgentType   = pg.Add(ChatSessions, pg.Text("agent_type").NotNull())
	ChatSessionTitle       = pg.Add(ChatSessions, pg.Text("title").NotNull().Default("'Nuova chat'"))
	ChatSessionCreatedAt   = pg.Add(ChatSessions, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	ChatSessionUpdatedAt   = pg.Add(ChatSessions, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	ChatMessages         = pg.NewTable("chat_messages")
	ChatMessageID        = pg.Add(ChatMessages, pg.UUID("id").Default("gen_random_uuid()").PrimaryKey())
	ChatMessageSessionID = pg.Add(ChatMessages, pg.UUID("session_id").NotNull().References(ChatSessionID, pg.OnDelete("CASCADE")))
	ChatMessageRole      = pg.Add(ChatMessages, pg.Text("role").NotNull())
	ChatMessageContent   = pg.Add(ChatMessages, pg.Text("content").NotNull())
	ChatMessageChoices   = pg.Add(ChatMessages, pg.JSONB("choices"))
	ChatMessageMetadata  = pg.Add(ChatMessages, pg.JSONB("metadata"))
	ChatMessageTenders   = pg.Add(ChatMessages, pg.JSONB("tenders"))
	ChatMessageCreatedAt = pg.Add(ChatMessages, pg.Timestamp("created_at", true).NotNull().Default("now()"))

	WorkspaceCredits              = pg.NewTable("workspace_credits")
	WCreditsID                    = pg.Add(WorkspaceCredits, pg.UUID("id").Default("gen_random_uuid()").PrimaryKey())
	WCreditsWorkspaceID           = pg.Add(WorkspaceCredits, pg.UUID("workspace_id").NotNull().Unique().References(WorkspaceID, pg.OnDelete("CASCADE")))
	WCreditsMonthlyTokenAllowance = pg.Add(WorkspaceCredits, pg.BigInt("monthly_token_allowance").NotNull().Default("2000000"))
	WCreditsCurrentCycleStart     = pg.Add(WorkspaceCredits, pg.Timestamp("current_cycle_start", true).NotNull().Default("now()"))
	WCreditsCurrentCycleTokens    = pg.Add(WorkspaceCredits, pg.BigInt("current_cycle_tokens").NotNull().Default("0"))
	WCreditsCreatedAt             = pg.Add(WorkspaceCredits, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	WCreditsUpdatedAt             = pg.Add(WorkspaceCredits, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	AgentPricing            = pg.NewTable("agent_pricing")
	APricingID              = pg.Add(AgentPricing, pg.UUID("id").Default("gen_random_uuid()").PrimaryKey())
	APricingAgentType       = pg.Add(AgentPricing, pg.Text("agent_type").NotNull().Unique())
	APricingInputTokenCost  = pg.Add(AgentPricing, pg.BigInt("input_token_cost").NotNull().Default("1"))
	APricingOutputTokenCost = pg.Add(AgentPricing, pg.BigInt("output_token_cost").NotNull().Default("1"))
	APricingCreatedAt       = pg.Add(AgentPricing, pg.Timestamp("created_at", true).NotNull().Default("now()"))

	TokenUsageLog           = pg.NewTable("token_usage_log")
	TUsageLogID             = pg.Add(TokenUsageLog, pg.UUID("id").Default("gen_random_uuid()").PrimaryKey())
	TUsageLogWorkspaceID    = pg.Add(TokenUsageLog, pg.UUID("workspace_id").NotNull())
	TUsageLogUserID         = pg.Add(TokenUsageLog, pg.Text("user_id").NotNull())
	TUsageLogAgentType      = pg.Add(TokenUsageLog, pg.Text("agent_type").NotNull())
	TUsageLogSessionID      = pg.Add(TokenUsageLog, pg.UUID("session_id").NotNull())
	TUsageLogModel          = pg.Add(TokenUsageLog, pg.Text("model").NotNull())
	TUsageLogInputTokens    = pg.Add(TokenUsageLog, pg.Integer("input_tokens").NotNull().Default("0"))
	TUsageLogOutputTokens   = pg.Add(TokenUsageLog, pg.Integer("output_tokens").NotNull().Default("0"))
	TUsageLogTotalTokens    = pg.Add(TokenUsageLog, pg.Integer("total_tokens").NotNull().Default("0"))
	TUsageLogCostMultiplier = pg.Add(TokenUsageLog, pg.BigInt("cost_multiplier").NotNull().Default("1"))
	TUsageLogCreatedAt      = pg.Add(TokenUsageLog, pg.Timestamp("created_at", true).NotNull().Default("now()"))
)

type DBChatSession struct {
	ID          string    `drop:"id"`
	MemberID    string    `drop:"member_id"`
	WorkspaceID string    `drop:"workspace_id"`
	WorkbenchID *string   `drop:"workbench_id"`
	AgentType   string    `drop:"agent_type"`
	Title       string    `drop:"title"`
	CreatedAt   time.Time `drop:"created_at"`
	UpdatedAt   time.Time `drop:"updated_at"`
}

type DBChatMessage struct {
	ID        string           `drop:"id"`
	SessionID string           `drop:"session_id"`
	Role      string           `drop:"role"`
	Content   string           `drop:"content"`
	Choices   *json.RawMessage `drop:"choices"`
	Metadata  *json.RawMessage `drop:"metadata"`
	Tenders   *json.RawMessage `drop:"tenders"`
	CreatedAt time.Time        `drop:"created_at"`
}

type DBWorkspaceCredits struct {
	ID                 string    `drop:"id"`
	WorkspaceID        string    `drop:"workspace_id"`
	MonthlyAllowance   int64     `drop:"monthly_token_allowance"`
	CurrentCycleStart  time.Time `drop:"current_cycle_start"`
	CurrentCycleTokens int64     `drop:"current_cycle_tokens"`
	CreatedAt          time.Time `drop:"created_at"`
	UpdatedAt          time.Time `drop:"updated_at"`
}

type DBAgentPricing struct {
	ID         string    `drop:"id"`
	AgentType  string    `drop:"agent_type"`
	InputCost  int64     `drop:"input_token_cost"`
	OutputCost int64     `drop:"output_token_cost"`
	CreatedAt  time.Time `drop:"created_at"`
}

type DBTokenUsage struct {
	ID             string    `drop:"id"`
	WorkspaceID    string    `drop:"workspace_id"`
	UserID         string    `drop:"user_id"`
	AgentType      string    `drop:"agent_type"`
	SessionID      string    `drop:"session_id"`
	Model          string    `drop:"model"`
	InputTokens    int32     `drop:"input_tokens"`
	OutputTokens   int32     `drop:"output_tokens"`
	TotalTokens    int32     `drop:"total_tokens"`
	CostMultiplier int64     `drop:"cost_multiplier"`
	CreatedAt      time.Time `drop:"created_at"`
}

// Tenders references tenders.ingested_tenders, owned and migrated by
// services/ingestion — this service only ever reads it (never migrates,
// never writes). Only the columns this service's search API actually
// needs are declared; the real table has more (raw, history, version,
// first_seen_at, last_seen_at, indexed_at, ...) that this service doesn't
// touch.
var (
	Tenders             = pg.NewSchemaTable("tenders", "ingested_tenders")
	TenderID            = pg.Add(Tenders, pg.BigInt("id").PrimaryKey())
	TenderSource        = pg.Add(Tenders, pg.Text("source").NotNull())
	TenderSourceRef     = pg.Add(Tenders, pg.Text("source_ref").NotNull())
	TenderTitle         = pg.Add(Tenders, pg.Text("title").NotNull())
	TenderBuyerName     = pg.Add(Tenders, pg.Text("buyer_name").NotNull())
	TenderStatus        = pg.Add(Tenders, pg.Text("status").NotNull())
	TenderProcedureType = pg.Add(Tenders, pg.Text("procedure_type").NotNull())
	TenderCountry       = pg.Add(Tenders, pg.Text("country").NotNull())
	TenderCPV           = pg.Add(Tenders, pg.Text("cpv").NotNull())
	TenderValue         = pg.Add(Tenders, pg.BigInt("value")) // nullable
	TenderCurrency      = pg.Add(Tenders, pg.Text("currency").NotNull())
	TenderPublishedAt   = pg.Add(Tenders, pg.Timestamp("published_at", true)) // nullable
	TenderDeadline      = pg.Add(Tenders, pg.Timestamp("deadline", true))     // nullable
	TenderNUTS          = pg.Add(Tenders, pg.Text("nuts").NotNull())
	// NOTE: cpv_secondary (text[]) is intentionally NOT declared here — drops
	// (v0.4.1) has no array-typed column constructor (its pg package only
	// exposes query-time array *operators* in array.go — ArrayContains,
	// ArrayAgg, Any, Unnest, etc. — not a column DSL type), so it cannot be
	// added to tenderResultColumns or scanned via the typed Select()+struct
	// path. See Task A0's report for the fuller analysis, including why the
	// candidate raw-SQL fallback (scanning text[] via the pgx/v5 stdlib
	// driver's plain database/sql path) doesn't work either. Secondary-CPV
	// matching is deferred; SectorMatch degrades to primary CPV only.
)

type DBTender struct {
	ID            int64      `drop:"id"`
	Source        string     `drop:"source"`
	SourceRef     string     `drop:"source_ref"`
	Title         string     `drop:"title"`
	BuyerName     string     `drop:"buyer_name"`
	Status        string     `drop:"status"`
	ProcedureType string     `drop:"procedure_type"`
	Country       string     `drop:"country"`
	CPV           string     `drop:"cpv"`
	Value         *int64     `drop:"value"`
	Currency      string     `drop:"currency"`
	PublishedAt   *time.Time `drop:"published_at"`
	Deadline      *time.Time `drop:"deadline"`
	NUTS          string     `drop:"nuts"`
	SourceURL     *string    `drop:"url"`
}

// TenderDocuments references tenders.ingested_tender_documents — like
// Tenders above, owned and migrated exclusively by services/ingestion; this
// service only ever reads it, to resolve one tender's notice-document URL
// (the eForms mapper writes at most one row of type "notice" per tender —
// see services/ingestion/internal/adapter/source/eforms/map.go).
var (
	TenderDocuments = pg.NewSchemaTable("tenders", "ingested_tender_documents")
	TDocID          = pg.Add(TenderDocuments, pg.BigInt("id").PrimaryKey())
	TDocTenderID    = pg.Add(TenderDocuments, pg.BigInt("tender_id").NotNull())
	TDocURL         = pg.Add(TenderDocuments, pg.Text("url").NotNull())
	TDocType        = pg.Add(TenderDocuments, pg.Text("type").NotNull())
)

// ── Client profile table (per-client bid-qualification agent, v1.0) ────────
// One row per workspace (PK = FK to workspaces.id) — the workspace IS the
// client (see docs/superpowers/specs/2026-07-17-per-client-bid-qualification-agent-design.md).
// sectors/countries are JSONB (drops has no array column type; JSONB is this
// codebase's existing precedent for list-shaped columns, e.g. chat_messages).
var (
	ClientProfiles   = pg.NewTable("workspace_client_profiles")
	CPWorkspaceID    = pg.Add(ClientProfiles, pg.UUID("workspace_id").PrimaryKey().References(WorkspaceID, pg.OnDelete("CASCADE")))
	CPSectors        = pg.Add(ClientProfiles, pg.JSONB("sectors").NotNull().Default("'[]'::jsonb"))
	CPCountries      = pg.Add(ClientProfiles, pg.JSONB("countries").NotNull().Default("'[]'::jsonb"))
	CPRegions        = pg.Add(ClientProfiles, pg.JSONB("regions").NotNull().Default("'[]'::jsonb"))
	CPProcedureTypes = pg.Add(ClientProfiles, pg.JSONB("procedure_types").NotNull().Default("'[]'::jsonb"))
	CPValueMin       = pg.Add(ClientProfiles, pg.BigInt("value_min")) // nullable
	CPValueMax       = pg.Add(ClientProfiles, pg.BigInt("value_max")) // nullable
	CPNotes          = pg.Add(ClientProfiles, pg.Text("notes").NotNull().Default("''"))
	CPUpdatedAt      = pg.Add(ClientProfiles, pg.Timestamp("updated_at", true).NotNull().Default("now()"))
)

type DBClientProfile struct {
	WorkspaceID    string          `drop:"workspace_id"`
	Sectors        json.RawMessage `drop:"sectors"`
	Countries      json.RawMessage `drop:"countries"`
	Regions        json.RawMessage `drop:"regions"`
	ProcedureTypes json.RawMessage `drop:"procedure_types"`
	ValueMin       *int64          `drop:"value_min"`
	ValueMax       *int64          `drop:"value_max"`
	Notes          string          `drop:"notes"`
	UpdatedAt      time.Time       `drop:"updated_at"`
}

// Extra ingested_tenders scalar columns the detail API needs.
// TenderNUTS is declared above, alongside the other Tenders columns.
var (
	TenderBuyerID  = pg.Add(Tenders, pg.Text("buyer_id").NotNull())
	TenderLanguage = pg.Add(Tenders, pg.Text("language").NotNull())
)

// The eForms detail columns, added to ingested_tenders by the ingestion
// service's 0010 migration. Every one of them is nullable except description,
// because ingested_tenders is shared with pl-bzp, fr-boamp and es-placsp, none
// of which publish eForms XML and none of which will ever populate one.
//
// TenderGridUsable is the first nullable BOOLEAN this service reads, and it
// must stay one: it is three-valued (NULL = not yet read, false = read with no
// weighted criterion, true = read with one). It scans into a *bool by the same
// mechanism the long-standing nullable TenderValue (*int64) and
// TenderPublishedAt (*time.Time) already use — database/sql's convertAssign
// allocates through the extra pointer level for a non-NULL value and zeroes it
// for NULL. Do NOT "simplify" it to a bool with a false default: that collapse
// is exactly what makes "we haven't looked" indistinguishable from "we looked
// and there is no grid".
var (
	TenderDescription       = pg.Add(Tenders, pg.Text("description").NotNull())
	TenderPublicationNumber = pg.Add(Tenders, pg.Text("publication_number")) // nullable
	TenderDocumentsURL      = pg.Add(Tenders, pg.Text("documents_url"))      // nullable
	TenderSubmissionURL     = pg.Add(Tenders, pg.Text("submission_url"))     // nullable
	TenderGridUsable        = pg.Add(Tenders, pg.Boolean("grid_usable"))     // nullable, THREE-valued
	TenderXMLFetchedAt      = pg.Add(Tenders, pg.Timestamp("xml_fetched_at", true))
	// TenderXMLStatus is the ingestion FETCHER's own vocabulary —
	// 'ok' | 'absent' | 'parse_error' | 'unavailable' — and it stops here. It
	// is read so that this adapter can decide what "successfully read" means
	// and hand core a plain answer; the string itself never crosses into a
	// domain type, because a domain that spoke it would make swapping the
	// fetcher a change to every layer above.
	TenderXMLStatus = pg.Add(Tenders, pg.Text("xml_status")) // nullable
)

// xmlStatusOK is the one value of ingested_tenders.xml_status that means the
// notice was actually read. The other three ('absent', 'parse_error',
// 'unavailable') all set xml_fetched_at as well — they are terminal outcomes,
// not pending ones — so the timestamp alone cannot distinguish "we read it"
// from "we gave up on it", and using it alone would report an unreadable
// notice as an enriched one.
const xmlStatusOK = "ok"

// NOTE: tenders.ingested_tender_award_criteria and
// tenders.ingested_tender_organizations get no drops handles at all, and
// ingested_tender_lots' three new eForms columns get none either. This is the
// same constraint the cpv_secondary note above records, hit from two more
// directions:
//
//   - organizations.roles and lots.cpv_secondary are text[], which drops has no
//     column DSL type for and no scan path to, so both tables are read through
//     array_to_string in raw SQL (see tender_detail_repo.go);
//   - award_criteria.weight is `numeric`, which will not scan into a *float64
//     without an explicit ::float8 projection — again not expressible through
//     the typed builder.
//
// Declaring handles nothing can use would only look like coverage. The raw SQL
// in tender_detail_repo.go and document_repo.go is the authoritative statement
// of what this service reads from those tables.

// Read-only child tables of tenders.ingested_tenders (owned by services/ingestion).
// TenderDocuments (the table itself) is declared above, alongside TDocID/TDocTenderID/
// TDocURL/TDocType; these are additional column bindings against that same table.
var (
	TenderDocTenderID = pg.Add(TenderDocuments, pg.BigInt("tender_id").NotNull())
	TenderDocURL      = pg.Add(TenderDocuments, pg.Text("url").NotNull())
	TenderDocType     = pg.Add(TenderDocuments, pg.Text("type").NotNull())
)

// tenders.ingested_tender_lots deliberately has NO handles here any more. It
// used to be read through the typed builder, but the lot row now includes
// cpv_secondary (text[]), which drops can neither declare nor scan — the same
// constraint recorded on ingested_tenders.cpv_secondary above — so the whole
// lot read moved to raw SQL in tender_detail_repo.go. Handles for the scalar
// columns alone would be dead declarations that read like coverage.

// DBTenderDetail scans the scalar detail columns (NOT cpv_secondary — that text[]
// column is read separately via array_to_string; see the repo).
type DBTenderDetail struct {
	ID            int64      `drop:"id"`
	Source        string     `drop:"source"`
	SourceRef     string     `drop:"source_ref"`
	Title         string     `drop:"title"`
	BuyerName     string     `drop:"buyer_name"`
	BuyerID       string     `drop:"buyer_id"`
	Status        string     `drop:"status"`
	ProcedureType string     `drop:"procedure_type"`
	Country       string     `drop:"country"`
	NUTS          string     `drop:"nuts"`
	Language      string     `drop:"language"`
	CPV           string     `drop:"cpv"`
	Value         *int64     `drop:"value"`
	Currency      string     `drop:"currency"`
	PublishedAt   *time.Time `drop:"published_at"`
	Deadline      *time.Time `drop:"deadline"`
	// The eForms detail. The pointer fields are nullable columns, not optional
	// modelling: see the TenderGridUsable note above for why grid_usable in
	// particular must never be flattened to a plain bool.
	Description       string     `drop:"description"`
	PublicationNumber *string    `drop:"publication_number"`
	DocumentsURL      *string    `drop:"documents_url"`
	SubmissionURL     *string    `drop:"submission_url"`
	GridUsable        *bool      `drop:"grid_usable"`
	XMLFetchedAt      *time.Time `drop:"xml_fetched_at"`
	XMLStatus         *string    `drop:"xml_status"`
}

type DBTenderDocument struct {
	URL  string `drop:"url"`
	Type string `drop:"type"`
}

// ── Bid tables (workbench bando hub) ────────────────────────────────────────
// Mirrors the workbench tables above: full DDL constraints on the drops handles
// so the 0009 migration generates CREATE TABLE from the same columns the repo
// queries with. The composite uniques ((workbench_id, tender_id) so a tender is
// taken into a workbench at most once, and (bid_id, item_code) which backs the
// checklist upsert) are added as raw ALTER TABLE in migrate_bids.go — drops does
// not emit them inline. tender_id is a plain BigInt with NO .References:
// tenders.ingested_tenders is owned and migrated by services/ingestion.
var (
	Bids           = pg.NewTable("bids")
	BidID          = pg.Add(Bids, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	BidWorkbenchID = pg.Add(Bids, pg.UUID("workbench_id").NotNull().References(WBID, pg.OnDelete("CASCADE")))
	BidTenderID    = pg.Add(Bids, pg.BigInt("tender_id").NotNull())
	BidGoNoGo      = pg.Add(Bids, pg.Text("go_no_go").NotNull().Default("'undecided'"))
	BidStage       = pg.Add(Bids, pg.Text("stage").NotNull().Default("'shortlisted'"))
	BidOutcome     = pg.Add(Bids, pg.Text("outcome")) // nullable: NULL = open

	// The decision record (0011): the eligibility recommendation the go/no-go
	// was taken against. Stored on the bid and not derived on read because
	// company.Assessment is deliberately never persisted — by the time anyone
	// asks "how often did the user disagree with us", the dossier and the
	// requirements have both moved and the baseline is gone.
	//
	// decision_recommendation is NOT NULL DEFAULT '' rather than nullable: ''
	// means "no recommendation existed" (the check could not be run), which is a
	// real, queryable state, and a NULL would additionally have to be
	// distinguished from a row written before this column existed.
	BidDecisionRecommendation = pg.Add(Bids, pg.Text("decision_recommendation").NotNull().Default("''"))
	BidDecisionOverridden     = pg.Add(Bids, pg.Boolean("decision_overridden").NotNull().Default("false"))
	BidDecisionBlockingGaps   = pg.Add(Bids, pg.BigInt("decision_blocking_gaps").NotNull().Default("0"))
	BidDecisionRecordedAt     = pg.Add(Bids, pg.Timestamp("decision_recorded_at", true)) // nullable: NULL = still undecided

	// BidLastRemindedBucket is the reminder watermark: the narrowest
	// days-before-deadline threshold already processed for this bid, 0 = none.
	// It lives on the bid rather than in its own table because it is one small
	// integer with the same lifetime as the row — a join table would be a second
	// place for the same fact to be wrong, and the row's own deletion already
	// takes it with it.
	BidLastRemindedBucket = pg.Add(Bids, pg.BigInt("last_reminded_bucket").NotNull().Default("0"))

	BidCreatedBy = pg.Add(Bids, pg.UUID("created_by").NotNull())
	BidCreatedAt = pg.Add(Bids, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	BidUpdatedAt = pg.Add(Bids, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	BidChecklistItems = pg.NewTable("bid_checklist_items")
	BCIID             = pg.Add(BidChecklistItems, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	BCIBidID          = pg.Add(BidChecklistItems, pg.UUID("bid_id").NotNull().References(BidID, pg.OnDelete("CASCADE")))
	BCISectionCode    = pg.Add(BidChecklistItems, pg.Text("section_code").NotNull())
	BCIItemCode       = pg.Add(BidChecklistItems, pg.Text("item_code").NotNull())
	BCIStatus         = pg.Add(BidChecklistItems, pg.Text("status").NotNull().Default("'pending'"))
	BCINote           = pg.Add(BidChecklistItems, pg.Text("note").NotNull().Default("''"))
	BCIRequired       = pg.Add(BidChecklistItems, pg.Boolean("required").NotNull().Default("true"))
	BCIPosition       = pg.Add(BidChecklistItems, pg.BigInt("position").NotNull().Default("0"))
	BCIUpdatedAt      = pg.Add(BidChecklistItems, pg.Timestamp("updated_at", true).NotNull().Default("now()"))
)

type DBBid struct {
	ID                     string     `drop:"id"`
	WorkbenchID            string     `drop:"workbench_id"`
	TenderID               int64      `drop:"tender_id"`
	GoNoGo                 string     `drop:"go_no_go"`
	Stage                  string     `drop:"stage"`
	Outcome                *string    `drop:"outcome"` // nullable: nil = open
	DecisionRecommendation string     `drop:"decision_recommendation"`
	DecisionOverridden     bool       `drop:"decision_overridden"`
	DecisionBlockingGaps   int64      `drop:"decision_blocking_gaps"`
	DecisionRecordedAt     *time.Time `drop:"decision_recorded_at"` // nullable: nil = still undecided
	LastRemindedBucket     int64      `drop:"last_reminded_bucket"`
	CreatedBy              string     `drop:"created_by"`
	CreatedAt              time.Time  `drop:"created_at"`
	UpdatedAt              time.Time  `drop:"updated_at"`
}

type DBChecklistItem struct {
	ID          string    `drop:"id"`
	BidID       string    `drop:"bid_id"`
	SectionCode string    `drop:"section_code"`
	ItemCode    string    `drop:"item_code"`
	Status      string    `drop:"status"`
	Note        string    `drop:"note"`
	Required    bool      `drop:"required"`
	Position    int64     `drop:"position"`
	UpdatedAt   time.Time `drop:"updated_at"`
}

// ── Reminder preferences ────────────────────────────────────────────────────
//
// One row per user, minted the first time they are considered for a reminder.
//
// unsubscribe_token is stored in PLAIN, unlike auth's tokens which are stored
// hashed, and that difference is a threat model rather than an inconsistency.
// An auth token grants access to an account, so the server must never be able
// to reproduce one. This token grants exactly one power — stop sending me these
// reminders — and it has to travel in every message and still work in a mail
// opened months later, which a hash cannot do. A leaked one buys an attacker
// the ability to silence a person's own reminders, and it is revoked by
// regenerating the column.
//
// opted_out_at is a timestamp rather than a boolean so the suppression list
// records WHEN, which is what a deliverability complaint is answered with.
var (
	ReminderPrefs      = pg.NewTable("reminder_preferences")
	RPUserID           = pg.Add(ReminderPrefs, pg.UUID("user_id").PrimaryKey())
	RPUnsubscribeToken = pg.Add(ReminderPrefs, pg.Text("unsubscribe_token").NotNull().Unique())
	RPOptedOutAt       = pg.Add(ReminderPrefs, pg.Timestamp("opted_out_at", true)) // nullable: NULL = still subscribed
	RPCreatedAt        = pg.Add(ReminderPrefs, pg.Timestamp("created_at", true).NotNull().Default("now()"))
)

type DBReminderPrefs struct {
	UserID           string     `drop:"user_id"`
	UnsubscribeToken string     `drop:"unsubscribe_token"`
	OptedOutAt       *time.Time `drop:"opted_out_at"`
	CreatedAt        time.Time  `drop:"created_at"`
}

// ── Company dossier tables (Phase 2) ────────────────────────────────────────
//
// One company per workspace, enforced in the SCHEMA and not only in code:
// workspace_companies' primary key IS the foreign key to workspaces.id, exactly
// as workspace_client_profiles does one block up. There is deliberately no
// company id anywhere — the workspace IS the company, so a second company in a
// workspace is not merely disallowed, it is unrepresentable.
//
// Every child table references workspaces.id DIRECTLY rather than
// workspace_companies.workspace_id. That is a real consequence of just-in-time
// capture: the first thing ever written may be a SOA category answered in chat,
// months before anyone types a VAT number, and a foreign key routed through the
// identity row would force an empty parent insert on that write path.
//
// The seven attribution columns repeat verbatim on all six child tables. They
// are columns rather than a single JSONB blob because provenance is queried and
// filtered ("show me everything the agent captured"), and because a NOT NULL
// provenance column makes an un-attributed fact impossible to insert — a
// constraint a JSON key cannot express.
var (
	WorkspaceCompanies = pg.NewTable("workspace_companies")
	CoWorkspaceID      = pg.Add(WorkspaceCompanies, pg.UUID("workspace_id").PrimaryKey().References(WorkspaceID, pg.OnDelete("CASCADE")))
	CoLegalName        = pg.Add(WorkspaceCompanies, pg.Text("legal_name").NotNull().Default("''"))
	CoVATNumber        = pg.Add(WorkspaceCompanies, pg.Text("vat_number").NotNull().Default("''"))
	CoFiscalCode       = pg.Add(WorkspaceCompanies, pg.Text("fiscal_code").NotNull().Default("''"))
	CoLegalForm        = pg.Add(WorkspaceCompanies, pg.Text("legal_form").NotNull().Default("''"))
	CoCCIAAOffice      = pg.Add(WorkspaceCompanies, pg.Text("cciaa_office").NotNull().Default("''"))
	CoCCIAANumber      = pg.Add(WorkspaceCompanies, pg.Text("cciaa_number").NotNull().Default("''"))
	CoCountry          = pg.Add(WorkspaceCompanies, pg.Text("country").NotNull().Default("''"))
	CoNUTS             = pg.Add(WorkspaceCompanies, pg.Text("nuts").NotNull().Default("''"))
	CoFoundedYear      = pg.Add(WorkspaceCompanies, pg.Integer("founded_year")) // nullable
	// CoAttribution is the per-FIELD provenance map, keyed by company.FieldKey.
	// It is JSONB rather than seven more column groups because the identity is a
	// fixed set of scalars whose provenance is read as a whole by the dossier UI
	// and never filtered on individually.
	CoAttribution = pg.Add(WorkspaceCompanies, pg.JSONB("attribution").NotNull().Default("'{}'::jsonb"))
	CoUpdatedAt   = pg.Add(WorkspaceCompanies, pg.Timestamp("updated_at", true).NotNull().Default("now()"))

	CompanySOACategories = pg.NewTable("company_soa_categories")
	SOAID                = pg.Add(CompanySOACategories, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	SOAWorkspaceID       = pg.Add(CompanySOACategories, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	SOACategoryCol       = pg.Add(CompanySOACategories, pg.Text("category").NotNull())
	SOAClassifica        = pg.Add(CompanySOACategories, pg.SmallInt("classifica").NotNull())
	SOAIssuedBy          = pg.Add(CompanySOACategories, pg.Text("issued_by").NotNull().Default("''"))
	SOAValidUntil        = pg.Add(CompanySOACategories, pg.Timestamp("valid_until", true)) // nullable
	// SOAVerifiedUntil is the triennial verification deadline. It lapses THREE
	// YEARS before valid_until and invalidates the attestation on its own, so a
	// schema that carried only valid_until would report a lapsed attestation as
	// valid for two more years.
	SOAVerifiedUntil      = pg.Add(CompanySOACategories, pg.Timestamp("verified_until", true)) // nullable
	SOAProvenance         = pg.Add(CompanySOACategories, pg.Text("provenance").NotNull())
	SOAConfidence         = pg.Add(CompanySOACategories, pg.DoublePrecision("confidence")) // nullable
	SOAStatedBy           = pg.Add(CompanySOACategories, pg.UUID("stated_by"))             // nullable
	SOAStatedAt           = pg.Add(CompanySOACategories, pg.Timestamp("stated_at", true).NotNull().Default("now()"))
	SOAPromptedBy         = pg.Add(CompanySOACategories, pg.Text("prompted_by").NotNull().Default("''"))
	SOAPromptedByTenderID = pg.Add(CompanySOACategories, pg.BigInt("prompted_by_tender_id")) // nullable, NO FK
	SOASourceNote         = pg.Add(CompanySOACategories, pg.Text("source_note").NotNull().Default("''"))

	CompanyCertifications = pg.NewTable("company_certifications")
	CertID                = pg.Add(CompanyCertifications, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	CertWorkspaceID       = pg.Add(CompanyCertifications, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	CertStandard          = pg.Add(CompanyCertifications, pg.Text("standard").NotNull())
	CertStandardRaw       = pg.Add(CompanyCertifications, pg.Text("standard_raw").NotNull().Default("''"))
	CertScope             = pg.Add(CompanyCertifications, pg.Text("scope").NotNull().Default("''"))
	CertIssuedBy          = pg.Add(CompanyCertifications, pg.Text("issued_by").NotNull().Default("''"))
	CertValidFrom         = pg.Add(CompanyCertifications, pg.Timestamp("valid_from", true))  // nullable
	CertValidUntil        = pg.Add(CompanyCertifications, pg.Timestamp("valid_until", true)) // nullable
	CertProvenance        = pg.Add(CompanyCertifications, pg.Text("provenance").NotNull())
	CertConfidence        = pg.Add(CompanyCertifications, pg.DoublePrecision("confidence")) // nullable
	CertStatedBy          = pg.Add(CompanyCertifications, pg.UUID("stated_by"))             // nullable
	CertStatedAt          = pg.Add(CompanyCertifications, pg.Timestamp("stated_at", true).NotNull().Default("now()"))
	CertPromptedBy        = pg.Add(CompanyCertifications, pg.Text("prompted_by").NotNull().Default("''"))
	CertPromptedByTender  = pg.Add(CompanyCertifications, pg.BigInt("prompted_by_tender_id")) // nullable, NO FK
	CertSourceNote        = pg.Add(CompanyCertifications, pg.Text("source_note").NotNull().Default("''"))

	CompanyFinancialYears = pg.NewTable("company_financial_years")
	FYID                  = pg.Add(CompanyFinancialYears, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	FYWorkspaceID         = pg.Add(CompanyFinancialYears, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	FYYear                = pg.Add(CompanyFinancialYears, pg.SmallInt("year").NotNull())
	FYTurnoverMinor       = pg.Add(CompanyFinancialYears, pg.BigInt("turnover_minor"))          // nullable
	FYSpecificTurnover    = pg.Add(CompanyFinancialYears, pg.BigInt("specific_turnover_minor")) // nullable
	FYCurrency            = pg.Add(CompanyFinancialYears, pg.Text("currency").NotNull().Default("'EUR'"))
	FYHeadcount           = pg.Add(CompanyFinancialYears, pg.Integer("headcount")) // nullable
	FYProvenance          = pg.Add(CompanyFinancialYears, pg.Text("provenance").NotNull())
	FYConfidence          = pg.Add(CompanyFinancialYears, pg.DoublePrecision("confidence")) // nullable
	FYStatedBy            = pg.Add(CompanyFinancialYears, pg.UUID("stated_by"))             // nullable
	FYStatedAt            = pg.Add(CompanyFinancialYears, pg.Timestamp("stated_at", true).NotNull().Default("now()"))
	FYPromptedBy          = pg.Add(CompanyFinancialYears, pg.Text("prompted_by").NotNull().Default("''"))
	FYPromptedByTender    = pg.Add(CompanyFinancialYears, pg.BigInt("prompted_by_tender_id")) // nullable, NO FK
	FYSourceNote          = pg.Add(CompanyFinancialYears, pg.Text("source_note").NotNull().Default("''"))

	CompanyPastContracts = pg.NewTable("company_past_contracts")
	PCID                 = pg.Add(CompanyPastContracts, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	PCWorkspaceID        = pg.Add(CompanyPastContracts, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	PCDescription        = pg.Add(CompanyPastContracts, pg.Text("description").NotNull().Default("''"))
	PCBuyerName          = pg.Add(CompanyPastContracts, pg.Text("buyer_name").NotNull().Default("''"))
	PCCPV                = pg.Add(CompanyPastContracts, pg.Text("cpv").NotNull().Default("''"))
	PCValueMinor         = pg.Add(CompanyPastContracts, pg.BigInt("value_minor")) // nullable
	PCCurrency           = pg.Add(CompanyPastContracts, pg.Text("currency").NotNull().Default("'EUR'"))
	PCStartedOn          = pg.Add(CompanyPastContracts, pg.Timestamp("started_on", true)) // nullable
	PCEndedOn            = pg.Add(CompanyPastContracts, pg.Timestamp("ended_on", true))   // nullable
	PCRole               = pg.Add(CompanyPastContracts, pg.Text("role").NotNull())
	// PCSharePct is the company's own share of an RTI/subcontracted contract. It
	// is nullable and NOT defaulted to 100: a shared role with no stated share
	// counts for nothing in the engine, which is the direction that stops a
	// dossier overstating its own track record.
	PCSharePct         = pg.Add(CompanyPastContracts, pg.DoublePrecision("share_pct")) // nullable
	PCProvenance       = pg.Add(CompanyPastContracts, pg.Text("provenance").NotNull())
	PCConfidence       = pg.Add(CompanyPastContracts, pg.DoublePrecision("confidence")) // nullable
	PCStatedBy         = pg.Add(CompanyPastContracts, pg.UUID("stated_by"))             // nullable
	PCStatedAt         = pg.Add(CompanyPastContracts, pg.Timestamp("stated_at", true).NotNull().Default("now()"))
	PCPromptedBy       = pg.Add(CompanyPastContracts, pg.Text("prompted_by").NotNull().Default("''"))
	PCPromptedByTender = pg.Add(CompanyPastContracts, pg.BigInt("prompted_by_tender_id")) // nullable, NO FK
	PCSourceNote       = pg.Add(CompanyPastContracts, pg.Text("source_note").NotNull().Default("''"))

	CompanyRegistrations = pg.NewTable("company_registrations")
	RegID                = pg.Add(CompanyRegistrations, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	RegWorkspaceID       = pg.Add(CompanyRegistrations, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	RegKind              = pg.Add(CompanyRegistrations, pg.Text("kind").NotNull())
	RegAuthority         = pg.Add(CompanyRegistrations, pg.Text("authority").NotNull().Default("''"))
	RegIdentifier        = pg.Add(CompanyRegistrations, pg.Text("identifier").NotNull().Default("''"))
	RegSection           = pg.Add(CompanyRegistrations, pg.Text("section").NotNull().Default("''"))
	RegValidFrom         = pg.Add(CompanyRegistrations, pg.Timestamp("valid_from", true))  // nullable
	RegValidUntil        = pg.Add(CompanyRegistrations, pg.Timestamp("valid_until", true)) // nullable
	RegProvenance        = pg.Add(CompanyRegistrations, pg.Text("provenance").NotNull())
	RegConfidence        = pg.Add(CompanyRegistrations, pg.DoublePrecision("confidence")) // nullable
	RegStatedBy          = pg.Add(CompanyRegistrations, pg.UUID("stated_by"))             // nullable
	RegStatedAt          = pg.Add(CompanyRegistrations, pg.Timestamp("stated_at", true).NotNull().Default("now()"))
	RegPromptedBy        = pg.Add(CompanyRegistrations, pg.Text("prompted_by").NotNull().Default("''"))
	RegPromptedByTender  = pg.Add(CompanyRegistrations, pg.BigInt("prompted_by_tender_id")) // nullable, NO FK
	RegSourceNote        = pg.Add(CompanyRegistrations, pg.Text("source_note").NotNull().Default("''"))

	// TenderRequirements holds participation requirements captured BY ONE
	// WORKSPACE for one tender — deliberately not corpus-shared. One workspace's
	// mis-extraction must not decide another workspace's go/no-go, and a
	// cross-tenant write path into shared tender data is a much larger security
	// surface than a duplicated extraction is a cost.
	TenderRequirements = pg.NewTable("tender_requirements")
	TReqID             = pg.Add(TenderRequirements, pg.UUID("id").PrimaryKey().Default("gen_random_uuid()"))
	TReqWorkspaceID    = pg.Add(TenderRequirements, pg.UUID("workspace_id").NotNull().References(WorkspaceID, pg.OnDelete("CASCADE")))
	// TReqTenderID carries NO foreign key: tenders.ingested_tenders is owned and
	// migrated by services/ingestion, the same rule bids.tender_id follows.
	TReqTenderID = pg.Add(TenderRequirements, pg.BigInt("tender_id").NotNull())
	// TReqLotRef is NOT NULL DEFAULT '' rather than nullable, for the reason the
	// ingestion service's 0010 migration records for
	// ingested_tender_award_criteria.lot_ref: PostgreSQL NULLs do not conflict in
	// a unique key, so a nullable column would permit unlimited duplicate
	// notice-level rows.
	TReqLotRef      = pg.Add(TenderRequirements, pg.Text("lot_ref").NotNull().Default("''"))
	TReqKind        = pg.Add(TenderRequirements, pg.Text("kind").NotNull())
	TReqPayload     = pg.Add(TenderRequirements, pg.JSONB("payload").NotNull().Default("'{}'::jsonb"))
	TReqText        = pg.Add(TenderRequirements, pg.Text("requirement_text").NotNull())
	TReqBlocking    = pg.Add(TenderRequirements, pg.Boolean("blocking").NotNull().Default("true"))
	TReqSource      = pg.Add(TenderRequirements, pg.Text("source").NotNull())
	TReqExcerpt     = pg.Add(TenderRequirements, pg.Text("excerpt").NotNull().Default("''"))
	TReqCitation    = pg.Add(TenderRequirements, pg.JSONB("citation")) // nullable
	TReqDedupeKey   = pg.Add(TenderRequirements, pg.Text("dedupe_key").NotNull())
	TReqConfirmedBy = pg.Add(TenderRequirements, pg.UUID("confirmed_by"))            // nullable: NULL = unconfirmed
	TReqConfirmedAt = pg.Add(TenderRequirements, pg.Timestamp("confirmed_at", true)) // nullable
	TReqCreatedAt   = pg.Add(TenderRequirements, pg.Timestamp("created_at", true).NotNull().Default("now()"))
	TReqUpdatedAt   = pg.Add(TenderRequirements, pg.Timestamp("updated_at", true).NotNull().Default("now()"))
)

type DBCompany struct {
	WorkspaceID string          `drop:"workspace_id"`
	LegalName   string          `drop:"legal_name"`
	VATNumber   string          `drop:"vat_number"`
	FiscalCode  string          `drop:"fiscal_code"`
	LegalForm   string          `drop:"legal_form"`
	CCIAAOffice string          `drop:"cciaa_office"`
	CCIAANumber string          `drop:"cciaa_number"`
	Country     string          `drop:"country"`
	NUTS        string          `drop:"nuts"`
	FoundedYear *int32          `drop:"founded_year"`
	Attribution json.RawMessage `drop:"attribution"`
	UpdatedAt   time.Time       `drop:"updated_at"`
}

type DBSOACategory struct {
	ID                 string     `drop:"id"`
	WorkspaceID        string     `drop:"workspace_id"`
	Category           string     `drop:"category"`
	Classifica         int16      `drop:"classifica"`
	IssuedBy           string     `drop:"issued_by"`
	ValidUntil         *time.Time `drop:"valid_until"`
	VerifiedUntil      *time.Time `drop:"verified_until"`
	Provenance         string     `drop:"provenance"`
	Confidence         *float64   `drop:"confidence"`
	StatedBy           *string    `drop:"stated_by"`
	StatedAt           time.Time  `drop:"stated_at"`
	PromptedBy         string     `drop:"prompted_by"`
	PromptedByTenderID *int64     `drop:"prompted_by_tender_id"`
	SourceNote         string     `drop:"source_note"`
}

type DBCertification struct {
	ID                 string     `drop:"id"`
	WorkspaceID        string     `drop:"workspace_id"`
	Standard           string     `drop:"standard"`
	StandardRaw        string     `drop:"standard_raw"`
	Scope              string     `drop:"scope"`
	IssuedBy           string     `drop:"issued_by"`
	ValidFrom          *time.Time `drop:"valid_from"`
	ValidUntil         *time.Time `drop:"valid_until"`
	Provenance         string     `drop:"provenance"`
	Confidence         *float64   `drop:"confidence"`
	StatedBy           *string    `drop:"stated_by"`
	StatedAt           time.Time  `drop:"stated_at"`
	PromptedBy         string     `drop:"prompted_by"`
	PromptedByTenderID *int64     `drop:"prompted_by_tender_id"`
	SourceNote         string     `drop:"source_note"`
}

type DBFinancialYear struct {
	ID                 string    `drop:"id"`
	WorkspaceID        string    `drop:"workspace_id"`
	Year               int16     `drop:"year"`
	TurnoverMinor      *int64    `drop:"turnover_minor"`
	SpecificTurnover   *int64    `drop:"specific_turnover_minor"`
	Currency           string    `drop:"currency"`
	Headcount          *int32    `drop:"headcount"`
	Provenance         string    `drop:"provenance"`
	Confidence         *float64  `drop:"confidence"`
	StatedBy           *string   `drop:"stated_by"`
	StatedAt           time.Time `drop:"stated_at"`
	PromptedBy         string    `drop:"prompted_by"`
	PromptedByTenderID *int64    `drop:"prompted_by_tender_id"`
	SourceNote         string    `drop:"source_note"`
}

type DBPastContract struct {
	ID                 string     `drop:"id"`
	WorkspaceID        string     `drop:"workspace_id"`
	Description        string     `drop:"description"`
	BuyerName          string     `drop:"buyer_name"`
	CPV                string     `drop:"cpv"`
	ValueMinor         *int64     `drop:"value_minor"`
	Currency           string     `drop:"currency"`
	StartedOn          *time.Time `drop:"started_on"`
	EndedOn            *time.Time `drop:"ended_on"`
	Role               string     `drop:"role"`
	SharePct           *float64   `drop:"share_pct"`
	Provenance         string     `drop:"provenance"`
	Confidence         *float64   `drop:"confidence"`
	StatedBy           *string    `drop:"stated_by"`
	StatedAt           time.Time  `drop:"stated_at"`
	PromptedBy         string     `drop:"prompted_by"`
	PromptedByTenderID *int64     `drop:"prompted_by_tender_id"`
	SourceNote         string     `drop:"source_note"`
}

type DBRegistration struct {
	ID                 string     `drop:"id"`
	WorkspaceID        string     `drop:"workspace_id"`
	Kind               string     `drop:"kind"`
	Authority          string     `drop:"authority"`
	Identifier         string     `drop:"identifier"`
	Section            string     `drop:"section"`
	ValidFrom          *time.Time `drop:"valid_from"`
	ValidUntil         *time.Time `drop:"valid_until"`
	Provenance         string     `drop:"provenance"`
	Confidence         *float64   `drop:"confidence"`
	StatedBy           *string    `drop:"stated_by"`
	StatedAt           time.Time  `drop:"stated_at"`
	PromptedBy         string     `drop:"prompted_by"`
	PromptedByTenderID *int64     `drop:"prompted_by_tender_id"`
	SourceNote         string     `drop:"source_note"`
}

type DBTenderRequirement struct {
	ID          string           `drop:"id"`
	WorkspaceID string           `drop:"workspace_id"`
	TenderID    int64            `drop:"tender_id"`
	LotRef      string           `drop:"lot_ref"`
	Kind        string           `drop:"kind"`
	Payload     json.RawMessage  `drop:"payload"`
	Text        string           `drop:"requirement_text"`
	Blocking    bool             `drop:"blocking"`
	Source      string           `drop:"source"`
	Excerpt     string           `drop:"excerpt"`
	Citation    *json.RawMessage `drop:"citation"`
	DedupeKey   string           `drop:"dedupe_key"`
	ConfirmedBy *string          `drop:"confirmed_by"`
	ConfirmedAt *time.Time       `drop:"confirmed_at"`
	CreatedAt   time.Time        `drop:"created_at"`
	UpdatedAt   time.Time        `drop:"updated_at"`
}
