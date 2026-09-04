package company

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// Service owns the company dossier and the eligibility read. Every method
// authorizes against workspace membership first, validates second, and only
// then touches a port — no business rule leaks into the transport or adapter
// layers.
type Service struct {
	repo      Repository
	reqs      RequirementRepository
	members   MemberRepository
	tenders   Tenders
	documents Documents

	// now is injected rather than called directly so the engine's expiry
	// comparisons and the validators' year bounds are reproducible in tests.
	// Evaluate itself takes now as a parameter and stays pure; this field only
	// decides what the service passes it.
	now func() time.Time
}

// NewService wires the Service.
func NewService(repo Repository, reqs RequirementRepository, members MemberRepository, tenders Tenders, documents Documents) *Service {
	return &Service{repo: repo, reqs: reqs, members: members, tenders: tenders, documents: documents, now: time.Now}
}

// requireMember gates a READ. Membership alone is enough: a dossier is a
// workspace-level asset every member works from.
func (s *Service) requireMember(ctx context.Context, workspaceID, userID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: workspace id and user id are required", ErrInvalidArgument)
	}
	_, err := s.members.LoadMembership(ctx, workspaceID, userID)
	return err
}

// requireManage gates a WRITE. The dossier is a workspace-level asset like the
// client profile, and PermManageWorkspace is the existing bit for that class of
// change.
//
// There is no separate owner bypass here, and none is needed:
// workspace.CreateWorkspace seeds the creator as a member holding the "Admin"
// role, whose mask includes PermAdministrator. Loading membership therefore
// already answers for the owner. (workspace.Service.authorize additionally
// short-circuits on ws.OwnerID because it holds the Workspace row; this port
// deliberately does not, since widening it to fetch workspaces would give the
// company domain a reason to know about workspace lifecycle.)
func (s *Service) requireManage(ctx context.Context, workspaceID, userID string) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: workspace id and user id are required", ErrInvalidArgument)
	}
	m, err := s.members.LoadMembership(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	perms := m.Role.Permissions
	if perms.Has(workspace.PermAdministrator) || perms.Has(workspace.PermManageWorkspace) {
		return nil
	}
	return workspace.ErrForbidden
}

// ── Dossier reads ───────────────────────────────────────────────────────────

// GetDossier returns workspaceID's company dossier, or ErrDossierNotFound when
// nothing has ever been asserted about it.
func (s *Service) GetDossier(ctx context.Context, userID, workspaceID string) (Dossier, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return Dossier{}, err
	}
	return s.repo.GetDossier(ctx, workspaceID)
}

// ── Identity ────────────────────────────────────────────────────────────────

// UpdateIdentity full-replaces the identity scalars and re-stamps provenance
// for the fields this edit actually changed.
//
// attr carries only the CONTEXT of the edit — why we asked, which gara prompted
// it, the human-readable trace. The provenance itself is NOT taken from the
// caller: a direct dossier write is by construction a human typing into the
// form, so it is stamped ProvenanceUserStated with StatedBy = the authenticated
// caller. A transport or agent layer therefore has no lever with which to forge
// a stronger claim than the one it actually made.
//
// A field left unchanged keeps its existing attribution, and a field the caller
// CLEARED keeps a fresh one: "no attribution entry" means nobody ever asserted
// this field, which is a different state from an empty string a human
// deliberately blanked.
func (s *Service) UpdateIdentity(ctx context.Context, userID, workspaceID string, id Identity, attr Attribution) (Identity, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Identity{}, err
	}
	now := s.now()
	stamp := s.stampAttribution(attr, ProvenanceUserStated, userID, now)

	prior, err := s.repo.GetDossier(ctx, workspaceID)
	if err != nil && !errors.Is(err, ErrDossierNotFound) {
		return Identity{}, err
	}

	merged := make(map[FieldKey]Attribution, len(validFieldKeys))
	for k := range validFieldKeys {
		if identityFieldValue(id, k) == identityFieldValue(prior.Identity, k) {
			if a, ok := prior.Identity.Attribution[k]; ok {
				merged[k] = a
			}
			continue
		}
		merged[k] = stamp
	}
	id.Attribution = merged

	if err := id.validate(now); err != nil {
		return Identity{}, err
	}
	return s.repo.UpsertIdentity(ctx, workspaceID, id)
}

