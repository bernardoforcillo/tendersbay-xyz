// Package workspace implements multi-user workspaces with Discord-like bitwise
// RBAC (one role per member), membership management, and invitations. A user may
// belong to many workspaces; each membership carries exactly one role whose
// permission bitmask decides what the member may do. The workspace owner and any
// role bearing the ADMINISTRATOR bit bypass permission checks.
package workspace

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// ── Permissions ─────────────────────────────────────────────────────────────

// Permission is a bitmask of workspace capabilities. New capabilities take the
// next free bit; bits 1<<20 and up are reserved for the future workbench so it
// ships without a schema change.
type Permission uint64

const (
	PermViewWorkspace   Permission = 1 << 0 // see the workspace (the default role)
	PermManageWorkspace Permission = 1 << 1 // rename / change slug
	PermManageMembers   Permission = 1 << 2 // change member roles, remove members
	PermManageRoles     Permission = 1 << 3 // create / update / delete roles
	PermCreateInvite    Permission = 1 << 4 // create email invites and invite links
	PermManageInvites   Permission = 1 << 5 // list / revoke invites and links
	PermAdministrator   Permission = 1 << 6 // bypass all non-owner-only checks

	// 1<<20.. — workbench feature bits (see internal/core/workbench).
	PermViewWorkbenches   Permission = 1 << 20 // see shared workbenches in the workspace
	PermCreateWorkbench   Permission = 1 << 21 // create new workbenches
	PermManageWorkbenches Permission = 1 << 22 // admin over all workbenches (bypass per-workbench ACL)
)

// permAdminRole is every currently-defined bit; the seeded "Admin" role and the
// workspace owner both hold this mask.
const permAdminRole = PermViewWorkspace | PermManageWorkspace | PermManageMembers |
	PermManageRoles | PermCreateInvite | PermManageInvites | PermAdministrator |
	PermViewWorkbenches | PermCreateWorkbench | PermManageWorkbenches

// Has reports whether p contains every bit in need.
func (p Permission) Has(need Permission) bool { return p&need == need }

// subsetOf reports whether every bit in p is also set in other — i.e. p grants
// nothing other doesn't already have. Used for the privilege-escalation guard.
func (p Permission) subsetOf(other Permission) bool { return p&^other == 0 }

// ── Entities ────────────────────────────────────────────────────────────────

type Workspace struct {
	ID        string
	Name      string
	Slug      string
	OwnerID   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Role struct {
	ID          string
	WorkspaceID string
	Name        string
	Permissions Permission
	IsDefault   bool
	CreatedAt   time.Time
}

type Member struct {
	WorkspaceID string
	UserID      string
	RoleID      string
	JoinedAt    time.Time
}

// Membership is a member together with the role it holds — the unit the
// authorizer loads to decide a request.
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
	TokenHash   string
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
)

// ── Ports ───────────────────────────────────────────────────────────────────

type WorkspaceRepository interface {
	Create(ctx context.Context, w Workspace) (Workspace, error)
	FindByID(ctx context.Context, id string) (Workspace, error)
	FindBySlug(ctx context.Context, slug string) (Workspace, error)
	ListByUserID(ctx context.Context, userID string) ([]Workspace, error)
	Update(ctx context.Context, id, name, slug string) (Workspace, error)
	UpdateOwner(ctx context.Context, id, newOwnerID string) error
	Delete(ctx context.Context, id string) error
}

type RoleRepository interface {
	Create(ctx context.Context, r Role) (Role, error)
	FindByID(ctx context.Context, id string) (Role, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Role, error)
	Update(ctx context.Context, id, name string, perms Permission) (Role, error)
	Delete(ctx context.Context, id string) error
	CountMembersUsing(ctx context.Context, roleID string) (int64, error)
}

type MemberRepository interface {
	Add(ctx context.Context, m Member) (Member, error)
	Find(ctx context.Context, workspaceID, userID string) (Member, error)
	LoadMembership(ctx context.Context, workspaceID, userID string) (Membership, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Member, error)
	UpdateRole(ctx context.Context, workspaceID, userID, roleID string) error
	Remove(ctx context.Context, workspaceID, userID string) error
	CountByWorkspace(ctx context.Context, workspaceID string) (int64, error)
}

