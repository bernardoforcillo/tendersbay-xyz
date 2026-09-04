package connectapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1/companyv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

// CompanyHandler is the transport for the company dossier and the eligibility
// read. It validates the wire shapes it cannot delegate (a stringified int64
// that is not an int64, a classifica string that names no tier), maps between
// the wire and the domain, and delegates — nothing here decides anything about
// the company or the gara.
//
// In particular it does NOT stamp provenance, does not decide whether a
// document-quoted requirement may block, and does not compute a verdict:
// company.Service and company.Evaluate own those, and a handler that
// second-guessed them would be the "fat controller" the code-organization rule
// names as the failure mode.
type CompanyHandler struct{ svc *company.Service }

func NewCompanyHandler(svc *company.Service) *CompanyHandler {
	return &CompanyHandler{svc: svc}
}

var _ companyv1connect.CompanyServiceHandler = (*CompanyHandler)(nil)

// ── Wire → domain, and back ─────────────────────────────────────────────────

// formatTime renders an optional timestamp as RFC3339, and "" when absent.
// Absent is a real state throughout this dossier — an attestation with no
// stated expiry, a contract still running — so it must be distinguishable from
// the zero time rather than rendered as year 1.
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// parseTime reads an optional RFC3339 timestamp. An empty string is "not
// stated" and is NOT an error; a non-empty string that does not parse is,
// because silently dropping a date the user typed would turn a lapsed
// certificate into one with no expiry — a wrong "go" in the making.
func parseTime(field, s string) (*time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%s must be an RFC3339 timestamp: %w", field, err))
	}
	return &t, nil
}

// parseTenderID reads the stringified int64 every tender id travels as.
func parseTenderID(s string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("tender_id must be a stringified int64: %w", err))
	}
	return id, nil
}

func toProtoAttribution(a company.Attribution) *companyv1.Attribution {
	out := &companyv1.Attribution{
		Provenance: string(a.Provenance),
		StatedBy:   a.StatedBy,
		StatedAt:   a.StatedAt.Format(time.RFC3339),
		PromptedBy: string(a.PromptedBy),
		SourceNote: a.SourceNote,
	}
	// Confidence is nil for every human-stated fact, and the paired _set bool is
	// what keeps that distinguishable from a genuine 0.0 on the wire. Coalescing
	// it to 1.0 would let a later scoring pass treat a typed fact and a model
	// guess as interchangeable.
	if a.Confidence != nil {
		out.Confidence = *a.Confidence
		out.ConfidenceSet = true
	}
	if a.PromptedByTenderID != nil {
		out.PromptedByTenderId = strconv.FormatInt(*a.PromptedByTenderID, 10)
	}
	return out
}

// fromProtoAttribution reads the CONTEXT of an edit off the wire. Provenance,
// stated_by and stated_at are carried through unchanged and are then overwritten
// by company.Service, which derives them from how the call arrived — so a
// client cannot forge a stronger claim than the one it actually made. The fields
// that do survive are the ones only the caller can know: why we asked, which
// gara prompted it, and the human-readable trace.
func fromProtoAttribution(a *companyv1.Attribution) (company.Attribution, error) {
	if a == nil {
		return company.Attribution{}, nil
	}
	out := company.Attribution{
		Provenance: company.Provenance(a.Provenance),
		StatedBy:   a.StatedBy,
		PromptedBy: company.PromptSource(a.PromptedBy),
		SourceNote: a.SourceNote,
	}
	if a.ConfidenceSet {
		c := a.Confidence
		out.Confidence = &c
	}
	if s := strings.TrimSpace(a.PromptedByTenderId); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return company.Attribution{}, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("attribution.prompted_by_tender_id must be a stringified int64: %w", err))
		}
		out.PromptedByTenderID = &id
	}
	return out, nil
}

func toProtoIdentity(id company.Identity) *companyv1.CompanyIdentity {
	out := &companyv1.CompanyIdentity{
		LegalName:   id.LegalName,
		VatNumber:   id.VATNumber,
		FiscalCode:  id.FiscalCode,
		LegalForm:   string(id.LegalForm),
		CciaaOffice: id.CCIAAOffice,
		CciaaNumber: id.CCIAANumber,
		Country:     id.Country,
		Nuts:        id.NUTS,
		IsSme:       id.IsSME,
		Attribution: make(map[string]*companyv1.Attribution, len(id.Attribution)),
	}
	if id.FoundedYear != nil {
		out.FoundedYear = *id.FoundedYear
		out.FoundedYearSet = true
	}
	for k, a := range id.Attribution {
		out.Attribution[string(k)] = toProtoAttribution(a)
	}
	return out
}

