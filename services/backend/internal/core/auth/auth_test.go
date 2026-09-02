package auth_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	alauth "github.com/bernardoforcillo/authlayer/auth"
	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

const (
	goodPassword = "Str0ng!Passphrase"
	testSecret   = "test-secret-that-is-long-enough-for-hs256"
)

// --- fakes ---

// mockProfiles is the profile port. It reads identity straight off the same
// authlayer store the service writes to — exactly as postgres.UserRepo reads
// the same users row authlayer's store writes — and adds the one column the
// library does not own.
type mockProfiles struct {
	store *memstore.AuthStore
	names map[string]string
}

func newMockProfiles(store *memstore.AuthStore) *mockProfiles {
	return &mockProfiles{store: store, names: map[string]string{}}
}

func (m *mockProfiles) view(u alauth.UserBase, err error) (auth.User, error) {
	if errors.Is(err, alauth.ErrUserNotFound) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{
		ID:              u.ID,
		Email:           u.Email,
		DisplayName:     m.names[u.ID],
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
		DeletedAt:       u.DeletedAt,
	}, nil
}

func (m *mockProfiles) FindByID(ctx context.Context, id string) (auth.User, error) {
	return m.view(m.store.FindUserByID(ctx, id))
}

func (m *mockProfiles) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	return m.view(m.store.FindUserByEmail(ctx, email))
}

func (m *mockProfiles) UpdateDisplayName(_ context.Context, id, displayName string) error {
	m.names[id] = displayName
	return nil
}

type mailCall struct {
	kind string // "verification", "reset", "email-change", "account-exists"
	to   string
	name string
	link string
}

type mockMailer struct{ calls []mailCall }

func (m *mockMailer) SendVerification(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"verification", to, name, link})
	return nil
}

func (m *mockMailer) SendPasswordReset(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"reset", to, name, link})
	return nil
}

func (m *mockMailer) SendEmailChangeVerification(_ context.Context, to, name, link string) error {
	m.calls = append(m.calls, mailCall{"email-change", to, name, link})
	return nil
}

func (m *mockMailer) SendAccountExists(_ context.Context, to, name string) error {
	m.calls = append(m.calls, mailCall{"account-exists", to, name, ""})
	return nil
}

func (m *mockMailer) only(t *testing.T, kind string) mailCall {
	t.Helper()
	var found []mailCall
	for _, c := range m.calls {
		if c.kind == kind {
			found = append(found, c)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one %q email, got %d (all: %+v)", kind, len(found), m.calls)
	}
	return found[0]
}

func (m *mockMailer) count(kind string) int {
	n := 0
	for _, c := range m.calls {
		if c.kind == kind {
			n++
		}
	}
	return n
}

// mockLimiter denies once a key has been seen denyAfter times, or fails with
// err on every call when one is set.
type mockLimiter struct {
	seen      map[string]int
	denyAfter int
	err       error
}

func newMockLimiter(denyAfter int) *mockLimiter {
	return &mockLimiter{seen: map[string]int{}, denyAfter: denyAfter}
}

func (m *mockLimiter) Allow(_ context.Context, key string, _ int64, _ time.Duration) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	m.seen[key]++
	return m.seen[key] <= m.denyAfter, nil
}

// --- fixture ---

type fixture struct {
	svc      *auth.Service
	store    *memstore.AuthStore
	profiles *mockProfiles
	mail     *mockMailer
	limiter  *mockLimiter
}

func newFixture(t *testing.T, limiter auth.RateLimiter) *fixture {
	t.Helper()
	store := memstore.NewAuthStore()
	profiles := newMockProfiles(store)
	mail := &mockMailer{}
	svc := auth.NewService(store, profiles, mail, limiter, auth.Config{
		JWTSecret:     testSecret,
		JWTExpiry:     15 * time.Minute,
		RefreshExpiry: 168 * time.Hour,
		AppBaseURL:    "https://app.test",
	})
	f := &fixture{svc: svc, store: store, profiles: profiles, mail: mail}
	if l, ok := limiter.(*mockLimiter); ok {
		f.limiter = l
	}
	return f
}

// tokenFrom pulls the `token` query parameter out of a link the service mailed.
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

