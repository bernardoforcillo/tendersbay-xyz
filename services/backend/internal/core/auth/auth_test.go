package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// --- mocks ---

type mockUsers struct {
	users map[string]auth.User
}

func newMockUsers() *mockUsers { return &mockUsers{users: map[string]auth.User{}} }

func (m *mockUsers) Create(_ context.Context, u auth.User) (auth.User, error) {
	u.ID = "user-1"
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	m.users[u.Email] = u
	return u, nil
}
func (m *mockUsers) FindByEmail(_ context.Context, email string) (auth.User, error) {
	u, ok := m.users[email]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return u, nil
}
func (m *mockUsers) FindByID(_ context.Context, id string) (auth.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}
func (m *mockUsers) UpdatePassword(_ context.Context, id, hash string) error {
	for k, u := range m.users {
		if u.ID == id {
			u.PasswordHash = hash
			m.users[k] = u
		}
	}
	return nil
}
func (m *mockUsers) UpdateEmail(_ context.Context, _, _ string) error  { return nil }
func (m *mockUsers) UpdateLocale(_ context.Context, _, _ string) error { return nil }

func (m *mockUsers) UpdateDisplayName(_ context.Context, _, _ string) error { return nil }
func (m *mockUsers) MarkEmailVerified(_ context.Context, id string, at time.Time) error {
	for k, u := range m.users {
		if u.ID == id {
			u.EmailVerifiedAt = &at
			m.users[k] = u
		}
	}
	return nil
}
func (m *mockUsers) Delete(_ context.Context, _ string) error { return nil }

type mockSessions struct{}

func (m *mockSessions) Create(_ context.Context, s auth.Session) (auth.Session, error) {
	s.ID = "sess-1"
	return s, nil
}
func (m *mockSessions) FindByTokenHash(_ context.Context, _ string) (auth.Session, error) {
	return auth.Session{}, auth.ErrNotFound
}
func (m *mockSessions) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockSessions) DeleteByUserID(_ context.Context, _ string) error { return nil }

type mockEVs struct{}

func (m *mockEVs) Create(_ context.Context, ev auth.EmailVerification) (auth.EmailVerification, error) {
	ev.ID = "ev-1"
	return ev, nil
}
func (m *mockEVs) FindByTokenHash(_ context.Context, _ string) (auth.EmailVerification, error) {
	return auth.EmailVerification{}, auth.ErrNotFound
}
func (m *mockEVs) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockEVs) DeleteByUserID(_ context.Context, _ string) error { return nil }

type mockPRs struct{}

func (m *mockPRs) Create(_ context.Context, pr auth.PasswordReset) (auth.PasswordReset, error) {
	pr.ID = "pr-1"
	return pr, nil
}
func (m *mockPRs) FindByTokenHash(_ context.Context, _ string) (auth.PasswordReset, error) {
	return auth.PasswordReset{}, auth.ErrNotFound
}
func (m *mockPRs) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockPRs) DeleteByUserID(_ context.Context, _ string) error { return nil }

type mockEmail struct{ sent []string }

func (m *mockEmail) SendVerification(_ context.Context, to, _, _ string) error {
	m.sent = append(m.sent, "verify:"+to)
	return nil
}
func (m *mockEmail) SendPasswordReset(_ context.Context, to, _, _ string) error {
	m.sent = append(m.sent, "reset:"+to)
	return nil
}
func (m *mockEmail) SendEmailChangeVerification(_ context.Context, to, _, _ string) error {
	m.sent = append(m.sent, "change:"+to)
	return nil
}
func (m *mockEmail) SendAccountExists(_ context.Context, to, _ string) error {
	m.sent = append(m.sent, "exists:"+to)
	return nil
}

// mockLimiter drives the auth rate-limit paths deterministically: it counts
// calls per key and denies once a key exceeds its per-call limit. A key listed
// in deny is always denied; a non-nil err makes every Allow error (to exercise
// the fail-open path). A nil *mockLimiter is never constructed — pass a real
// limiter or nil to NewService instead.
type mockLimiter struct {
	counts map[string]int64
	deny   map[string]bool
	err    error
}

func newMockLimiter() *mockLimiter {
	return &mockLimiter{counts: map[string]int64{}, deny: map[string]bool{}}
}

func (m *mockLimiter) Allow(_ context.Context, key string, limit int64, _ time.Duration) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.deny[key] {
		return false, nil
	}
	m.counts[key]++
	return m.counts[key] <= limit, nil
}

// --- tests ---

func newService() (*auth.Service, *mockUsers, *mockEmail) {
	svc, users, email, _ := newServiceRL(nil)
	return svc, users, email
}

func newServiceRL(rl auth.RateLimiter) (*auth.Service, *mockUsers, *mockEmail, *mockSessions) {
	users := newMockUsers()
	email := &mockEmail{}
	sessions := &mockSessions{}
	svc := auth.NewService(users, sessions, &mockEVs{}, &mockPRs{}, email, rl, auth.Config{
		JWTSecret:     "test-secret-at-least-32-chars!!",
		JWTExpiry:     15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
		AppBaseURL:    "https://example.com",
	})
	return svc, users, email, sessions
}

func TestSignUp_SendsVerificationEmail(t *testing.T) {
	svc, _, email := newService()
	err := svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie", "1.2.3.4")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if len(email.sent) != 1 || email.sent[0] != "verify:a@b.com" {
		t.Errorf("expected verification email, got %v", email.sent)
	}
}

