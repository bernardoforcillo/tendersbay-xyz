// Package user is account self-management: the profile a member renders and
// the three sensitive changes an authenticated user can make to their own
// account. The credential half of each of those changes belongs to
// github.com/bernardoforcillo/authlayer/auth — this package decides what the
// product does around it (which address gets the mail, what the link looks
// like) and nothing else.
package user

import (
	"context"
	"fmt"

	alauth "github.com/bernardoforcillo/authlayer/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// emailChangeLinkPath is the front-end route that redeems an email-change
// token. type=email-change is kept in the query string because the SPA routes
// on it; the backend no longer trusts it (authlayer reads the purpose off the
// stored row) — see auth.Service.VerifyEmail.
const emailChangeLinkPath = "%s/%s/auth/verify-email?token=%s&type=email-change"

type Service struct {
	al    *alauth.Service
	users auth.UserRepository
	email auth.EmailSender
	cfg   auth.Config
}

// NewService takes the authlayer service already built by core/auth rather
// than building a second one: two services over the same store would each
// carry their own clock, hasher and signing keys, and the first divergence
// between them would be silent.
func NewService(
	al *alauth.Service,
	users auth.UserRepository,
	email auth.EmailSender,
	cfg auth.Config,
) *Service {
	return &Service{al: al, users: users, email: email, cfg: cfg}
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

// ChangeEmail mints an email-change verification for newEmail and mails the
// link to it. The address does not move until the link is redeemed
// (auth.Service.VerifyEmail), so a typo costs a lost email rather than a lost
// account.
func (s *Service) ChangeEmail(ctx context.Context, userID, sessionID, newEmail, plainPassword, locale string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	plain, err := s.al.RequestEmailChange(ctx, userID, sessionID, plainPassword, newEmail)
	if err != nil {
		return auth.MapLibraryError(err)
	}
	link := fmt.Sprintf(emailChangeLinkPath, s.cfg.AppBaseURL, locale, plain)
	return s.email.SendEmailChangeVerification(ctx, alauth.NormalizeEmail(newEmail), user.DisplayName, link)
}

// ChangePassword rotates the credential and revokes every OTHER session, plus
// every outstanding reset / email-change / magic-link token. sessionID names
// the caller's own session so they are not logged out of the tab they are
// using.
func (s *Service) ChangePassword(ctx context.Context, userID, sessionID, currentPassword, newPassword string) error {
	return auth.MapLibraryError(s.al.ChangePassword(ctx, userID, sessionID, currentPassword, newPassword))
}

// DeleteAccount removes the account outright, after re-proving the password.
// This is the hard posture: the users row is gone, and so is everything keyed
// to it by ON DELETE CASCADE. authlayer also offers AnonymizeAccount, which
// keeps the row so foreign keys resolve; switching postures is a one-line
// change here if retention rules ever require it.
func (s *Service) DeleteAccount(ctx context.Context, userID, sessionID, plainPassword string) error {
	return auth.MapLibraryError(s.al.DeleteAccount(ctx, userID, sessionID, plainPassword))
}
