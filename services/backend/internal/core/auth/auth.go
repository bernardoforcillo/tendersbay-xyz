// Package auth is the backend's authentication domain. It no longer implements
// authentication itself: the credential model, session families with refresh
// rotation, verification tokens and every timing/enumeration guard around them
// live in github.com/bernardoforcillo/authlayer/auth, and this package is the
// thin layer that adapts that library to what tendersbay owns on top of it —
// display names, localized email links, the Redis rate-limit budgets, and the
// sentinel errors connectapi already maps to ConnectRPC status codes.
//
// The split is deliberate and follows .claude/rules/code-organization.md: the
// library is a capability (one external concern, behind an interface the domain
// calls), this package is the domain decision layer, and connectapi stays a
// transport that validates and delegates. Keeping the sentinels and the method
// signatures stable here is what let the whole migration happen without
// touching the wire contract.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	alauth "github.com/bernardoforcillo/authlayer/auth"
	altoken "github.com/bernardoforcillo/authlayer/token"
)

// Rate-limit budgets for the unauthenticated auth endpoints, keyed by client
// IP. These are generous enough not to trip a real user while making online
// password guessing / signup or reset flooding infeasible. Login is keyed by
// IP only (never by email) so an attacker can't lock a victim out of their own
// account by exhausting an email bucket.
//
// They are enforced HERE rather than through authlayer's own WithRateLimiter /
// WithPasswordResetRateLimiter options, deliberately. authlayer exposes a
// single IP-keyed limiter shared by login and password reset, and it fails
// CLOSED on a limiter error; this service has always run three separate budgets
// and failed OPEN when Redis is unreachable (see allow). Wiring the library's
// options would have quietly changed both. One limiter, three budgets, one
// fail-open policy — all still in one place.
const (
	loginRateMax     = 20
	loginRateWindow  = 15 * time.Minute
	signupRateMax    = 10
	signupRateWindow = time.Hour
	forgotRateMax    = 5
	forgotRateWindow = time.Hour
)

// NormalizeEmail canonicalizes an address for storage and lookup: trimmed and
// lowercased. It delegates to authlayer so this service and the library agree
// byte-for-byte on which rows are the same account — a divergence here would
// let a case variant miss authlayer's uniqueness check.
func NormalizeEmail(email string) string { return alauth.NormalizeEmail(email) }

// User is the profile view of an account: what the rest of the backend renders
// and joins against. The credential fields authlayer owns (password_hash) are
// deliberately absent — nothing outside the library needs them any more.
type User struct {
	ID              string
	Email           string
	DisplayName     string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	// DeletedAt is set when the account was anonymized by
	// authlayer's AnonymizeAccount: the row survives so foreign keys keep
	// resolving, but nobody can authenticate as it.
	DeletedAt *time.Time
}

// Sentinel errors. These are the vocabulary connectapi.toConnectError maps, so
// they are kept stable across the authlayer migration: every authlayer sentinel
// is translated into one of these by MapLibraryError.
var (
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidCreds     = errors.New("invalid credentials")
	ErrEmailNotVerified = errors.New("email not verified")
	ErrTokenInvalid     = errors.New("token expired or invalid")
	ErrWeakPassword     = errors.New("password does not meet requirements")
	ErrNotFound         = errors.New("not found")
	ErrRateLimited      = errors.New("too many attempts; try again later")
)

// UserRepository is the profile port. It is everything left of the old
// user-facing repository once authlayer's auth.Store took over the credential
// columns: two reads and the one write (display_name) the library has no
// opinion about.
type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	UpdateDisplayName(ctx context.Context, id, displayName string) error
}

type EmailSender interface {
	SendVerification(ctx context.Context, to, displayName, link string) error
	SendPasswordReset(ctx context.Context, to, displayName, link string) error
	SendEmailChangeVerification(ctx context.Context, to, displayName, link string) error
	// SendAccountExists notifies an address that a signup was attempted for it
	// while an account already exists — the enumeration-safe alternative to
	// returning an "email already registered" error to the caller.
	SendAccountExists(ctx context.Context, to, displayName string) error
}

// RateLimiter is the narrow slice of the rate limiter the auth service needs.
// Satisfied by *redis.RateLimiter unchanged. Allow reports whether one more
// request under key is permitted within window given a per-window maximum.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type Config struct {
	JWTSecret     string
	JWTExpiry     time.Duration
	RefreshExpiry time.Duration
	AppBaseURL    string
}

type LoginResult struct {
	User         User
	AccessToken  string
	RefreshPlain string
}

// Claims is the access-token payload the transport reads back. Subject is the
// user id and SessionID names the refresh session the token was minted for —
// the value ChangePassword needs so it can revoke every OTHER session without
// logging the caller out of the one they are using.
type Claims = altoken.Claims

// Service wraps authlayer's auth service with this product's own concerns.
type Service struct {
	al    *alauth.Service
	users UserRepository
	email EmailSender
	rl    RateLimiter
	cfg   Config
}