// stampAttribution derives the provenance envelope from HOW the call arrived
// rather than from what the caller asked for. Confidence is dropped outright:
// nothing in this phase produces a model-scored fact, and an envelope that
// carried one on a human-stated value would make the two look interchangeable
// to any later scoring pass.
func (s *Service) stampAttribution(in Attribution, p Provenance, userID string, now time.Time) Attribution {
	promptedBy := in.PromptedBy
	if !promptedBy.Valid() {
		promptedBy = PromptOnboarding
	}
	note := in.SourceNote
	if len(note) > maxSourceNoteLen {
		note = note[:maxSourceNoteLen]
	}
	out := Attribution{
		Provenance:         p,
		StatedBy:           userID,
		StatedAt:           now,
		PromptedBy:         promptedBy,
		PromptedByTenderID: in.PromptedByTenderID,
		SourceNote:         note,
	}
	// A non-human provenance is the only one that may carry a confidence, and
	// nothing in Phase 2 produces one — the branch exists so the rule is written
	// before a producer arrives, not after.
	if !p.Authoritative() {
		out.StatedBy = ""
		out.Confidence = in.Confidence
	}
	return out
}

// identityFieldValue renders one identity scalar as the comparable string the
// change detection above works on. CCIAA office and number collapse into one
// key because they are one answer to one question — a REA number without its
// provincia identifies nothing.
func identityFieldValue(id Identity, k FieldKey) string {
	switch k {
	case FieldLegalName:
		return id.LegalName
	case FieldVATNumber:
		return id.VATNumber
	case FieldFiscalCode:
		return id.FiscalCode
	case FieldLegalForm:
		return string(id.LegalForm)
	case FieldCCIAA:
		return id.CCIAAOffice + "/" + id.CCIAANumber
	case FieldCountry:
		return id.Country
	case FieldNUTS:
		return id.NUTS
	case FieldFoundedYear:
		if id.FoundedYear == nil {
			return ""
		}
		return strconv.FormatInt(int64(*id.FoundedYear), 10)
	case FieldIsSME:
		return strconv.FormatBool(id.IsSME)
	default:
		return ""
	}
}

// ── Repeated records ────────────────────────────────────────────────────────

// existingDossier reads what is already on file so a partial write can be
// completed from it. A dossier that has never existed is not an error — the
// first answer a workspace ever gives has nothing to merge with.
//
// Any OTHER read failure is returned, deliberately. Proceeding on a failed read
// would write the partial row and destroy whatever it could not see, which is
// the precise outcome the merge exists to prevent; failing the write leaves the
// user's stored facts intact and costs them one retry.
func (s *Service) existingDossier(ctx context.Context, workspaceID string) (Dossier, error) {
	d, err := s.repo.GetDossier(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, ErrDossierNotFound) {
			return Dossier{}, nil
		}
		return Dossier{}, err
	}
	return d, nil
}

