// Package clientprofile holds the per-client (workspace) bid profile an
// advisor sets up so the agent can surface a best-fit tender shortlist for
// that specific client — sectors (CPV prefixes), target countries, a value
// band, and free-text notes that also feed the semantic search query. One
// profile per workspace (1:1); membership-checked the same way agent.Service
// checks workspace membership before every operation.
package clientprofile

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

const maxNotesLen = 2000

var (
	ErrProfileNotFound  = errors.New("clientprofile: no profile for workspace")
	ErrInvalidSector    = errors.New("clientprofile: sector must be a 2-8 digit CPV prefix")
	ErrInvalidCountry   = errors.New("clientprofile: country must be an uppercase 3-letter alpha code")
	ErrInvalidValueBand = errors.New("clientprofile: value_min must not exceed value_max")
	ErrNotesTooLong     = errors.New("clientprofile: notes must be 2000 characters or fewer")
)

// Profile is one client's (workspace's) bid-matching preferences.
type Profile struct {
	WorkspaceID string
	Sectors     []string // CPV prefixes, e.g. ["45", "72"]
	Countries   []string // alpha-3, e.g. ["ITA", "DEU"]
	ValueMin    *int64   // nil = unset
	ValueMax    *int64   // nil = unset
	Notes       string   // free-text intent; feeds the semantic search query
	UpdatedAt   time.Time
}

// Repository is the port a postgres adapter satisfies. Get returns
// ErrProfileNotFound when no row exists for workspaceID; Upsert is a full
// replace (not a partial patch) — every field is written, including
// clearing a previously-set value bound back to nil.
type Repository interface {
	Get(ctx context.Context, workspaceID string) (Profile, error)
	Upsert(ctx context.Context, p Profile) (Profile, error)
}

// MemberRepository is the minimal membership-check port this service needs
// — satisfied by *postgres.MemberRepo unchanged, the same concrete type
// agent.Service already depends on via its own narrower MemberRepository.
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

type Service struct {
	repo    Repository
	members MemberRepository
}

func NewService(repo Repository, members MemberRepository) *Service {
	return &Service{repo: repo, members: members}
}

func (s *Service) requireMember(ctx context.Context, workspaceID, userID string) error {
	_, err := s.members.LoadMembership(ctx, workspaceID, userID)
	return err
}

// Get returns workspaceID's client profile, or ErrProfileNotFound if the
// advisor hasn't set one up yet.
func (s *Service) Get(ctx context.Context, userID, workspaceID string) (Profile, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return Profile{}, err
	}
	return s.repo.Get(ctx, workspaceID)
}

// Update validates and full-replaces the profile for p.WorkspaceID.
func (s *Service) Update(ctx context.Context, userID string, p Profile) (Profile, error) {
	if err := s.requireMember(ctx, p.WorkspaceID, userID); err != nil {
		return Profile{}, err
	}
	if err := validate(p); err != nil {
		return Profile{}, err
	}
	return s.repo.Upsert(ctx, p)
}

var (
	cpvPrefixRe = regexp.MustCompile(`^\d{2,8}$`)
	countryRe   = regexp.MustCompile(`^[A-Z]{3}$`)
)

func validate(p Profile) error {
	for _, sec := range p.Sectors {
		if !cpvPrefixRe.MatchString(sec) {
			return ErrInvalidSector
		}
	}
	for _, c := range p.Countries {
		if !countryRe.MatchString(c) {
			return ErrInvalidCountry
		}
	}
	if p.ValueMin != nil && p.ValueMax != nil && *p.ValueMin > *p.ValueMax {
		return ErrInvalidValueBand
	}
	if len(p.Notes) > maxNotesLen {
		return ErrNotesTooLong
	}
	return nil
}