// NewService builds the service over an authlayer store (in production
// *dropsstore.AuthStore) plus the profile repository, mailer and rate limiter
// this package adds on top.
func NewService(
	store alauth.Store,
	users UserRepository,
	email EmailSender,
	rl RateLimiter,
	cfg Config,
) *Service {
	al := alauth.New(store,
		alauth.WithJWT([][]byte{[]byte(cfg.JWTSecret)}, cfg.JWTExpiry),
		alauth.WithRefreshTTL(cfg.RefreshExpiry),
		// Preserves this service's long-standing rule that an unverified
		// address cannot sign in; authlayer's own default is permissive.
		alauth.WithRequireVerifiedEmail(true),
	)
	return &Service{al: al, users: users, email: email, rl: rl, cfg: cfg}
}

// Authlayer exposes the underlying library service, for the few call sites that
// need a capability this wrapper deliberately does not re-export (core/user's
// password and email changes, which are account management rather than
// authentication).
func (s *Service) Authlayer() *alauth.Service { return s.al }

// allow consults the rate limiter for key. It fails OPEN — a nil limiter (tests
// / no Redis configured) or a limiter error (e.g. Redis unreachable) both
// permit the request — so a limiter outage degrades brute-force protection
// rather than locking every user out of a working auth service.
func (s *Service) allow(ctx context.Context, key string, max int64, window time.Duration) bool {
	if s.rl == nil {
		return true
	}
	ok, err := s.rl.Allow(ctx, key, max, window)
	if err != nil {
		slog.WarnContext(ctx, "auth rate limiter unavailable; allowing request", "error", err)
		return true
	}
	return ok
}

// fallbackIP is what a caller with no resolvable address is bucketed under.
// authlayer refuses an empty ip outright (auth.ErrMissingIP), and answering an
// unauthenticated request with an internal error would be worse than sharing
// one bucket between the handful of callers the middleware could not place.
const fallbackIP = "unknown"

func orFallbackIP(ip string) string {
	if ip == "" {
		return fallbackIP
	}
	return ip
}

func (s *Service) SignUp(ctx context.Context, email, plainPassword, displayName, locale, clientIP string) error {
	email = NormalizeEmail(email)
	if clientIP != "" && !s.allow(ctx, "signup:ip:"+clientIP, signupRateMax, signupRateWindow) {
		return ErrRateLimited
	}
	// authlayer's SignUp is enumeration-safe by construction: it performs the
	// identical sequence of store calls on both branches and reports which one
	// it took only in Created, never in the error. What it does NOT do is send
	// mail — that is this layer's job, and it is the one place the two branches
	// legitimately differ, because the whole point of the duplicate branch is
	// to tell the REAL accountholder that someone tried to register their
	// address. The response to the caller stays byte-identical either way.
	res, err := s.al.SignUp(ctx, email, plainPassword)
	if err != nil {
		return MapLibraryError(err)
	}
	if !res.Created {
		existing, ferr := s.users.FindByEmail(ctx, email)
		if ferr != nil {
			// The address is registered (that is why Created is false) but its
			// profile row would not load. Nothing actionable can be sent, and
			// saying so to the caller would leak the very fact this branch
			// exists to hide.
			slog.WarnContext(ctx, "signup duplicate: profile lookup failed", "error", ferr)
			return nil
		}
		return s.email.SendAccountExists(ctx, existing.Email, existing.DisplayName)
	}
	if err := s.users.UpdateDisplayName(ctx, res.User.ID, displayName); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/%s/auth/verify-email?token=%s&type=signup", s.cfg.AppBaseURL, locale, res.VerifyToken)
	return s.email.SendVerification(ctx, email, displayName, link)
}

func (s *Service) Login(ctx context.Context, email, plainPassword, clientIP, userAgent string) (*LoginResult, error) {
	email = NormalizeEmail(email)
	if clientIP != "" && !s.allow(ctx, "login:ip:"+clientIP, loginRateMax, loginRateWindow) {
		return nil, ErrRateLimited
	}
	res, err := s.al.Login(ctx, email, plainPassword, orFallbackIP(clientIP), userAgent)
	if err != nil {
		return nil, MapLibraryError(err)
	}
	if res.MFA != nil {
		// Unreachable while no MFAStore is wired (authlayer issues a challenge
		// only for an enrolled user, and enrolment needs that port). Stated
		// rather than assumed: if MFA is switched on later, this returns a
		// clean error instead of handing back an empty token pair.
		return nil, errors.New("multi-factor authentication is not enabled for this deployment")
	}
	return s.result(ctx, res)
}

func (s *Service) Logout(ctx context.Context, refreshPlain string) error {
	err := s.al.Logout(ctx, refreshPlain)
	// Logging out with a token that is already gone is the ordinary outcome of
	// a double-click or a stale tab, not a failure the client can act on.
	if err != nil && errors.Is(err, alauth.ErrTokenInvalid) {
		return nil
	}
	return MapLibraryError(err)
}

