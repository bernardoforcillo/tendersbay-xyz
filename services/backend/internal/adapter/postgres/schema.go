package postgres

import (
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