// PutSOACategory upserts one attestazione line. The natural key is
// (workspace, category): a company holds one attestation per category at a
// time, so a re-answered question is a correction and not a second row.
//
// Every Put on this surface MERGES with the record already on file rather than
// replacing it, because every caller writes only what it was asked about. The
// just-in-time capture question on the scheda gara asks for a classifica and
// sends a classifica; the settings form's SOA panel is an add form, not an
// editor. Replacing would mean answering "OG1 III" to a gap question silently
// blanks the SOA body and both expiry dates the user typed months earlier —
// and VerifiedUntil going missing turns a lapsed attestation back into a valid
// one, the single most likely wrong "go" this engine can produce (see
// SOACategory.VerifiedUntil).
//
// Clearing a field is therefore not expressible through Put, and must not be:
// the way to retract an assertion is DeleteSOACategory, which is unambiguous.
// agent.Service.mergeFinancialYear reached the same conclusion from the chat
// side; doing it here covers both callers with one rule instead of two.
func (s *Service) PutSOACategory(ctx context.Context, userID, workspaceID string, c SOACategory) (SOACategory, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return SOACategory{}, err
	}
	c.Attribution = s.stampAttribution(c.Attribution, ProvenanceUserStated, userID, s.now())
	if norm, ok := NormalizeSOACategory(c.Category); ok {
		c.Category = norm
	}
	d, err := s.existingDossier(ctx, workspaceID)
	if err != nil {
		return SOACategory{}, err
	}
	c = mergeSOACategory(c, d.SOA)
	if err := c.validate(); err != nil {
		return SOACategory{}, err
	}
	return s.repo.PutSOACategory(ctx, workspaceID, c)
}

// mergeSOACategory fills from the stored attestation for the same category the
// fields this write did not carry. Classifica is never merged: it is the datum
// the natural key exists to correct, and a zero one fails validate() loudly
// instead of silently inheriting the old value.
func mergeSOACategory(in SOACategory, stored []SOACategory) SOACategory {
	for _, s := range stored {
		if s.Category != in.Category {
			continue
		}
		if strings.TrimSpace(in.IssuedBy) == "" {
			in.IssuedBy = s.IssuedBy
		}
		if in.ValidUntil == nil {
			in.ValidUntil = s.ValidUntil
		}
		if in.VerifiedUntil == nil {
			in.VerifiedUntil = s.VerifiedUntil
		}
		break
	}
	return in
}

// DeleteSOACategory removes one attestazione line.
func (s *Service) DeleteSOACategory(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteSOACategory(ctx, workspaceID, id)
}

// PutCertification upserts one certificate on (workspace, standard, scope) — a
// company genuinely can hold ISO 9001 under two scopes, so scope is part of the
// key. It merges with the stored certificate for the same reason
// PutSOACategory does; see that method's comment.
func (s *Service) PutCertification(ctx context.Context, userID, workspaceID string, c Certification) (Certification, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Certification{}, err
	}
	c.Attribution = s.stampAttribution(c.Attribution, ProvenanceUserStated, userID, s.now())
	d, err := s.existingDossier(ctx, workspaceID)
	if err != nil {
		return Certification{}, err
	}
	c = mergeCertification(c, d.Certifications)
	if err := c.validate(); err != nil {
		return Certification{}, err
	}
	return s.repo.PutCertification(ctx, workspaceID, c)
}

// mergeCertification fills from the stored certificate with the same
// (standard, scope) — the record's natural key — the fields this write did not
// carry. Scope is part of the key and so is never merged: a write naming a
// different scope is a different certificate, not an incomplete one.
func mergeCertification(in Certification, stored []Certification) Certification {
	for _, c := range stored {
		if c.Standard != in.Standard || c.Scope != in.Scope {
			continue
		}
		if strings.TrimSpace(in.StandardRaw) == "" {
			in.StandardRaw = c.StandardRaw
		}
		if strings.TrimSpace(in.IssuedBy) == "" {
			in.IssuedBy = c.IssuedBy
		}
		if in.ValidFrom == nil {
			in.ValidFrom = c.ValidFrom
		}
		if in.ValidUntil == nil {
			in.ValidUntil = c.ValidUntil
		}
		break
	}
	return in
}

// DeleteCertification removes one certificate.
func (s *Service) DeleteCertification(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteCertification(ctx, workspaceID, id)
}

