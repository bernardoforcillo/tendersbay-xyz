package connectapi

import (
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/agent"
)

// TestToConnectError_AgentDomain pins the codes the agent's sentinels map to.
//
// The default arm of toConnectError is CodeInternal, so a sentinel with no case
// here does not fail loudly — it reaches the client as a 500 with the domain's
// own message in it. For ErrEmptyMessage that would be the worst of both: the
// client is told the server broke when what actually happened is that it sent a
// request the server correctly refused, and a 500 is the one code a frontend is
// least likely to surface as "type something first".
func TestToConnectError_AgentDomain(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"insufficient credits", agent.ErrInsufficientCredits, connect.CodeResourceExhausted},
		{"choice not pending", agent.ErrChoiceNotPending, connect.CodeFailedPrecondition},
		{"empty message", agent.ErrEmptyMessage, connect.CodeInvalidArgument},
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
