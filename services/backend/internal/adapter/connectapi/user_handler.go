package connectapi

import (
	"context"

	"connectrpc.com/connect"
	userv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1/userv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
)

type UserHandler struct{ svc *user.Service }

func NewUserHandler(svc *user.Service) *UserHandler { return &UserHandler{svc: svc} }

var _ userv1connect.UserServiceHandler = (*UserHandler)(nil)

func (h *UserHandler) GetProfile(ctx context.Context, _ *connect.Request[userv1.GetProfileRequest]) (*connect.Response[userv1.GetProfileResponse], error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	u, err := h.svc.GetProfile(ctx, id)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&userv1.GetProfileResponse{User: toProtoUser(u)}), nil
}

func (h *UserHandler) UpdateProfile(ctx context.Context, req *connect.Request[userv1.UpdateProfileRequest]) (*connect.Response[userv1.UpdateProfileResponse], error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	u, err := h.svc.UpdateProfile(ctx, id, req.Msg.DisplayName)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&userv1.UpdateProfileResponse{User: toProtoUser(u)}), nil
}

func (h *UserHandler) ChangeEmail(ctx context.Context, req *connect.Request[userv1.ChangeEmailRequest]) (*connect.Response[userv1.ChangeEmailResponse], error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	if err := h.svc.ChangeEmail(ctx, id, req.Msg.NewEmail, req.Msg.Password, req.Msg.Locale); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&userv1.ChangeEmailResponse{}), nil
}

func (h *UserHandler) ChangePassword(ctx context.Context, req *connect.Request[userv1.ChangePasswordRequest]) (*connect.Response[userv1.ChangePasswordResponse], error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	if err := h.svc.ChangePassword(ctx, id, req.Msg.CurrentPassword, req.Msg.NewPassword); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&userv1.ChangePasswordResponse{}), nil
}

func (h *UserHandler) DeleteAccount(ctx context.Context, req *connect.Request[userv1.DeleteAccountRequest]) (*connect.Response[userv1.DeleteAccountResponse], error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	if err := h.svc.DeleteAccount(ctx, id, req.Msg.Password); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&userv1.DeleteAccountResponse{}), nil
}
