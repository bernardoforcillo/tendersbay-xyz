package postgres

import (
	"context"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// TestBidRepoChecklistSignatures pins BidRepo's checklist methods to the exact
// bid.Repository signatures at compile time (no DB needed). r is never used.
func TestBidRepoChecklistSignatures(t *testing.T) {
	var r *BidRepo
	var _ func(context.Context, string, []bid.ChecklistItemSeed) error = r.SeedChecklist
	var _ func(context.Context, string) ([]bid.ChecklistItem, error) = r.ListChecklistItems
	var _ func(context.Context, string, string, string, string) (bid.ChecklistItem, error) = r.UpsertChecklistItem
}
