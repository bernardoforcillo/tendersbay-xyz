// Package workspace implements multi-user workspaces: RBAC (one role per
// member), membership management, and invitations.
//
// The authorization engine is github.com/bernardoforcillo/authlayer — scope for
// containers, members, roles and the privilege-escalation guard, invite for
// email invitations and shareable links, both persisted by authlayer's own
// drops store. What this package owns is everything around that: the product's
// permission vocabulary (see permissions.go), slug rules, localized invitation
// mail, and the sentinel errors connectapi maps to ConnectRPC codes.
//
// The service's method set is unchanged from the hand-rolled implementation it
// replaced, deliberately: the transport, the proto and the client all speak the
// same shapes they always did.
package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/invite"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

// ── Entities ────────────────────────────────────────────────────────────────

// Workspace is the scope container. The id, owner and timestamps come from
// scope.ContainerBase and are stamped by the engine on create; Name and Slug
// are this product's. The slug's "unique" tag is what the drops store turns
// into the UNIQUE constraint behind ErrSlugTaken.
type Workspace struct {
	scope.ContainerBase
	Name string `drop:"name"`
	Slug string `drop:"slug,unique"`
}

// Member is a workspace membership. It carries nothing beyond scope.MemberBase
// — the workspace id, the user id, the role KEY, and when they joined.
type Member struct {
	scope.MemberBase
}

// Role is the product's view of a role: authlayer keys roles by a string that
// is unique per workspace, and that key is what travels the wire as the role
// id. CreatedAt is absent because scope.RoleView does not carry it — the
// registry's code-defined roles have no row to have been created.
type Role struct {
	ID          string // authlayer's role key
	WorkspaceID string
	Name        string
	Permissions Permission
	// IsDefault marks a code-defined role (owner/admin/member). Such a role
	// exists in every workspace without a stored row and cannot be edited or
	// deleted, which is what the old is_default column effectively guarded too.
	IsDefault bool
}

// Membership is a member together with the permissions it holds — the unit a
// permission check loads.
//
// LoadMembership fills Member.ContainerID, Member.UserID and Role.Permissions,
// and leaves Role.ID and Role.Name empty: resolving the role's key and label
// costs a second read that no caller of this method needs, and ListMembers is
// the API that returns whole role rows.
type Membership struct {
	Member Member
	Role   Role
}

// MemberView is a member enriched with its role and user profile for the API.
type MemberView struct {
	Member Member
	Role   Role
	User   auth.User
}

type EmailInvitation struct {
	ID          string
	WorkspaceID string
	Email       string
	RoleID      string
	InvitedBy   string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

type InviteLink struct {
	ID          string
	WorkspaceID string
	Code        string
	RoleID      string
	CreatedBy   string
	MaxUses     int32
	UseCount    int32
	ExpiresAt   *time.Time
	Revoked     bool
	CreatedAt   time.Time
}

// InvitePreview / LinkPreview are unauthenticated previews shown before joining.
type InvitePreview struct {
	WorkspaceName string
	RoleName      string
	Email         string
	Valid         bool
}

type LinkPreview struct {
	WorkspaceName string
	RoleName      string
	Valid         bool
}

// ── Sentinel errors ─────────────────────────────────────────────────────────
//
// These are the vocabulary connectapi.toConnectError maps, kept stable across
// the authlayer migration: mapLibraryError translates every scope and invite
// sentinel into one of them.
var (
	ErrWorkspaceNotFound   = errors.New("workspace not found")
	ErrNotMember           = errors.New("not a member of this workspace")
	ErrForbidden           = errors.New("insufficient permissions")
	ErrPrivilegeEscalation = errors.New("cannot grant permissions you do not have")
	ErrRoleNotFound        = errors.New("role not found")
	ErrRoleInUse           = errors.New("role is assigned to members")
	ErrDefaultRole         = errors.New("cannot delete the default role")
	ErrLastOwner           = errors.New("cannot remove or demote the workspace owner")
	ErrOwnerOnly           = errors.New("only the workspace owner may do this")
	ErrAlreadyMember       = errors.New("user is already a member")
	ErrInviteInvalid       = errors.New("invitation invalid")
	ErrInviteExpired       = errors.New("invitation expired")
	ErrLinkInvalid         = errors.New("invite link invalid or revoked")
	ErrLinkExpired         = errors.New("invite link expired")
	ErrLinkExhausted       = errors.New("invite link has reached its maximum uses")
	ErrSlugTaken           = errors.New("workspace slug already taken")
	ErrRoleKeyTaken        = errors.New("a role with this name already exists")
)

// ── Ports ───────────────────────────────────────────────────────────────────

// Store is authlayer's scope persistence port, typed for this product.
// *dropsstore.Store[Workspace, Member] satisfies it in production and
// memory.New[Workspace, Member]() in tests.
type Store = scope.Store[Workspace, Member]

// InviteStore is authlayer's invitation persistence port.
type InviteStore = invite.Store

// Repository is the handful of workspace queries authlayer's scope store does
// not cover, because they are about this product's own columns rather than
// about containment: looking a workspace up by slug, renaming it, deleting it.
type Repository interface {
	FindBySlug(ctx context.Context, slug string) (Workspace, error)
	UpdateNameSlug(ctx context.Context, id, name, slug string) (Workspace, error)
	Delete(ctx context.Context, id string) error
}

// UserLookup is the narrow slice of the user profile store this service needs.
// FindByIDs is what a member listing uses: one query for the whole page rather
// than one per row.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (auth.User, error)
	FindByEmail(ctx context.Context, email string) (auth.User, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]auth.User, error)
}

