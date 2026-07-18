package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
)

type ClientProfileRepo struct{ db *pg.DB }

func NewClientProfileRepo(db *pg.DB) *ClientProfileRepo { return &ClientProfileRepo{db: db} }

var _ clientprofile.Repository = (*ClientProfileRepo)(nil)

func (r *ClientProfileRepo) Get(ctx context.Context, workspaceID string) (clientprofile.Profile, error) {
	var row DBClientProfile
	err := r.db.Select().From(ClientProfiles).Where(CPWorkspaceID.Eq(workspaceID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return clientprofile.Profile{}, clientprofile.ErrProfileNotFound
	}
	if err != nil {
		return clientprofile.Profile{}, err
	}
	return dbClientProfileToDomain(row)
}

// Upsert is a full replace: every column is written on every call, including
// explicit SQL NULL for a value bound the caller cleared (a nil ValueMin/Max
// must overwrite a previously-stored value, not leave it untouched — see
// valueMinCol/valueMaxCol below) and a re-marshaled `[]` for a nil/empty
// Sectors/Countries/Regions/ProcedureTypes (all four slice fields are JSONB
// and need no NULL-clearing special case — an empty slice just writes `[]`).
func (r *ClientProfileRepo) Upsert(ctx context.Context, p clientprofile.Profile) (clientprofile.Profile, error) {
	sectors, err := json.Marshal(p.Sectors)
	if err != nil {
		return clientprofile.Profile{}, err
	}
	countries, err := json.Marshal(p.Countries)
	if err != nil {
		return clientprofile.Profile{}, err
	}
	regions, err := json.Marshal(p.Regions)
	if err != nil {
		return clientprofile.Profile{}, err
	}
	procedureTypes, err := json.Marshal(p.ProcedureTypes)
	if err != nil {
		return clientprofile.Profile{}, err
	}
	now := time.Now()

	values := []pg.ColumnValue{
		CPWorkspaceID.Val(p.WorkspaceID), CPSectors.Val(sectors), CPCountries.Val(countries),
		CPRegions.Val(regions), CPProcedureTypes.Val(procedureTypes),
		valueMinCol(p.ValueMin), valueMaxCol(p.ValueMax), CPNotes.Val(p.Notes), CPUpdatedAt.Val(now),
	}

	var row DBClientProfile
	err = r.db.Insert(ClientProfiles).
		Row(values...).
		OnConflictUpdate(CPWorkspaceID).
		Set(CPSectors.Val(sectors), CPCountries.Val(countries), CPRegions.Val(regions), CPProcedureTypes.Val(procedureTypes),
			valueMinCol(p.ValueMin), valueMaxCol(p.ValueMax), CPNotes.Val(p.Notes), CPUpdatedAt.Val(now)).
		Done().
		Returning(CPWorkspaceID, CPSectors, CPCountries, CPRegions, CPProcedureTypes, CPValueMin, CPValueMax, CPNotes, CPUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return clientprofile.Profile{}, err
	}
	return dbClientProfileToDomain(row)
}

// valueMinCol/valueMaxCol write the bound when set, or the SQL DEFAULT
// keyword when cleared — CPValueMin/CPValueMax carry no explicit .Default()
// in schema.go, so DEFAULT resolves to Postgres's implicit column default,
// NULL. (*Col[int64]).Val only accepts int64, not *int64, so a nil pointer
// can't be passed through Val directly; SetDefault is the same mechanism
// ChatRepo.CreateSession already uses to null out an unset nullable column.
func valueMinCol(v *int64) pg.ColumnValue {
	if v == nil {
		return CPValueMin.SetDefault()
	}
	return CPValueMin.Val(*v)
}

func valueMaxCol(v *int64) pg.ColumnValue {
	if v == nil {
		return CPValueMax.SetDefault()
	}
	return CPValueMax.Val(*v)
}

func dbClientProfileToDomain(row DBClientProfile) (clientprofile.Profile, error) {
	var sectors, countries, regions, procedureTypes []string
	if err := json.Unmarshal(row.Sectors, &sectors); err != nil {
		return clientprofile.Profile{}, err
	}
	if err := json.Unmarshal(row.Countries, &countries); err != nil {
		return clientprofile.Profile{}, err
	}
	if err := json.Unmarshal(row.Regions, &regions); err != nil {
		return clientprofile.Profile{}, err
	}
	if err := json.Unmarshal(row.ProcedureTypes, &procedureTypes); err != nil {
		return clientprofile.Profile{}, err
	}
	return clientprofile.Profile{
		WorkspaceID:    row.WorkspaceID,
		Sectors:        sectors,
		Countries:      countries,
		Regions:        regions,
		ProcedureTypes: procedureTypes,
		ValueMin:       row.ValueMin,
		ValueMax:       row.ValueMax,
		Notes:          row.Notes,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}