// fromProtoIdentity maps the identity scalars. The attribution map is NOT read
// back off the wire: per-field provenance is re-derived by the service from
// which fields this edit actually changed, and accepting a client's map would
// let a caller re-label somebody else's typed fact as its own.
func fromProtoIdentity(id *companyv1.CompanyIdentity) company.Identity {
	if id == nil {
		return company.Identity{}
	}
	out := company.Identity{
		LegalName:   id.LegalName,
		VATNumber:   id.VatNumber,
		FiscalCode:  id.FiscalCode,
		LegalForm:   company.LegalForm(id.LegalForm),
		CCIAAOffice: id.CciaaOffice,
		CCIAANumber: id.CciaaNumber,
		Country:     id.Country,
		NUTS:        id.Nuts,
		IsSME:       id.IsSme,
	}
	if id.FoundedYearSet {
		y := id.FoundedYear
		out.FoundedYear = &y
	}
	return out
}

func toProtoSOA(c company.SOACategory) *companyv1.SoaCategory {
	return &companyv1.SoaCategory{
		Id:            c.ID,
		Category:      c.Category,
		Classifica:    c.Classifica.String(),
		IssuedBy:      c.IssuedBy,
		ValidUntil:    formatTime(c.ValidUntil),
		VerifiedUntil: formatTime(c.VerifiedUntil),
		Attribution:   toProtoAttribution(c.Attribution),
	}
}

func fromProtoSOA(c *companyv1.SoaCategory) (company.SOACategory, error) {
	if c == nil {
		return company.SOACategory{}, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("soa is required"))
	}
	// The classifica travels as its published string ("III-bis") and is parsed
	// through the domain's own parser, so the wire and the engine cannot disagree
	// about which tier a label names.
	cl, ok := company.ParseClassifica(c.Classifica)
	if !ok {
		return company.SOACategory{}, connect.NewError(connect.CodeInvalidArgument, company.ErrInvalidClassifica)
	}
	validUntil, err := parseTime("soa.valid_until", c.ValidUntil)
	if err != nil {
		return company.SOACategory{}, err
	}
	verifiedUntil, err := parseTime("soa.verified_until", c.VerifiedUntil)
	if err != nil {
		return company.SOACategory{}, err
	}
	attr, err := fromProtoAttribution(c.Attribution)
	if err != nil {
		return company.SOACategory{}, err
	}
	return company.SOACategory{
		ID:            c.Id,
		Category:      c.Category,
		Classifica:    cl,
		IssuedBy:      c.IssuedBy,
		ValidUntil:    validUntil,
		VerifiedUntil: verifiedUntil,
		Attribution:   attr,
	}, nil
}

func toProtoCertification(c company.Certification) *companyv1.Certification {
	return &companyv1.Certification{
		Id:          c.ID,
		Standard:    string(c.Standard),
		StandardRaw: c.StandardRaw,
		Scope:       c.Scope,
		IssuedBy:    c.IssuedBy,
		ValidFrom:   formatTime(c.ValidFrom),
		ValidUntil:  formatTime(c.ValidUntil),
		Attribution: toProtoAttribution(c.Attribution),
	}
}

func fromProtoCertification(c *companyv1.Certification) (company.Certification, error) {
	if c == nil {
		return company.Certification{}, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("certification is required"))
	}
	validFrom, err := parseTime("certification.valid_from", c.ValidFrom)
	if err != nil {
		return company.Certification{}, err
	}
	validUntil, err := parseTime("certification.valid_until", c.ValidUntil)
	if err != nil {
		return company.Certification{}, err
	}
	attr, err := fromProtoAttribution(c.Attribution)
	if err != nil {
		return company.Certification{}, err
	}
	return company.Certification{
		ID:          c.Id,
		Standard:    company.CertificationStandard(c.Standard),
		StandardRaw: c.StandardRaw,
		Scope:       c.Scope,
		IssuedBy:    c.IssuedBy,
		ValidFrom:   validFrom,
		ValidUntil:  validUntil,
		Attribution: attr,
	}, nil
}