// signUpVerified registers an account and redeems its verification token, so
// it can actually log in.
func (f *fixture) signUpVerified(t *testing.T, email string) {
	t.Helper()
	ctx := context.Background()
	if err := f.svc.SignUp(ctx, email, goodPassword, "Test User", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := f.svc.VerifyEmail(ctx, tokenFrom(t, f.mail.only(t, "verification").link), "signup"); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	f.mail.calls = nil
}

// --- sign up ---

func TestSignUp_SendsVerificationEmailAndSetsDisplayName(t *testing.T) {
	f := newFixture(t, nil)
	if err := f.svc.SignUp(context.Background(), "a@b.com", goodPassword, "Alice", "it-it", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	call := f.mail.only(t, "verification")
	if call.to != "a@b.com" || call.name != "Alice" {
		t.Fatalf("verification mail addressed wrong: %+v", call)
	}
	if !strings.HasPrefix(call.link, "https://app.test/it-it/auth/verify-email?token=") {
		t.Fatalf("unexpected link: %q", call.link)
	}
	u, err := f.profiles.FindByEmail(context.Background(), "a@b.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if u.DisplayName != "Alice" {
		t.Fatalf("display name = %q, want Alice", u.DisplayName)
	}
}

func TestSignUp_WeakPassword(t *testing.T) {
	f := newFixture(t, nil)
	err := f.svc.SignUp(context.Background(), "a@b.com", "short", "Alice", "en-ie", "1.2.3.4")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
	if len(f.mail.calls) != 0 {
		t.Fatalf("weak password must not send mail, got %+v", f.mail.calls)
	}
}

// A duplicate signup must be indistinguishable from a fresh one to the caller:
// same nil error, no verification link to redeem. The only difference is a
// notice to the address that is already registered — which reaches the real
// accountholder, not the caller.
func TestSignUp_DuplicateEmailIsNeutral(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	if err := f.svc.SignUp(ctx, "a@b.com", goodPassword, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("first SignUp: %v", err)
	}
	f.mail.calls = nil

	if err := f.svc.SignUp(ctx, "a@b.com", goodPassword, "Mallory", "en-ie", "5.6.7.8"); err != nil {
		t.Fatalf("duplicate SignUp returned %v, want nil", err)
	}
	if f.mail.count("verification") != 0 {
		t.Fatalf("duplicate signup must not mail a redeemable link")
	}
	exists := f.mail.only(t, "account-exists")
	if exists.to != "a@b.com" || exists.name != "Alice" {
		t.Fatalf("account-exists mail addressed wrong: %+v", exists)
	}
}

func TestSignUp_NormalizesEmail(t *testing.T) {
	f := newFixture(t, nil)
	if err := f.svc.SignUp(context.Background(), "  A@B.CoM ", goodPassword, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := f.profiles.FindByEmail(context.Background(), "a@b.com"); err != nil {
		t.Fatalf("normalized lookup: %v", err)
	}
	if f.mail.only(t, "verification").to != "a@b.com" {
		t.Fatalf("verification sent to a non-normalized address")
	}
}

// --- login ---

func TestLogin_UnverifiedEmail(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	if err := f.svc.SignUp(ctx, "a@b.com", goodPassword, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	_, err := f.svc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if !errors.Is(err, auth.ErrEmailNotVerified) {
		t.Fatalf("err = %v, want ErrEmailNotVerified", err)
	}
}

func TestLogin_UnknownEmailIsInvalidCreds(t *testing.T) {
	f := newFixture(t, nil)
	_, err := f.svc.Login(context.Background(), "nobody@b.com", goodPassword, "1.2.3.4", "test-agent")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
}

func TestLogin_WrongPasswordIsInvalidCreds(t *testing.T) {
	f := newFixture(t, nil)
	f.signUpVerified(t, "a@b.com")
	_, err := f.svc.Login(context.Background(), "a@b.com", "Wr0ng!Passphrase", "1.2.3.4", "test-agent")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("err = %v, want ErrInvalidCreds", err)
	}
}

func TestLogin_NormalizesEmail(t *testing.T) {
	f := newFixture(t, nil)
	f.signUpVerified(t, "a@b.com")
	res, err := f.svc.Login(context.Background(), " A@B.com ", goodPassword, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.User.Email != "a@b.com" {
		t.Fatalf("user email = %q, want a@b.com", res.User.Email)
	}
}

func TestLogin_RateLimitedPerIP(t *testing.T) {
	f := newFixture(t, newMockLimiter(0))
	_, err := f.svc.Login(context.Background(), "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if !errors.Is(err, auth.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if _, keyed := f.limiter.seen["login:ip:1.2.3.4"]; !keyed {
		t.Fatalf("login bucket must be keyed by IP alone, got %v", f.limiter.seen)
	}
}

// A limiter outage degrades brute-force protection; it must never lock a
// working auth service.
func TestLogin_LimiterErrorFailsOpen(t *testing.T) {
	limiter := newMockLimiter(0)
	limiter.err = errors.New("redis down")
	f := newFixture(t, limiter)
	f.signUpVerified(t, "a@b.com")
	if _, err := f.svc.Login(context.Background(), "a@b.com", goodPassword, "1.2.3.4", "test-agent"); err != nil {
		t.Fatalf("Login: %v", err)
	}
}

// --- the full round trip authlayer now owns ---

func TestLoginRefreshLogout_RotatesAndRevokes(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	f.signUpVerified(t, "a@b.com")

	first, err := f.svc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := f.svc.VerifyAccessToken(first.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.Subject != first.User.ID {
		t.Fatalf("sub = %q, want %q", claims.Subject, first.User.ID)
	}
	if claims.SessionID == "" {
		t.Fatal("access token carries no session id; ChangePassword could not spare the caller's own session")
	}

	second, err := f.svc.RefreshToken(ctx, first.RefreshPlain)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if second.RefreshPlain == first.RefreshPlain {
		t.Fatal("refresh did not rotate the token")
	}
	// Replaying the spent token must not merely fail — it revokes the family.
	if _, err := f.svc.RefreshToken(ctx, first.RefreshPlain); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("replay err = %v, want ErrTokenInvalid", err)
	}
	if _, err := f.svc.RefreshToken(ctx, second.RefreshPlain); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("post-replay refresh err = %v, want the family revoked", err)
	}
}

func TestLogout_UnknownTokenIsNotAnError(t *testing.T) {
	f := newFixture(t, nil)
	if err := f.svc.Logout(context.Background(), "never-issued"); err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestVerifyAccessToken_RejectsTampered(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	f.signUpVerified(t, "a@b.com")
	res, err := f.svc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := f.svc.VerifyAccessToken(res.AccessToken + "x"); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

// --- forgot / reset password ---

func TestForgotPassword_RateLimitedPerIPReturnsError(t *testing.T) {
	f := newFixture(t, newMockLimiter(0))
	err := f.svc.ForgotPassword(context.Background(), "a@b.com", "en-ie", "1.2.3.4")
	if !errors.Is(err, auth.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

// The per-address cap must stay silent: surfacing it would turn the limiter
// into an existence oracle for the address it is keyed by.
func TestForgotPassword_PerEmailCapIsSilent(t *testing.T) {
	limiter := newMockLimiter(1) // the IP bucket passes, the email one denies
	f := newFixture(t, limiter)
	f.signUpVerified(t, "a@b.com")
	limiter.seen["forgot:email:a@b.com"] = 1

	if err := f.svc.ForgotPassword(context.Background(), "a@b.com", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if f.mail.count("reset") != 0 {
		t.Fatal("capped address must not be mailed")
	}
}

func TestForgotPassword_UnknownAddressIsSilent(t *testing.T) {
	f := newFixture(t, nil)
	if err := f.svc.ForgotPassword(context.Background(), "nobody@b.com", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(f.mail.calls) != 0 {
		t.Fatalf("unknown address must not be mailed, got %+v", f.mail.calls)
	}
}

func TestResetPassword_RotatesCredentialAndRevokesSessions(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	f.signUpVerified(t, "a@b.com")

	live, err := f.svc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := f.svc.ForgotPassword(ctx, "a@b.com", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	const next = "N3w!Passphrase42"
	if err := f.svc.ResetPassword(ctx, tokenFrom(t, f.mail.only(t, "reset").link), next); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := f.svc.Login(ctx, "a@b.com", goodPassword, "1.2.3.4", "test-agent"); !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, err := f.svc.Login(ctx, "a@b.com", next, "1.2.3.4", "test-agent"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	if _, err := f.svc.RefreshToken(ctx, live.RefreshPlain); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("session survived a password reset: %v", err)
	}
}

func TestResetPassword_WeakPassword(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	f.signUpVerified(t, "a@b.com")
	if err := f.svc.ForgotPassword(ctx, "a@b.com", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	err := f.svc.ResetPassword(ctx, tokenFrom(t, f.mail.only(t, "reset").link), "short")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
}

func TestResetPassword_UnknownTokenIsTokenInvalid(t *testing.T) {
	f := newFixture(t, nil)
	err := f.svc.ResetPassword(context.Background(), "never-issued", "N3w!Passphrase42")
	if !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

// --- verify email ---

func TestVerifyEmail_SpentTokenIsRejected(t *testing.T) {
	f := newFixture(t, nil)
	ctx := context.Background()
	if err := f.svc.SignUp(ctx, "a@b.com", goodPassword, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	tok := tokenFrom(t, f.mail.only(t, "verification").link)
	if err := f.svc.VerifyEmail(ctx, tok, "signup"); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	if err := f.svc.VerifyEmail(ctx, tok, "signup"); !errors.Is(err, auth.ErrTokenInvalid) {
		t.Fatalf("replayed token err = %v, want ErrTokenInvalid", err)
	}
}
