package postgres

import (
	"context"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// TestBidRepoLifecycleSignatures pins BidRepo's lifecycle methods to the exact
// bid.Repository signatures at compile time (no DB needed). It fails to build
// until every method exists with the right shape; r is never dereferenced.
func TestBidRepoLifecycleSignatures(t *testing.T) {
	var r *BidRepo
	var _ func(context.Context, bid.Bid) (bid.Bid, error) = r.CreateBid
	var _ func(context.Context, string, string) (bid.Bid, error) = r.FindBidByID
	var _ func(context.Context, string) ([]bid.Bid, error) = r.ListBidsByWorkbench
	var _ func(context.Context, string, bid.GoNoGo, bid.DecisionRecord) (bid.Bid, error) = r.UpdateGoNoGo
	var _ func(context.Context, string, bid.Stage) (bid.Bid, error) = r.UpdateStage
	var _ func(context.Context, string, bid.Outcome) (bid.Bid, error) = r.UpdateOutcome
	var _ func(context.Context, string) error = r.DeleteBid
}
