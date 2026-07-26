package connectapi

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

func TestToConnectError_BidDomain(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"not found", bid.ErrBidNotFound, connect.CodeNotFound},
		{"checklist item not found", bid.ErrChecklistItemNotFound, connect.CodeNotFound},
		{"exists", bid.ErrBidExists, connect.CodeAlreadyExists},
		{"not go", bid.ErrBidNotGo, connect.CodeFailedPrecondition},
		{"invalid transition", bid.ErrInvalidTransition, connect.CodeFailedPrecondition},
		{"invalid argument", bid.ErrInvalidArgument, connect.CodeInvalidArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mapped := toConnectError(c.err)
			var ce *connect.Error
			if !errors.As(mapped, &ce) || ce.Code() != c.want {
				t.Fatalf("toConnectError(%v) = %v, want code %v", c.err, mapped, c.want)
			}
		})
	}
}