func toProtoFinancialYear(f company.FinancialYear) *companyv1.FinancialYear {
	out := &companyv1.FinancialYear{
		Year:        f.Year,
		Currency:    f.Currency,
		Attribution: toProtoAttribution(f.Attribution),
	}
	if f.TurnoverMinor != nil {
		out.TurnoverMinor = *f.TurnoverMinor
		out.TurnoverSet = true
	}
	if f.SpecificTurnoverMinor != nil {
		out.SpecificTurnoverMinor = *f.SpecificTurnoverMinor
		out.SpecificTurnoverSet = true
	}
	if f.Headcount != nil {
		out.Headcount = *f.Headcount
		out.HeadcountSet = true
	}
	return out
}

func fromProtoFinancialYear(f *companyv1.FinancialYear) (company.FinancialYear, error) {
	if f == nil {
		return company.FinancialYear{}, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("financial_year is required"))
	}
	attr, err := fromProtoAttribution(f.Attribution)
	if err != nil {
		return company.FinancialYear{}, err
	}
	out := company.FinancialYear{Year: f.Year, Currency: f.Currency, Attribution: attr}
	if f.TurnoverSet {
		v := f.TurnoverMinor
		out.TurnoverMinor = &v
	}
	if f.SpecificTurnoverSet {
		v := f.SpecificTurnoverMinor
		out.SpecificTurnoverMinor = &v
	}
	if f.HeadcountSet {
		v := f.Headcount
		out.Headcount = &v
	}
	return out, nil
}

func toProtoPastContract(p company.PastContract) *companyv1.PastContract {
	out := &companyv1.PastContract{
		Id:          p.ID,
		Description: p.Description,
		BuyerName:   p.BuyerName,
		Cpv:         p.CPV,
		Currency:    p.Currency,
		StartedOn:   formatTime(p.StartedOn),
		EndedOn:     formatTime(p.EndedOn),
		Role:        string(p.Role),
		Attribution: toProtoAttribution(p.Attribution),
	}
	if p.ValueMinor != nil {
		out.ValueMinor = *p.ValueMinor
		out.ValueSet = true
	}
	if p.SharePct != nil {
		out.SharePct = *p.SharePct
		out.SharePctSet = true
	}
	return out
}

func fromProtoPastContract(p *companyv1.PastContract) (company.PastContract, error) {
	if p == nil {
		return company.PastContract{}, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("past_contract is required"))
	}
	startedOn, err := parseTime("past_contract.started_on", p.StartedOn)
	if err != nil {
		return company.PastContract{}, err
	}
	endedOn, err := parseTime("past_contract.ended_on", p.EndedOn)
	if err != nil {
		return company.PastContract{}, err
	}
	attr, err := fromProtoAttribution(p.Attribution)
	if err != nil {
		return company.PastContract{}, err
	}
	out := company.PastContract{
		ID:          p.Id,
		Description: p.Description,
		BuyerName:   p.BuyerName,
		CPV:         p.Cpv,
		Currency:    p.Currency,
		StartedOn:   startedOn,
		EndedOn:     endedOn,
		Role:        company.ContractRole(p.Role),
		Attribution: attr,
	}
	if p.ValueSet {
		v := p.ValueMinor
		out.ValueMinor = &v
	}
	// An unset share_pct stays nil rather than defaulting to 100: the domain
	// counts a shared contract with no stated share as unusable evidence, and
	// filling it in here would be exactly the overstatement SharePct exists to
	// prevent.
	if p.SharePctSet {
		v := p.SharePct
		out.SharePct = &v
	}
	return out, nil
}

func toProtoRegistration(r company.Registration) *companyv1.Registration {
	return &companyv1.Registration{
		Id:          r.ID,
		Kind:        string(r.Kind),
		Authority:   r.Authority,
		Identifier:  r.Identifier,
		Section:     r.Section,
		ValidFrom:   formatTime(r.ValidFrom),
		ValidUntil:  formatTime(r.ValidUntil),
		Attribution: toProtoAttribution(r.Attribution),
	}
}

