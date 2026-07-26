package connectapi_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	bidv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// testBidHandler builds a handler over a service with nil ports: the guards
// under test (requireUser and the tender_id parse) both return before any
// h.svc port is dereferenced, so the nil ports are never reached.
func testBidHandler() *connectapi.BidHandler {
	return connectapi.NewBidHandler(bid.NewService(nil, nil, nil))
}

func TestAddBid_RejectsUnauthenticated(t *testing.T) {
	h := testBidHandler()
	_, err := h.AddBid(context.Background(), connect.NewRequest(&bidv1.AddBidRequest{WorkbenchId: "wb-1", TenderId: "42"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeUnauthenticated {
		t.Fatalf("err = %v, want CodeUnauthenticated", err)
	}
}

func TestAddBid_RejectsUnparseableTenderIDAsInvalidArgument(t *testing.T) {
	h := testBidHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	_, err := h.AddBid(ctx, connect.NewRequest(&bidv1.AddBidRequest{WorkbenchId: "wb-1", TenderId: "not-a-number"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v, want CodeInvalidArgument", err)
	}
}