type EmailInvitationRepository interface {
	Create(ctx context.Context, inv EmailInvitation) (EmailInvitation, error)
	FindByTokenHash(ctx context.Context, hash string) (EmailInvitation, error)
	FindByID(ctx context.Context, id string) (EmailInvitation, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]EmailInvitation, error)
	Delete(ctx context.Context, id string) error
	DeleteByWorkspaceEmail(ctx context.Context, workspaceID, email string) error
}

type InviteLinkRepository interface {
	Create(ctx context.Context, l InviteLink) (InviteLink, error)
	FindByCode(ctx context.Context, code string) (InviteLink, error)
	FindByID(ctx context.Context, id string) (InviteLink, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]InviteLink, error)
	IncrementUse(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
}

// UserLookup is the narrow slice of the auth user store the workspace service
// needs; the existing postgres UserRepo satisfies it.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (auth.User, error)
	FindByEmail(ctx context.Context, email string) (auth.User, error)
}

// EmailSender delivers workspace invitation emails.
type EmailSender interface {
	SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error
}

// Repos is the set of tx-scoped repositories handed to a UnitOfWork closure.
type Repos struct {
	Workspaces WorkspaceRepository
	Roles      RoleRepository
	Members    MemberRepository
	EmailInvs  EmailInvitationRepository
	Links      InviteLinkRepository
}

// UnitOfWork runs fn inside a single database transaction, providing tx-scoped
// repositories so multi-row writes commit or roll back atomically.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(Repos) error) error
}

// ── Service ─────────────────────────────────────────────────────────────────

type Config struct {
	AppBaseURL   string
	InviteExpiry time.Duration
}

type Service struct {
	workspaces WorkspaceRepository
	roles      RoleRepository
	members    MemberRepository
	emailInvs  EmailInvitationRepository
	links      InviteLinkRepository
	users      UserLookup
	email      EmailSender
	uow        UnitOfWork
	cfg        Config
}

func NewService(
	workspaces WorkspaceRepository,
	roles RoleRepository,
	members MemberRepository,
	emailInvs EmailInvitationRepository,
	links InviteLinkRepository,
	users UserLookup,
	email EmailSender,
	uow UnitOfWork,
	cfg Config,
) *Service {
	return &Service{
		workspaces: workspaces,
		roles:      roles,
		members:    members,
		emailInvs:  emailInvs,
		links:      links,
		users:      users,
		email:      email,
		uow:        uow,
		cfg:        cfg,
	}
}

// authz is the outcome of an authorization check.
type authz struct {
	ws       Workspace
	role     Role       // caller's role (zero value for the owner)
	perms    Permission // effective permissions (all bits for owner/administrator)
	elevated bool       // owner or administrator — bypasses the subset guard
}

// authorize loads the caller's workspace + membership and verifies the required
// permission bit. The owner bypasses everything; an ADMINISTRATOR role bypasses
// non-owner-only checks.
func (s *Service) authorize(ctx context.Context, workspaceID, userID string, need Permission) (authz, error) {
	ws, err := s.workspaces.FindByID(ctx, workspaceID)
	if err != nil {
		return authz{}, err
	}
	if ws.OwnerID == userID {
		return authz{ws: ws, perms: permAdminRole, elevated: true}, nil
	}
	m, err := s.members.LoadMembership(ctx, workspaceID, userID)
	if err != nil {
		return authz{}, err
	}
	perms := m.Role.Permissions
	elevated := perms.Has(PermAdministrator)
	if !elevated && !perms.Has(need) {
		return authz{}, ErrForbidden
	}
	return authz{ws: ws, role: m.Role, perms: perms, elevated: elevated}, nil
}