func fromProtoRegistration(r *companyv1.Registration) (company.Registration, error) {
	if r == nil {
		return company.Registration{}, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("registration is required"))
	}
	validFrom, err := parseTime("registration.valid_from", r.ValidFrom)
	if err != nil {
		return company.Registration{}, err
	}
	validUntil, err := parseTime("registration.valid_until", r.ValidUntil)
	if err != nil {
		return company.Registration{}, err
	}
	attr, err := fromProtoAttribution(r.Attribution)
	if err != nil {
		return company.Registration{}, err
	}
	return company.Registration{
		ID:          r.Id,
		Kind:        company.RegistrationKind(r.Kind),
		Authority:   r.Authority,
		Identifier:  r.Identifier,
		Section:     r.Section,
		ValidFrom:   validFrom,
		ValidUntil:  validUntil,
		Attribution: attr,
	}, nil
}

func toProtoDossier(d company.Dossier) *companyv1.CompanyDossier {
	out := &companyv1.CompanyDossier{
		WorkspaceId:    d.WorkspaceID,
		Identity:       toProtoIdentity(d.Identity),
		Soa:            make([]*companyv1.SoaCategory, len(d.SOA)),
		Certifications: make([]*companyv1.Certification, len(d.Certifications)),
		FinancialYears: make([]*companyv1.FinancialYear, len(d.FinancialYears)),
		PastContracts:  make([]*companyv1.PastContract, len(d.PastContracts)),
		Registrations:  make([]*companyv1.Registration, len(d.Registrations)),
		UpdatedAt:      d.UpdatedAt.Format(time.RFC3339),
	}
	for _, r := range d.Representatives {
		out.Representatives = append(out.Representatives, toProtoRepresentative(r))
	}
	for _, dec := range d.Declarations {
		out.Declarations = append(out.Declarations, toProtoDeclaration(dec))
	}
	for _, g := range d.NationalGrounds {
		out.NationalGrounds = append(out.NationalGrounds, toProtoNationalGround(g))
	}
	for i, c := range d.SOA {
		out.Soa[i] = toProtoSOA(c)
	}
	for i, c := range d.Certifications {
		out.Certifications[i] = toProtoCertification(c)
	}
	for i, f := range d.FinancialYears {
		out.FinancialYears[i] = toProtoFinancialYear(f)
	}
	for i, p := range d.PastContracts {
		out.PastContracts[i] = toProtoPastContract(p)
	}
	for i, r := range d.Registrations {
		out.Registrations[i] = toProtoRegistration(r)
	}
	return out
}

// toProtoRequirement flattens the domain's document.Citation into the three
// scalars the wire carries (document_url, page, section). The page and section
// halves are optional in the corpus — documents extracted before those columns
// existed keep NULL — so both degrade to "" rather than to a placeholder.
func toProtoRequirement(r company.Requirement) *companyv1.TenderRequirement {
	out := &companyv1.TenderRequirement{
		Id:              r.ID,
		TenderId:        strconv.FormatInt(r.TenderID, 10),
		LotRef:          r.LotRef,
		Kind:            string(r.Kind),
		RequirementText: r.Text,
		Blocking:        r.Blocking,
		Source:          string(r.Source),
		Excerpt:         r.Excerpt,
		ConfirmedBy:     r.ConfirmedBy,
		ConfirmedAt:     formatTime(r.ConfirmedAt),
	}
	if r.Citation != nil {
		out.DocumentUrl = r.Citation.DocumentURL
		out.Page = citationPage(*r.Citation)
		out.Section = citationSection(*r.Citation)
	}
	return out
}

// citationPage renders a page range as "14" or "14-16". A range whose end does
// not exceed its start is one page, not a degenerate span.
func citationPage(c document.Citation) string {
	if c.PageStart == nil {
		return ""
	}
	if c.PageEnd == nil || *c.PageEnd <= *c.PageStart {
		return strconv.Itoa(*c.PageStart)
	}
	return fmt.Sprintf("%d-%d", *c.PageStart, *c.PageEnd)
}

// citationSection renders "7.2 Requisiti di capacità tecnica" from the most
// specific element of the section path plus the section title. The last path
// element already carries the full dotted number in this corpus's numbering, so
// joining the whole chain would repeat it.
func citationSection(c document.Citation) string {
	var number string
	if n := len(c.SectionPath); n > 0 {
		number = c.SectionPath[n-1]
	}
	return strings.TrimSpace(number + " " + c.SectionTitle)
}

