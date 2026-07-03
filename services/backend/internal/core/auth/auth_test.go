package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
func (m *mockUsers) UpdateEmail(_ context.Context, _, _ string) error        { return nil }
func (m *mockUsers) UpdateDisplayName(_ context.Context, _, _ string) error  { return nil }
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
func (m *mockSessions) Delete(_ context.Context, _ string) error        { return nil }
func (m *mockSessions) DeleteByUserID(_ context.Context, _ string) error { return nil }

type mockEVs struct{}

func (m *mockEVs) Create(_ context.Context, ev auth.EmailVerification) (auth.EmailVerification, error) {
	ev.ID = "ev-1"
	return ev, nil
}
func (m *mockEVs) FindByTokenHash(_ context.Context, _ string) (auth.EmailVerification, error) {
	return auth.EmailVerification{}, auth.ErrNotFound
}
func (m *mockEVs) Delete(_ context.Context, _ string) error        { return nil }
func (m *mockEVs) DeleteByUserID(_ context.Context, _ string) error { return nil }

type mockPRs struct{}

func (m *mockPRs) Create(_ context.Context, pr auth.PasswordReset) (auth.PasswordReset, error) {
	pr.ID = "pr-1"
	return pr, nil
}
func (m *mockPRs) FindByTokenHash(_ context.Context, _ string) (auth.PasswordReset, error) {
	return auth.PasswordReset{}, auth.ErrNotFound
}
func (m *mockPRs) Delete(_ context.Context, _ string) error        { return nil }
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

// --- tests ---

func newService() (*auth.Service, *mockUsers, *mockEmail) {
	users := newMockUsers()
	email := &mockEmail{}
	svc := auth.NewService(users, &mockSessions{}, &mockEVs{}, &mockPRs{}, email, auth.Config{
		JWTSecret:     "test-secret-at-least-32-chars!!",
		JWTExpiry:     15 * time.Minute,
		RefreshExpiry: 7 * 24 * time.Hour,
		AppBaseURL:    "https://example.com",
	})
	return svc, users, email
}

func TestSignUp_SendsVerificationEmail(t *testing.T) {
	svc, _, email := newService()
	err := svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie")
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if len(email.sent) != 1 || email.sent[0] != "verify:a@b.com" {
		t.Errorf("expected verification email, got %v", email.sent)
	}
}

func TestSignUp_WeakPassword(t *testing.T) {
	svc, _, _ := newService()
	err := svc.SignUp(context.Background(), "a@b.com", "weak", "Alice", "en-ie")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	svc, users, _ := newService()
	now := time.Now()
	users.users["a@b.com"] = auth.User{ID: "x", Email: "a@b.com", EmailVerifiedAt: &now}
	err := svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie")
	if !errors.Is(err, auth.ErrEmailExists) {
		t.Errorf("expected ErrEmailExists, got %v", err)
	}
}

func TestLogin_UnverifiedEmail(t *testing.T) {
	svc, _, _ := newService()
	_ = svc.SignUp(context.Background(), "a@b.com", "Secure!Pass123", "Alice", "en-ie")
	_, err := svc.Login(context.Background(), "a@b.com", "Secure!Pass123")
	if !errors.Is(err, auth.ErrEmailNotVerified) {
		t.Errorf("expected ErrEmailNotVerified, got %v", err)
	}
}