// requireOwner asserts the caller owns the workspace (for owner-only actions).
func (s *Service) requireOwner(ctx context.Context, workspaceID, userID string) (Workspace, error) {
	ws, err := s.workspaces.FindByID(ctx, workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	if ws.OwnerID != userID {
		return Workspace{}, ErrOwnerOnly
	}
	return ws, nil
}

// ── Workspace lifecycle ─────────────────────────────────────────────────────

// CreateWorkspace creates a workspace, seeds it with "Admin" and "Member" roles,
// and adds the creator as an Admin member — all in one transaction.
func (s *Service) CreateWorkspace(ctx context.Context, userID, name, slug string) (Workspace, error) {
	slug = normalizeSlug(slug, name)
	if _, err := s.workspaces.FindBySlug(ctx, slug); err == nil {
		return Workspace{}, ErrSlugTaken
	} else if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}

	var created Workspace
	err := s.uow.Do(ctx, func(r Repos) error {
		ws, err := r.Workspaces.Create(ctx, Workspace{Name: name, Slug: slug, OwnerID: userID})
		if err != nil {
			return err
		}
		admin, err := r.Roles.Create(ctx, Role{WorkspaceID: ws.ID, Name: "Admin", Permissions: permAdminRole})
		if err != nil {
			return err
		}
		if _, err := r.Roles.Create(ctx, Role{WorkspaceID: ws.ID, Name: "Member", Permissions: PermViewWorkspace | PermViewWorkbenches | PermCreateWorkbench, IsDefault: true}); err != nil {
			return err
		}
		if _, err := r.Members.Add(ctx, Member{WorkspaceID: ws.ID, UserID: userID, RoleID: admin.ID}); err != nil {
			return err
		}
		created = ws
		return nil
	})
	if err != nil {
		return Workspace{}, err
	}
	return created, nil
}

func (s *Service) ListMyWorkspaces(ctx context.Context, userID string) ([]Workspace, error) {
	return s.workspaces.ListByUserID(ctx, userID)
}

func (s *Service) GetWorkspace(ctx context.Context, userID, workspaceID string) (Workspace, Permission, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermViewWorkspace)
	if err != nil {
		return Workspace{}, 0, err
	}
	return a.ws, a.perms, nil
}

func (s *Service) UpdateWorkspace(ctx context.Context, userID, workspaceID, name, slug string) (Workspace, error) {
	if _, err := s.authorize(ctx, workspaceID, userID, PermManageWorkspace); err != nil {
		return Workspace{}, err
	}
	slug = normalizeSlug(slug, name)
	if existing, err := s.workspaces.FindBySlug(ctx, slug); err == nil {
		if existing.ID != workspaceID {
			return Workspace{}, ErrSlugTaken
		}
	} else if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}
	return s.workspaces.Update(ctx, workspaceID, name, slug)
}

func (s *Service) DeleteWorkspace(ctx context.Context, userID, workspaceID string) error {
	if _, err := s.requireOwner(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.workspaces.Delete(ctx, workspaceID)
}

func (s *Service) TransferOwnership(ctx context.Context, userID, workspaceID, newOwnerUserID string) error {
	if _, err := s.requireOwner(ctx, workspaceID, userID); err != nil {
		return err
	}
	if _, err := s.members.Find(ctx, workspaceID, newOwnerUserID); err != nil {
		return err // NoRows -> ErrNotMember
	}
	return s.workspaces.UpdateOwner(ctx, workspaceID, newOwnerUserID)
}

func (s *Service) LeaveWorkspace(ctx context.Context, userID, workspaceID string) error {
	ws, err := s.workspaces.FindByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID == userID {
		return ErrLastOwner
	}
	if _, err := s.members.Find(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.members.Remove(ctx, workspaceID, userID)
}

// ── Members ─────────────────────────────────────────────────────────────────

func (s *Service) ListMembers(ctx context.Context, userID, workspaceID string) ([]MemberView, error) {
	if _, err := s.authorize(ctx, workspaceID, userID, PermViewWorkspace); err != nil {
		return nil, err
	}
	members, err := s.members.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	roles, err := s.roles.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]Role, len(roles))
	for _, r := range roles {
		byID[r.ID] = r
	}
	views := make([]MemberView, 0, len(members))
	for _, m := range members {
		u, err := s.users.FindByID(ctx, m.UserID)
		if err != nil {
			return nil, err
		}
		views = append(views, MemberView{Member: m, Role: byID[m.RoleID], User: u})
	}
	return views, nil
}