func toProtoGap(g company.Gap) *companyv1.EligibilityGap {
	out := &companyv1.EligibilityGap{
		Requirement: toProtoRequirement(g.Requirement),
		Status:      string(g.Status),
		Blocking:    g.Blocking,
		Held:        g.Held,
		Remedy:      string(g.Remedy.Kind),
	}
	if g.Question != nil {
		out.Question = g.Question.Prompt
		out.QuestionOptions = g.Question.Options
	}
	return out
}

func toProtoAssessment(a company.Assessment) *companyv1.EligibilityAssessment {
	gaps := make([]*companyv1.EligibilityGap, len(a.Gaps))
	for i, g := range a.Gaps {
		gaps[i] = toProtoGap(g)
	}
	// Caveats come from Recommendation() rather than being recomputed here: it
	// is the domain's own derivation, already ordered deterministically, and a
	// second copy in the transport is exactly the business logic this layer is
	// forbidden to own. Always a non-nil slice — the field reaches JSON, where
	// nil and [] read differently to a client.
	caveats := a.Recommendation().Caveats
	wireCaveats := make([]string, len(caveats))
	for i, c := range caveats {
		wireCaveats[i] = string(c)
	}
	return &companyv1.EligibilityAssessment{
		Caveats:                       wireCaveats,
		TenderId:                      strconv.FormatInt(a.TenderID, 10),
		Gaps:                          gaps,
		RequirementCount:              int32(a.RequirementCount),
		AuthoritativeRequirementCount: int32(a.AuthoritativeRequirements),
		BlockingGapCount:              int32(a.BlockingGapCount),
		UnknownGapCount:               int32(a.UnknownGapCount),
		DocumentCoverage:              string(a.DocumentCoverage),
		Verdict:                       string(a.Verdict),
		DocumentReason:                string(a.DocumentReason),
		LotRef:                        a.LotRef,
		EvaluatedAt:                   a.EvaluatedAt.Format(time.RFC3339),
	}
}

// ── Dossier ─────────────────────────────────────────────────────────────────

