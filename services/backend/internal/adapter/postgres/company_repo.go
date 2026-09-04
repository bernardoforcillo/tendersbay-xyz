package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// CompanyRepo persists the per-workspace company dossier.
type CompanyRepo struct{ db *pg.DB }

func NewCompanyRepo(db *pg.DB) *CompanyRepo { return &CompanyRepo{db: db} }

var _ company.Repository = (*CompanyRepo)(nil)

// ── Stored attribution shapes ───────────────────────────────────────────────

// storedAttribution is the JSONB shape of one identity field's provenance.
//
// It is declared HERE rather than by tagging company.Attribution, because the
// on-disk encoding is a storage concern this adapter owns: a rename in the
// domain must be free, and a rename here is a migration. Every field is
// spelled out so the stored document stays readable in psql, which is the whole
// argument for JSONB over an opaque blob.
type storedAttribution struct {
	Provenance         string   `json:"provenance"`
	Confidence         *float64 `json:"confidence,omitempty"`
	StatedBy           string   `json:"stated_by,omitempty"`
	StatedAt           string   `json:"stated_at,omitempty"`
	PromptedBy         string   `json:"prompted_by,omitempty"`
	PromptedByTenderID *int64   `json:"prompted_by_tender_id,omitempty"`
	SourceNote         string   `json:"source_note,omitempty"`
}

func toStoredAttribution(a company.Attribution) storedAttribution {
	s := storedAttribution{
		Provenance:         string(a.Provenance),
		Confidence:         a.Confidence,
		StatedBy:           a.StatedBy,
		PromptedBy:         string(a.PromptedBy),
		PromptedByTenderID: a.PromptedByTenderID,
		SourceNote:         a.SourceNote,
	}
	if !a.StatedAt.IsZero() {
		s.StatedAt = a.StatedAt.UTC().Format(time.RFC3339Nano)
	}
	return s
}

func fromStoredAttribution(s storedAttribution) company.Attribution {
	a := company.Attribution{
		Provenance:         company.Provenance(s.Provenance),
		Confidence:         s.Confidence,
		StatedBy:           s.StatedBy,
		PromptedBy:         company.PromptSource(s.PromptedBy),
		PromptedByTenderID: s.PromptedByTenderID,
		SourceNote:         s.SourceNote,
	}
	if s.StatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, s.StatedAt); err == nil {
			a.StatedAt = t
		}
	}
	return a
}

// ── Dossier read ────────────────────────────────────────────────────────────

