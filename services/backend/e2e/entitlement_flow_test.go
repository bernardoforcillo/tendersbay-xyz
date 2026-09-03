package e2e

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/agent/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// Creating a workspace has to leave it able to run an agent turn. That is a
// chain nothing else exercises whole: the handler seeds a subscription row,
// featurelayer reads it back through the Postgres store, resolves the free
// plan's limit and the flag on the feature it depends on, and the counter's
// period is anchored to the row's billing anchor.
//
// A workspace with no subscription is entitled to nothing — the deliberate
// fail-closed answer — so a seed that silently did not happen would show up
// here as a brand-new workspace that cannot use the product.
func TestNewWorkspaceCanRunAnAgentTurn(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "ent-owner")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Entitled")

	got, err := owner.agent.GetCredits(ctx,
		authed(&agentv1.GetCreditsRequest{WorkspaceId: ws.Id}, owner.access))
	if err != nil {
		t.Fatalf("GetCredits: %v", err)
	}
	if got.Msg.MonthlyMax != features.FreeMonthlyTokenAllowance {
		t.Fatalf("a new workspace's allowance is %d, want the free plan's %d",
			got.Msg.MonthlyMax, features.FreeMonthlyTokenAllowance)
	}
	if got.Msg.Remaining != features.FreeMonthlyTokenAllowance || got.Msg.Used != 0 {
		t.Fatalf("a new workspace has already spent something: remaining %d, used %d",
			got.Msg.Remaining, got.Msg.Used)
	}

	// The reset date comes from the entitlement period anchored to this
	// workspace's own billing anchor — not from "the first of next month",
	// which would be wrong for a workspace that started mid-month.
	if got.Msg.ResetDate == "" {
		t.Fatal("no reset date: the counter is not anchored to a period")
	}
	reset, err := time.Parse("2006-01-02", got.Msg.ResetDate)
	if err != nil {
		t.Fatalf("reset date %q does not parse: %v", got.Msg.ResetDate, err)
	}
	if !reset.After(time.Now().UTC()) {
		t.Fatalf("the period resets at %s, which is not in the future", got.Msg.ResetDate)
	}
}

// The budget belongs to the workspace, and reading it is a membership check —
// so a member sees it and a stranger does not. Both halves matter: a budget
// only the owner could read would break the credits banner for everyone else.
func TestCreditsFollowWorkspaceMembership(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "cred-owner")
	member := s.signUp(t, "cred-member")
	stranger := s.signUp(t, "cred-stranger")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Budget")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)

	mine, err := member.agent.GetCredits(ctx,
		authed(&agentv1.GetCreditsRequest{WorkspaceId: ws.Id}, member.access))
	if err != nil {
		t.Fatalf("a member cannot read the workspace budget: %v", err)
	}
	if mine.Msg.MonthlyMax != features.FreeMonthlyTokenAllowance {
		t.Fatalf("a member sees an allowance of %d, want %d",
			mine.Msg.MonthlyMax, features.FreeMonthlyTokenAllowance)
	}

	if got := codeOf(mustErr(stranger.agent.GetCredits(ctx,
		authed(&agentv1.GetCreditsRequest{WorkspaceId: ws.Id}, stranger.access)))); got == connect.CodeUnknown {
		t.Fatal("a stranger read somebody else's token budget")
	}
}
