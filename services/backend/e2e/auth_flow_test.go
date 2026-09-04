package e2e

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	authv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1"
	userv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1"
)

const strongPassword = "Correct-Horse-Battery-Staple-9"

// account is a signed-up, verified, logged-in user: their own session (cookie
// jar and clients), their access token, and the address the mail went to.
type account struct {
	clients
	email, userID, access string
}

// signUp walks the whole front door: sign up, read the verification mail,
// verify, log in. Every test below needs a real account, and building one any
// other way — inserting rows, minting a token — would skip the part of the
// stack most likely to be wrong.
func (s *stack) signUp(t *testing.T, name string) *account {
	t.Helper()
	ctx := context.Background()
	c := s.newClients(t)
	email := uniq(name) + "@e2e.test"

	if _, err := c.auth.SignUp(ctx, connect.NewRequest(&authv1.SignUpRequest{
		Email: email, Password: strongPassword, DisplayName: "E2E " + name, Locale: "en-ie",
	})); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := c.auth.VerifyEmail(ctx, connect.NewRequest(&authv1.VerifyEmailRequest{
		Token: s.mail.lastTo(t, "verification", email), Type: "signup",
	})); err != nil {
		t.Fatalf("VerifyEmail: %v", err)
	}
	login, err := c.auth.Login(ctx, connect.NewRequest(&authv1.LoginRequest{
		Email: email, Password: strongPassword,
	}))
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return &account{clients: c, email: email, userID: login.Msg.User.Id, access: login.Msg.AccessToken}
}

// The journey every user takes before anything else works, end to end: the
// credential is stored by authlayer, the token travels by mail, the session is
// signed here and read back by the middleware on the next call.
func TestSignUpVerifyLogin(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "signup")

	me, err := a.user.GetProfile(context.Background(),
		authed(&userv1.GetProfileRequest{}, a.access))
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if me.Msg.User.Email != a.email {
		t.Fatalf("GetProfile returned %q, want %q", me.Msg.User.Email, a.email)
	}
}

// An unverified account must not be able to log in, or the verification mail
// gates nothing at all.
func TestLoginBeforeVerificationIsRefused(t *testing.T) {
	s := newStack(t)
	c := s.newClients(t)
	ctx := context.Background()
	email := uniq("unverified") + "@e2e.test"

	if _, err := c.auth.SignUp(ctx, connect.NewRequest(&authv1.SignUpRequest{
		Email: email, Password: strongPassword, DisplayName: "Unverified", Locale: "en-ie",
	})); err != nil {
		t.Fatalf("SignUp: %v", err)
	}
	if _, err := c.auth.Login(ctx, connect.NewRequest(&authv1.LoginRequest{
		Email: email, Password: strongPassword,
	})); err == nil {
		t.Fatal("an unverified account logged in")
	}
}

// Signing up for an address that already exists must not tell the caller so.
// The account holder is warned by mail instead — that is the design, and the
// only place to check it is from outside, where an enumeration oracle would be
// visible to whoever is probing.
func TestSignUpDoesNotDiscloseAnExistingAccount(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "enumeration")
	c := s.newClients(t)
	ctx := context.Background()

	fresh, err := c.auth.SignUp(ctx, connect.NewRequest(&authv1.SignUpRequest{
		Email: uniq("stranger") + "@e2e.test", Password: strongPassword,
		DisplayName: "Stranger", Locale: "en-ie",
	}))
	if err != nil {
		t.Fatalf("SignUp for a new address: %v", err)
	}
	taken, err := c.auth.SignUp(ctx, connect.NewRequest(&authv1.SignUpRequest{
		Email: a.email, Password: strongPassword, DisplayName: "Impostor", Locale: "en-ie",
	}))
	if err != nil {
		t.Fatalf("signing up for a taken address failed loudly: %v", err)
	}
	// The same answer both times, or the difference IS the oracle.
	if taken.Msg.String() != fresh.Msg.String() {
		t.Fatalf("a taken address answered %q and a free one %q", taken.Msg, fresh.Msg)
	}
	if s.mail.countTo("account_exists", a.email) == 0 {
		t.Fatal("the account holder was not warned that somebody tried to sign up as them")
	}
	if n := s.mail.countTo("verification", a.email); n != 1 {
		t.Fatalf("%d verification mails went to an address that already has an account, want 1", n)
	}
}