// GetDossier loads the identity row and all eight child collections.
//
// ErrDossierNotFound is returned only when NOTHING exists — no identity row and
// no child record. A workspace whose first captured fact was a SOA line
// answered in chat has no identity row yet and must still return a dossier,
// because that is the normal shape under just-in-time capture, not an error.
func (r *CompanyRepo) GetDossier(ctx context.Context, workspaceID string) (company.Dossier, error) {
	d := company.Dossier{WorkspaceID: workspaceID}

	var row DBCompany
	err := r.db.Select().From(WorkspaceCompanies).Where(CoWorkspaceID.Eq(workspaceID)).One(ctx, &row)
	hasIdentity := true
	switch {
	case errors.Is(err, pg.ErrNoRows):
		hasIdentity = false
	case err != nil:
		return company.Dossier{}, err
	default:
		d.Identity, err = dbCompanyToIdentity(row)
		if err != nil {
			return company.Dossier{}, err
		}
		d.UpdatedAt = row.UpdatedAt
	}

	if d.SOA, err = r.listSOA(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.Certifications, err = r.listCertifications(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.FinancialYears, err = r.listFinancialYears(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.PastContracts, err = r.listPastContracts(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.Registrations, err = r.listRegistrations(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.Representatives, err = r.listRepresentatives(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.Declarations, err = r.listDeclarations(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}
	if d.NationalGrounds, err = r.listNationalGrounds(ctx, workspaceID); err != nil {
		return company.Dossier{}, err
	}

	if !hasIdentity && d.Empty() {
		return company.Dossier{}, company.ErrDossierNotFound
	}
	return d, nil
}

// ── Identity ────────────────────────────────────────────────────────────────

// UpsertIdentity is a FULL REPLACE: every column is written on every call,
// including SQL DEFAULT (→ NULL) for a cleared founded_year and a re-marshaled
// attribution map. Mirrors ClientProfileRepo.Upsert — an Update is a full
// replace, not a partial patch, so a field the caller blanked must overwrite
// what is stored rather than leave it standing.
func (r *CompanyRepo) UpsertIdentity(ctx context.Context, workspaceID string, id company.Identity) (company.Identity, error) {
	attribution := make(map[company.FieldKey]storedAttribution, len(id.Attribution))
	for k, a := range id.Attribution {
		attribution[k] = toStoredAttribution(a)
	}
	attrJSON, err := json.Marshal(attribution)
	if err != nil {
		return company.Identity{}, err
	}
	now := time.Now()

	set := []pg.ColumnValue{
		CoLegalName.Val(id.LegalName),
		CoVATNumber.Val(id.VATNumber),
		CoFiscalCode.Val(id.FiscalCode),
		CoLegalForm.Val(string(id.LegalForm)),
		CoCCIAAOffice.Val(id.CCIAAOffice),
		CoCCIAANumber.Val(id.CCIAANumber),
		CoCountry.Val(id.Country),
		CoNUTS.Val(id.NUTS),
		foundedYearCol(id.FoundedYear),
		CoIsSME.Val(id.IsSME),
		CoAttribution.Val(attrJSON),
		CoUpdatedAt.Val(now),
	}

	var row DBCompany
	err = r.db.Insert(WorkspaceCompanies).
		Row(append([]pg.ColumnValue{CoWorkspaceID.Val(workspaceID)}, set...)...).
		OnConflictUpdate(CoWorkspaceID).
		Set(set...).
		Done().
		Returning(companyColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.Identity{}, err
	}
	return dbCompanyToIdentity(row)
}

func companyColumns() []drops.Expression {
	return []drops.Expression{
		CoWorkspaceID, CoLegalName, CoVATNumber, CoFiscalCode, CoLegalForm,
		CoCCIAAOffice, CoCCIAANumber, CoCountry, CoNUTS, CoFoundedYear, CoIsSME,
		CoAttribution, CoUpdatedAt,
	}
}

// foundedYearCol writes the bound when set, or the SQL DEFAULT keyword (→ NULL)
// when cleared. Same mechanism and same reason as valueMinCol in
// client_profile_repo.go: (*Col[int32]).Val cannot express a NULL.
func foundedYearCol(v *int32) pg.ColumnValue {
	if v == nil {
		return CoFoundedYear.SetDefault()
	}
	return CoFoundedYear.Val(*v)
}

func dbCompanyToIdentity(row DBCompany) (company.Identity, error) {
	stored := map[company.FieldKey]storedAttribution{}
	if len(row.Attribution) > 0 {
		if err := json.Unmarshal(row.Attribution, &stored); err != nil {
			return company.Identity{}, err
		}
	}
	attribution := make(map[company.FieldKey]company.Attribution, len(stored))
	for k, s := range stored {
		attribution[k] = fromStoredAttribution(s)
	}
	return company.Identity{
		LegalName:   row.LegalName,
		VATNumber:   row.VATNumber,
		FiscalCode:  row.FiscalCode,
		LegalForm:   company.LegalForm(row.LegalForm),
		CCIAAOffice: row.CCIAAOffice,
		CCIAANumber: row.CCIAANumber,
		Country:     row.Country,
		NUTS:        row.NUTS,
		FoundedYear: row.FoundedYear,
		IsSME:       row.IsSME,
		Attribution: attribution,
	}, nil
}

// ── SOA categories ──────────────────────────────────────────────────────────

// PutSOACategory upserts on the (workspace_id, category) unique. The classifica
// and the validity dates are corrections of one attestation, never a second row.
func (r *CompanyRepo) PutSOACategory(ctx context.Context, workspaceID string, c company.SOACategory) (company.SOACategory, error) {
	set := []pg.ColumnValue{
		SOAClassifica.Val(int16(c.Classifica)),
		SOAIssuedBy.Val(c.IssuedBy),
		nullableTime(SOAValidUntil, c.ValidUntil),
		nullableTime(SOAVerifiedUntil, c.VerifiedUntil),
		SOAProvenance.Val(string(c.Provenance)),
		nullableFloat(SOAConfidence, c.Confidence),
		nullableUUID(SOAStatedBy, c.StatedBy),
		SOAStatedAt.Val(statedAtOrNow(c.StatedAt)),
		SOAPromptedBy.Val(string(c.PromptedBy)),
		nullableInt64(SOAPromptedByTenderID, c.PromptedByTenderID),
		SOASourceNote.Val(c.SourceNote),
	}
	row := DBSOACategory{}
	err := r.db.Insert(CompanySOACategories).
		Row(append([]pg.ColumnValue{SOAWorkspaceID.Val(workspaceID), SOACategoryCol.Val(c.Category)}, set...)...).
		OnConflictUpdate(SOAWorkspaceID, SOACategoryCol).
		Set(set...).
		Done().
		Returning(soaColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.SOACategory{}, err
	}
	return dbSOAToDomain(row), nil
}

func (r *CompanyRepo) DeleteSOACategory(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanySOACategories).
		Where(SOAWorkspaceID.Eq(workspaceID), SOAID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listSOA(ctx context.Context, workspaceID string) ([]company.SOACategory, error) {
	var rows []DBSOACategory
	err := r.db.Select().From(CompanySOACategories).
		Where(SOAWorkspaceID.Eq(workspaceID)).
		OrderBy(SOACategoryCol.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.SOACategory, len(rows))
	for i, row := range rows {
		out[i] = dbSOAToDomain(row)
	}
	return out, nil
}

func soaColumns() []drops.Expression {
	return []drops.Expression{
		SOAID, SOAWorkspaceID, SOACategoryCol, SOAClassifica, SOAIssuedBy,
		SOAValidUntil, SOAVerifiedUntil, SOAProvenance, SOAConfidence, SOAStatedBy,
		SOAStatedAt, SOAPromptedBy, SOAPromptedByTenderID, SOASourceNote,
	}
}

func dbSOAToDomain(row DBSOACategory) company.SOACategory {
	return company.SOACategory{
		ID:            row.ID,
		Category:      row.Category,
		Classifica:    company.Classifica(row.Classifica),
		IssuedBy:      row.IssuedBy,
		ValidUntil:    row.ValidUntil,
		VerifiedUntil: row.VerifiedUntil,
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

// ── Certifications ──────────────────────────────────────────────────────────

// PutCertification upserts on (workspace_id, standard, scope): a company can
// genuinely hold the same standard under two scopes, so scope is part of the
// record's identity rather than an attribute of it.
func (r *CompanyRepo) PutCertification(ctx context.Context, workspaceID string, c company.Certification) (company.Certification, error) {
	set := []pg.ColumnValue{
		CertStandardRaw.Val(c.StandardRaw),
		CertIssuedBy.Val(c.IssuedBy),
		nullableTime(CertValidFrom, c.ValidFrom),
		nullableTime(CertValidUntil, c.ValidUntil),
		CertProvenance.Val(string(c.Provenance)),
		nullableFloat(CertConfidence, c.Confidence),
		nullableUUID(CertStatedBy, c.StatedBy),
		CertStatedAt.Val(statedAtOrNow(c.StatedAt)),
		CertPromptedBy.Val(string(c.PromptedBy)),
		nullableInt64(CertPromptedByTender, c.PromptedByTenderID),
		CertSourceNote.Val(c.SourceNote),
	}
	var row DBCertification
	err := r.db.Insert(CompanyCertifications).
		Row(append([]pg.ColumnValue{
			CertWorkspaceID.Val(workspaceID),
			CertStandard.Val(string(c.Standard)),
			CertScope.Val(c.Scope),
		}, set...)...).
		OnConflictUpdate(CertWorkspaceID, CertStandard, CertScope).
		Set(set...).
		Done().
		Returning(certColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.Certification{}, err
	}
	return dbCertToDomain(row), nil
}

func (r *CompanyRepo) DeleteCertification(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyCertifications).
		Where(CertWorkspaceID.Eq(workspaceID), CertID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listCertifications(ctx context.Context, workspaceID string) ([]company.Certification, error) {
	var rows []DBCertification
	err := r.db.Select().From(CompanyCertifications).
		Where(CertWorkspaceID.Eq(workspaceID)).
		OrderBy(CertStandard.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.Certification, len(rows))
	for i, row := range rows {
		out[i] = dbCertToDomain(row)
	}
	return out, nil
}

func certColumns() []drops.Expression {
	return []drops.Expression{
		CertID, CertWorkspaceID, CertStandard, CertStandardRaw, CertScope, CertIssuedBy,
		CertValidFrom, CertValidUntil, CertProvenance, CertConfidence, CertStatedBy,
		CertStatedAt, CertPromptedBy, CertPromptedByTender, CertSourceNote,
	}
}

func dbCertToDomain(row DBCertification) company.Certification {
	return company.Certification{
		ID:          row.ID,
		Standard:    company.CertificationStandard(row.Standard),
		StandardRaw: row.StandardRaw,
		Scope:       row.Scope,
		IssuedBy:    row.IssuedBy,
		ValidFrom:   row.ValidFrom,
		ValidUntil:  row.ValidUntil,
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

// ── Financial years ─────────────────────────────────────────────────────────

// PutFinancialYear upserts on (workspace_id, year) — the year IS the identity
// of an esercizio.
func (r *CompanyRepo) PutFinancialYear(ctx context.Context, workspaceID string, f company.FinancialYear) (company.FinancialYear, error) {
	set := []pg.ColumnValue{
		nullableInt64(FYTurnoverMinor, f.TurnoverMinor),
		nullableInt64(FYSpecificTurnover, f.SpecificTurnoverMinor),
		FYCurrency.Val(currencyOrEUR(f.Currency)),
		nullableInt32(FYHeadcount, f.Headcount),
		FYProvenance.Val(string(f.Provenance)),
		nullableFloat(FYConfidence, f.Confidence),
		nullableUUID(FYStatedBy, f.StatedBy),
		FYStatedAt.Val(statedAtOrNow(f.StatedAt)),
		FYPromptedBy.Val(string(f.PromptedBy)),
		nullableInt64(FYPromptedByTender, f.PromptedByTenderID),
		FYSourceNote.Val(f.SourceNote),
	}
	var row DBFinancialYear
	err := r.db.Insert(CompanyFinancialYears).
		Row(append([]pg.ColumnValue{FYWorkspaceID.Val(workspaceID), FYYear.Val(int16(f.Year))}, set...)...).
		OnConflictUpdate(FYWorkspaceID, FYYear).
		Set(set...).
		Done().
		Returning(fyColumns()...).
		One(ctx, &row)
	if err != nil {
		return company.FinancialYear{}, err
	}
	return dbFYToDomain(row), nil
}

func (r *CompanyRepo) DeleteFinancialYear(ctx context.Context, workspaceID string, year int32) error {
	res, err := r.db.Delete(CompanyFinancialYears).
		Where(FYWorkspaceID.Eq(workspaceID), FYYear.Eq(int16(year))).Exec(ctx)
	return deletedOne(res, err)
}

// listFinancialYears returns the esercizi NEWEST FIRST, which is the order
// company.Dossier documents and the order the turnover window depends on: the
// engine reads d.FinancialYears[0].Year as the newest esercizio on file.
func (r *CompanyRepo) listFinancialYears(ctx context.Context, workspaceID string) ([]company.FinancialYear, error) {
	var rows []DBFinancialYear
	err := r.db.Select().From(CompanyFinancialYears).
		Where(FYWorkspaceID.Eq(workspaceID)).
		OrderBy(FYYear.Desc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.FinancialYear, len(rows))
	for i, row := range rows {
		out[i] = dbFYToDomain(row)
	}
	return out, nil
}

func fyColumns() []drops.Expression {
	return []drops.Expression{
		FYID, FYWorkspaceID, FYYear, FYTurnoverMinor, FYSpecificTurnover, FYCurrency,
		FYHeadcount, FYProvenance, FYConfidence, FYStatedBy, FYStatedAt, FYPromptedBy,
		FYPromptedByTender, FYSourceNote,
	}
}

func dbFYToDomain(row DBFinancialYear) company.FinancialYear {
	return company.FinancialYear{
		Year:                  int32(row.Year),
		TurnoverMinor:         row.TurnoverMinor,
		SpecificTurnoverMinor: row.SpecificTurnover,
		Currency:              row.Currency,
		Headcount:             row.Headcount,
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

// ── Past contracts ──────────────────────────────────────────────────────────

// PutPastContract inserts when p.ID is empty and updates that row otherwise.
// There is no natural key here on purpose: two genuinely different referenze can
// share a buyer, a CPV and a value, so collapsing them on any content tuple
// would silently delete a reference the company is entitled to claim.
func (r *CompanyRepo) PutPastContract(ctx context.Context, workspaceID string, p company.PastContract) (company.PastContract, error) {
	set := []pg.ColumnValue{
		PCDescription.Val(p.Description),
		PCBuyerName.Val(p.BuyerName),
		PCCPV.Val(p.CPV),
		nullableInt64(PCValueMinor, p.ValueMinor),
		PCCurrency.Val(currencyOrEUR(p.Currency)),
		nullableTime(PCStartedOn, p.StartedOn),
		nullableTime(PCEndedOn, p.EndedOn),
		PCRole.Val(string(p.Role)),
		nullableFloat(PCSharePct, p.SharePct),
		PCProvenance.Val(string(p.Provenance)),
		nullableFloat(PCConfidence, p.Confidence),
		nullableUUID(PCStatedBy, p.StatedBy),
		PCStatedAt.Val(statedAtOrNow(p.StatedAt)),
		PCPromptedBy.Val(string(p.PromptedBy)),
		nullableInt64(PCPromptedByTender, p.PromptedByTenderID),
		PCSourceNote.Val(p.SourceNote),
	}

	var row DBPastContract
	if p.ID == "" {
		err := r.db.Insert(CompanyPastContracts).
			Row(append([]pg.ColumnValue{PCWorkspaceID.Val(workspaceID)}, set...)...).
			Returning(pcColumns()...).
			One(ctx, &row)
		if err != nil {
			return company.PastContract{}, err
		}
		return dbPCToDomain(row), nil
	}

	err := r.db.Update(CompanyPastContracts).
		Set(set...).
		Where(PCWorkspaceID.Eq(workspaceID), PCID.Eq(p.ID)).
		Returning(pcColumns()...).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return company.PastContract{}, company.ErrRecordNotFound
	}
	if err != nil {
		return company.PastContract{}, err
	}
	return dbPCToDomain(row), nil
}

func (r *CompanyRepo) DeletePastContract(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyPastContracts).
		Where(PCWorkspaceID.Eq(workspaceID), PCID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listPastContracts(ctx context.Context, workspaceID string) ([]company.PastContract, error) {
	var rows []DBPastContract
	err := r.db.Select().From(CompanyPastContracts).
		Where(PCWorkspaceID.Eq(workspaceID)).
		OrderBy(PCEndedOn.Desc(), PCID.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.PastContract, len(rows))
	for i, row := range rows {
		out[i] = dbPCToDomain(row)
	}
	return out, nil
}

func pcColumns() []drops.Expression {
	return []drops.Expression{
		PCID, PCWorkspaceID, PCDescription, PCBuyerName, PCCPV, PCValueMinor, PCCurrency,
		PCStartedOn, PCEndedOn, PCRole, PCSharePct, PCProvenance, PCConfidence, PCStatedBy,
		PCStatedAt, PCPromptedBy, PCPromptedByTender, PCSourceNote,
	}
}

func dbPCToDomain(row DBPastContract) company.PastContract {
	return company.PastContract{
		ID:          row.ID,
		Description: row.Description,
		BuyerName:   row.BuyerName,
		CPV:         row.CPV,
		ValueMinor:  row.ValueMinor,
		Currency:    row.Currency,
		StartedOn:   row.StartedOn,
		EndedOn:     row.EndedOn,
		Role:        company.ContractRole(row.Role),
		SharePct:    row.SharePct,
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

// ── Registrations ───────────────────────────────────────────────────────────

// PutRegistration inserts when r.ID is empty and updates that row otherwise —
// same id-only identity as past contracts, since one company can hold several
// iscrizioni of the same kind in different sections and authorities.
func (r *CompanyRepo) PutRegistration(ctx context.Context, workspaceID string, reg company.Registration) (company.Registration, error) {
	set := []pg.ColumnValue{
		RegKind.Val(string(reg.Kind)),
		RegAuthority.Val(reg.Authority),
		RegIdentifier.Val(reg.Identifier),
		RegSection.Val(reg.Section),
		nullableTime(RegValidFrom, reg.ValidFrom),
		nullableTime(RegValidUntil, reg.ValidUntil),
		RegProvenance.Val(string(reg.Provenance)),
		nullableFloat(RegConfidence, reg.Confidence),
		nullableUUID(RegStatedBy, reg.StatedBy),
		RegStatedAt.Val(statedAtOrNow(reg.StatedAt)),
		RegPromptedBy.Val(string(reg.PromptedBy)),
		nullableInt64(RegPromptedByTender, reg.PromptedByTenderID),
		RegSourceNote.Val(reg.SourceNote),
	}

	var row DBRegistration
	if reg.ID == "" {
		err := r.db.Insert(CompanyRegistrations).
			Row(append([]pg.ColumnValue{RegWorkspaceID.Val(workspaceID)}, set...)...).
			Returning(regColumns()...).
			One(ctx, &row)
		if err != nil {
			return company.Registration{}, err
		}
		return dbRegToDomain(row), nil
	}

	err := r.db.Update(CompanyRegistrations).
		Set(set...).
		Where(RegWorkspaceID.Eq(workspaceID), RegID.Eq(reg.ID)).
		Returning(regColumns()...).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return company.Registration{}, company.ErrRecordNotFound
	}
	if err != nil {
		return company.Registration{}, err
	}
	return dbRegToDomain(row), nil
}

func (r *CompanyRepo) DeleteRegistration(ctx context.Context, workspaceID, id string) error {
	res, err := r.db.Delete(CompanyRegistrations).
		Where(RegWorkspaceID.Eq(workspaceID), RegID.Eq(id)).Exec(ctx)
	return deletedOne(res, err)
}

func (r *CompanyRepo) listRegistrations(ctx context.Context, workspaceID string) ([]company.Registration, error) {
	var rows []DBRegistration
	err := r.db.Select().From(CompanyRegistrations).
		Where(RegWorkspaceID.Eq(workspaceID)).
		OrderBy(RegKind.Asc(), RegID.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.Registration, len(rows))
	for i, row := range rows {
		out[i] = dbRegToDomain(row)
	}
	return out, nil
}

func regColumns() []drops.Expression {
	return []drops.Expression{
		RegID, RegWorkspaceID, RegKind, RegAuthority, RegIdentifier, RegSection,
		RegValidFrom, RegValidUntil, RegProvenance, RegConfidence, RegStatedBy,
		RegStatedAt, RegPromptedBy, RegPromptedByTender, RegSourceNote,
	}
}

func dbRegToDomain(row DBRegistration) company.Registration {
	return company.Registration{
		ID:         row.ID,
		Kind:       company.RegistrationKind(row.Kind),
		Authority:  row.Authority,
		Identifier: row.Identifier,
		Section:    row.Section,
		ValidFrom:  row.ValidFrom,
		ValidUntil: row.ValidUntil,
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

// ── Nullable-column helpers ─────────────────────────────────────────────────
//
// (*Col[T]).Val is typed on T and cannot express a SQL NULL, so a cleared
// optional column is written with SetDefault — the same mechanism
// valueMinCol/valueMaxCol and bidOutcomeCol already use. These are generic
// because the company tables carry twenty-odd nullable columns across six
// tables, and a per-column helper for each would be twenty copies of one line.

func nullableTime(c *pg.Col[time.Time], v *time.Time) pg.ColumnValue {
	if v == nil {
		return c.SetDefault()
	}
	return c.Val(*v)
}

func nullableFloat(c *pg.Col[float64], v *float64) pg.ColumnValue {
	if v == nil {
		return c.SetDefault()
	}
	return c.Val(*v)
}

func nullableInt64(c *pg.Col[int64], v *int64) pg.ColumnValue {
	if v == nil {
		return c.SetDefault()
	}
	return c.Val(*v)
}

func nullableInt32(c *pg.Col[int32], v *int32) pg.ColumnValue {
	if v == nil {
		return c.SetDefault()
	}
	return c.Val(*v)
}

// nullableUUID writes NULL for an empty id. The distinction matters here more
// than elsewhere: stated_by is empty exactly when NO HUMAN was involved, and a
// uuid column cannot hold the empty string to say so.
func nullableUUID(c *pg.Col[string], v string) pg.ColumnValue {
	if v == "" {
		return c.SetDefault()
	}
	return c.Val(v)
}

// statedAtOrNow keeps stated_at NOT NULL even if a caller forgot to stamp it.
// The service always does, so this is a belt on the braces rather than a policy.
func statedAtOrNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

// currencyOrEUR keeps the currency column NOT NULL. EUR is the column default
// and the only currency this corpus carries in practice.
func currencyOrEUR(c string) string {
	if c == "" {
		return "EUR"
	}
	return c
}

// deletedOne turns a zero-row DELETE into ErrRecordNotFound, so a caller can
// tell "removed it" from "there was nothing to remove" — a distinction the
// transport layer needs in order to answer 404 rather than 200.
func deletedOne(res drops.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return company.ErrRecordNotFound
	}
	return nil
}
