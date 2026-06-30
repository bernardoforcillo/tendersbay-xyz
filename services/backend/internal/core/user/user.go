package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/password"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/token"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type Service struct {
	users    auth.UserRepository
	sessions auth.SessionRepository
	evs      auth.EmailVerificationRepository
	email    auth.EmailSender
	cfg      auth.Config
}

func NewService(
	users auth.UserRepository,
	sessions auth.SessionRepository,
	evs auth.EmailVerificationRepository,
	email auth.EmailSender,
	cfg auth.Config,
) *Service {
	return &Service{users: users, sessions: sessions, evs: evs, email: email, cfg: cfg}
}

func (s *Service) GetProfile(ctx context.Context, userID string) (auth.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID, displayName string) (auth.User, error) {
	if err := s.users.UpdateDisplayName(ctx, userID, displayName); err != nil {
		return auth.User{}, err
	}
	return s.users.FindByID(ctx, userID)
}

func (s *Service) ChangeEmail(ctx context.Context, userID, newEmail, plainPassword, locale string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return auth.ErrNotFound
	}
	if !password.Verify(plainPassword, user.PasswordHash) {
		return auth.ErrInvalidCreds
	}
	_ = s.evs.DeleteByUserID(ctx, userID)
	plain, tokenHash, err := token.GenerateOpaque()
	if err != nil {
		return err
	}
	if _, err = s.evs.Create(ctx, auth.EmailVerification{
		UserID:    userID,
		NewEmail:  newEmail,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return err
	}
	link := fmt.Sprintf("%s/%s/auth/verify-email?token=%s&type=email-change", s.cfg.AppBaseURL, locale, plain)
	return s.email.SendEmailChangeVerification(ctx, newEmail, user.DisplayName, link)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return auth.ErrNotFound
	}
	if !password.Verify(currentPassword, user.PasswordHash) {
		return auth.ErrInvalidCreds
	}
	if fails := password.Validate(newPassword); len(fails) > 0 {
		return fmt.Errorf("%w: %s", auth.ErrWeakPassword, strings.Join(fails, ", "))
	}
	hash, err := password.Hash(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.sessions.DeleteByUserID(ctx, userID)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, plainPassword string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return auth.ErrNotFound
	}
	if !password.Verify(plainPassword, user.PasswordHash) {
		return auth.ErrInvalidCreds
	}
	return s.users.Delete(ctx, userID)
}
