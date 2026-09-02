package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth/authtest"
	coreuser "github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
)

type fixture struct {
	authSvc  *auth.Service
	userSvc  *coreuser.Service
	profiles *authtest.Profiles
	mail     *authtest.Mailer

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
	prof := authtest.NewProfiles(store)
	mail := &authtest.Mailer{}
	cfg := auth.Config{
		JWTSecret:     authtest.Secret,
		JWTExpiry:     15 * time.Minute,
		RefreshExpiry: 168 * time.Hour,
		AppBaseURL:    "https://app.test",
	}
	authSvc := auth.NewService(store, prof, mail, nil, cfg)
	userSvc := coreuser.NewService(authSvc.Authlayer(), prof, mail, cfg)

	if err := authSvc.SignUp(ctx, "a@b.com", authtest.Strong, "Alice", "en-ie", "1.2.3.4"); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if err := authSvc.VerifyEmail(ctx, authtest.TokenFrom(t, mail.Last(t, authtest.KindVerification).Link), "signup"); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	login, err := authSvc.Login(ctx, "a@b.com", authtest.Strong, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	claims, err := authSvc.VerifyAccessToken(login.AccessToken)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	mail.Reset()
	return &fixture{
		authSvc: authSvc, userSvc: userSvc, profiles: prof, mail: mail,
		userID: login.User.ID, sessionID: claims.SessionID, refresh: login.RefreshPlain,
	}
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
	err := f.userSvc.ChangePassword(context.Background(), f.userID, f.sessionID, authtest.Strong, "short")
	if !errors.Is(err, auth.ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
}

// The caller keeps the session they are working in; everything else is a
// credential minted under the old password and goes.
func TestChangePassword_SparesTheCallersOwnSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	other, err := f.authSvc.Login(ctx, "a@b.com", authtest.Strong, "9.9.9.9", "other-agent")
	if err != nil {
		t.Fatalf("second Login: %v", err)
	}
	const next = "N3w!Passphrase42"
	if err := f.userSvc.ChangePassword(ctx, f.userID, f.sessionID, authtest.Strong, next); err != nil {
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
	if err := f.userSvc.ChangeEmail(ctx, f.userID, f.sessionID, "New@B.com", authtest.Strong, "it-it"); err != nil {
		t.Fatalf("ChangeEmail: %v", err)
	}
	sent := f.mail.Last(t, authtest.KindEmailChange)
	if sent.To != "new@b.com" {
		t.Fatalf("mailed %q, want the normalized new address", sent.To)
	}
	before, err := f.profiles.FindByID(ctx, f.userID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if before.Email != "a@b.com" {
		t.Fatalf("address moved before redemption: %q", before.Email)
	}

	if err := f.authSvc.VerifyEmail(ctx, authtest.TokenFrom(t, sent.Link), authtest.KindEmailChange); err != nil {
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
	if len(f.mail.Sent) != 0 {
		t.Fatalf("no mail may be sent without the password, got %+v", f.mail.Sent)
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
	if err := f.userSvc.DeleteAccount(ctx, f.userID, f.sessionID, authtest.Strong); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := f.profiles.FindByID(ctx, f.userID); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := f.authSvc.Login(ctx, "a@b.com", authtest.Strong, "1.2.3.4", "test-agent"); !errors.Is(err, auth.ErrInvalidCreds) {
		t.Fatalf("a deleted account could still sign in: %v", err)
	}
}
