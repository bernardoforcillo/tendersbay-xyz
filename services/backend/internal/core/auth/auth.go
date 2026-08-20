package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/token"
)

// Rate-limit budgets for the unauthenticated auth endpoints, keyed by client
// IP. These are generous enough not to trip a real user while making online
// password guessing / signup or reset flooding infeasible. Login is keyed by
// IP only (never by email) so an attacker can't lock a victim out of their own
// account by exhausting an email bucket.
const (
	loginRateMax     = 20
	loginRateWindow  = 15 * time.Minute
	signupRateMax    = 10
	signupRateWindow = time.Hour
	forgotRateMax    = 5
	forgotRateWindow = time.Hour
)

// NormalizeEmail canonicalizes an address for storage and lookup: trimmed and
// lowercased. Applied on every read and write so a case or whitespace variant
// can't create a duplicate account or slip past a uniqueness check.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Domain types

type User struct {
	ID           string
	Email        string
	PasswordHash string
	DisplayName  string
	// Locale is the user's chosen interface and email language, as one of
	// SupportedLocales; "" means nobody has told us. It is NOT defaulted to
	// English on read — see NormalizeLocale for why the absence has to stay
	// visible.
	Locale          string
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type EmailVerification struct {
	ID        string
	UserID    string
	NewEmail  string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type PasswordReset struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Sentinel errors

var (
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidCreds     = errors.New("invalid credentials")
	ErrEmailNotVerified = errors.New("email not verified")
	ErrTokenInvalid     = errors.New("token expired or invalid")
	ErrWeakPassword     = errors.New("password does not meet requirements")
	ErrNotFound         = errors.New("not found")
	ErrRateLimited      = errors.New("too many attempts; try again later")
)

// Repository interfaces

type UserRepository interface {
	Create(ctx context.Context, u User) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	UpdatePassword(ctx context.Context, id, hash string) error
	UpdateEmail(ctx context.Context, id, email string) error
	UpdateDisplayName(ctx context.Context, id, displayName string) error
	// UpdateLocale stores a validated locale. Callers must pass a value that
	// came through NormalizeLocale; the repository does not re-validate,
	// because two validators drift.
	UpdateLocale(ctx context.Context, id, locale string) error
	MarkEmailVerified(ctx context.Context, id string, at time.Time) error
	Delete(ctx context.Context, id string) error
}

type SessionRepository interface {
	Create(ctx context.Context, s Session) (Session, error)
	FindByTokenHash(ctx context.Context, hash string) (Session, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

type EmailVerificationRepository interface {
	Create(ctx context.Context, ev EmailVerification) (EmailVerification, error)
	FindByTokenHash(ctx context.Context, hash string) (EmailVerification, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

type PasswordResetRepository interface {
	Create(ctx context.Context, pr PasswordReset) (PasswordReset, error)
	FindByTokenHash(ctx context.Context, hash string) (PasswordReset, error)
	Delete(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
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

// Service config and result types

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

// Service

type Service struct {
	users    UserRepository
	sessions SessionRepository
	evs      EmailVerificationRepository
	prs      PasswordResetRepository
	email    EmailSender
	rl       RateLimiter
	cfg      Config
}

func NewService(
	users UserRepository,
	sessions SessionRepository,
	evs EmailVerificationRepository,
	prs PasswordResetRepository,
	email EmailSender,
	rl RateLimiter,
	cfg Config,
) *Service {
	return &Service{users: users, sessions: sessions, evs: evs, prs: prs, email: email, rl: rl, cfg: cfg}
}

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

func (s *Service) SignUp(ctx context.Context, email, plainPassword, displayName, locale, clientIP string) error {
	email = NormalizeEmail(email)
	if clientIP != "" && !s.allow(ctx, "signup:ip:"+clientIP, signupRateMax, signupRateWindow) {
		return ErrRateLimited
	}
	if fails := password.Validate(plainPassword); len(fails) > 0 {
		return fmt.Errorf("%w: %s", ErrWeakPassword, strings.Join(fails, ", "))
	}
	// Enumeration-safe duplicate handling: never disclose to the caller whether
	// the address is already registered. If it is, notify the existing account
	// by email and return success — the response is byte-identical to a genuine
	// new signup, so the endpoint can't be used to probe which emails exist.
	if existing, err := s.users.FindByEmail(ctx, email); err == nil {
		_ = s.email.SendAccountExists(ctx, existing.Email, existing.DisplayName)
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return err
	}
	// Persist the locale the signup form (or the browser) told us. Until this
	// line existed, the argument was used ONLY to build the verification link
	// below and then thrown away — so every user in the system had told us their
	// language and nothing kept it. Normalising here as well as at the edge is
	// cheap and idempotent, and it means a caller that forgets cannot store a
	// tag no renderer ships.
	stored := NormalizeLocale(locale)
	user, err := s.users.Create(ctx, User{
		Email: email, PasswordHash: hash, DisplayName: displayName, Locale: stored,
	})
	if err != nil {
		return err
	}
	plain, tokenHash, err := token.GenerateOpaque()
	if err != nil {
		return err
	}
	if _, err = s.evs.Create(ctx, EmailVerification{
		UserID:    user.ID,
		NewEmail:  email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return err
	}
	// The LINK needs a path segment even when we have no locale to store, or
	// the URL collapses to "//auth/verify-email" and 404s. So "" is a real
	// stored value and never a real URL: the two uses diverge deliberately here.
	linkLocale := stored
	if linkLocale == "" {
		linkLocale = defaultLinkLocale
	}
	link := fmt.Sprintf("%s/%s/auth/verify-email?token=%s&type=signup", s.cfg.AppBaseURL, linkLocale, plain)
	return s.email.SendVerification(ctx, email, displayName, link)
}

func (s *Service) Login(ctx context.Context, email, plainPassword, clientIP string) (*LoginResult, error) {
	email = NormalizeEmail(email)
	if clientIP != "" && !s.allow(ctx, "login:ip:"+clientIP, loginRateMax, loginRateWindow) {
		return nil, ErrRateLimited
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Spend the same ~bcrypt time a real check would, so the response
		// latency can't reveal whether this email is registered.
		password.VerifyDummy(plainPassword)
		return nil, ErrInvalidCreds
	}
	if !password.Verify(plainPassword, user.PasswordHash) {
		return nil, ErrInvalidCreds
	}
	if user.EmailVerifiedAt == nil {
		return nil, ErrEmailNotVerified
	}
	accessToken, err := token.IssueJWT(token.Claims{
		UserID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
	}, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, err
	}
	plain, tokenHash, err := token.GenerateOpaque()
	if err != nil {
		return nil, err
	}
	if _, err = s.sessions.Create(ctx, Session{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.cfg.RefreshExpiry),
	}); err != nil {
		return nil, err
	}
	return &LoginResult{User: user, AccessToken: accessToken, RefreshPlain: plain}, nil
}

func (s *Service) Logout(ctx context.Context, refreshPlain string) error {
	hash := hashOpaque(refreshPlain)
	session, err := s.sessions.FindByTokenHash(ctx, hash)
	if err != nil {
		return nil
	}
	return s.sessions.Delete(ctx, session.ID)
}

func (s *Service) RefreshToken(ctx context.Context, refreshPlain string) (*LoginResult, error) {
	hash := hashOpaque(refreshPlain)
	session, err := s.sessions.FindByTokenHash(ctx, hash)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if session.ExpiresAt.Before(time.Now()) {
		// Delete the dead row rather than leave expired sessions to accumulate.
		_ = s.sessions.Delete(ctx, session.ID)
		return nil, ErrTokenInvalid
	}
	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	if err := s.sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}
	newPlain, newHash, err := token.GenerateOpaque()
	if err != nil {
		return nil, err
	}
	if _, err = s.sessions.Create(ctx, Session{
		UserID:    user.ID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(s.cfg.RefreshExpiry),
	}); err != nil {
		return nil, err
	}
	accessToken, err := token.IssueJWT(token.Claims{
		UserID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
	}, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: user, AccessToken: accessToken, RefreshPlain: newPlain}, nil
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
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return nil // don't reveal whether email exists
	}
	_ = s.prs.DeleteByUserID(ctx, user.ID)
	plain, tokenHash, err := token.GenerateOpaque()
	if err != nil {
		return err
	}
	if _, err = s.prs.Create(ctx, PasswordReset{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/%s/auth/reset-password?token=%s", s.cfg.AppBaseURL, locale, plain)
	return s.email.SendPasswordReset(ctx, user.Email, user.DisplayName, link)
}

func (s *Service) ResetPassword(ctx context.Context, plainToken, newPassword string) error {
	if fails := password.Validate(newPassword); len(fails) > 0 {
		return fmt.Errorf("%w: %s", ErrWeakPassword, strings.Join(fails, ", "))
	}
	hash := hashOpaque(plainToken)
	pr, err := s.prs.FindByTokenHash(ctx, hash)
	if err != nil || pr.ExpiresAt.Before(time.Now()) {
		return ErrTokenInvalid
	}
	if err := s.prs.Delete(ctx, pr.ID); err != nil {
		return err
	}
	newHash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, pr.UserID, newHash); err != nil {
		return err
	}
	return s.sessions.DeleteByUserID(ctx, pr.UserID)
}

func (s *Service) VerifyEmail(ctx context.Context, plainToken, verifyType string) error {
	hash := hashOpaque(plainToken)
	ev, err := s.evs.FindByTokenHash(ctx, hash)
	if err != nil || ev.ExpiresAt.Before(time.Now()) {
		return ErrTokenInvalid
	}
	if err := s.evs.Delete(ctx, ev.ID); err != nil {
		return err
	}
	now := time.Now()
	switch verifyType {
	case "signup":
		return s.users.MarkEmailVerified(ctx, ev.UserID, now)
	case "email-change":
		if err := s.users.UpdateEmail(ctx, ev.UserID, ev.NewEmail); err != nil {
			return err
		}
		return s.users.MarkEmailVerified(ctx, ev.UserID, now)
	default:
		return fmt.Errorf("unknown verification type: %q", verifyType)
	}
}

func hashOpaque(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