// EmailSender delivers workspace invitation emails.
type EmailSender interface {
	SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error
}

// ── Service ─────────────────────────────────────────────────────────────────

type Config struct {
	AppBaseURL   string
	InviteExpiry time.Duration
}

type scopeService = scope.Service[Workspace, Member, *Workspace, *Member]
type inviteService = invite.Service[Workspace, Member, *Workspace, *Member]

type Service struct {
	sc    *scopeService
	inv   *inviteService
	store Store
	repo  Repository
	users UserLookup
	email EmailSender
	cfg   Config
}

// NewService wires the authlayer engines and this product's own dependencies.
// ac comes from NewAccess and is shared process-wide.
func NewService(
	ac *access.Access,
	store Store,
	inviteStore InviteStore,
	repo Repository,
	users UserLookup,
	email EmailSender,
	cfg Config,
) *Service {
	sc := scope.New[Workspace, Member](ac, store,
		scope.WithContainerResource(ResourceWorkspace),
	)
	inv := invite.New(sc, inviteStore, invite.WithInviteExpiry(cfg.InviteExpiry))
	return &Service{sc: sc, inv: inv, store: store, repo: repo, users: users, email: email, cfg: cfg}
}

// Scope exposes the underlying engine for the adapters that need to ask it a
// question directly — the workbench domain's workspace-standing lookup, and any
// future query guard built from scope.PermissionGuard.
func (s *Service) Scope() *scopeService { return s.sc }

// actor builds the context authlayer's engines read: who is asking, and which
// workspace they are asking about.
func actor(ctx context.Context, userID, workspaceID string) context.Context {
	return scope.WithScope(scope.WithSubject(ctx, userID), workspaceID)
}

// scopeErrors is this domain's vocabulary for authlayer's scope sentinels.
// The translation itself is rbac.Errors.Translate; what is declared here is
// only which of this package's errors each condition means.
var scopeErrors = rbac.Errors{
	NotFound:            ErrWorkspaceNotFound,
	NotMember:           ErrNotMember,
	Forbidden:           ErrForbidden,
	PrivilegeEscalation: ErrPrivilegeEscalation,
	RoleNotFound:        ErrRoleNotFound,
	RoleInUse:           ErrRoleInUse,
	DefaultRole:         ErrDefaultRole,
	LastOwner:           ErrLastOwner,
	OwnerOnly:           ErrOwnerOnly,
	AlreadyMember:       ErrAlreadyMember,
	RoleKeyTaken:        ErrRoleKeyTaken,
	// The only unique constraint a workspace can violate is its slug.
	Conflict: ErrSlugTaken,
}

// mapLibraryError translates authlayer's sentinels into this package's, so
// connectapi.toConnectError keeps its existing switch and the wire status codes
// do not move. Anything unrecognised is passed through untouched and surfaces
// as CodeInternal, which is the correct answer for a store or transport failure.
func mapLibraryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, invite.ErrInviteNotFound):
		return ErrInviteInvalid
	case errors.Is(err, invite.ErrInviteExpired):
		return ErrInviteExpired
	case errors.Is(err, invite.ErrLinkNotFound), errors.Is(err, invite.ErrLinkRevoked):
		return ErrLinkInvalid
	case errors.Is(err, invite.ErrLinkExpired):
		return ErrLinkExpired
	case errors.Is(err, invite.ErrLinkExhausted):
		return ErrLinkExhausted
	default:
		return scopeErrors.Translate(err)
	}
}