// PutFinancialYear upserts one esercizio on (workspace, year), merging with the
// esercizio already on file.
//
// This is the record the merge matters most for, because turnover and headcount
// deliberately share it (see FinancialYear) while the questions that write it
// ask about exactly one of them: a headcount gap on the scheda gara sends a
// headcount and no turnover, so a replace would blank a fatturato the user had
// typed into settings — and the next evaluateTurnover would ask a question the
// product already had the answer to.
func (s *Service) PutFinancialYear(ctx context.Context, userID, workspaceID string, f FinancialYear) (FinancialYear, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return FinancialYear{}, err
	}
	now := s.now()
	f.Attribution = s.stampAttribution(f.Attribution, ProvenanceUserStated, userID, now)
	d, err := s.existingDossier(ctx, workspaceID)
	if err != nil {
		return FinancialYear{}, err
	}
	f = MergeFinancialYear(f, d.FinancialYears)
	if err := f.validate(now); err != nil {
		return FinancialYear{}, err
	}
	return s.repo.PutFinancialYear(ctx, workspaceID, f)
}

// MergeFinancialYear fills the figures a write did not carry from the esercizio
// already on file for the same year.
//
// Exported because agent.Service performs the same merge one layer up, against
// the dossier it has already loaded for the turn — it is the same rule, and two
// independently-maintained copies of it would be one blanked fatturato apart.
func MergeFinancialYear(in FinancialYear, stored []FinancialYear) FinancialYear {
	for _, f := range stored {
		if f.Year != in.Year {
			continue
		}
		if in.TurnoverMinor == nil {
			in.TurnoverMinor = f.TurnoverMinor
		}
		if in.SpecificTurnoverMinor == nil {
			in.SpecificTurnoverMinor = f.SpecificTurnoverMinor
		}
		if in.Headcount == nil {
			in.Headcount = f.Headcount
		}
		if strings.TrimSpace(in.Currency) == "" {
			in.Currency = f.Currency
		}
		break
	}
	return in
}

// DeleteFinancialYear removes one esercizio. It is keyed by year and not by id
// for the same reason the row is: the year IS the identity of the record.
func (s *Service) DeleteFinancialYear(ctx context.Context, userID, workspaceID string, year int32) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	return s.repo.DeleteFinancialYear(ctx, workspaceID, year)
}

// PutPastContract inserts a referenza, or updates one when p.ID is set. Past
// contracts carry no natural key — two genuinely different contracts can share
// a buyer, a CPV and a value — so the row id is the only identity there is.
func (s *Service) PutPastContract(ctx context.Context, userID, workspaceID string, p PastContract) (PastContract, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return PastContract{}, err
	}
	p.Attribution = s.stampAttribution(p.Attribution, ProvenanceUserStated, userID, s.now())
	if err := p.validate(); err != nil {
		return PastContract{}, err
	}
	return s.repo.PutPastContract(ctx, workspaceID, p)
}

// DeletePastContract removes one referenza.
func (s *Service) DeletePastContract(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeletePastContract(ctx, workspaceID, id)
}

// PutRegistration inserts an iscrizione, or updates one when r.ID is set.
func (s *Service) PutRegistration(ctx context.Context, userID, workspaceID string, r Registration) (Registration, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Registration{}, err
	}
	r.Attribution = s.stampAttribution(r.Attribution, ProvenanceUserStated, userID, s.now())
	if err := r.validate(); err != nil {
		return Registration{}, err
	}
	return s.repo.PutRegistration(ctx, workspaceID, r)
}

// DeleteRegistration removes one iscrizione.
func (s *Service) DeleteRegistration(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteRegistration(ctx, workspaceID, id)
}

// ── ESPD sections: representatives and Part III declarations ────────────────

// PutRepresentative inserts a signatory, or updates one when r.ID is set.
func (s *Service) PutRepresentative(ctx context.Context, userID, workspaceID string, r Representative) (Representative, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Representative{}, err
	}
	r.Attribution = s.stampAttribution(r.Attribution, ProvenanceUserStated, userID, s.now())
	if err := r.validate(); err != nil {
		return Representative{}, err
	}
	return s.repo.PutRepresentative(ctx, workspaceID, r)
}