func TestSignUp_WeakPassword(t *testing.T) {
	svc, _, _ := newService()
	err := svc.SignUp(context.Background(), "a@b.com", "weak", "Alice", "en-ie", "1.2.3.4")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

// A signup for an already-registered address must be enumeration-safe: it
// returns success (never ErrEmailExists) and notifies the existing account.
func TestSignUp_DuplicateEmailIsNeutral(t *testing.T) {
	svc, users, email := newService()
	now := time.Now()
	users.users["a@b.com"] = auth.User{ID: "x", Email: "a@b.com", DisplayName: "Alice", EmailVerifiedAt: &now}

	err := svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie", "1.2.3.4")
	if err != nil {
		t.Fatalf("SignUp on duplicate should return nil, got %v", err)
	}
	if len(email.sent) != 1 || email.sent[0] != "exists:a@b.com" {
		t.Errorf("expected account-exists email, got %v", email.sent)
	}
}

// Casing/whitespace variants must resolve to the same account, so a mixed-case
// duplicate signup is still caught by the normalized lookup.
func TestSignUp_NormalizesEmail(t *testing.T) {
	svc, users, email := newService()
	now := time.Now()
	users.users["a@b.com"] = auth.User{ID: "x", Email: "a@b.com", DisplayName: "Alice", EmailVerifiedAt: &now}

	if err := svc.SignUp(context.Background(), "  A@B.com ", "Secure!Pass123", "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if len(email.sent) != 1 || email.sent[0] != "exists:a@b.com" {
		t.Errorf("mixed-case duplicate should hit the existing account, got %v", email.sent)
	}
}

func TestLogin_UnverifiedEmail(t *testing.T) {
	svc, _, _ := newService()
	_ = svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie", "1.2.3.4")
	_, err := svc.Login(context.Background(), "a@b.com", "Secure!Pass123", "1.2.3.4")
	if !errors.Is(err, auth.ErrEmailNotVerified) {
		t.Errorf("expected ErrEmailNotVerified, got %v", err)
	}
}

// An unknown email and a wrong password both return ErrInvalidCreds — the
// endpoint must not disclose which one failed (no enumeration).
func TestLogin_UnknownEmailIsInvalidCreds(t *testing.T) {
	svc, _, _ := newService()
	_, err := svc.Login(context.Background(), "nobody@b.com", "whatever", "1.2.3.4")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Errorf("expected ErrInvalidCreds, got %v", err)
	}
}

// Login is case-insensitive: an account created with lowercase can sign in with
// mixed-case input.
func TestLogin_NormalizesEmail(t *testing.T) {
	svc, users, _ := newService()
	hash, _ := passwordHash(t, "Secure!Pass123")
	now := time.Now()
	users.users["a@b.com"] = auth.User{ID: "x", Email: "a@b.com", PasswordHash: hash, DisplayName: "Alice", EmailVerifiedAt: &now}

	if _, err := svc.Login(context.Background(), "A@B.COM", "Secure!Pass123", "1.2.3.4"); err != nil {
		t.Fatalf("Login with mixed-case email: %v", err)
	}
}

func TestLogin_RateLimitedPerIP(t *testing.T) {
	rl := newMockLimiter()
	rl.deny["login:ip:9.9.9.9"] = true
	svc, _, _, _ := newServiceRL(rl)

	_, err := svc.Login(context.Background(), "a@b.com", "Secure!Pass123", "9.9.9.9")
	if !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

// A limiter error (e.g. Redis down) must fail OPEN — the request proceeds so an
// outage can't lock everyone out. Here the credentials are still wrong, so we
// expect ErrInvalidCreds (proof the request got past the limiter), not
// ErrRateLimited.
func TestLogin_LimiterErrorFailsOpen(t *testing.T) {
	rl := newMockLimiter()
	rl.err = errors.New("redis down")
	svc, _, _, _ := newServiceRL(rl)

	_, err := svc.Login(context.Background(), "a@b.com", "wrong", "9.9.9.9")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Errorf("expected fail-open ErrInvalidCreds, got %v", err)
	}
}

func TestForgotPassword_RateLimitedPerIPReturnsError(t *testing.T) {
	rl := newMockLimiter()
	rl.deny["forgot:ip:9.9.9.9"] = true
	svc, _, _, _ := newServiceRL(rl)

	if err := svc.ForgotPassword(context.Background(), "a@b.com", "en-ie", "9.9.9.9"); !errors.Is(err, auth.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited on IP throttle, got %v", err)
	}
}

// The per-email cap must stay silent (nil, no email sent) so it can't be used
// as an existence oracle.
func TestForgotPassword_PerEmailCapIsSilent(t *testing.T) {
	rl := newMockLimiter()
	rl.deny["forgot:email:a@b.com"] = true
	svc, users, email, _ := newServiceRL(rl)
	now := time.Now()
	users.users["a@b.com"] = auth.User{ID: "x", Email: "a@b.com", EmailVerifiedAt: &now}

	if err := svc.ForgotPassword(context.Background(), "a@b.com", "en-ie", "9.9.9.9"); err != nil {
		t.Fatalf("per-email cap should be silent, got %v", err)
	}
	if len(email.sent) != 0 {
		t.Errorf("expected no reset email when per-email cap hit, got %v", email.sent)
	}
}

// passwordHash bcrypts a plaintext for seeding a user in tests.
func passwordHash(t *testing.T, plain string) (string, error) {
	t.Helper()
	return password.Hash(plain)
}
