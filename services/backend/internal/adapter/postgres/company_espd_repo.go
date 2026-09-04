package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// The ESPD sections of the dossier (migration 0016): representatives, Part III
// declarations, national grounds. Same shape as the sections in
// company_repo.go — workspace-scoped, per-row provenance, upsert on the natural
// key where one exists.

// ── Representatives ─────────────────────────────────────────────────────────

// PutRepresentative inserts when r.ID is empty and updates that row otherwise.
// Id-only identity, like past contracts: two representatives can share a name.
func (r *CompanyRepo) PutRepresentative(ctx context.Context, workspaceID string, rep company.Representative) (company.Representative, error) {
	set := []pg.ColumnValue{
		RepRole.Val(rep.Role),
		RepGivenName.Val(rep.GivenName),
		RepFamilyName.Val(rep.FamilyName),
		nullableTime(RepBirthDate, rep.BirthDate),
		RepBirthPlace.Val(rep.BirthPlace),
		RepAddress.Val(rep.Address),
		RepEmail.Val(rep.Email),
		RepPowerOfAttorney.Val(rep.PowerOfAttorney),
		RepProvenance.Val(string(rep.Provenance)),
		nullableFloat(RepConfidence, rep.Confidence),
		nullableUUID(RepStatedBy, rep.StatedBy),
		RepStatedAt.Val(statedAtOrNow(rep.StatedAt)),
		RepPromptedBy.Val(string(rep.PromptedBy)),
		nullableInt64(RepPromptedByTender, rep.PromptedByTenderID),
		RepSourceNote.Val(rep.SourceNote),
	}
	var row DBRepresentative
	if rep.ID == "" {
		err := r.db.Insert(CompanyRepresentatives).
			Row(append([]pg.ColumnValue{RepWorkspaceID.Val(workspaceID)}, set...)...).
			Returning(repColumns()...).
			One(ctx, &row)
		if err != nil {
			return company.Representative{}, err
		}
		return dbRepToDomain(row), nil
	}
	err := r.db.Update(CompanyRepresentatives).
		Set(set...).
		Where(RepWorkspaceID.Eq(workspaceID), RepID.Eq(rep.ID)).
		Returning(repColumns()...).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return company.Representative{}, company.ErrRecordNotFound
	}
	if err != nil {
		return company.Representative{}, err
	}
	return dbRepToDomain(row), nil
}