// DeleteRepresentative removes one signatory.
func (s *Service) DeleteRepresentative(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteRepresentative(ctx, workspaceID, id)
}

// PutDeclaration records the operator's answer to one Part III exclusion
// ground, upserting on the criterion. The provenance is stamped user_stated
// here and re-checked by validate, so the only way a non-authoritative
// declaration could reach the repository is a caller that bypassed this
// service — and the repository port carries the same validate for that case.
func (s *Service) PutDeclaration(ctx context.Context, userID, workspaceID string, d Declaration) (Declaration, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Declaration{}, err
	}
	d.Attribution = s.stampAttribution(d.Attribution, ProvenanceUserStated, userID, s.now())
	if err := d.validate(); err != nil {
		return Declaration{}, err
	}
	return s.repo.PutDeclaration(ctx, workspaceID, d)
}

// DeleteDeclaration withdraws an answer; the question becomes unanswered again.
func (s *Service) DeleteDeclaration(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteDeclaration(ctx, workspaceID, id)
}

// PutNationalGround records a Part III.D answer, upserting on (country,
// criterion). Same provenance wall as PutDeclaration.
func (s *Service) PutNationalGround(ctx context.Context, userID, workspaceID string, g NationalGround) (NationalGround, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return NationalGround{}, err
	}
	g.Country = strings.ToUpper(strings.TrimSpace(g.Country))
	g.Attribution = s.stampAttribution(g.Attribution, ProvenanceUserStated, userID, s.now())
	if err := g.validate(); err != nil {
		return NationalGround{}, err
	}
	return s.repo.PutNationalGround(ctx, workspaceID, g)
}

// DeleteNationalGround withdraws a Part III.D answer.
func (s *Service) DeleteNationalGround(ctx context.Context, userID, workspaceID, id string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: record id is required", ErrInvalidArgument)
	}
	return s.repo.DeleteNationalGround(ctx, workspaceID, id)
}

// ── Just-in-time capture ────────────────────────────────────────────────────

// RecordFact is the JIT capture entry point shared by the agent tool and the
// UI's inline answer. It stamps Attribution from (userID, src, tenderID) so no
// caller can write a fact with a forged or missing provenance — the field is
// not a parameter, it is DERIVED from how the call arrived:
//
//   - PromptTender / PromptOnboarding — a human answered a closed question, so
//     the value is ProvenanceUserConfirmed with StatedBy set. What the agent
//     contributed is the question, not the answer.
//   - PromptExtraction — pulled out of a document with no human in the loop, so
//     the value is ProvenanceImported with no StatedBy, and the engine will
//     never let it produce a blocking verdict.
func (s *Service) RecordFact(ctx context.Context, userID, workspaceID string, f Fact, src PromptSource, tenderID *int64) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	kind, ok := f.Kind()
	if !ok {
		return fmt.Errorf("%w: a fact carries exactly one record", ErrInvalidArgument)
	}
	if !src.Valid() {
		return fmt.Errorf("%w: unknown prompt source %q", ErrInvalidArgument, src)
	}

	provenance := ProvenanceUserConfirmed
	if src == PromptExtraction {
		provenance = ProvenanceImported
	}
	now := s.now()
	base := Attribution{PromptedBy: src, PromptedByTenderID: tenderID}

	switch kind {
	case FactIdentity:
		return s.recordIdentityFact(ctx, userID, workspaceID, *f.Identity, base, provenance, now)
	case FactSOA:
		c := *f.SOA
		c.Attribution = s.stampFact(c.Attribution, base, provenance, userID, now)
		if norm, ok := NormalizeSOACategory(c.Category); ok {
			c.Category = norm
		}
		if err := c.validate(); err != nil {
			return err
		}
		_, err := s.repo.PutSOACategory(ctx, workspaceID, c)
		return err
	case FactCertification:
		c := *f.Certification
		c.Attribution = s.stampFact(c.Attribution, base, provenance, userID, now)
		if err := c.validate(); err != nil {
			return err
		}
		_, err := s.repo.PutCertification(ctx, workspaceID, c)
		return err
	case FactFinancialYear:
		fy := *f.FinancialYear
		fy.Attribution = s.stampFact(fy.Attribution, base, provenance, userID, now)
		if err := fy.validate(now); err != nil {
			return err
		}
		_, err := s.repo.PutFinancialYear(ctx, workspaceID, fy)
		return err
	case FactPastContract:
		p := *f.PastContract
		p.Attribution = s.stampFact(p.Attribution, base, provenance, userID, now)
		if err := p.validate(); err != nil {
			return err
		}
		_, err := s.repo.PutPastContract(ctx, workspaceID, p)
		return err
	case FactRegistration:
		r := *f.Registration
		r.Attribution = s.stampFact(r.Attribution, base, provenance, userID, now)
		if err := r.validate(); err != nil {
			return err
		}
		_, err := s.repo.PutRegistration(ctx, workspaceID, r)
		return err
	default:
		return fmt.Errorf("%w: unknown fact kind %q", ErrInvalidArgument, kind)
	}
}

