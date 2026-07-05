package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// UnitOfWork runs a function inside a single database transaction, exposing
// transaction-scoped workspace repositories. Because each repo is just a thin
// wrapper over a *pg.DB, the tx-bound repos share the transaction, so multi-row
// writes (workspace seeding, invite/link acceptance) commit or roll back as one.
type UnitOfWork struct{ db *pg.DB }

func NewUnitOfWork(db *pg.DB) *UnitOfWork { return &UnitOfWork{db: db} }

var _ workspace.UnitOfWork = (*UnitOfWork)(nil)

func (u *UnitOfWork) Do(ctx context.Context, fn func(workspace.Repos) error) error {
	return u.db.InTx(ctx, func(tx *pg.DB) error {
		return fn(workspace.Repos{
			Workspaces: NewWorkspaceRepo(tx),
			Roles:      NewRoleRepo(tx),
			Members:    NewMemberRepo(tx),
			EmailInvs:  NewEmailInviteRepo(tx),
			Links:      NewInviteLinkRepo(tx),
		})
	})
}