func (r *CompanyRepo) DeleteRepresentative(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyRepresentatives).
		Where(RepWorkspaceID.Eq(workspaceID), RepID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listRepresentatives(ctx context.Context, workspaceID string) ([]company.Representative, error) {
	var rows []DBRepresentative
	err := r.db.Select().From(CompanyRepresentatives).
		Where(RepWorkspaceID.Eq(workspaceID)).
		OrderBy(RepFamilyName.Asc(), RepGivenName.Asc(), RepID.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.Representative, len(rows))
	for i, row := range rows {
		out[i] = dbRepToDomain(row)
	}
	return out, nil
}

func repColumns() []drops.Expression {
	return []drops.Expression{
		RepID, RepWorkspaceID, RepRole, RepGivenName, RepFamilyName, RepBirthDate, RepBirthPlace,
		RepAddress, RepEmail, RepPowerOfAttorney, RepProvenance, RepConfidence, RepStatedBy,
		RepStatedAt, RepPromptedBy, RepPromptedByTender, RepSourceNote,
	}
}

func dbRepToDomain(row DBRepresentative) company.Representative {
	return company.Representative{
		ID:              row.ID,
		Role:            row.Role,
		GivenName:       row.GivenName,
		FamilyName:      row.FamilyName,
		BirthDate:       row.BirthDate,
		BirthPlace:      row.BirthPlace,
		Address:         row.Address,
		Email:           row.Email,
		PowerOfAttorney: row.PowerOfAttorney,
		Attribution: company.Attribution{
			Provenance:         company.Provenance(row.Provenance),
			Confidence:         row.Confidence,
			StatedBy:           derefString(row.StatedBy),
			StatedAt:           row.StatedAt,
			PromptedBy:         company.PromptSource(row.PromptedBy),
			PromptedByTenderID: row.PromptedByTenderID,
			SourceNote:         row.SourceNote,
		},
	}
}

// ── Declarations ────────────────────────────────────────────────────────────

// PutDeclaration upserts on (workspace_id, criterion). The domain has already
// refused a non-authoritative provenance; the adapter re-checks because a
// repository is a port any caller could reach, and a declaration written with
// an agent's provenance would be a legal fact the product invented.
func (r *CompanyRepo) PutDeclaration(ctx context.Context, workspaceID string, d company.Declaration) (company.Declaration, error) {
	if !d.Authoritative() {
		return company.Declaration{}, fmt.Errorf("%w: %s", company.ErrDeclarationNotAuthoritative, d.Provenance)
	}
	set := []pg.ColumnValue{
		DecAnswer.Val(d.Answer),
		DecSelfCleaning.Val(d.SelfCleaning),
		DecProvenance.Val(string(d.Provenance)),
		nullableFloat(DecConfidence, d.Confidence),
		nullableUUID(DecStatedBy, d.StatedBy),
		DecStatedAt.Val(statedAtOrNow(d.StatedAt)),
		DecPromptedBy.Val(string(d.PromptedBy)),
		nullableInt64(DecPromptedByTender, d.PromptedByTenderID),
		DecSourceNote.Val(d.SourceNote),
	}
	var row DBDeclaration
	err := r.db.Insert(CompanyDeclarations).
		Row(append([]pg.ColumnValue{DecWorkspaceID.Val(workspaceID), DecCriterion.Val(d.Criterion)}, set...)...).
		OnConflictUpdate(DecWorkspaceID, DecCriterion).
		Set(set...).
		Done().
		Returning(decColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.Declaration{}, err
	}
	return dbDecToDomain(row), nil
}

func (r *CompanyRepo) DeleteDeclaration(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyDeclarations).
		Where(DecWorkspaceID.Eq(workspaceID), DecID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listDeclarations(ctx context.Context, workspaceID string) ([]company.Declaration, error) {
	var rows []DBDeclaration
	err := r.db.Select().From(CompanyDeclarations).
		Where(DecWorkspaceID.Eq(workspaceID)).
		OrderBy(DecCriterion.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.Declaration, len(rows))
	for i, row := range rows {
		out[i] = dbDecToDomain(row)
	}
	return out, nil
}

func decColumns() []drops.Expression {
	return []drops.Expression{
		DecID, DecWorkspaceID, DecCriterion, DecAnswer, DecSelfCleaning, DecProvenance, DecConfidence,
		DecStatedBy, DecStatedAt, DecPromptedBy, DecPromptedByTender, DecSourceNote,
	}
}

func dbDecToDomain(row DBDeclaration) company.Declaration {
	return company.Declaration{
		ID:           row.ID,
		Criterion:    row.Criterion,
		Answer:       row.Answer,
		SelfCleaning: row.SelfCleaning,
		Attribution: company.Attribution{
			Provenance:         company.Provenance(row.Provenance),
			Confidence:         row.Confidence,
			StatedBy:           derefString(row.StatedBy),
			StatedAt:           row.StatedAt,
			PromptedBy:         company.PromptSource(row.PromptedBy),
			PromptedByTenderID: row.PromptedByTenderID,
			SourceNote:         row.SourceNote,
		},
	}
}

// ── National grounds ────────────────────────────────────────────────────────

// PutNationalGround upserts on (workspace_id, country, criterion), with the
// same provenance wall as PutDeclaration.
func (r *CompanyRepo) PutNationalGround(ctx context.Context, workspaceID string, g company.NationalGround) (company.NationalGround, error) {
	if !g.Authoritative() {
		return company.NationalGround{}, fmt.Errorf("%w: %s", company.ErrDeclarationNotAuthoritative, g.Provenance)
	}
	set := []pg.ColumnValue{
		NGAnswer.Val(g.Answer),
		NGNote.Val(g.Note),
		NGProvenance.Val(string(g.Provenance)),
		nullableFloat(NGConfidence, g.Confidence),
		nullableUUID(NGStatedBy, g.StatedBy),
		NGStatedAt.Val(statedAtOrNow(g.StatedAt)),
		NGPromptedBy.Val(string(g.PromptedBy)),
		nullableInt64(NGPromptedByTender, g.PromptedByTenderID),
		NGSourceNote.Val(g.SourceNote),
	}
	var row DBNationalGround
	err := r.db.Insert(CompanyNationalGrounds).
		Row(append([]pg.ColumnValue{NGWorkspaceID.Val(workspaceID), NGCountry.Val(g.Country), NGCriterion.Val(g.Criterion)}, set...)...).
		OnConflictUpdate(NGWorkspaceID, NGCountry, NGCriterion).
		Set(set...).
		Done().
		Returning(ngColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.NationalGround{}, err
	}
	return dbNGToDomain(row), nil
}

func (r *CompanyRepo) DeleteNationalGround(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyNationalGrounds).
		Where(NGWorkspaceID.Eq(workspaceID), NGID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listNationalGrounds(ctx context.Context, workspaceID string) ([]company.NationalGround, error) {
	var rows []DBNationalGround
	err := r.db.Select().From(CompanyNationalGrounds).
		Where(NGWorkspaceID.Eq(workspaceID)).
		OrderBy(NGCountry.Asc(), NGCriterion.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.NationalGround, len(rows))
	for i, row := range rows {
		out[i] = dbNGToDomain(row)
	}
	return out, nil
}

func ngColumns() []drops.Expression {
	return []drops.Expression{
		NGID, NGWorkspaceID, NGCountry, NGCriterion, NGAnswer, NGNote, NGProvenance, NGConfidence,
		NGStatedBy, NGStatedAt, NGPromptedBy, NGPromptedByTender, NGSourceNote,
	}
}

func dbNGToDomain(row DBNationalGround) company.NationalGround {
	return company.NationalGround{
		ID:        row.ID,
		Country:   row.Country,
		Criterion: row.Criterion,
		Answer:    row.Answer,
		Note:      row.Note,
		Attribution: company.Attribution{
			Provenance:         company.Provenance(row.Provenance),
			Confidence:         row.Confidence,
			StatedBy:           derefString(row.StatedBy),
			StatedAt:           row.StatedAt,
			PromptedBy:         company.PromptSource(row.PromptedBy),
			PromptedByTenderID: row.PromptedByTenderID,
			SourceNote:         row.SourceNote,
		},
	}
}
