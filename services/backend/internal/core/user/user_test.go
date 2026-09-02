package user_test

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	alauth "github.com/bernardoforcillo/authlayer/auth"
	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	coreuser "github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
)

const (
	goodPassword = "Str0ng!Passphrase"
	testSecret   = "test-secret-that-is-long-enough-for-hs256"
)

// profiles reads identity off the same authlayer store the services write to,
// and owns display_name — the split postgres.UserRepo implements for real.
type profiles struct {
	store *memstore.AuthStore
	names map[string]string
}

func (p *profiles) view(u alauth.UserBase, err error) (auth.User, error) {
	if errors.Is(err, alauth.ErrUserNotFound) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{
		ID: u.ID, Email: u.Email, DisplayName: p.names[u.ID],
		EmailVerifiedAt: u.EmailVerifiedAt, CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt, DeletedAt: u.DeletedAt,
	}, nil
}

func (p *profiles) FindByID(ctx context.Context, id string) (auth.User, error) {
	return p.view(p.store.FindUserByID(ctx, id))
}

func (p *profiles) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	return p.view(p.store.FindUserByEmail(ctx, email))
}

func (p *profiles) UpdateDisplayName(_ context.Context, id, name string) error {
	p.names[id] = name
	return nil
}

type mailCall struct{ kind, to, name, link string }

type mailer struct{ calls []mailCall }

func (m *mailer) SendVerification(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"verification", to, name, link})
	return nil
}

func (m *mailer) SendPasswordReset(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"reset", to, name, link})
	return nil
}

func (m *mailer) SendEmailChangeVerification(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"email-change", to, name, link})
	return nil
}

func (m *mailer) SendAccountExists(_ context.Context, to, name string) error {
	m.calls = append(m.calls, mailCall{"account-exists", to, name, ""})
	return nil
}

func (m *mailer) last(t *testing.T, kind string) mailCall {
	t.Helper()
	for i := len(m.calls) - 1; i >= 0; i-- {
		if m.calls[i].kind == kind {
			return m.calls[i]
		}
	}
	t.Fatalf("no %q email sent (all: %+v)", kind, m.calls)
	return mailCall{}
}

type fixture struct {
	authSvc  *auth.Service
	userSvc  *coreuser.Service
	profiles *profiles
	mail     *mailer

	userID    string
	sessionID string
	refresh   string
}

// newFixture builds both services exactly the way main.go does — one authlayer
// service shared by core/auth and core/user — then registers, verifies and
// signs in one account.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	store := memstore.NewAuthStore()
	prof := &profiles{store: store, names: map[string]string{}}
	mail := &mailer{}
	cfg := auth.Config{
		JWTSecret:     testSecret,
		JWTExpiry:     15 * time.Minute,
		RefreshExpiry: 168 * time.Hour,
		AppBaseURL:    "https://app.test",
	}
	authSvc := auth.NewService(store, prof, mail, nil, cfg)
	userSvc := coreuser.NewService(authSvc.Authlayer(), prof, mail, cfg)

	if err := authSvc.SignUp(ctx, "a@b.com", goodPassword, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := authSvc.VerifyEmail(ctx, tokenFrom(t, mail.last(t, "verification").link), "signup"); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	login, err := authSvc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := authSvc.VerifyAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	mail.calls = nil
	return &fixture{
		authSvc: authSvc, userSvc: userSvc, profiles: prof, mail: mail,
		userID: login.User.ID, sessionID: claims.SessionID, refresh: login.RefreshPlain,
	}
}

func tokenFrom(t *testing.T, link string) string {
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

func TestUpdateProfile(t *testing.T) {
	f := newFixture(t)
	got, err := f.userSvc.UpdateProfile(context.Background(), f.userID, "Alice Cooper")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if got.DisplayName != "Alice Cooper" {
		t.Fatalf("display name = %q, want Alice Cooper", got.DisplayName)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	f := newFixture(t)
	err := f.userSvc.ChangePassword(context.Background(), f.userID, f.sessionID, "Wr0ng!Passphrase", "N3w!Passphrase42")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
}

func TestChangePassword_WeakNew(t *testing.T) {
	f := newFixture(t)
	err := f.userSvc.ChangePassword(context.Background(), f.userID, f.sessionID, goodPassword, "short")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
}

// The caller keeps the session they are working in; everything else is a
// credential minted under the old password and goes.
func TestChangePassword_SparesTheCallersOwnSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	other, err := f.authSvc.Login(ctx, "a@b.com", goodPassword, "9.9.9.9", "other-agent")
	if err != nil {
		t.Fatalf("second Login: %v", err)
	}
	const next = "N3w!Passphrase42"
	if err := f.userSvc.ChangePassword(ctx, f.userID, f.sessionID, goodPassword, next); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, err := f.authSvc.RefreshToken(ctx, other.RefreshPlain); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("the other session survived: %v", err)
	}
	if _, err := f.authSvc.RefreshToken(ctx, f.refresh); err != nil {
		t.Fatalf("the caller's own session was revoked: %v", err)
	}
	if _, err := f.authSvc.Login(ctx, "a@b.com", next, "1.2.3.4", "test-agent"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
}

// The address must not move until the emailed link is redeemed — otherwise a
// typo locks the account out of its own recovery path.
func TestChangeEmail_MovesOnlyOnRedemption(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.userSvc.ChangeEmail(ctx, f.userID, f.sessionID, "New@B.com", goodPassword, "it-it"); err != nil {
		t.Fatalf("ChangeEmail: %v", err)
	}
	sent := f.mail.last(t, "email-change")
	if sent.to != "new@b.com" {
		t.Fatalf("mailed %q, want the normalized new address", sent.to)
	}
	before, err := f.profiles.FindByID(ctx, f.userID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if before.Email != "a@b.com" {
		t.Fatalf("address moved before redemption: %q", before.Email)
	}

	if err := f.authSvc.VerifyEmail(ctx, tokenFrom(t, sent.link), "email-change"); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	after, err := f.profiles.FindByID(ctx, f.userID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if after.Email != "new@b.com" {
		t.Fatalf("address = %q, want new@b.com", after.Email)
	}
}

func TestChangeEmail_WrongPassword(t *testing.T) {
	f := newFixture(t)
	err := f.userSvc.ChangeEmail(context.Background(), f.userID, f.sessionID, "new@b.com", "Wr0ng!Passphrase", "en-ie")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
	if len(f.mail.calls) != 0 {
		t.Fatalf("no mail may be sent without the password, got %+v", f.mail.calls)
	}
}

func TestDeleteAccount_WrongPassword(t *testing.T) {
	f := newFixture(t)
	err := f.userSvc.DeleteAccount(context.Background(), f.userID, f.sessionID, "Wr0ng!Passphrase")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
	if _, err := f.profiles.FindByID(context.Background(), f.userID); err != nil {
		t.Fatalf("account was touched: %v", err)
	}
}

func TestDeleteAccount_RemovesTheAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.userSvc.DeleteAccount(ctx, f.userID, f.sessionID, goodPassword); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := f.profiles.FindByID(ctx, f.userID); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := f.authSvc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent"); !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("a deleted account could still sign in: %v", err)
	}
}
