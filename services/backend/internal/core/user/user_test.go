package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	coreuser "github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
)

type mockUserStore struct{ u auth.User }

func (m *mockUserStore) Create(_ context.Context, u auth.User) (auth.User, error) { return u, nil }
func (m *mockUserStore) FindByEmail(_ context.Context, _ string) (auth.User, error) {
	return m.u, nil
}
func (m *mockUserStore) FindByID(_ context.Context, _ string) (auth.User, error) { return m.u, nil }
func (m *mockUserStore) UpdatePassword(_ context.Context, _, hash string) error {
	m.u.PasswordHash = hash
	return nil
}
func (m *mockUserStore) UpdateEmail(_ context.Context, _, email string) error {
	m.u.Email = email
	return nil
}
func (m *mockUserStore) UpdateDisplayName(_ context.Context, _, name string) error {
	m.u.DisplayName = name
	return nil
}
func (m *mockUserStore) MarkEmailVerified(_ context.Context, _ string, at time.Time) error {
	m.u.EmailVerifiedAt = &at
	return nil
}
func (m *mockUserStore) Delete(_ context.Context, _ string) error { return nil }

type nopSessions struct{}

func (n *nopSessions) Create(_ context.Context, s auth.Session) (auth.Session, error) {
	return s, nil
}
func (n *nopSessions) FindByTokenHash(_ context.Context, _ string) (auth.Session, error) {
	return auth.Session{}, auth.ErrNotFound
}
func (n *nopSessions) Delete(_ context.Context, _ string) error        { return nil }
func (n *nopSessions) DeleteByUserID(_ context.Context, _ string) error { return nil }

type nopEVs struct{}

func (n *nopEVs) Create(_ context.Context, ev auth.EmailVerification) (auth.EmailVerification, error) {
	return ev, nil
}
func (n *nopEVs) FindByTokenHash(_ context.Context, _ string) (auth.EmailVerification, error) {
	return auth.EmailVerification{}, auth.ErrNotFound
}
func (n *nopEVs) Delete(_ context.Context, _ string) error        { return nil }
func (n *nopEVs) DeleteByUserID(_ context.Context, _ string) error { return nil }

type nopEmail struct{}

func (n *nopEmail) SendVerification(_ context.Context, _, _, _ string) error            { return nil }
func (n *nopEmail) SendPasswordReset(_ context.Context, _, _, _ string) error           { return nil }
func (n *nopEmail) SendEmailChangeVerification(_ context.Context, _, _, _ string) error { return nil }

func TestChangePassword_WrongCurrent(t *testing.T) {
	hash, _ := password.Hash("Secure!Pass123")
	store := &mockUserStore{u: auth.User{ID: "u1", PasswordHash: hash}}
	svc := coreuser.NewService(store, &nopSessions{}, &nopEVs{}, &nopEmail{}, auth.Config{
		JWTSecret: "test-secret-at-least-32-chars!!",
		JWTExpiry: 15 * time.Minute,
	})
	err := svc.ChangePassword(context.Background(), "u1", "wrongpassword", "NewSecure!Pass456")
	if !errors.Is(err, auth.ErrInvalidCreds) {
		t.Errorf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestUpdateProfile(t *testing.T) {
	store := &mockUserStore{u: auth.User{ID: "u1", DisplayName: "Old Name"}}
	svc := coreuser.NewService(store, &nopSessions{}, &nopEVs{}, &nopEmail{}, auth.Config{})
	u, err := svc.UpdateProfile(context.Background(), "u1", "New Name")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if u.DisplayName != "New Name" {
		t.Errorf("DisplayName = %q, want %q", u.DisplayName, "New Name")
	}
}