func (s *Service) ChangeMemberRole(ctx context.Context, userID, workspaceID, targetUserID, roleID string) (MemberView, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermManageMembers)
	if err != nil {
		return MemberView{}, err
	}
	if targetUserID == a.ws.OwnerID {
		return MemberView{}, ErrLastOwner
	}
	role, err := s.roleInWorkspace(ctx, workspaceID, roleID)
	if err != nil {
		return MemberView{}, err
	}
	if !a.elevated && !role.Permissions.subsetOf(a.perms) {
		return MemberView{}, ErrPrivilegeEscalation
	}
	if _, err := s.members.Find(ctx, workspaceID, targetUserID); err != nil {
		return MemberView{}, err
	}
	if err := s.members.UpdateRole(ctx, workspaceID, targetUserID, roleID); err != nil {
		return MemberView{}, err
	}
	u, err := s.users.FindByID(ctx, targetUserID)
	if err != nil {
		return MemberView{}, err
	}
	return MemberView{
		Member: Member{WorkspaceID: workspaceID, UserID: targetUserID, RoleID: roleID},
		Role:   role,
		User:   u,
	}, nil
}

func (s *Service) RemoveMember(ctx context.Context, userID, workspaceID, targetUserID string) error {
	a, err := s.authorize(ctx, workspaceID, userID, PermManageMembers)
	if err != nil {
		return err
	}
	if targetUserID == a.ws.OwnerID {
		return ErrLastOwner
	}
	target, err := s.members.LoadMembership(ctx, workspaceID, targetUserID)
	if err != nil {
		return err
	}
	if !a.elevated && !target.Role.Permissions.subsetOf(a.perms) {
		return ErrPrivilegeEscalation
	}
	return s.members.Remove(ctx, workspaceID, targetUserID)
}

// ── Roles ───────────────────────────────────────────────────────────────────

func (s *Service) ListRoles(ctx context.Context, userID, workspaceID string) ([]Role, error) {
	if _, err := s.authorize(ctx, workspaceID, userID, PermViewWorkspace); err != nil {
		return nil, err
	}
	return s.roles.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) CreateRole(ctx context.Context, userID, workspaceID, name string, perms Permission) (Role, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermManageRoles)
	if err != nil {
		return Role{}, err
	}
	if !a.elevated && !perms.subsetOf(a.perms) {
		return Role{}, ErrPrivilegeEscalation
	}
	return s.roles.Create(ctx, Role{WorkspaceID: workspaceID, Name: name, Permissions: perms})
}

func (s *Service) UpdateRole(ctx context.Context, userID, workspaceID, roleID, name string, perms Permission) (Role, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermManageRoles)
	if err != nil {
		return Role{}, err
	}
	if _, err := s.roleInWorkspace(ctx, workspaceID, roleID); err != nil {
		return Role{}, err
	}
	if !a.elevated && !perms.subsetOf(a.perms) {
		return Role{}, ErrPrivilegeEscalation
	}
	return s.roles.Update(ctx, roleID, name, perms)
}

func (s *Service) DeleteRole(ctx context.Context, userID, workspaceID, roleID string) error {
	a, err := s.authorize(ctx, workspaceID, userID, PermManageRoles)
	if err != nil {
		return err
	}
	role, err := s.roleInWorkspace(ctx, workspaceID, roleID)
	if err != nil {
		return err
	}
	if role.IsDefault {
		return ErrDefaultRole
	}
	if !a.elevated && !role.Permissions.subsetOf(a.perms) {
		return ErrPrivilegeEscalation
	}
	count, err := s.roles.CountMembersUsing(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrRoleInUse
	}
	return s.roles.Delete(ctx, roleID)
}

// ── Email invitations ───────────────────────────────────────────────────────

func (s *Service) InviteByEmail(ctx context.Context, userID, workspaceID, email, roleID, locale string) (EmailInvitation, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermCreateInvite)
	if err != nil {
		return EmailInvitation{}, err
	}
	role, err := s.roleInWorkspace(ctx, workspaceID, roleID)
	if err != nil {
		return EmailInvitation{}, err
	}
	if !a.elevated && !role.Permissions.subsetOf(a.perms) {
		return EmailInvitation{}, ErrPrivilegeEscalation
	}
	if u, err := s.users.FindByEmail(ctx, email); err == nil {
		if _, err := s.members.Find(ctx, workspaceID, u.ID); err == nil {
			return EmailInvitation{}, ErrAlreadyMember
		}
	}
	if err := s.emailInvs.DeleteByWorkspaceEmail(ctx, workspaceID, email); err != nil {
		return EmailInvitation{}, err
	}
	plain, hash, err := generateOpaque()
	if err != nil {
		return EmailInvitation{}, err
	}
	inv, err := s.emailInvs.Create(ctx, EmailInvitation{
		WorkspaceID: workspaceID,
		Email:       email,
		RoleID:      roleID,
		TokenHash:   hash,
		InvitedBy:   userID,
		ExpiresAt:   time.Now().Add(s.cfg.InviteExpiry),
	})
	if err != nil {
		return EmailInvitation{}, err
	}
	inviterName := a.ws.Name
	if u, err := s.users.FindByID(ctx, userID); err == nil {
		inviterName = u.DisplayName
	}
	link := s.cfg.AppBaseURL + "/" + locale + "/workspace/accept-invite?token=" + plain
	if err := s.email.SendWorkspaceInvite(ctx, email, a.ws.Name, inviterName, link); err != nil {
		return EmailInvitation{}, err
	}
	return inv, nil
}

