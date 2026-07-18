package connectapi_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	workspacev1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
)

type fakeClientProfiles struct {
	profile clientprofile.Profile
	found   bool
	getErr  error
	updated clientprofile.Profile
}

func (f *fakeClientProfiles) Get(context.Context, string, string) (clientprofile.Profile, error) {
	if f.getErr != nil {
		return clientprofile.Profile{}, f.getErr
	}
	if !f.found {
		return clientprofile.Profile{}, clientprofile.ErrProfileNotFound
	}
	return f.profile, nil
}

func (f *fakeClientProfiles) Update(_ context.Context, _ string, p clientprofile.Profile) (clientprofile.Profile, error) {
	f.updated = p
	return p, nil
}

func TestWorkspaceHandler_GetClientProfile_ReportsExistsFalseWhenNoneStored(t *testing.T) {
	h := connectapi.NewWorkspaceHandler(nil, nil, &fakeClientProfiles{found: false})
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	resp, err := h.GetClientProfile(ctx, connect.NewRequest(&workspacev1.GetClientProfileRequest{WorkspaceId: "ws-1"}))
	if err != nil {
		t.Fatalf("GetClientProfile: %v", err)
	}
	if resp.Msg.Exists {
		t.Fatal("Exists = true, want false when no profile is stored")
	}
}

func TestWorkspaceHandler_UpdateClientProfile_MapsValueBandSetFlags(t *testing.T) {
	fake := &fakeClientProfiles{}
	h := connectapi.NewWorkspaceHandler(nil, nil, fake)
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	_, err := h.UpdateClientProfile(ctx, connect.NewRequest(&workspacev1.UpdateClientProfileRequest{
		WorkspaceId: "ws-1",
		Sectors:     []string{"45"},
		Countries:   []string{"ITA"},
		ValueMin:    100_000,
		ValueMinSet: true,
		// ValueMax intentionally left unset (ValueMaxSet: false)
		Notes: "test",
	}))
	if err != nil {
		t.Fatalf("UpdateClientProfile: %v", err)
	}
	if fake.updated.ValueMin == nil || *fake.updated.ValueMin != 100_000 {
		t.Fatalf("updated.ValueMin = %v, want 100000", fake.updated.ValueMin)
	}
	if fake.updated.ValueMax != nil {
		t.Fatalf("updated.ValueMax = %v, want nil (value_max_set was false)", fake.updated.ValueMax)
	}
}

func TestWorkspaceHandler_UpdateClientProfile_MapsValidationErrorToInvalidArgument(t *testing.T) {
	fake := &fakeClientProfiles{}
	fake.getErr = nil
	h := connectapi.NewWorkspaceHandler(nil, nil, &failingClientProfiles{err: clientprofile.ErrInvalidSector})
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	_, err := h.UpdateClientProfile(ctx, connect.NewRequest(&workspacev1.UpdateClientProfileRequest{WorkspaceId: "ws-1"}))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v, want CodeInvalidArgument", err)
	}
}

type failingClientProfiles struct{ err error }

func (f *failingClientProfiles) Get(context.Context, string, string) (clientprofile.Profile, error) {
	return clientprofile.Profile{}, f.err
}
func (f *failingClientProfiles) Update(context.Context, string, clientprofile.Profile) (clientprofile.Profile, error) {
	return clientprofile.Profile{}, f.err
}