// ── Workspace lifecycle ─────────────────────────────────────────────────────

// CreateWorkspace creates a workspace owned by userID, who becomes its first
// member with the owner role. No role rows are seeded: owner, admin and member
// are code-defined and exist in every workspace (see NewAccess).
func (s *Service) CreateWorkspace(ctx context.Context, userID, name, slug string) (Workspace, error) {
	slug = normalizeSlug(slug, name)
	ws, err := s.sc.CreateContainer(actor(ctx, userID, ""), Workspace{Name: name, Slug: slug})
	if err != nil {
		return Workspace{}, mapLibraryError(err)
	}
	return ws, nil
}

func (s *Service) ListMyWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	wss, err := s.store.ListUserContainers(ctx, userID)
	if err != nil {
		return nil, mapLibraryError(err)
	}
	return wss, nil
}

// GetWorkspace returns the workspace and the caller's effective permissions.
// Membership alone is the read right, so there is no grant to check here — a
// caller with no standing gets ErrNotMember from Standing itself.
func (s *Service) GetWorkspace(ctx context.Context, userID, workspaceID string) (Workspace, Permission, error) {
	perms, elevated, err := s.sc.Standing(ctx, workspaceID, userID)
	if err != nil {
		return Workspace{}, 0, mapLibraryError(err)
	}
	ws, err := s.sc.Container(ctx, workspaceID)
	if err != nil {
		return Workspace{}, 0, mapLibraryError(err)
	}
	return ws, maskOf(perms, elevated), nil
}

func (s *Service) UpdateWorkspace(ctx context.Context, userID, workspaceID, name, slug string) (Workspace, error) {
	if err := s.sc.Authorize(actor(ctx, userID, workspaceID), ResourceWorkspace, scope.ActionUpdate); err != nil {
		return Workspace{}, mapLibraryError(err)
	}
	slug = normalizeSlug(slug, name)
	if existing, err := s.repo.FindBySlug(ctx, slug); err == nil {
		if existing.ID != workspaceID {
			return Workspace{}, ErrSlugTaken
		}
	} else if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}
	return s.repo.UpdateNameSlug(ctx, workspaceID, name, slug)
}

