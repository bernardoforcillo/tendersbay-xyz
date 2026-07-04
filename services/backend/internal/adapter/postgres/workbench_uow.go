package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// WorkbenchUnitOfWork runs a function inside a single transaction, exposing
// tx-scoped workbench repositories so workbench seeding commits atomically.
type WorkbenchUnitOfWork struct{ db *pg.DB }

func NewWorkbenchUnitOfWork(db *pg.DB) *WorkbenchUnitOfWork { return &WorkbenchUnitOfWork{db: db} }

var _ workbench.UnitOfWork = (*WorkbenchUnitOfWork)(nil)

func (u *WorkbenchUnitOfWork) Do(ctx context.Context, fn func(workbench.Repos) error) error {
	return u.db.InTx(ctx, func(tx *pg.DB) error {
		return fn(workbench.Repos{
			Workbenches: NewWorkbenchRepo(tx),
			Roles:       NewWorkbenchRoleRepo(tx),
			Members:     NewWorkbenchMemberRepo(tx),
		})
	})
}
