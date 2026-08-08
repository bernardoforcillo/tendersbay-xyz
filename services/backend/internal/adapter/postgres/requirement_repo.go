package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// RequirementRepo persists the participation requirements one workspace has
// captured for one tender.
type RequirementRepo struct{ db *pg.DB }

func NewRequirementRepo(db *pg.DB) *RequirementRepo { return &RequirementRepo{db: db} }

var _ company.RequirementRepository = (*RequirementRepo)(nil)

// ── Stored payload shapes ───────────────────────────────────────────────────
//
// The five per-kind payloads and the citation are stored as JSONB rather than
// as twenty sparse columns, because exactly one of them is ever populated and a
// column set that is 80% NULL by construction describes the storage engine
// rather than the data. As with storedAttribution in company_repo.go, the
// on-disk field names are declared HERE: the encoding is this adapter's
// contract, so a domain rename must not silently orphan stored rows.

type storedSOARequirement struct {
	Category   string `json:"category"`
	Classifica int    `json:"classifica"`
	Prevalente bool   `json:"prevalente"`
}

type storedCertificationRequirement struct {
	Standard    string `json:"standard"`
	StandardRaw string `json:"standard_raw,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type storedAmountRequirement struct {
	MinMinor int64  `json:"min_minor"`
	Currency string `json:"currency,omitempty"`
	Years    int32  `json:"years,omitempty"`
	Specific bool   `json:"specific,omitempty"`
	CPV      string `json:"cpv,omitempty"`
}

type storedCountRequirement struct {
	Min int32 `json:"min"`
}

type storedRegistrationRequirement struct {
	Kind    string `json:"kind"`
	Section string `json:"section,omitempty"`
}

type storedRequirementPayload struct {
	SOA           *storedSOARequirement           `json:"soa,omitempty"`
	Certification *storedCertificationRequirement `json:"certification,omitempty"`
	Amount        *storedAmountRequirement        `json:"amount,omitempty"`
	Count         *storedCountRequirement         `json:"count,omitempty"`
	Registration  *storedRegistrationRequirement  `json:"registration,omitempty"`
}

// storedCitation mirrors document.Citation. PageStart/PageEnd and SectionPath
// stay optional because they are optional in the corpus itself — the ingestion
// pass populates them going forward only — so a renderer must degrade all the
// way to the document URL alone.
type storedCitation struct {
	DocumentURL  string   `json:"document_url"`
	DocumentType string   `json:"document_type,omitempty"`
	PartIndex    int      `json:"part_index"`
	PageStart    *int     `json:"page_start,omitempty"`
	PageEnd      *int     `json:"page_end,omitempty"`
	SectionPath  []string `json:"section_path,omitempty"`
	SectionTitle string   `json:"section_title,omitempty"`
	HasTable     bool     `json:"has_table,omitempty"`
}

func marshalRequirementPayload(r company.Requirement) (json.RawMessage, error) {
	var p storedRequirementPayload
	if r.SOA != nil {
		p.SOA = &storedSOARequirement{
			Category:   r.SOA.Category,
			Classifica: int(r.SOA.Classifica),
			Prevalente: r.SOA.Prevalente,
		}
	}
	if r.Certification != nil {
		p.Certification = &storedCertificationRequirement{
			Standard:    string(r.Certification.Standard),
			StandardRaw: r.Certification.StandardRaw,
			Scope:       r.Certification.Scope,
		}
	}
	if r.Amount != nil {
		p.Amount = &storedAmountRequirement{
			MinMinor: r.Amount.MinMinor,
			Currency: r.Amount.Currency,
			Years:    r.Amount.Years,
			Specific: r.Amount.Specific,
			CPV:      r.Amount.CPV,
		}
	}
	if r.Count != nil {
		p.Count = &storedCountRequirement{Min: r.Count.Min}
	}
	if r.Registration != nil {
		p.Registration = &storedRegistrationRequirement{
			Kind:    string(r.Registration.Kind),
			Section: r.Registration.Section,
		}
	}
	return json.Marshal(p)
}

func unmarshalRequirementPayload(raw json.RawMessage, r *company.Requirement) error {
	if len(raw) == 0 {
		return nil
	}
	var p storedRequirementPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.SOA != nil {
		r.SOA = &company.SOARequirement{
			Category:   p.SOA.Category,
			Classifica: company.Classifica(p.SOA.Classifica),
			Prevalente: p.SOA.Prevalente,
		}
	}
	if p.Certification != nil {
		r.Certification = &company.CertificationRequirement{
			Standard:    company.CertificationStandard(p.Certification.Standard),
			StandardRaw: p.Certification.StandardRaw,
			Scope:       p.Certification.Scope,
		}
	}
	if p.Amount != nil {
		r.Amount = &company.AmountRequirement{
			MinMinor: p.Amount.MinMinor,
			Currency: p.Amount.Currency,
			Years:    p.Amount.Years,
			Specific: p.Amount.Specific,
			CPV:      p.Amount.CPV,
		}
	}
	if p.Count != nil {
		r.Count = &company.CountRequirement{Min: p.Count.Min}
	}
	if p.Registration != nil {
		r.Registration = &company.RegistrationRequirement{
			Kind:    company.RegistrationKind(p.Registration.Kind),
			Section: p.Registration.Section,
		}
	}
	return nil
}

func citationCol(c *document.Citation) (pg.ColumnValue, error) {
	if c == nil {
		return TReqCitation.SetDefault(), nil
	}
	raw, err := json.Marshal(storedCitation{
		DocumentURL:  c.DocumentURL,
		DocumentType: c.DocumentType,
		PartIndex:    c.PartIndex,
		PageStart:    c.PageStart,
		PageEnd:      c.PageEnd,
		SectionPath:  c.SectionPath,
		SectionTitle: c.SectionTitle,
		HasTable:     c.HasTable,
	})
	if err != nil {
		return nil, err
	}
	return TReqCitation.Val(raw), nil
}

func unmarshalCitation(raw *json.RawMessage) (*document.Citation, error) {
	if raw == nil || len(*raw) == 0 {
		return nil, nil
	}
	var s storedCitation
	if err := json.Unmarshal(*raw, &s); err != nil {
		return nil, err
	}
	return &document.Citation{
		DocumentURL:  s.DocumentURL,
		DocumentType: s.DocumentType,
		PartIndex:    s.PartIndex,
		PageStart:    s.PageStart,
		PageEnd:      s.PageEnd,
		SectionPath:  s.SectionPath,
		SectionTitle: s.SectionTitle,
		HasTable:     s.HasTable,
	}, nil
}

// ── Reads ───────────────────────────────────────────────────────────────────

// ListRequirements returns every requirement this workspace holds for tenderID,
// across all lots. Lot filtering is the domain's job (company.Service knows that
// a notice-level requirement is inherited by every lot); the repository returns
// the whole set so that rule lives in exactly one place.
func (r *RequirementRepo) ListRequirements(ctx context.Context, workspaceID string, tenderID int64) ([]company.Requirement, error) {
	var rows []DBTenderRequirement
	err := r.db.Select().From(TenderRequirements).
		Where(TReqWorkspaceID.Eq(workspaceID), TReqTenderID.Eq(tenderID)).
		OrderBy(TReqCreatedAt.Asc(), TReqID.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]company.Requirement, 0, len(rows))
	for _, row := range rows {
		req, err := dbRequirementToDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, req)
	}
	return out, nil
}

// ── Writes ──────────────────────────────────────────────────────────────────

// PutRequirements upserts the whole batch on
// (workspace_id, tender_id, lot_ref, dedupe_key) inside ONE transaction.
//
// The transaction is not incidental: an agent re-extracts on every turn, and a
// batch that failed halfway would leave a tender's requirement set describing
// neither the previous extraction nor the new one — a state no consumer could
// interpret and no user could have caused.
//
// confirmed_by/confirmed_at are deliberately NOT in the conflict-update set. A
// re-extraction of an already-confirmed requirement must not silently revoke the
// human's confirmation, and must not carry one forward either — the confirmation
// belongs to the row, and ConfirmRequirement is the only thing that grants it.
func (r *RequirementRepo) PutRequirements(ctx context.Context, workspaceID string, tenderID int64, reqs []company.Requirement) ([]company.Requirement, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	out := make([]company.Requirement, 0, len(reqs))
	err := r.db.InTx(ctx, func(tx *pg.DB) error {
		out = out[:0]
		now := time.Now()
		for _, req := range reqs {
			payload, err := marshalRequirementPayload(req)
			if err != nil {
				return err
			}
			citation, err := citationCol(req.Citation)
			if err != nil {
				return err
			}
			set := []pg.ColumnValue{
				TReqKind.Val(string(req.Kind)),
				TReqPayload.Val(payload),
				TReqText.Val(req.Text),
				TReqBlocking.Val(req.Blocking),
				TReqSource.Val(string(req.Source)),
				TReqExcerpt.Val(req.Excerpt),
				citation,
				TReqUpdatedAt.Val(now),
			}
			row := DBTenderRequirement{}
			insert := []pg.ColumnValue{
				TReqWorkspaceID.Val(workspaceID),
				TReqTenderID.Val(tenderID),
				TReqLotRef.Val(req.LotRef),
				TReqDedupeKey.Val(req.DedupeKey()),
				nullableUUID(TReqConfirmedBy, req.ConfirmedBy),
				nullableTime(TReqConfirmedAt, req.ConfirmedAt),
			}
			err = tx.Insert(TenderRequirements).
				Row(append(insert, set...)...).
				OnConflictUpdate(TReqWorkspaceID, TReqTenderID, TReqLotRef, TReqDedupeKey).
				Set(set...).
				Done().
				Returning(requirementColumns()...).
				One(ctx, &row)
			if err != nil {
				return err
			}
			domain, err := dbRequirementToDomain(row)
			if err != nil {
				return err
			}
			out = append(out, domain)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConfirmRequirement records that userID checked the extraction against the
// quoted passage. It is scoped by workspace_id as well as by id, so a
// requirement id leaked into another tenant cannot be confirmed from there.
func (r *RequirementRepo) ConfirmRequirement(ctx context.Context, workspaceID, requirementID, userID string) (company.Requirement, error) {
	now := time.Now()
	var row DBTenderRequirement
	err := r.db.Update(TenderRequirements).
		Set(TReqConfirmedBy.Val(userID), TReqConfirmedAt.Val(now), TReqUpdatedAt.Val(now)).
		Where(TReqWorkspaceID.Eq(workspaceID), TReqID.Eq(requirementID)).
		Returning(requirementColumns()...).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return company.Requirement{}, company.ErrRecordNotFound
	}
	if err != nil {
		return company.Requirement{}, err
	}
	return dbRequirementToDomain(row)
}

func (r *RequirementRepo) DeleteRequirement(ctx context.Context, workspaceID, requirementID string) error {
	res, err := r.db.Delete(TenderRequirements).
		Where(TReqWorkspaceID.Eq(workspaceID), TReqID.Eq(requirementID)).Exec(ctx)
	return deletedOne(res, err)
}

func requirementColumns() []drops.Expression {
	return []drops.Expression{
		TReqID, TReqWorkspaceID, TReqTenderID, TReqLotRef, TReqKind, TReqPayload,
		TReqText, TReqBlocking, TReqSource, TReqExcerpt, TReqCitation, TReqDedupeKey,
		TReqConfirmedBy, TReqConfirmedAt, TReqCreatedAt, TReqUpdatedAt,
	}
}

func dbRequirementToDomain(row DBTenderRequirement) (company.Requirement, error) {
	req := company.Requirement{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		TenderID:    row.TenderID,
		LotRef:      row.LotRef,
		Kind:        company.RequirementKind(row.Kind),
		Text:        row.Text,
		Blocking:    row.Blocking,
		Source:      company.RequirementSource(row.Source),
		Excerpt:     row.Excerpt,
		ConfirmedBy: derefString(row.ConfirmedBy),
		ConfirmedAt: row.ConfirmedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if err := unmarshalRequirementPayload(row.Payload, &req); err != nil {
		return company.Requirement{}, err
	}
	citation, err := unmarshalCitation(row.Citation)
	if err != nil {
		return company.Requirement{}, err
	}
	req.Citation = citation
	return req, nil
}
