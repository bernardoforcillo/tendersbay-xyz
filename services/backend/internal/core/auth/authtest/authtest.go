// Package authtest holds the fakes the auth and user domains both need.
//
// The two packages test the same two collaborators — the profile store and the
// mailer — against the same authlayer memory store, and had a byte-identical
// copy of each. They are here so a change to what a profile row looks like, or
// to what the service mails, is made once.
//
// It follows authlayer's own auth/authtest and the standard library's
// net/http/httptest: a test-only package that ships in the tree, so packages
// that cannot share a _test.go file can still share a fake.
package authtest

import (
	"context"
	"errors"
	"net/url"
	"testing"

	alauth "github.com/bernardoforcillo/authlayer/auth"
	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// Profiles is the profile port. It reads identity straight off the same
// authlayer store the service writes to — exactly as postgres.UserRepo reads
// the same users row authlayer's store writes — and adds the one column the
// library does not own.
type Profiles struct {
	store *memstore.AuthStore
	names map[string]string
}

func NewProfiles(store *memstore.AuthStore) *Profiles {
	return &Profiles{store: store, names: map[string]string{}}
}

var _ auth.UserRepository = (*Profiles)(nil)

func (p *Profiles) view(u alauth.UserBase, err error) (auth.User, error) {
	if errors.Is(err, alauth.ErrUserNotFound) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{
		ID:              u.ID,
		Email:           u.Email,
		DisplayName:     p.names[u.ID],
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
		DeletedAt:       u.DeletedAt,
	}, nil
}

func (p *Profiles) FindByID(ctx context.Context, id string) (auth.User, error) {
	return p.view(p.store.FindUserByID(ctx, id))
}

func (p *Profiles) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	return p.view(p.store.FindUserByEmail(ctx, email))
}

func (p *Profiles) UpdateDisplayName(_ context.Context, id, displayName string) error {
	p.names[id] = displayName
	return nil
}

// Mail kinds, matching the four methods of auth.EmailSender.
const (
	KindVerification  = "verification"
	KindReset         = "reset"
	KindEmailChange   = "email-change"
	KindAccountExists = "account-exists"
)

// Mail is one message the service asked to be sent.
type Mail struct {
	Kind string
	To   string
	Name string
	Link string
}

// Mailer records every message instead of sending it.
type Mailer struct{ Sent []Mail }

var _ auth.EmailSender = (*Mailer)(nil)

func (m *Mailer) SendVerification(_ context.Context, to, name, link string) error {
	m.Sent = append(m.Sent, Mail{KindVerification, to, name, link})
	return nil
}

func (m *Mailer) SendPasswordReset(_ context.Context, to, name, link string) error {
	m.Sent = append(m.Sent, Mail{KindReset, to, name, link})
	return nil
}

func (m *Mailer) SendEmailChangeVerification(_ context.Context, to, name, link string) error {
	m.Sent = append(m.Sent, Mail{KindEmailChange, to, name, link})
	return nil
}

func (m *Mailer) SendAccountExists(_ context.Context, to, name string) error {
	m.Sent = append(m.Sent, Mail{Kind: KindAccountExists, To: to, Name: name})
	return nil
}

// Reset forgets everything sent so far, for a test that has finished setting up
// and wants to assert on what happens next.
func (m *Mailer) Reset() { m.Sent = nil }

// Count is how many messages of a kind were sent.
func (m *Mailer) Count(kind string) int {
	n := 0
	for _, c := range m.Sent {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

// Only asserts exactly one message of a kind was sent, and returns it. Use it
// where "and nothing else" is part of what is being asserted.
func (m *Mailer) Only(t *testing.T, kind string) Mail {
	t.Helper()
	var found []Mail
	for _, c := range m.Sent {
		if c.Kind == kind {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %q email, got %d (all: %+v)", kind, len(found), m.Sent)
	}
	return found[0]
}

// Last returns the most recent message of a kind.
func (m *Mailer) Last(t *testing.T, kind string) Mail {
	t.Helper()
	for i := len(m.Sent) - 1; i >= 0; i-- {
		if m.Sent[i].Kind == kind {
			return m.Sent[i]
		}
	}
	t.Fatalf("no %q email sent (all: %+v)", kind, m.Sent)
	return Mail{}
}

// TokenFrom pulls the `token` query parameter out of a link the service mailed.
func TokenFrom(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse link %q: %v", link, err)
	}
	tok := u.Query().Get("token")
	if tok == "" {
		t.Fatalf("no token in link %q", link)
	}
	return tok
}

// Strong is a password that satisfies authlayer's default rules, so a test that
// is not about password strength does not have to think about them.
const Strong = "Str0ng!Passphrase"

// Secret is a JWT signing key long enough for HS256's 32-byte floor.
const Secret = "test-secret-that-is-long-enough-for-hs256"