// GetCompanyDossier returns the dossier, or an empty one with exists=false.
// ErrDossierNotFound is answered as a successful empty read rather than as
// CodeNotFound: "nothing has been asserted about this company yet" is the
// starting state of every workspace, and a 404 on it would make the settings
// page look broken on first visit.
func (h *CompanyHandler) GetCompanyDossier(ctx context.Context, req *connect.Request[companyv1.GetCompanyDossierRequest]) (*connect.Response[companyv1.GetCompanyDossierResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	d, err := h.svc.GetDossier(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		if errors.Is(err, company.ErrDossierNotFound) {
			return connect.NewResponse(&companyv1.GetCompanyDossierResponse{
				Dossier: toProtoDossier(company.Dossier{WorkspaceID: req.Msg.WorkspaceId}),
				Exists:  false,
			}), nil
		}
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.GetCompanyDossierResponse{
		Dossier: toProtoDossier(d),
		Exists:  !d.Empty(),
	}), nil
}

func (h *CompanyHandler) UpdateCompanyIdentity(ctx context.Context, req *connect.Request[companyv1.UpdateCompanyIdentityRequest]) (*connect.Response[companyv1.UpdateCompanyIdentityResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	attr, err := fromProtoAttribution(req.Msg.Attribution)
	if err != nil {
		return nil, err
	}
	id, err := h.svc.UpdateIdentity(ctx, uid, req.Msg.WorkspaceId, fromProtoIdentity(req.Msg.Identity), attr)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.UpdateCompanyIdentityResponse{Identity: toProtoIdentity(id)}), nil
}

func (h *CompanyHandler) PutSoaCategory(ctx context.Context, req *connect.Request[companyv1.PutSoaCategoryRequest]) (*connect.Response[companyv1.PutSoaCategoryResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoSOA(req.Msg.Soa)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutSOACategory(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutSoaCategoryResponse{Soa: toProtoSOA(out)}), nil
}

func (h *CompanyHandler) RemoveSoaCategory(ctx context.Context, req *connect.Request[companyv1.RemoveSoaCategoryRequest]) (*connect.Response[companyv1.RemoveSoaCategoryResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteSOACategory(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveSoaCategoryResponse{}), nil
}

func (h *CompanyHandler) PutCertification(ctx context.Context, req *connect.Request[companyv1.PutCertificationRequest]) (*connect.Response[companyv1.PutCertificationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoCertification(req.Msg.Certification)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutCertification(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutCertificationResponse{Certification: toProtoCertification(out)}), nil
}

func (h *CompanyHandler) RemoveCertification(ctx context.Context, req *connect.Request[companyv1.RemoveCertificationRequest]) (*connect.Response[companyv1.RemoveCertificationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteCertification(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveCertificationResponse{}), nil
}

func (h *CompanyHandler) PutFinancialYear(ctx context.Context, req *connect.Request[companyv1.PutFinancialYearRequest]) (*connect.Response[companyv1.PutFinancialYearResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoFinancialYear(req.Msg.FinancialYear)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutFinancialYear(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutFinancialYearResponse{FinancialYear: toProtoFinancialYear(out)}), nil
}

func (h *CompanyHandler) RemoveFinancialYear(ctx context.Context, req *connect.Request[companyv1.RemoveFinancialYearRequest]) (*connect.Response[companyv1.RemoveFinancialYearResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteFinancialYear(ctx, uid, req.Msg.WorkspaceId, req.Msg.Year); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveFinancialYearResponse{}), nil
}

func (h *CompanyHandler) PutPastContract(ctx context.Context, req *connect.Request[companyv1.PutPastContractRequest]) (*connect.Response[companyv1.PutPastContractResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoPastContract(req.Msg.PastContract)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutPastContract(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutPastContractResponse{PastContract: toProtoPastContract(out)}), nil
}

func (h *CompanyHandler) RemovePastContract(ctx context.Context, req *connect.Request[companyv1.RemovePastContractRequest]) (*connect.Response[companyv1.RemovePastContractResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeletePastContract(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemovePastContractResponse{}), nil
}

func (h *CompanyHandler) PutRegistration(ctx context.Context, req *connect.Request[companyv1.PutRegistrationRequest]) (*connect.Response[companyv1.PutRegistrationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoRegistration(req.Msg.Registration)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutRegistration(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutRegistrationResponse{Registration: toProtoRegistration(out)}), nil
}

func (h *CompanyHandler) RemoveRegistration(ctx context.Context, req *connect.Request[companyv1.RemoveRegistrationRequest]) (*connect.Response[companyv1.RemoveRegistrationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteRegistration(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveRegistrationResponse{}), nil
}

// ── Eligibility ─────────────────────────────────────────────────────────────

func (h *CompanyHandler) CheckEligibility(ctx context.Context, req *connect.Request[companyv1.CheckEligibilityRequest]) (*connect.Response[companyv1.CheckEligibilityResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	tenderID, err := parseTenderID(req.Msg.TenderId)
	if err != nil {
		return nil, err
	}
	a, err := h.svc.CheckEligibility(ctx, uid, req.Msg.WorkspaceId, tenderID, req.Msg.LotRef)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.CheckEligibilityResponse{Assessment: toProtoAssessment(a)}), nil
}

func (h *CompanyHandler) ListTenderRequirements(ctx context.Context, req *connect.Request[companyv1.ListTenderRequirementsRequest]) (*connect.Response[companyv1.ListTenderRequirementsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	tenderID, err := parseTenderID(req.Msg.TenderId)
	if err != nil {
		return nil, err
	}
	reqs, err := h.svc.ListRequirements(ctx, uid, req.Msg.WorkspaceId, tenderID)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*companyv1.TenderRequirement, len(reqs))
	for i, r := range reqs {
		out[i] = toProtoRequirement(r)
	}
	return connect.NewResponse(&companyv1.ListTenderRequirementsResponse{Requirements: out}), nil
}

func (h *CompanyHandler) ConfirmTenderRequirement(ctx context.Context, req *connect.Request[companyv1.ConfirmTenderRequirementRequest]) (*connect.Response[companyv1.ConfirmTenderRequirementResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.svc.ConfirmRequirement(ctx, uid, req.Msg.WorkspaceId, req.Msg.RequirementId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.ConfirmTenderRequirementResponse{Requirement: toProtoRequirement(r)}), nil
}

func (h *CompanyHandler) RemoveTenderRequirement(ctx context.Context, req *connect.Request[companyv1.RemoveTenderRequirementRequest]) (*connect.Response[companyv1.RemoveTenderRequirementResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteRequirement(ctx, uid, req.Msg.WorkspaceId, req.Msg.RequirementId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveTenderRequirementResponse{}), nil
}