// stampFact merges the caller's own envelope (a SourceNote naming the gara and
// the clause) with the derived provenance. Only SourceNote and Confidence
// survive from the caller — every field that decides what the engine may
// conclude is overwritten.
func (s *Service) stampFact(in, base Attribution, p Provenance, userID string, now time.Time) Attribution {
	base.SourceNote = in.SourceNote
	base.Confidence = in.Confidence
	return s.stampAttribution(base, p, userID, now)
}

// recordIdentityFact writes ONE identity scalar without disturbing the others,
// and stamps provenance for that key alone. It re-reads the dossier because the
// identity row is a full replace at the repository — writing a single field
// through a partial struct would blank every other one.
func (s *Service) recordIdentityFact(ctx context.Context, userID, workspaceID string, fact IdentityFact, base Attribution, p Provenance, now time.Time) error {
	if !validFieldKeys[fact.Key] {
		return fmt.Errorf("%w: %q", ErrUnknownFieldKey, fact.Key)
	}
	current, err := s.repo.GetDossier(ctx, workspaceID)
	if err != nil && !errors.Is(err, ErrDossierNotFound) {
		return err
	}
	id := current.Identity
	if err := applyIdentityField(&id, fact); err != nil {
		return err
	}
	if id.Attribution == nil {
		id.Attribution = map[FieldKey]Attribution{}
	}
	id.Attribution[fact.Key] = s.stampAttribution(base, p, userID, now)
	if err := id.validate(now); err != nil {
		return err
	}
	_, err = s.repo.UpsertIdentity(ctx, workspaceID, id)
	return err
}

// applyIdentityField writes one text answer into the typed identity. The value
// arrives as text for every key because that is what a chat answer and a form
// field both are; the per-key parsing lives here so it cannot differ between
// the two callers.
func applyIdentityField(id *Identity, fact IdentityFact) error {
	v := strings.TrimSpace(fact.Value)
	switch fact.Key {
	case FieldLegalName:
		id.LegalName = v
	case FieldVATNumber:
		id.VATNumber = v
	case FieldFiscalCode:
		id.FiscalCode = v
	case FieldLegalForm:
		id.LegalForm = LegalForm(strings.ToLower(v))
	case FieldCCIAA:
		// "<sigla>/<numero REA>" — one answer to one question. A value with no
		// separator is taken as the REA number, since that is the half a user
		// types when the provincia is already implied by the conversation.
		office, number, found := strings.Cut(v, "/")
		if !found {
			id.CCIAANumber = strings.TrimSpace(office)
			break
		}
		id.CCIAAOffice = strings.ToUpper(strings.TrimSpace(office))
		id.CCIAANumber = strings.TrimSpace(number)
	case FieldCountry:
		id.Country = strings.ToUpper(v)
	case FieldNUTS:
		id.NUTS = strings.ToUpper(v)
	case FieldFoundedYear:
		if v == "" {
			id.FoundedYear = nil
			break
		}
		y, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return fmt.Errorf("%w: founded year %q is not an integer", ErrInvalidArgument, fact.Value)
		}
		y32 := int32(y)
		id.FoundedYear = &y32
	case FieldIsSME:
		b, err := strconv.ParseBool(strings.ToLower(v))
		if err != nil {
			return fmt.Errorf("%w: is_sme %q is not a boolean", ErrInvalidArgument, fact.Value)
		}
		id.IsSME = b
	default:
		return fmt.Errorf("%w: %q", ErrUnknownFieldKey, fact.Key)
	}
	return nil
}

