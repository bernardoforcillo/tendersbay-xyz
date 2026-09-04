package rbac

import (
	"errors"

	"github.com/bernardoforcillo/authlayer/scope"
)

// Errors is a domain's own vocabulary for authlayer's scope sentinels.
//
// Each scope publishes its own — connectapi maps them to status codes, and
// "not a member of this workspace" and "not a member of this workbench" are
// different things to tell a caller — but the translation between the two sets
// is mechanical and was written out twice before this.
//
// A nil field passes that sentinel through untranslated, which is the right
// answer for a condition a scope cannot produce: a workbench has no unique
// constraint to violate, so it has no Conflict of its own.
type Errors struct {
	NotFound            error // scope.ErrContainerNotFound
	NotMember           error // ErrNotMember, and a context missing its subject or scope
	NotParentMember     error // ErrNotParentMember; falls back to NotMember when nil
	Forbidden           error
	PrivilegeEscalation error
	RoleNotFound        error
	RoleInUse           error
	DefaultRole         error
	LastOwner           error
	OwnerOnly           error
	AlreadyMember       error
	RoleKeyTaken        error
	Conflict            error // scope.ErrConflict — a container violating a unique constraint
}

// Translate maps a scope error onto this domain's vocabulary. Anything it does
// not recognise passes through untouched and surfaces as an internal error,
// which is the correct answer for a store or transport failure.
//
// A missing subject or scope on the context is reported as NotMember rather
// than as a distinct error: it means the caller was never identified, and the
// one thing an unidentified caller must not learn is whether the container
// exists.
func (e Errors) Translate(err error) error {
	if err == nil {
		return nil
	}
	for _, m := range []struct {
		sentinel error
		mapped   error
	}{
		{scope.ErrContainerNotFound, e.NotFound},
		{scope.ErrNotParentMember, e.notParentMember()},
		{scope.ErrNotMember, e.NotMember},
		{scope.ErrSubjectMissing, e.NotMember},
		{scope.ErrScopeMissing, e.NotMember},
		{scope.ErrForbidden, e.Forbidden},
		{scope.ErrPrivilegeEscalation, e.PrivilegeEscalation},
		{scope.ErrRoleNotFound, e.RoleNotFound},
		{scope.ErrRoleInUse, e.RoleInUse},
		{scope.ErrDefaultRole, e.DefaultRole},
		{scope.ErrLastOwner, e.LastOwner},
		{scope.ErrOwnerOnly, e.OwnerOnly},
		{scope.ErrAlreadyMember, e.AlreadyMember},
		{scope.ErrRoleKeyTaken, e.RoleKeyTaken},
		{scope.ErrConflict, e.Conflict},
	} {
		if m.mapped != nil && errors.Is(err, m.sentinel) {
			return m.mapped
		}
	}
	return err
}

func (e Errors) notParentMember() error {
	if e.NotParentMember != nil {
		return e.NotParentMember
	}
	return e.NotMember
}
