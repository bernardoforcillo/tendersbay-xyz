package rbac_test

import (
	"testing"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

type mask uint64

const (
	view   mask = 1 << 0
	edit   mask = 1 << 1
	invite mask = 1 << 2
	admin  mask = 1 << 6
)

func statements() map[string][]access.Action {
	return map[string][]access.Action{
		"thing":  {scope.ActionUpdate},
		"invite": {scope.ActionCreate, scope.ActionDelete},
	}
}

func newCodec() *rbac.Codec[mask] {
	return rbac.New(statements(), view, admin, []rbac.Grant[mask]{
		{Bit: edit, Grants: map[string][]access.Action{"thing": {scope.ActionUpdate}}},
		{Bit: invite, Grants: map[string][]access.Action{"invite": {scope.ActionCreate, scope.ActionDelete}}},
	}, map[string][]access.Action{})
}

// Every bit has to survive mask -> grants -> permission -> mask, or a role
// editor silently drops capabilities on save.
func TestRoundTrip(t *testing.T) {
	c := newCodec()
	for _, bit := range []mask{edit, invite, edit | invite} {
		perms, err := c.Access().Permission(c.GrantsFor(bit))
		if err != nil {
			t.Fatalf("GrantsFor(%d): %v", bit, err)
		}
		got := c.Mask(perms, false)
		if want := view | bit; got != want {
			t.Fatalf("round trip of %d gave %d, want %d", bit, got, want)
		}
	}
}

// The baseline bit is membership, so it is set for a caller holding nothing.
func TestMaskAlwaysCarriesTheBaseline(t *testing.T) {
	c := newCodec()
	perms, err := c.Access().Permission(map[string][]access.Action{})
	if err != nil {
		t.Fatalf("Permission: %v", err)
	}
	if got := c.Mask(perms, false); got != view {
		t.Fatalf("mask = %d, want just the baseline %d", got, view)
	}
}

// An elevated caller holds every bit. This is the rule that matters for an
// INHERITED elevation, where the caller has no grants of their own: a mask
// derived from grants alone would say "you may only look" about someone whose
// every write will in fact succeed.
func TestElevatedHoldsEveryBit(t *testing.T) {
	c := newCodec()
	empty, err := c.Access().Permission(map[string][]access.Action{})
	if err != nil {
		t.Fatalf("Permission: %v", err)
	}
	got := c.Mask(empty, true)
	for _, bit := range []mask{view, admin, edit, invite} {
		if got&bit != bit {
			t.Fatalf("elevated mask %d is missing bit %d", got, bit)
		}
	}
}

// The admin bit is not a grant: it expands to every declared one, which is what
// makes the resulting role elevated.
func TestAdminBitExpandsToEverything(t *testing.T) {
	c := newCodec()
	perms, err := c.Access().Permission(c.GrantsFor(admin))
	if err != nil {
		t.Fatalf("GrantsFor(admin): %v", err)
	}
	if !perms.IsFull() {
		t.Fatal("the admin bit must expand to a full permission, or the role is not elevated")
	}
}

// GrantsFor hands its result to authlayer; it must not hand out the codec's own
// declarations, which every later call depends on.
func TestGrantsForReturnsACopy(t *testing.T) {
	c := newCodec()
	got := c.GrantsFor(admin)
	got["thing"] = nil
	delete(got, "invite")

	again := c.GrantsFor(admin)
	if len(again["thing"]) == 0 || len(again["invite"]) == 0 {
		t.Fatalf("mutating a returned grant map corrupted the codec: %v", again)
	}
}

func TestEncodeIsReadableByTheEngine(t *testing.T) {
	c := newCodec()
	encoded, err := c.Encode(edit)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	perms, err := c.Access().Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !perms.Allows("thing", scope.ActionUpdate) {
		t.Fatal("the encoded permission did not survive a decode")
	}
}

func TestRoleKeyAndSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Bid Reviewer": "bid-reviewer",
		"  ADMIN  ":    "admin",
		"a//b":         "a-b",
		"":             "role",
		"!!!":          "role",
	} {
		if got := rbac.RoleKey(in); got != want {
			t.Errorf("RoleKey(%q) = %q, want %q", in, got, want)
		}
	}
	if got := rbac.Slug("!!!"); got != "" {
		t.Errorf("Slug(%q) = %q, want empty — RoleKey is what supplies a fallback", "!!!", got)
	}
}