func (s *Service) ListEmailInvitations(ctx context.Context, userID, workspaceID string) ([]EmailInvitation, error) {
	if _, err := s.authorize(ctx, workspaceID, userID, PermManageInvites); err != nil {
		return nil, err
	}
	return s.emailInvs.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) RevokeEmailInvitation(ctx context.Context, userID, workspaceID, invitationID string) error {
	if _, err := s.authorize(ctx, workspaceID, userID, PermManageInvites); err != nil {
		return err
	}
	inv, err := s.emailInvs.FindByID(ctx, invitationID)
	if err != nil {
		return err
	}
	if inv.WorkspaceID != workspaceID {
		return ErrInviteInvalid
	}
	return s.emailInvs.Delete(ctx, invitationID)
}

func (s *Service) AcceptEmailInvite(ctx context.Context, userID, token string) (Workspace, error) {
	inv, err := s.emailInvs.FindByTokenHash(ctx, hashOpaque(token))
	if err != nil {
		if errors.Is(err, ErrInviteInvalid) {
			return Workspace{}, ErrInviteInvalid
		}
		return Workspace{}, err
	}
	if inv.ExpiresAt.Before(time.Now()) {
		return Workspace{}, ErrInviteExpired
	}
	err = s.uow.Do(ctx, func(r Repos) error {
		if _, err := r.Members.Find(ctx, inv.WorkspaceID, userID); err != nil {
			if !errors.Is(err, ErrNotMember) {
				return err
			}
			if _, err := r.Members.Add(ctx, Member{WorkspaceID: inv.WorkspaceID, UserID: userID, RoleID: inv.RoleID}); err != nil {
				return err
			}
		}
		return r.EmailInvs.Delete(ctx, inv.ID)
	})
	if err != nil {
		return Workspace{}, err
	}
	return s.workspaces.FindByID(ctx, inv.WorkspaceID)
}

func (s *Service) PreviewEmailInvite(ctx context.Context, token string) (InvitePreview, error) {
	inv, err := s.emailInvs.FindByTokenHash(ctx, hashOpaque(token))
	if err != nil {
		if errors.Is(err, ErrInviteInvalid) {
			return InvitePreview{Valid: false}, nil
		}
		return InvitePreview{}, err
	}
	if inv.ExpiresAt.Before(time.Now()) {
		return InvitePreview{Valid: false}, nil
	}
	ws, err := s.workspaces.FindByID(ctx, inv.WorkspaceID)
	if err != nil {
		return InvitePreview{}, err
	}
	role, err := s.roles.FindByID(ctx, inv.RoleID)
	if err != nil {
		return InvitePreview{}, err
	}
	return InvitePreview{WorkspaceName: ws.Name, RoleName: role.Name, Email: inv.Email, Valid: true}, nil
}

// ── Invite links ────────────────────────────────────────────────────────────

func (s *Service) CreateInviteLink(ctx context.Context, userID, workspaceID, roleID string, maxUses int32, expiresAt *time.Time) (InviteLink, error) {
	a, err := s.authorize(ctx, workspaceID, userID, PermCreateInvite)
	if err != nil {
		return InviteLink{}, err
	}
	role, err := s.roleInWorkspace(ctx, workspaceID, roleID)
	if err != nil {
		return InviteLink{}, err
	}
	if !a.elevated && !role.Permissions.subsetOf(a.perms) {
		return InviteLink{}, ErrPrivilegeEscalation
	}
	code, err := generateCode()
	if err != nil {
		return InviteLink{}, err
	}
	if maxUses < 0 {
		maxUses = 0
	}
	return s.links.Create(ctx, InviteLink{
		WorkspaceID: workspaceID,
		Code:        code,
		RoleID:      roleID,
		CreatedBy:   userID,
		MaxUses:     maxUses,
		ExpiresAt:   expiresAt,
	})
}