// Refresh rotation and replay detection — the property the session family
// exists for. The token lives in a cookie, so this is also the check that the
// handler sets and reads it: a rotation the client never receives is a session
// that ends at the first refresh.
func TestRefreshRotationDetectsReplay(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "rotation")
	ctx := context.Background()

	original := a.refreshCookie(t)
	if _, err := a.auth.RefreshToken(ctx, connect.NewRequest(&authv1.RefreshTokenRequest{})); err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	rotated := a.refreshCookie(t)
	if rotated == original {
		t.Fatal("rotation left the same refresh token in the jar")
	}

	// The successor works.
	if _, err := a.auth.RefreshToken(ctx, connect.NewRequest(&authv1.RefreshTokenRequest{})); err != nil {
		t.Fatalf("the rotated token was refused: %v", err)
	}

	// Replaying the original must fail: it was spent two rotations ago.
	a.setRefreshCookie(t, original)
	if _, err := a.auth.RefreshToken(ctx, connect.NewRequest(&authv1.RefreshTokenRequest{})); err == nil {
		t.Fatal("a spent refresh token was accepted a second time")
	}
}

// Logging out ends the session on the server, not only in the browser. Both
// halves are checked: the cookie is cleared, AND presenting the token it held
// no longer rotates — a logout that only cleared the cookie would leave a live
// session for anyone who had copied it.
func TestLogoutEndsTheSession(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "logout")
	ctx := context.Background()

	held := a.refreshCookie(t)
	if _, err := a.auth.Logout(ctx, authed(&authv1.LogoutRequest{}, a.access)); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if got := a.cookie("refresh_token"); got != "" {
		t.Fatalf("the refresh cookie survived logout: %q", got)
	}

	a.setRefreshCookie(t, held)
	if _, err := a.auth.RefreshToken(ctx, connect.NewRequest(&authv1.RefreshTokenRequest{})); err == nil {
		t.Fatal("a logged-out refresh token still rotates")
	}
}

// A password reset completes end to end and the old credential stops working —
// the check that it replaced the stored hash rather than adding a second one.
func TestPasswordResetReplacesTheCredential(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "reset")
	ctx := context.Background()

	if _, err := a.auth.ForgotPassword(ctx, connect.NewRequest(&authv1.ForgotPasswordRequest{
		Email: a.email, Locale: "en-ie",
	})); err != nil {
		t.Fatalf("ForgotPassword: %v", err)
	}
	const next = "A-Different-Correct-Horse-77"
	if _, err := a.auth.ResetPassword(ctx, connect.NewRequest(&authv1.ResetPasswordRequest{
		Token: s.mail.lastTo(t, "password_reset", a.email), NewPassword: next,
	})); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	fresh := s.newClients(t)
	if _, err := fresh.auth.Login(ctx, connect.NewRequest(&authv1.LoginRequest{
		Email: a.email, Password: strongPassword,
	})); err == nil {
		t.Fatal("the old password still works after a reset")
	}
	if _, err := fresh.auth.Login(ctx, connect.NewRequest(&authv1.LoginRequest{
		Email: a.email, Password: next,
	})); err != nil {
		t.Fatalf("the new password does not work after a reset: %v", err)
	}
}

// An anonymous caller reaches nothing behind the middleware, and is told so
// with the code the client branches on.
func TestUnauthenticatedCallsAreRefused(t *testing.T) {
	s := newStack(t)
	_, err := s.anon.user.GetProfile(context.Background(), connect.NewRequest(&userv1.GetProfileRequest{}))
	if got := codeOf(err); got != connect.CodeUnauthenticated {
		t.Fatalf("GetProfile without a token = %v (%v), want unauthenticated", got, err)
	}
}