// ── Requirements ────────────────────────────────────────────────────────────

// ListRequirements returns every participation requirement captured for a
// tender in this workspace (a read — any member may see it).
func (s *Service) ListRequirements(ctx context.Context, userID, workspaceID string, tenderID int64) ([]Requirement, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.reqs.ListRequirements(ctx, workspaceID, tenderID)
}

// RecordRequirements upserts a batch of requirements for one tender.
//
// A document-quoted requirement is written UNCONFIRMED whatever the caller
// says: ConfirmedBy is cleared here, not trusted, so the only path to a binding
// requirement is ConfirmRequirement — a human checking the quote against the
// passage. That gate is the stand-in for the extraction eval this phase does not
// have, and removing it is what the eval would buy.
func (s *Service) RecordRequirements(ctx context.Context, userID, workspaceID string, tenderID int64, reqs []Requirement) ([]Requirement, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	if tenderID <= 0 {
		return nil, fmt.Errorf("%w: tender id is required", ErrInvalidArgument)
	}
	out := make([]Requirement, 0, len(reqs))
	for _, r := range reqs {
		r.WorkspaceID = workspaceID
		r.TenderID = tenderID
		if r.Source != RequirementUserStated {
			r.ConfirmedBy, r.ConfirmedAt = "", nil
		} else {
			// A requirement the user typed IS the confirmation; there is no
			// second passage to check it against.
			r.ConfirmedBy = userID
			now := s.now()
			r.ConfirmedAt = &now
		}
		if r.SOA != nil {
			if norm, ok := NormalizeSOACategory(r.SOA.Category); ok {
				r.SOA.Category = norm
			}
		}
		if err := r.Validate(); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return s.reqs.PutRequirements(ctx, workspaceID, tenderID, out)
}

// ConfirmRequirement records that a human checked an extracted requirement
// against the quoted passage. It is the ONLY thing that lets a document-sourced
// requirement contribute to a no_go.
func (s *Service) ConfirmRequirement(ctx context.Context, userID, workspaceID, requirementID string) (Requirement, error) {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return Requirement{}, err
	}
	if strings.TrimSpace(requirementID) == "" {
		return Requirement{}, fmt.Errorf("%w: requirement id is required", ErrInvalidArgument)
	}
	return s.reqs.ConfirmRequirement(ctx, workspaceID, requirementID, userID)
}

// DeleteRequirement removes a mis-extracted requirement.
func (s *Service) DeleteRequirement(ctx context.Context, userID, workspaceID, requirementID string) error {
	if err := s.requireManage(ctx, workspaceID, userID); err != nil {
		return err
	}
	if strings.TrimSpace(requirementID) == "" {
		return fmt.Errorf("%w: requirement id is required", ErrInvalidArgument)
	}
	return s.reqs.DeleteRequirement(ctx, workspaceID, requirementID)
}

// ── Eligibility ─────────────────────────────────────────────────────────────

