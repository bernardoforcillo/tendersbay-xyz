package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/token"
)

// Domain types

type User struct {
	ID              string
	Email           string
	PasswordHash    string
	DisplayName     string
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
)

// Repository interfaces

type UserRepository interface {
	Create(ctx context.Context, u User) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	UpdatePassword(ctx context.Context, id, hash string) error
	UpdateEmail(ctx context.Context, id, email string) error
	UpdateDisplayName(ctx context.Context, id, displayName string) error
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
	cfg      Config
}

func NewService(
	users UserRepository,
	sessions SessionRepository,
	evs EmailVerificationRepository,
	prs PasswordResetRepository,
	email EmailSender,
	cfg Config,
) *Service {
	return &Service{users: users, sessions: sessions, evs: evs, prs: prs, email: email, cfg: cfg}
}

func (s *Service) SignUp(ctx context.Context, email, plainPassword, displayName, locale string) error {
	if fails := password.Validate(plainPassword); len(fails) > 0 {
		return fmt.Errorf("%w: %s", ErrWeakPassword, strings.Join(fails, ", "))
	}
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return ErrEmailExists
	}
	hash, err := password.Hash(plainPassword)
	if err != nil {
		return err
	}
	user, err := s.users.Create(ctx, User{Email: email, PasswordHash: hash, DisplayName: displayName})
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
	link := fmt.Sprintf("%s/%s/auth/verify-email?token=%s&type=signup", s.cfg.AppBaseURL, locale, plain)
	return s.email.SendVerification(ctx, email, displayName, link)
}

func (s *Service) Login(ctx context.Context, email, plainPassword string) (*LoginResult, error) {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
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
	if err != nil || session.ExpiresAt.Before(time.Now()) {
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

func (s *Service) ForgotPassword(ctx context.Context, email, locale string) error {
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
