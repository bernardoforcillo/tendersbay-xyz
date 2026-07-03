package connectapi

import (
	"context"

	"connectrpc.com/connect"
	authv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1/authv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type AuthHandler struct {
	svc        *auth.Service
	refreshTTL int // seconds
}

func NewAuthHandler(svc *auth.Service, refreshTTL int) *AuthHandler {
	return &AuthHandler{svc: svc, refreshTTL: refreshTTL}
}

var _ authv1connect.AuthServiceHandler = (*AuthHandler)(nil)

func (h *AuthHandler) SignUp(ctx context.Context, req *connect.Request[authv1.SignUpRequest]) (*connect.Response[authv1.SignUpResponse], error) {
	if err := h.svc.SignUp(ctx, req.Msg.Email, req.Msg.Password, req.Msg.DisplayName, req.Msg.Locale); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&authv1.SignUpResponse{}), nil
}

func (h *AuthHandler) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	result, err := h.svc.Login(ctx, req.Msg.Email, req.Msg.Password)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := connect.NewResponse(&authv1.LoginResponse{
		AccessToken: result.AccessToken,
		User:        toProtoUser(result.User),
	})
	setRefreshCookie(resp.Header(), result.RefreshPlain, h.refreshTTL)
	return resp, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	plain := cookieFromHeader(req.Header(), "refresh_token")
	if err := h.svc.Logout(ctx, plain); err != nil {
		return nil, toConnectError(err)
	}
	resp := connect.NewResponse(&authv1.LogoutResponse{})
	clearRefreshCookie(resp.Header())
	return resp, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *connect.Request[authv1.RefreshTokenRequest]) (*connect.Response[authv1.RefreshTokenResponse], error) {
	plain := cookieFromHeader(req.Header(), "refresh_token")
	result, err := h.svc.RefreshToken(ctx, plain)
	if err != nil {
		return nil, toConnectError(err)
	}
	resp := connect.NewResponse(&authv1.RefreshTokenResponse{
		AccessToken: result.AccessToken,
		User:        toProtoUser(result.User),
	})
	setRefreshCookie(resp.Header(), result.RefreshPlain, h.refreshTTL)
	return resp, nil
}

func (h *AuthHandler) ForgotPassword(ctx context.Context, req *connect.Request[authv1.ForgotPasswordRequest]) (*connect.Response[authv1.ForgotPasswordResponse], error) {
	if err := h.svc.ForgotPassword(ctx, req.Msg.Email, req.Msg.Locale); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&authv1.ForgotPasswordResponse{}), nil
}

func (h *AuthHandler) ResetPassword(ctx context.Context, req *connect.Request[authv1.ResetPasswordRequest]) (*connect.Response[authv1.ResetPasswordResponse], error) {
	if err := h.svc.ResetPassword(ctx, req.Msg.Token, req.Msg.NewPassword); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&authv1.ResetPasswordResponse{}), nil
}

func (h *AuthHandler) VerifyEmail(ctx context.Context, req *connect.Request[authv1.VerifyEmailRequest]) (*connect.Response[authv1.VerifyEmailResponse], error) {
	if err := h.svc.VerifyEmail(ctx, req.Msg.Token, req.Msg.Type); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&authv1.VerifyEmailResponse{}), nil
}

func toProtoUser(u auth.User) *authv1.UserInfo {
	return &authv1.UserInfo{Id: u.ID, Email: u.Email, DisplayName: u.DisplayName}
}