func (s *Service) RefreshToken(ctx context.Context, refreshPlain string) (*LoginResult, error) {
	res, err := s.al.Refresh(ctx, refreshPlain)
	if err != nil {
		return nil, MapLibraryError(err)
	}
	return s.result(ctx, res)
}

func (s *Service) ForgotPassword(ctx context.Context, email, locale, clientIP string) error {
	email = NormalizeEmail(email)
	if clientIP != "" && !s.allow(ctx, "forgot:ip:"+clientIP, forgotRateMax, forgotRateWindow) {
		return ErrRateLimited
	}
	// Cap reset emails per address to stop inbox flooding. Return nil (not
	// ErrRateLimited) so the response stays identical whether the address
	// exists or not — the per-email limit must not become an existence oracle.
	if !s.allow(ctx, "forgot:email:"+email, forgotRateMax, forgotRateWindow) {
		return nil
	}
	plain, known, err := s.al.RequestPasswordReset(ctx, email, orFallbackIP(clientIP))
	if err != nil {
		return MapLibraryError(err)
	}
	if !known {
		return nil // don't reveal whether email exists
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	link := fmt.Sprintf("%s/%s/auth/reset-password?token=%s", s.cfg.AppBaseURL, locale, plain)
	return s.email.SendPasswordReset(ctx, user.Email, user.DisplayName, link)
}

func (s *Service) ResetPassword(ctx context.Context, plainToken, newPassword string) error {
	return MapLibraryError(s.al.ResetPassword(ctx, plainToken, newPassword))
}

// VerifyEmail redeems a signup or email-change token.
//
// verifyType is accepted for wire compatibility with the proto (the frontend
// still round-trips it through the link) but is no longer consulted: authlayer
// reads the purpose off the stored verification row instead of trusting a
// query parameter, and refuses a token whose purpose does not match the flow it
// was minted for. Deriving it from the row is strictly safer than deriving it
// from the URL, so the parameter is ignored rather than re-validated.
func (s *Service) VerifyEmail(ctx context.Context, plainToken, _ string) error {
	_, err := s.al.VerifyEmail(ctx, plainToken)
	return MapLibraryError(err)
}

// PurgeExpired deletes every session and verification token that expired
// before the given instant, and reports how many rows went.
//
// It exists because nothing was calling it: sessions, signup tokens and reset
// tokens all carry an expiry the service honours on read, but a row that is
// merely ignored is still a row, and before this the two tables only ever grew.
// authlayer has always offered the sweep; this is the service scheduling it.
func (s *Service) PurgeExpired(ctx context.Context, before time.Time) (int, error) {
	return s.al.PurgeExpired(ctx, before)
}

// VerifyAccessToken parses and verifies a bearer access token. Used by the
// transport middleware; it performs no store access.
func (s *Service) VerifyAccessToken(raw string) (Claims, error) {
	claims, err := s.al.VerifyAccessToken(raw)
	if err != nil {
		return Claims{}, ErrTokenInvalid
	}
	return claims, nil
}

// result turns an authlayer LoginResult into this package's, joining the
// profile row for the display name the client renders.
func (s *Service) result(ctx context.Context, res alauth.LoginResult) (*LoginResult, error) {
	user, err := s.users.FindByID(ctx, res.User.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		User:         user,
		AccessToken:  res.AccessToken,
		RefreshPlain: res.RefreshToken,
	}, nil
}

// MapLibraryError translates authlayer's sentinels into this package's, so
// connectapi.toConnectError keeps its existing switch and the wire status codes
// do not move. Anything unrecognised is passed through untouched and surfaces
// as CodeInternal, which is the correct answer for a store or transport failure.
//
// It is exported because core/user calls authlayer directly for the three
// account-management flows (password change, email change, deletion) and must
// translate their errors into the same vocabulary this package publishes.
func MapLibraryError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, alauth.ErrWeakPassword):
		// authlayer names the failed rules in the wrapped message; keep them,
		// the client renders them.
		return fmt.Errorf("%w: %s", ErrWeakPassword, strings.TrimPrefix(err.Error(), alauth.ErrWeakPassword.Error()+": "))
	case errors.Is(err, alauth.ErrInvalidCredentials):
		return ErrInvalidCreds
	case errors.Is(err, alauth.ErrEmailNotVerified):
		return ErrEmailNotVerified
	case errors.Is(err, alauth.ErrEmailTaken):
		return ErrEmailExists
	case errors.Is(err, alauth.ErrRateLimited), errors.Is(err, alauth.ErrMissingIP):
		return ErrRateLimited
	case errors.Is(err, alauth.ErrUserNotFound):
		return ErrNotFound
	case errors.Is(err, alauth.ErrTokenInvalid),
		errors.Is(err, alauth.ErrTokenReuse),
		errors.Is(err, alauth.ErrSessionRevoked),
		errors.Is(err, alauth.ErrVerificationExpired),
		errors.Is(err, alauth.ErrVerificationPurpose),
		errors.Is(err, alauth.ErrVerificationNotFound),
		errors.Is(err, alauth.ErrSessionNotFound):
		return ErrTokenInvalid
	default:
		return err
	}
}
