package clientprofile_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type fakeRepo struct {
	profiles map[string]clientprofile.Profile
}

func newFakeRepo() *fakeRepo { return &fakeRepo{profiles: map[string]clientprofile.Profile{}} }

func (f *fakeRepo) Get(_ context.Context, workspaceID string) (clientprofile.Profile, error) {
	p, ok := f.profiles[workspaceID]
	if !ok {
		return clientprofile.Profile{}, clientprofile.ErrProfileNotFound
	}
	return p, nil
}

func (f *fakeRepo) Upsert(_ context.Context, p clientprofile.Profile) (clientprofile.Profile, error) {
	p.UpdatedAt = time.Unix(0, 0)
	f.profiles[p.WorkspaceID] = p
	return p, nil
}

type fakeMembers struct {
	member bool
}

func (f *fakeMembers) LoadMembership(_ context.Context, _, _ string) (workspace.Membership, error) {
	if !f.member {
		return workspace.Membership{}, workspace.ErrNotMember
	}
	return workspace.Membership{}, nil
}

func TestService_Get_ReturnsErrNotMemberWithoutTouchingRepo(t *testing.T) {
	repo := newFakeRepo()
	svc := clientprofile.NewService(repo, &fakeMembers{member: false})

	_, err := svc.Get(context.Background(), "user-1", "ws-1")
	if !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("Get error = %v, want ErrNotMember", err)
	}
}

func TestService_Get_ReturnsErrProfileNotFoundWhenNoneStored(t *testing.T) {
	repo := newFakeRepo()
	svc := clientprofile.NewService(repo, &fakeMembers{member: true})

	_, err := svc.Get(context.Background(), "user-1", "ws-1")
	if !errors.Is(err, clientprofile.ErrProfileNotFound) {
		t.Fatalf("Get error = %v, want ErrProfileNotFound", err)
	}
}

func TestService_Update_RoundTripsThroughGet(t *testing.T) {
	repo := newFakeRepo()
	svc := clientprofile.NewService(repo, &fakeMembers{member: true})

	min, max := int64(100_000), int64(500_000)
	in := clientprofile.Profile{
		WorkspaceID: "ws-1",
		Sectors:     []string{"45", "72"},
		Countries:   []string{"ITA", "DEU"},
		ValueMin:    &min,
		ValueMax:    &max,
		Notes:       "Renovation and IT modernisation work",
	}
	if _, err := svc.Update(context.Background(), "user-1", in); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := svc.Get(context.Background(), "user-1", "ws-1")
	if err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if len(got.Sectors) != 2 || got.Sectors[0] != "45" || got.Countries[1] != "DEU" {
		t.Fatalf("Get after Update = %+v", got)
	}
	if got.ValueMin == nil || *got.ValueMin != min || got.ValueMax == nil || *got.ValueMax != max {
		t.Fatalf("value band = %+v", got)
	}
}

func TestService_Update_RejectsUnmemberedCaller(t *testing.T) {
	repo := newFakeRepo()
	svc := clientprofile.NewService(repo, &fakeMembers{member: false})

	_, err := svc.Update(context.Background(), "user-1", clientprofile.Profile{WorkspaceID: "ws-1"})
	if !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("Update error = %v, want ErrNotMember", err)
	}
	if len(repo.profiles) != 0 {
		t.Fatal("Update must not reach the repo when membership fails")
	}
}

func TestValidate_TableDriven(t *testing.T) {
	min, max := int64(200), int64(100) // inverted band
	longNotes := make([]byte, 2001)
	for i := range longNotes {
		longNotes[i] = 'x'
	}

	cases := []struct {
		name    string
		profile clientprofile.Profile
		wantErr error
	}{
		{"valid empty profile", clientprofile.Profile{WorkspaceID: "ws-1"}, nil},
		{
			"valid full profile",
			clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"45", "7211"}, Countries: []string{"ITA"}, Notes: "ok"},
			nil,
		},
		{
			"sector too short",
			clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"4"}},
			clientprofile.ErrInvalidSector,
		},
		{
			"sector not numeric",
			clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"IT"}},
			clientprofile.ErrInvalidSector,
		},
		{
			"country not alpha-3",
			clientprofile.Profile{WorkspaceID: "ws-1", Countries: []string{"IT"}},
			clientprofile.ErrInvalidCountry,
		},
		{
			"country lowercase rejected",
			clientprofile.Profile{WorkspaceID: "ws-1", Countries: []string{"ita"}},
			clientprofile.ErrInvalidCountry,
		},
		{
			"inverted value band",
			clientprofile.Profile{WorkspaceID: "ws-1", ValueMin: &min, ValueMax: &max},
			clientprofile.ErrInvalidValueBand,
		},
		{
			"notes too long",
			clientprofile.Profile{WorkspaceID: "ws-1", Notes: string(longNotes)},
			clientprofile.ErrNotesTooLong,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newFakeRepo()
			svc := clientprofile.NewService(repo, &fakeMembers{member: true})
			_, err := svc.Update(context.Background(), "user-1", tc.profile)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("Update: unexpected error %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Update error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