func (s *Service) ListInviteLinks(ctx context.Context, userID, workspaceID string) ([]InviteLink, error) {
	if _, err := s.authorize(ctx, workspaceID, userID, PermManageInvites); err != nil {
		return nil, err
	}
	return s.links.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) RevokeInviteLink(ctx context.Context, userID, workspaceID, linkID string) error {
	if _, err := s.authorize(ctx, workspaceID, userID, PermManageInvites); err != nil {
		return err
	}
	link, err := s.links.FindByID(ctx, linkID)
	if err != nil {
		return err
	}
	if link.WorkspaceID != workspaceID {
		return ErrLinkInvalid
	}
	return s.links.Revoke(ctx, linkID)
}

func (s *Service) PreviewInviteLink(ctx context.Context, code string) (LinkPreview, error) {
	link, err := s.links.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrLinkInvalid) {
			return LinkPreview{Valid: false}, nil
		}
		return LinkPreview{}, err
	}
	if !linkUsable(link) {
		return LinkPreview{Valid: false}, nil
	}
	ws, err := s.workspaces.FindByID(ctx, link.WorkspaceID)
	if err != nil {
		return LinkPreview{}, err
	}
	role, err := s.roles.FindByID(ctx, link.RoleID)
	if err != nil {
		return LinkPreview{}, err
	}
	return LinkPreview{WorkspaceName: ws.Name, RoleName: role.Name, Valid: true}, nil
}

func (s *Service) JoinViaInviteLink(ctx context.Context, userID, code string) (Workspace, error) {
	link, err := s.links.FindByCode(ctx, code)
	if err != nil {
		return Workspace{}, err
	}
	if link.Revoked {
		return Workspace{}, ErrLinkInvalid
	}
	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		return Workspace{}, ErrLinkExpired
	}
	if link.MaxUses > 0 && link.UseCount >= link.MaxUses {
		return Workspace{}, ErrLinkExhausted
	}
	// Already a member? Return the workspace without consuming a use.
	if _, err := s.members.Find(ctx, link.WorkspaceID, userID); err == nil {
		return s.workspaces.FindByID(ctx, link.WorkspaceID)
	} else if !errors.Is(err, ErrNotMember) {
		return Workspace{}, err
	}
	err = s.uow.Do(ctx, func(r Repos) error {
		if _, err := r.Members.Add(ctx, Member{WorkspaceID: link.WorkspaceID, UserID: userID, RoleID: link.RoleID}); err != nil {
			return err
		}
		return r.Links.IncrementUse(ctx, link.ID)
	})
	if err != nil {
		return Workspace{}, err
	}
	return s.workspaces.FindByID(ctx, link.WorkspaceID)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// roleInWorkspace loads a role and asserts it belongs to the workspace.
func (s *Service) roleInWorkspace(ctx context.Context, workspaceID, roleID string) (Role, error) {
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil {
		return Role{}, err
	}
	if role.WorkspaceID != workspaceID {
		return Role{}, ErrRoleNotFound
	}
	return role, nil
}

func linkUsable(l InviteLink) bool {
	if l.Revoked {
		return false
	}
	if l.ExpiresAt != nil && l.ExpiresAt.Before(time.Now()) {
		return false
	}
	if l.MaxUses > 0 && l.UseCount >= l.MaxUses {
		return false
	}
	return true
}

func normalizeSlug(slug, name string) string {
	s := slug
	if s == "" {
		s = name
	}
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "workspace"
	}
	return out
}

// generateOpaque mirrors token.GenerateOpaque: a plain token for the invitee and
// its sha256 hash for storage.
func generateOpaque() (plain, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain = hex.EncodeToString(b)
	return plain, hashOpaque(plain), nil
}

// generateCode returns a short, URL-safe code shown in shareable invite links.
func generateCode() (string, error) {
	b := make([]byte, 9)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashOpaque(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
