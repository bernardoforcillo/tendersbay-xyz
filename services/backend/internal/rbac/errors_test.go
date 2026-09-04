package rbac_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

var (
	errNotFound  = errors.New("not found")
	errNotMember = errors.New("not a member")
	errForbidden = errors.New("forbidden")
	errParent    = errors.New("not a parent member")
)

func TestTranslate(t *testing.T) {
	e := rbac.Errors{
		NotFound:        errNotFound,
		NotMember:       errNotMember,
		NotParentMember: errParent,
		Forbidden:       errForbidden,
	}
	for _, tc := range []struct {
		in   error
		want error
	}{
		{scope.ErrContainerNotFound, errNotFound},
		{scope.ErrNotMember, errNotMember},
		{scope.ErrNotParentMember, errParent},
		{scope.ErrForbidden, errForbidden},
		// A caller the context never identified is reported as a non-member:
		// the one thing they must not learn is whether the container exists.
		{scope.ErrSubjectMissing, errNotMember},
		{scope.ErrScopeMissing, errNotMember},
		// Wrapped sentinels resolve the same way.
		{fmt.Errorf("store: %w", scope.ErrForbidden), errForbidden},
	} {
		if got := e.Translate(tc.in); !errors.Is(got, tc.want) {
			t.Errorf("Translate(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A nil field passes its sentinel through, which is the right answer for a
// condition the domain cannot produce — and for anything unrecognised, which
// has to surface as an internal error rather than as a domain one.
func TestTranslateLeavesUnmappedErrorsAlone(t *testing.T) {
	e := rbac.Errors{NotMember: errNotMember}
	boom := errors.New("connection reset")

	for _, in := range []error{scope.ErrConflict, scope.ErrLastOwner, boom} {
		if got := e.Translate(in); !errors.Is(got, in) {
			t.Errorf("Translate(%v) = %v, want it passed through", in, got)
		}
	}
	if e.Translate(nil) != nil {
		t.Error("Translate(nil) must stay nil")
	}
}

// Without a NotParentMember of its own a scope reports the parent condition as
// its own not-a-member, rather than leaking authlayer's sentinel.
func TestNotParentMemberFallsBackToNotMember(t *testing.T) {
	e := rbac.Errors{NotMember: errNotMember}
	if got := e.Translate(scope.ErrNotParentMember); !errors.Is(got, errNotMember) {
		t.Fatalf("Translate = %v, want the domain's not-a-member", got)
	}
}
