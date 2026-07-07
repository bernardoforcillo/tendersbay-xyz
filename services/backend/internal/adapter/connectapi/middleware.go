package connectapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/bernardoforcillo/tendersbay-xyz/go-services/token"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type contextKey string

const userIDKey contextKey = "user_id"

// UserIDFromContext extracts the authenticated user ID injected by JWTMiddleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok && id != ""
}

// JWTMiddleware parses the Bearer token and injects user_id into the request context.
// Unauthenticated requests pass through — handlers that require auth check UserIDFromContext.
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.HasPrefix(header, "Bearer ") {
				raw := strings.TrimPrefix(header, "Bearer ")
				if claims, err := token.ParseJWT(raw, secret); err == nil {
					ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
					r = r.WithContext(ctx)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Cookie helpers — accept http.Header so they work with both http.ResponseWriter and
// connect.Response.Header() (both return http.Header).

// cookieFromHeader parses a named cookie from an http.Header (request headers).
// Works with connect.Request.Header() which returns http.Header directly.
func cookieFromHeader(h http.Header, name string) string {
	r := &http.Request{Header: h}
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func setRefreshCookie(h http.Header, plain string, maxAge int) {
	c := &http.Cookie{
		Name:     "refresh_token",
		Value:    plain,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
		MaxAge:   maxAge,
	}
	h.Add("Set-Cookie", c.String())
}

func clearRefreshCookie(h http.Header) {
	c := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
		MaxAge:   -1,
	}
	h.Add("Set-Cookie", c.String())
}

// toConnectError maps domain errors to ConnectRPC status codes.
func toConnectError(err error) error {
	switch {
	case errors.Is(err, auth.ErrEmailExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, auth.ErrInvalidCreds):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, auth.ErrEmailNotVerified):
		e := connect.NewError(connect.CodeFailedPrecondition, err)
		e.Meta().Set("x-error-code", "EMAIL_NOT_VERIFIED")
		return e
	case errors.Is(err, auth.ErrTokenInvalid):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, auth.ErrWeakPassword):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, auth.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)

	// Workspace domain
	case errors.Is(err, workspace.ErrForbidden),
		errors.Is(err, workspace.ErrNotMember),
		errors.Is(err, workspace.ErrOwnerOnly),
		errors.Is(err, workspace.ErrPrivilegeEscalation):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, workspace.ErrWorkspaceNotFound),
		errors.Is(err, workspace.ErrRoleNotFound),
		errors.Is(err, workspace.ErrInviteInvalid),
		errors.Is(err, workspace.ErrLinkInvalid):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, workspace.ErrLastOwner),
		errors.Is(err, workspace.ErrDefaultRole),
		errors.Is(err, workspace.ErrRoleInUse),
		errors.Is(err, workspace.ErrInviteExpired),
		errors.Is(err, workspace.ErrLinkExpired):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, workspace.ErrAlreadyMember),
		errors.Is(err, workspace.ErrSlugTaken):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, workspace.ErrLinkExhausted):
		return connect.NewError(connect.CodeResourceExhausted, err)

	// Workbench domain
	case errors.Is(err, workbench.ErrForbidden),
		errors.Is(err, workbench.ErrNotMember),
		errors.Is(err, workbench.ErrOwnerOnly),
		errors.Is(err, workbench.ErrPrivilegeEscalation),
		errors.Is(err, workbench.ErrNotWorkspaceMember):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, workbench.ErrWorkbenchNotFound),
		errors.Is(err, workbench.ErrRoleNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, workbench.ErrLastOwner),
		errors.Is(err, workbench.ErrDefaultRole),
		errors.Is(err, workbench.ErrRoleInUse):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, workbench.ErrAlreadyMember):
		return connect.NewError(connect.CodeAlreadyExists, err)

	// Agent domain
	case errors.Is(err, agent.ErrInsufficientCredits):
		return connect.NewError(connect.CodeResourceExhausted, err)

	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