// DeleteWorkspace is owner-only. It is not expressed as a grant — see
// permissionGrants' note on why "workspace" declares only update.
func (s *Service) DeleteWorkspace(ctx context.Context, userID, workspaceID string) error {
	if _, err := s.requireOwner(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.Delete(ctx, workspaceID)
}

func (s *Service) TransferOwnership(ctx context.Context, userID, workspaceID, newOwnerUserID string) error {
	return mapLibraryError(s.sc.TransferOwnership(actor(ctx, userID, workspaceID), newOwnerUserID))
}

func (s *Service) LeaveWorkspace(ctx context.Context, userID, workspaceID string) error {
	return mapLibraryError(s.sc.LeaveContainer(actor(ctx, userID, workspaceID)))
}

func (s *Service) requireOwner(ctx context.Context, workspaceID, userID string) (Workspace, error) {
	ws, err := s.sc.Container(ctx, workspaceID)
	if err != nil {
		return Workspace{}, mapLibraryError(err)
	}
	if ws.OwnerID != userID {
		return Workspace{}, ErrOwnerOnly
	}
	return ws, nil
}

// ── Membership checks ───────────────────────────────────────────────────────

// LoadMembership reports a caller's standing in a workspace. It is the port the
// agent, company, client-profile and tender-search paths gate on, and it stays
// a single read: scope.Standing resolves owner bypass, the member row and the
// role's grants in one ladder.
func (s *Service) LoadMembership(ctx context.Context, workspaceID, userID string) (Membership, error) {
	perms, elevated, err := s.sc.Standing(ctx, workspaceID, userID)
	if err != nil {
		return Membership{}, mapLibraryError(err)
	}
	m := Membership{Role: Role{WorkspaceID: workspaceID, Permissions: maskOf(perms, elevated)}}
	m.Member.ContainerID = workspaceID
	m.Member.UserID = userID
	return m, nil
}

// Standing is a caller's position in a workspace, for the domains that gate on
// it without being part of it — today the workbench domain's coarse
// (workspace-level) access layer.
type Standing struct {
	WorkspaceName string
	IsMember      bool
	IsOwner       bool
	Permissions   Permission
}

// Standing reports userID's position in a workspace. A non-member is not an
// error: the caller gets IsMember false and decides what that means, which is
// what lets the workbench domain distinguish "no such workspace" from "you may
// not see this workbench".
func (s *Service) Standing(ctx context.Context, workspaceID, userID string) (Standing, error) {
	ws, err := s.sc.Container(ctx, workspaceID)
	if err != nil {
		return Standing{}, mapLibraryError(err)
	}
	out := Standing{WorkspaceName: ws.Name, IsOwner: ws.OwnerID == userID}
	perms, elevated, err := s.sc.Standing(ctx, workspaceID, userID)
	if errors.Is(err, scope.ErrNotMember) {
		return out, nil
	}
	if err != nil {
		return Standing{}, mapLibraryError(err)
	}
	out.IsMember = true
	out.Permissions = maskOf(perms, elevated)
	return out, nil
}

// ── Members ─────────────────────────────────────────────────────────────────

func (s *Service) ListMembers(ctx context.Context, userID, workspaceID string) ([]MemberView, error) {
	if _, err := s.LoadMembership(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	ac := actor(ctx, userID, workspaceID)
	members, err := s.sc.ListMembers(ac)
	if err != nil {
		return nil, mapLibraryError(err)
	}
	roles, err := s.rolesByKey(ac, workspaceID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	profiles, err := s.users.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	views := make([]MemberView, 0, len(members))
	for _, m := range members {
		u, ok := profiles[m.UserID]
		if !ok {
			// A membership pointing at a user that is gone. The foreign key
			// makes it impossible, so it means the two tables disagree — which
			// is worth failing on rather than rendering as a blank name.
			return nil, auth.ErrNotFound
		}
		views = append(views, MemberView{Member: m, Role: roles[m.RoleKey], User: u})
	}
	return views, nil
}

func (s *Service) ChangeMemberRole(ctx context.Context, userID, workspaceID, targetUserID, roleID string) (MemberView, error) {
	ac := actor(ctx, userID, workspaceID)
	if err := s.sc.ChangeMemberRole(ac, targetUserID, roleID); err != nil {
		return MemberView{}, mapLibraryError(err)
	}
	role, err := s.role(ac, workspaceID, roleID)
	if err != nil {
		return MemberView{}, err
	}
	u, err := s.users.FindByID(ctx, targetUserID)
	if err != nil {
		return MemberView{}, err
	}
	view := MemberView{Role: role, User: u}
	view.Member.ContainerID = workspaceID
	view.Member.UserID = targetUserID
	view.Member.RoleKey = roleID
	return view, nil
}

func (s *Service) RemoveMember(ctx context.Context, userID, workspaceID, targetUserID string) error {
	return mapLibraryError(s.sc.RemoveMember(actor(ctx, userID, workspaceID), targetUserID))
}

// ── Roles ───────────────────────────────────────────────────────────────────

func (s *Service) ListRoles(ctx context.Context, userID, workspaceID string) ([]Role, error) {
	if _, err := s.LoadMembership(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	views, err := s.sc.ListRoles(actor(ctx, userID, workspaceID))
	if err != nil {
		return nil, mapLibraryError(err)
	}
	out := make([]Role, len(views))
	for i, v := range views {
		out[i] = roleFromView(workspaceID, v)
	}
	return out, nil
}

func (s *Service) CreateRole(ctx context.Context, userID, workspaceID, name string, perms Permission) (Role, error) {
	view, err := s.sc.CreateRole(actor(ctx, userID, workspaceID), rbac.RoleKey(name), name, grantsFor(perms))
	if err != nil {
		return Role{}, mapLibraryError(err)
	}
	return roleFromView(workspaceID, view), nil
}

func (s *Service) UpdateRole(ctx context.Context, userID, workspaceID, roleID, name string, perms Permission) (Role, error) {
	view, err := s.sc.UpdateRole(actor(ctx, userID, workspaceID), roleID, name, grantsFor(perms))
	if err != nil {
		return Role{}, mapLibraryError(err)
	}
	return roleFromView(workspaceID, view), nil
}

func (s *Service) DeleteRole(ctx context.Context, userID, workspaceID, roleID string) error {
	return mapLibraryError(s.sc.DeleteRole(actor(ctx, userID, workspaceID), roleID))
}

// defaultRoleNames are the labels the client shows for the code-defined roles.
// authlayer reports a code-defined role's Name as its key, because a role built
// in code carries no separate label; the product has always shown these three
// capitalized, and a role list reading "owner / admin / member" would be a
// visible regression from a migration that is supposed to be invisible.
var defaultRoleNames = map[string]string{
	RoleOwner:  "Owner",
	RoleAdmin:  "Admin",
	RoleMember: "Member",
}

func roleFromView(workspaceID string, v scope.RoleView) Role {
	name := v.Name
	if v.IsDefault {
		if label, ok := defaultRoleNames[v.Key]; ok {
			name = label
		}
	}
	return Role{
		ID:          v.Key,
		WorkspaceID: workspaceID,
		Name:        name,
		Permissions: maskOf(v.Permissions, v.Permissions.IsFull()),
		IsDefault:   v.IsDefault,
	}
}

func (s *Service) rolesByKey(ctx context.Context, workspaceID string) (map[string]Role, error) {
	views, err := s.sc.ListRoles(ctx)
	if err != nil {
		return nil, mapLibraryError(err)
	}
	out := make(map[string]Role, len(views))
	for _, v := range views {
		out[v.Key] = roleFromView(workspaceID, v)
	}
	return out, nil
}

func (s *Service) role(ctx context.Context, workspaceID, key string) (Role, error) {
	byKey, err := s.rolesByKey(ctx, workspaceID)
	if err != nil {
		return Role{}, err
	}
	r, ok := byKey[key]
	if !ok {
		return Role{}, ErrRoleNotFound
	}
	return r, nil
}

// ── Email invitations ───────────────────────────────────────────────────────

func (s *Service) InviteByEmail(ctx context.Context, userID, workspaceID, email, roleID, locale string) (EmailInvitation, error) {
	email = auth.NormalizeEmail(email)
	ac := actor(ctx, userID, workspaceID)
	// Refusing to invite an existing member is this product's rule, not
	// authlayer's — the library is happy to mint an invitation that would be a
	// no-op on acceptance.
	if u, err := s.users.FindByEmail(ctx, email); err == nil {
		if _, err := s.store.FindMember(ctx, workspaceID, u.ID); err == nil {
			return EmailInvitation{}, ErrAlreadyMember
		}
	}
	stored, plain, err := s.inv.InviteByEmail(ac, email, roleID)
	if err != nil {
		return EmailInvitation{}, mapLibraryError(err)
	}
	ws, err := s.sc.Container(ctx, workspaceID)
	if err != nil {
		return EmailInvitation{}, mapLibraryError(err)
	}
	inviterName := ws.Name
	if u, err := s.users.FindByID(ctx, userID); err == nil {
		inviterName = u.DisplayName
	}
	link := s.cfg.AppBaseURL + "/" + locale + "/workspace/accept-invite?token=" + plain
	if err := s.email.SendWorkspaceInvite(ctx, email, ws.Name, inviterName, link); err != nil {
		return EmailInvitation{}, err
	}
	return emailInvitationOf(stored), nil
}

func (s *Service) ListEmailInvitations(ctx context.Context, userID, workspaceID string) ([]EmailInvitation, error) {
	invs, err := s.inv.ListInvites(actor(ctx, userID, workspaceID))
	if err != nil {
		return nil, mapLibraryError(err)
	}
	out := make([]EmailInvitation, len(invs))
	for i, inv := range invs {
		out[i] = emailInvitationOf(inv)
	}
	return out, nil
}

func (s *Service) RevokeEmailInvitation(ctx context.Context, userID, workspaceID, invitationID string) error {
	return mapLibraryError(s.inv.RevokeInvite(actor(ctx, userID, workspaceID), invitationID))
}

func (s *Service) AcceptEmailInvite(ctx context.Context, userID, token string) (Workspace, error) {
	ws, err := s.inv.AcceptInvite(scope.WithSubject(ctx, userID), token)
	if err != nil {
		return Workspace{}, mapLibraryError(err)
	}
	return ws, nil
}

func (s *Service) PreviewEmailInvite(ctx context.Context, token string) (InvitePreview, error) {
	p, err := s.inv.PreviewInvite(ctx, token)
	if errors.Is(err, invite.ErrInviteNotFound) {
		return InvitePreview{Valid: false}, nil
	}
	if err != nil {
		return InvitePreview{}, mapLibraryError(err)
	}
	if !p.Valid {
		return InvitePreview{Email: p.Email, Valid: false}, nil
	}
	name, roleName, err := s.previewNames(ctx, p.ContainerID, p.RoleKey)
	if err != nil {
		return InvitePreview{}, err
	}
	return InvitePreview{WorkspaceName: name, RoleName: roleName, Email: p.Email, Valid: true}, nil
}

// ── Invite links ────────────────────────────────────────────────────────────

func (s *Service) CreateInviteLink(ctx context.Context, userID, workspaceID, roleID string, maxUses int32, expiresAt *time.Time) (InviteLink, error) {
	if maxUses < 0 {
		maxUses = 0
	}
	l, _, err := s.inv.CreateLink(actor(ctx, userID, workspaceID), roleID, int(maxUses), expiresAt)
	if err != nil {
		return InviteLink{}, mapLibraryError(err)
	}
	return inviteLinkOf(l), nil
}

func (s *Service) ListInviteLinks(ctx context.Context, userID, workspaceID string) ([]InviteLink, error) {
	links, err := s.inv.ListLinks(actor(ctx, userID, workspaceID))
	if err != nil {
		return nil, mapLibraryError(err)
	}
	out := make([]InviteLink, len(links))
	for i, l := range links {
		out[i] = inviteLinkOf(l)
	}
	return out, nil
}

func (s *Service) RevokeInviteLink(ctx context.Context, userID, workspaceID, linkID string) error {
	return mapLibraryError(s.inv.RevokeLink(actor(ctx, userID, workspaceID), linkID))
}

func (s *Service) PreviewInviteLink(ctx context.Context, code string) (LinkPreview, error) {
	p, err := s.inv.PreviewLink(ctx, code)
	if errors.Is(err, invite.ErrLinkNotFound) {
		return LinkPreview{Valid: false}, nil
	}
	if err != nil {
		return LinkPreview{}, mapLibraryError(err)
	}
	if !p.Valid {
		return LinkPreview{Valid: false}, nil
	}
	name, roleName, err := s.previewNames(ctx, p.ContainerID, p.RoleKey)
	if err != nil {
		return LinkPreview{}, err
	}
	return LinkPreview{WorkspaceName: name, RoleName: roleName, Valid: true}, nil
}

// PurgeExpiredInvites deletes every email invitation and invite link that
// expired before the given instant, and reports how many rows went. Same
// reasoning as auth.Service.PurgeExpired: an expired invitation is refused on
// redemption but never removed, so the table only grew.
func (s *Service) PurgeExpiredInvites(ctx context.Context, before time.Time) (int, error) {
	return s.inv.PurgeExpired(ctx, before)
}

func (s *Service) JoinViaInviteLink(ctx context.Context, userID, code string) (Workspace, error) {
	ws, err := s.inv.JoinViaLink(scope.WithSubject(ctx, userID), code)
	if err != nil {
		return Workspace{}, mapLibraryError(err)
	}
	return ws, nil
}

// previewNames resolves the workspace and role labels an unauthenticated
// preview shows. It reads the role registry directly rather than through
// ListRoles' authorization, because a preview is by definition served to
// someone who is not yet a member.
func (s *Service) previewNames(ctx context.Context, workspaceID, key string) (workspaceName, roleName string, err error) {
	ws, err := s.sc.Container(ctx, workspaceID)
	if err != nil {
		return "", "", mapLibraryError(err)
	}
	roleName = key
	if rec, err := s.store.FindRole(ctx, workspaceID, key); err == nil && rec.Name != "" {
		roleName = rec.Name
	}
	return ws.Name, roleName, nil
}

func emailInvitationOf(inv invite.EmailInvite) EmailInvitation {
	return EmailInvitation{
		ID:          inv.ID,
		WorkspaceID: inv.ContainerID,
		Email:       inv.Email,
		RoleID:      inv.RoleKey,
		InvitedBy:   inv.InvitedBy,
		ExpiresAt:   inv.ExpiresAt,
		CreatedAt:   inv.CreatedAt,
	}
}

func inviteLinkOf(l invite.Link) InviteLink {
	return InviteLink{
		ID:          l.ID,
		WorkspaceID: l.ContainerID,
		Code:        l.Code,
		RoleID:      l.RoleKey,
		CreatedBy:   l.CreatedBy,
		MaxUses:     int32(l.MaxUses),
		UseCount:    int32(l.UseCount),
		ExpiresAt:   l.ExpiresAt,
		Revoked:     l.RevokedAt != nil,
		CreatedAt:   l.CreatedAt,
	}
}
