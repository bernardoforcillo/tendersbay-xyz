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