// CheckEligibility loads dossier + requirements + the tender's deadline +
// document availability and calls Evaluate. It is the ONLY place that touches
// all four, and Evaluate itself stays pure.
//
// An absent dossier is NOT an error here. "We hold nothing about this company"
// is a real answer the engine knows how to express — every gap comes back
// GapUnknown with a question attached — and failing instead would replace a
// product behaviour (ask) with an outage.
func (s *Service) CheckEligibility(ctx context.Context, userID, workspaceID string, tenderID int64, lotRef string) (Assessment, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return Assessment{}, err
	}
	if tenderID <= 0 {
		return Assessment{}, fmt.Errorf("%w: tender id is required", ErrInvalidArgument)
	}

	dossier, err := s.repo.GetDossier(ctx, workspaceID)
	if err != nil && !errors.Is(err, ErrDossierNotFound) {
		return Assessment{}, err
	}

	all, err := s.reqs.ListRequirements(ctx, workspaceID, tenderID)
	if err != nil {
		return Assessment{}, err
	}
	reqs := filterByLot(all, lotRef)

	// The deadline is loaded rather than defaulted: without it an expiring
	// attestation reads as met, which is the single most likely wrong "go" this
	// engine could produce. A tender lookup failure therefore propagates instead
	// of degrading to nil.
	detail, err := s.tenders.GetTender(ctx, tender.GetTenderParams{
		ID:           strconv.FormatInt(tenderID, 10),
		RateLimitKey: userID,
	})
	if err != nil {
		return Assessment{}, err
	}

	// Coverage is evidence for how much the answer is worth, so a failure to
	// establish it fails the read rather than silently presenting ignorance as
	// a clean bill of health.
	av, err := s.documents.Availability(ctx, tenderID)
	if err != nil {
		return Assessment{}, err
	}

	// The criteria the source itself published, merged in after the captured
	// ones. They are derived on read from tender-owned rows rather than stored
	// per workspace: the notice's requirement is a fact about the tender, true
	// for every tenant, so persisting a copy per workspace would duplicate rows
	// and mis-model who owns them.
	//
	// No attempt is made to dedupe them against captured requirements. A
	// published entry is prose and a captured one is a parsed payload; matching
	// the two would mean deciding that "volumen anual de negocios igual o
	// superior a 309.552,00 €" is the same requirement as a turnover threshold
	// somebody typed, which is the extraction problem this whole path declines
	// to pretend it has solved. Showing both, clearly sourced, is the honest
	// shape.
	reqs = append(reqs, filterByLot(
		RequirementsFromSelectionCriteria(workspaceID, tenderID, detail.SelectionCriteria),
		lotRef,
	)...)

	a := Evaluate(reqs, dossier, detail.Deadline, s.now())
	a.TenderID = tenderID
	a.LotRef = lotRef
	a.DocumentCoverage = av.Coverage
	a.DocumentReason = av.Reason

	// Server-side product metric. Go carries no product-analytics SDK — server
	// metrics are slog → OTLP — and deliberately no workspace id, no tender id
	// and no identifier of any kind travels on this line.
	slog.InfoContext(ctx, "eligibility evaluated",
		"verdict", string(a.Verdict), "requirement_count", a.RequirementCount,
		"authoritative_requirements", a.AuthoritativeRequirements,
		"blocking_gaps", a.BlockingGapCount, "unknown_gaps", a.UnknownGapCount,
		"document_coverage", string(a.DocumentCoverage), "document_reason", string(a.DocumentReason))

	return a, nil
}

// filterByLot keeps the requirements that apply to lotRef. Notice-level
// requirements (LotRef == "") always apply — a multi-lot notice usually states
// its requisiti once and every lot inherits them, the same inheritance
// tender.AwardCriterion.LotRef documents.
func filterByLot(all []Requirement, lotRef string) []Requirement {
	out := make([]Requirement, 0, len(all))
	for _, r := range all {
		if r.LotRef == "" || r.LotRef == lotRef {
			out = append(out, r)
		}
	}
	return out
}
