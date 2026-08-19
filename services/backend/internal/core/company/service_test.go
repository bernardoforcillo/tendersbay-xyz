package company

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// ── Fakes ───────────────────────────────────────────────────────────────────

// fakeRepo is an in-memory Repository with the SAME upsert semantics the
// postgres adapter implements — identity is a full replace, SOA keys on
// category, financial years key on year. A fake that upserted differently would
// let the service tests pass against behaviour the real adapter does not have.
type fakeRepo struct {
	dossier  *Dossier
	putCalls int
}

func (f *fakeRepo) GetDossier(_ context.Context, workspaceID string) (Dossier, error) {
	if f.dossier == nil {
		return Dossier{}, ErrDossierNotFound
	}
	d := *f.dossier
	d.WorkspaceID = workspaceID
	return d, nil
}

func (f *fakeRepo) ensure(workspaceID string) *Dossier {
	if f.dossier == nil {
		f.dossier = &Dossier{WorkspaceID: workspaceID}
	}
	return f.dossier
}

func (f *fakeRepo) UpsertIdentity(_ context.Context, workspaceID string, id Identity) (Identity, error) {
	d := f.ensure(workspaceID)
	d.Identity = id
	d.UpdatedAt = time.Now()
	f.putCalls++
	return id, nil
}

func (f *fakeRepo) PutSOACategory(_ context.Context, workspaceID string, c SOACategory) (SOACategory, error) {
	d := f.ensure(workspaceID)
	f.putCalls++
	for i, existing := range d.SOA {
		if existing.Category == c.Category {
			c.ID = existing.ID
			d.SOA[i] = c
			return c, nil
		}
	}
	c.ID = "soa-" + c.Category
	d.SOA = append(d.SOA, c)
	return c, nil
}

func (f *fakeRepo) DeleteSOACategory(_ context.Context, workspaceID, id string) error {
	d := f.ensure(workspaceID)
	for i, c := range d.SOA {
		if c.ID == id {
			d.SOA = append(d.SOA[:i], d.SOA[i+1:]...)
			return nil
		}
	}
	return ErrRecordNotFound
}

// PutCertification keys on (standard, scope) — the adapter's real unique — so a
// second write of the same certificate is a correction, not a duplicate row.
func (f *fakeRepo) PutCertification(_ context.Context, workspaceID string, c Certification) (Certification, error) {
	d := f.ensure(workspaceID)
	f.putCalls++
	c.ID = "cert-" + string(c.Standard) + "-" + c.Scope
	for i, existing := range d.Certifications {
		if existing.Standard == c.Standard && existing.Scope == c.Scope {
			d.Certifications[i] = c
			return c, nil
		}
	}
	d.Certifications = append(d.Certifications, c)
	return c, nil
}

func (f *fakeRepo) DeleteCertification(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeRepo) PutFinancialYear(_ context.Context, workspaceID string, fy FinancialYear) (FinancialYear, error) {
	d := f.ensure(workspaceID)
	f.putCalls++
	for i, existing := range d.FinancialYears {
		if existing.Year == fy.Year {
			d.FinancialYears[i] = fy
			return fy, nil
		}
	}
	d.FinancialYears = append(d.FinancialYears, fy)
	return fy, nil
}

func (f *fakeRepo) DeleteFinancialYear(_ context.Context, _ string, _ int32) error { return nil }

func (f *fakeRepo) PutPastContract(_ context.Context, workspaceID string, p PastContract) (PastContract, error) {
	d := f.ensure(workspaceID)
	f.putCalls++
	if p.ID == "" {
		p.ID = "pc-1"
	}
	d.PastContracts = append(d.PastContracts, p)
	return p, nil
}

func (f *fakeRepo) DeletePastContract(_ context.Context, _ string, _ string) error { return nil }

func (f *fakeRepo) PutRegistration(_ context.Context, workspaceID string, r Registration) (Registration, error) {
	d := f.ensure(workspaceID)
	f.putCalls++
	if r.ID == "" {
		r.ID = "reg-1"
	}
	d.Registrations = append(d.Registrations, r)
	return r, nil
}

func (f *fakeRepo) DeleteRegistration(_ context.Context, _ string, _ string) error { return nil }

type fakeReqRepo struct {
	stored []Requirement
	puts   [][]Requirement
}

func (f *fakeReqRepo) ListRequirements(_ context.Context, _ string, _ int64) ([]Requirement, error) {
	return f.stored, nil
}

func (f *fakeReqRepo) PutRequirements(_ context.Context, _ string, _ int64, reqs []Requirement) ([]Requirement, error) {
	f.puts = append(f.puts, reqs)
	f.stored = reqs
	return reqs, nil
}

func (f *fakeReqRepo) ConfirmRequirement(_ context.Context, _, requirementID, userID string) (Requirement, error) {
	for i, r := range f.stored {
		if r.ID == requirementID {
			now := time.Now()
			f.stored[i].ConfirmedBy = userID
			f.stored[i].ConfirmedAt = &now
			return f.stored[i], nil
		}
	}
	return Requirement{}, ErrRecordNotFound
}

func (f *fakeReqRepo) DeleteRequirement(_ context.Context, _, _ string) error { return nil }

type fakeMembers struct {
	perms workspace.Permission
	err   error
}

func (f fakeMembers) LoadMembership(_ context.Context, workspaceID, userID string) (workspace.Membership, error) {
	if f.err != nil {
		return workspace.Membership{}, f.err
	}
	return workspace.Membership{
		Member: workspace.Member{WorkspaceID: workspaceID, UserID: userID},
		Role:   workspace.Role{Permissions: f.perms},
	}, nil
}

type fakeTenders struct {
	deadline  *time.Time
	selection []tender.SelectionCriterion
	err       error
}

func (f fakeTenders) GetTender(_ context.Context, p tender.GetTenderParams) (tender.TenderDetail, error) {
	if f.err != nil {
		return tender.TenderDetail{}, f.err
	}
	return tender.TenderDetail{ID: p.ID, Deadline: f.deadline, SelectionCriteria: f.selection}, nil
}

type fakeDocuments struct {
	availability document.Availability
	err          error
}

func (f fakeDocuments) Availability(_ context.Context, tenderID int64) (document.Availability, error) {
	if f.err != nil {
		return document.Availability{}, f.err
	}
	av := f.availability
	av.TenderID = tenderID
	return av, nil
}

// newTestService wires the fakes with a frozen clock, so every stamped
// StatedAt and every expiry comparison is reproducible.
func newTestService(repo *fakeRepo, reqs *fakeReqRepo, members fakeMembers, tenders fakeTenders, docs fakeDocuments) *Service {
	s := NewService(repo, reqs, members, tenders, docs)
	s.now = func() time.Time { return evalNow }
	return s
}

const (
	testUser = "11111111-1111-1111-1111-111111111111"
	testWS   = "22222222-2222-2222-2222-222222222222"
)

func manager() fakeMembers {
	return fakeMembers{perms: workspace.PermViewWorkspace | workspace.PermManageWorkspace}
}

func viewer() fakeMembers {
	return fakeMembers{perms: workspace.PermViewWorkspace}
}

// ── Authorization ───────────────────────────────────────────────────────────

// TestServiceWritesRequireManageWorkspace pins the read/write split: the
// dossier is a workspace-level asset, so reading it needs membership and
// changing it needs PermManageWorkspace.
func TestServiceWritesRequireManageWorkspace(t *testing.T) {
	ctx := context.Background()
	soa := SOACategory{Category: "OG1", Classifica: ClassificaIII}

	t.Run("a plain member may read", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{SOA: []SOACategory{{Category: "OG1", Classifica: ClassificaI, Attribution: stated()}}}}
		s := newTestService(repo, &fakeReqRepo{}, viewer(), fakeTenders{}, fakeDocuments{})
		if _, err := s.GetDossier(ctx, testUser, testWS); err != nil {
			t.Fatalf("GetDossier: %v", err)
		}
	})

	t.Run("a plain member may not write", func(t *testing.T) {
		repo := &fakeRepo{}
		s := newTestService(repo, &fakeReqRepo{}, viewer(), fakeTenders{}, fakeDocuments{})
		if _, err := s.PutSOACategory(ctx, testUser, testWS, soa); !errors.Is(err, workspace.ErrForbidden) {
			t.Fatalf("PutSOACategory = %v, want ErrForbidden", err)
		}
		if repo.putCalls != 0 {
			t.Error("a forbidden write must not reach the repository")
		}
	})

	t.Run("an administrator may write without the manage bit", func(t *testing.T) {
		repo := &fakeRepo{}
		s := newTestService(repo, &fakeReqRepo{}, fakeMembers{perms: workspace.PermAdministrator}, fakeTenders{}, fakeDocuments{})
		if _, err := s.PutSOACategory(ctx, testUser, testWS, soa); err != nil {
			t.Fatalf("PutSOACategory: %v", err)
		}
	})

	t.Run("a non-member is refused by the membership port", func(t *testing.T) {
		s := newTestService(&fakeRepo{}, &fakeReqRepo{}, fakeMembers{err: workspace.ErrNotMember}, fakeTenders{}, fakeDocuments{})
		if _, err := s.GetDossier(ctx, testUser, testWS); !errors.Is(err, workspace.ErrNotMember) {
			t.Fatalf("GetDossier = %v, want ErrNotMember", err)
		}
	})
}

// ── Provenance stamping ─────────────────────────────────────────────────────

// TestPutStampsUserStatedProvenance is the anti-forgery rule: a direct dossier
// write is a human typing into the form, and no caller can claim otherwise.
func TestPutStampsUserStatedProvenance(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	// The caller ASKS for an agent-inferred, high-confidence, someone-else's
	// fact. Every one of those claims is overwritten.
	got, err := s.PutSOACategory(ctx, testUser, testWS, SOACategory{
		Category:   "og 1",
		Classifica: ClassificaIII,
		Attribution: Attribution{
			Provenance: ProvenanceAgentInferred,
			Confidence: p(0.99),
			StatedBy:   "someone-else",
			SourceNote: "gara 545620-2026, disciplinare §7.2",
		},
	})
	if err != nil {
		t.Fatalf("PutSOACategory: %v", err)
	}
	if got.Provenance != ProvenanceUserStated {
		t.Errorf("provenance = %q, want user_stated", got.Provenance)
	}
	if got.Confidence != nil {
		t.Errorf("confidence = %v, want nil on a human-stated fact", *got.Confidence)
	}
	if got.StatedBy != testUser {
		t.Errorf("stated_by = %q, want the authenticated caller %q", got.StatedBy, testUser)
	}
	if !got.StatedAt.Equal(evalNow) {
		t.Errorf("stated_at = %v, want the service clock", got.StatedAt)
	}
	// The caller's own trace survives — it is context, not a claim.
	if got.SourceNote != "gara 545620-2026, disciplinare §7.2" {
		t.Errorf("source_note = %q, want the caller's trace preserved", got.SourceNote)
	}
	// The published category is normalised on the way in, so the engine's
	// equality match cannot miss it.
	if got.Category != "OG1" {
		t.Errorf("category = %q, want OG1", got.Category)
	}
}

// TestRecordFactDerivesProvenanceFromPromptSource covers the rule that makes
// provenance unforgeable: it is derived from HOW the call arrived, not passed.
func TestRecordFactDerivesProvenanceFromPromptSource(t *testing.T) {
	ctx := context.Background()
	tenderID := int64(545620)

	tests := []struct {
		name           string
		src            PromptSource
		wantProvenance Provenance
		wantStatedBy   string
	}{
		{"a tender-prompted answer is user_confirmed", PromptTender, ProvenanceUserConfirmed, testUser},
		{"an onboarding answer is user_confirmed", PromptOnboarding, ProvenanceUserConfirmed, testUser},
		{"a document extraction has no human behind it", PromptExtraction, ProvenanceImported, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})
			err := s.RecordFact(ctx, testUser, testWS, Fact{
				SOA: &SOACategory{Category: "OG1", Classifica: ClassificaIII},
			}, tt.src, &tenderID)
			if err != nil {
				t.Fatalf("RecordFact: %v", err)
			}
			got := repo.dossier.SOA[0]
			if got.Provenance != tt.wantProvenance {
				t.Errorf("provenance = %q, want %q", got.Provenance, tt.wantProvenance)
			}
			if got.StatedBy != tt.wantStatedBy {
				t.Errorf("stated_by = %q, want %q", got.StatedBy, tt.wantStatedBy)
			}
			if got.PromptedBy != tt.src {
				t.Errorf("prompted_by = %q, want %q", got.PromptedBy, tt.src)
			}
			if got.PromptedByTenderID == nil || *got.PromptedByTenderID != tenderID {
				t.Errorf("prompted_by_tender_id = %v, want %d", got.PromptedByTenderID, tenderID)
			}
		})
	}
}

// TestRecordFactRejectsAmbiguousFacts: a Fact is the answer to ONE question and
// its attribution is stamped once, for one datum.
func TestRecordFactRejectsAmbiguousFacts(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	for _, f := range []Fact{
		{},
		{SOA: &SOACategory{Category: "OG1", Classifica: ClassificaI}, Registration: &Registration{Kind: RegMEPA}},
	} {
		if err := s.RecordFact(ctx, testUser, testWS, f, PromptTender, nil); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("RecordFact = %v, want ErrInvalidArgument", err)
		}
	}
	if repo.putCalls != 0 {
		t.Error("an invalid fact must not reach the repository")
	}
}

// TestRecordFactIdentityTouchesOneFieldOnly: the identity row is a full replace
// at the repository, so a single chat answer must carry every other field
// forward untouched — and must stamp provenance for its own key alone.
func TestRecordFactIdentityTouchesOneFieldOnly(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{dossier: &Dossier{Identity: Identity{
		LegalName:   "Acme Costruzioni Srl",
		VATNumber:   "12345678901",
		Attribution: map[FieldKey]Attribution{FieldLegalName: stated()},
	}}}
	s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	tenderID := int64(545620)
	err := s.RecordFact(ctx, testUser, testWS, Fact{
		Identity: &IdentityFact{Key: FieldCCIAA, Value: "mi/1234567"},
	}, PromptTender, &tenderID)
	if err != nil {
		t.Fatalf("RecordFact: %v", err)
	}

	id := repo.dossier.Identity
	if id.LegalName != "Acme Costruzioni Srl" || id.VATNumber != "12345678901" {
		t.Fatalf("a one-field answer blanked the rest of the identity: %+v", id)
	}
	if id.CCIAAOffice != "MI" || id.CCIAANumber != "1234567" {
		t.Fatalf("cciaa = %q/%q, want MI/1234567", id.CCIAAOffice, id.CCIAANumber)
	}
	if got := id.Attribution[FieldCCIAA]; got.Provenance != ProvenanceUserConfirmed {
		t.Errorf("cciaa provenance = %q, want user_confirmed", got.Provenance)
	}
	// The untouched field keeps the provenance it already had — a fresh stamp
	// would claim the user re-asserted something they never saw.
	if got := id.Attribution[FieldLegalName]; got.Provenance != ProvenanceUserStated {
		t.Errorf("legal_name provenance = %q, want the stored user_stated", got.Provenance)
	}
}

func TestRecordFactIdentityRejectsUnknownKeyAndBadYear(t *testing.T) {
	ctx := context.Background()
	s := newTestService(&fakeRepo{}, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	err := s.RecordFact(ctx, testUser, testWS, Fact{Identity: &IdentityFact{Key: "partita_iva", Value: "x"}}, PromptTender, nil)
	if !errors.Is(err, ErrUnknownFieldKey) {
		t.Fatalf("unknown key = %v, want ErrUnknownFieldKey", err)
	}
	err = s.RecordFact(ctx, testUser, testWS, Fact{Identity: &IdentityFact{Key: FieldFoundedYear, Value: "millenovecento"}}, PromptTender, nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("bad year = %v, want ErrInvalidArgument", err)
	}
}

// TestUpdateIdentityRestampsOnlyChangedFields covers the distinction the
// attribution map exists to hold: "never asserted" (no entry) vs "deliberately
// cleared" (an entry over an empty value).
func TestUpdateIdentityRestampsOnlyChangedFields(t *testing.T) {
	ctx := context.Background()
	old := stated()
	old.StatedBy = "someone-earlier"
	old.SourceNote = "onboarding"
	repo := &fakeRepo{dossier: &Dossier{Identity: Identity{
		LegalName:   "Acme Costruzioni Srl",
		VATNumber:   "12345678901",
		Attribution: map[FieldKey]Attribution{FieldLegalName: old, FieldVATNumber: old},
	}}}
	s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

	got, err := s.UpdateIdentity(ctx, testUser, testWS, Identity{
		LegalName: "Acme Costruzioni Srl", // unchanged
		VATNumber: "",                     // deliberately cleared
		Country:   "IT",                   // newly asserted
	}, Attribution{PromptedBy: PromptOnboarding})
	if err != nil {
		t.Fatalf("UpdateIdentity: %v", err)
	}

	if a := got.Attribution[FieldLegalName]; a.StatedBy != "someone-earlier" {
		t.Errorf("an unchanged field must keep its original attribution, got stated_by %q", a.StatedBy)
	}
	if a, ok := got.Attribution[FieldVATNumber]; !ok || a.StatedBy != testUser {
		t.Errorf("a cleared field must carry a fresh attribution recording who cleared it, got %+v (present: %v)", a, ok)
	}
	if a, ok := got.Attribution[FieldCountry]; !ok || a.Provenance != ProvenanceUserStated {
		t.Errorf("a newly asserted field must be stamped, got %+v (present: %v)", a, ok)
	}
	// A field nobody has ever touched carries NO entry — that is a different
	// state from an empty string a human blanked.
	if _, ok := got.Attribution[FieldNUTS]; ok {
		t.Error("an untouched field must have no attribution entry")
	}
}

func TestUpdateIdentityValidates(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})
	if _, err := s.UpdateIdentity(ctx, testUser, testWS, Identity{Country: "italia"}, Attribution{}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("UpdateIdentity = %v, want ErrInvalidArgument", err)
	}
	if repo.putCalls != 0 {
		t.Error("an invalid identity must not reach the repository")
	}
}

// ── Requirements ────────────────────────────────────────────────────────────

// TestRecordRequirementsClearsForgedConfirmation is the ConfirmedBy gate, which
// stands in for the extraction eval this phase does not have: nothing the agent
// writes arrives pre-confirmed.
func TestRecordRequirementsClearsForgedConfirmation(t *testing.T) {
	ctx := context.Background()
	reqs := &fakeReqRepo{}
	s := newTestService(&fakeRepo{}, reqs, manager(), fakeTenders{}, fakeDocuments{})

	confirmedAt := evalNow
	out, err := s.RecordRequirements(ctx, testUser, testWS, 545620, []Requirement{
		{
			Kind: RequirementSOA, Source: RequirementDocumentExcerpt, Blocking: true,
			Text: "SOA OG1 III", SOA: &SOARequirement{Category: "og1", Classifica: ClassificaIII},
			ConfirmedBy: "forged-user", ConfirmedAt: &confirmedAt,
		},
		{
			Kind: RequirementOther, Source: RequirementUserStated, Blocking: true,
			Text: "Polizza RCT",
		},
	})
	if err != nil {
		t.Fatalf("RecordRequirements: %v", err)
	}
	if out[0].ConfirmedBy != "" || out[0].ConfirmedAt != nil {
		t.Errorf("a document-quoted requirement must arrive unconfirmed, got %q", out[0].ConfirmedBy)
	}
	if out[0].CanBlock() {
		t.Error("an unconfirmed extraction must not be able to block")
	}
	// A requirement the user typed IS the confirmation — there is no second
	// passage to check it against.
	if out[1].ConfirmedBy != testUser {
		t.Errorf("a user-stated requirement is self-confirming, got %q", out[1].ConfirmedBy)
	}
	// Workspace + tender are stamped by the service, never trusted from input.
	if out[0].WorkspaceID != testWS || out[0].TenderID != 545620 {
		t.Errorf("scope not stamped: %q / %d", out[0].WorkspaceID, out[0].TenderID)
	}
	if out[0].SOA.Category != "OG1" {
		t.Errorf("category not normalised: %q", out[0].SOA.Category)
	}
}

func TestRecordRequirementsRejectsInvalidBatchWhole(t *testing.T) {
	ctx := context.Background()
	reqs := &fakeReqRepo{}
	s := newTestService(&fakeRepo{}, reqs, manager(), fakeTenders{}, fakeDocuments{})

	_, err := s.RecordRequirements(ctx, testUser, testWS, 545620, []Requirement{
		{Kind: RequirementOther, Source: RequirementUserStated, Text: "valid"},
		{Kind: RequirementSOA, Source: RequirementUserStated, Text: "broken", SOA: &SOARequirement{Category: "ZZ9", Classifica: ClassificaI}},
	})
	if !errors.Is(err, ErrInvalidSOACategory) {
		t.Fatalf("RecordRequirements = %v, want ErrInvalidSOACategory", err)
	}
	if len(reqs.puts) != 0 {
		t.Error("a batch with one bad requirement must not be partially written")
	}
}

// ── CheckEligibility ────────────────────────────────────────────────────────

func TestCheckEligibilityWiresAllFourInputs(t *testing.T) {
	ctx := context.Background()
	deadline := evalNow.AddDate(0, 0, 20)
	repo := &fakeRepo{dossier: &Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaI, Attribution: stated()},
	}}}
	reqs := &fakeReqRepo{stored: []Requirement{
		{ID: "r1", Kind: RequirementSOA, Source: RequirementUserStated, Blocking: true,
			Text: "SOA OG1 III", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIII}},
	}}
	docs := fakeDocuments{availability: document.Availability{
		Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
		NoticeRead: true, KnownDocumentLinks: 1,
	}}
	s := newTestService(repo, reqs, viewer(), fakeTenders{deadline: &deadline}, docs)

	a, err := s.CheckEligibility(ctx, testUser, testWS, 545620, "")
	if err != nil {
		t.Fatalf("CheckEligibility: %v", err)
	}
	if a.TenderID != 545620 {
		t.Errorf("tender id = %d", a.TenderID)
	}
	if a.Verdict != VerdictNoGo {
		t.Errorf("verdict = %q, want no_go", a.Verdict)
	}
	// Coverage travels with the answer so the UI can say "we hold only the
	// notice PDF" in the same sentence as the verdict.
	if a.DocumentCoverage != document.CoverageNotice || a.DocumentReason != document.ReasonBodyNotRetrieved {
		t.Errorf("coverage = %q/%q, want notice_only/body_not_retrieved", a.DocumentCoverage, a.DocumentReason)
	}
	if !a.Consistent() {
		t.Error("CheckEligibility produced an inconsistent assessment")
	}
	if !a.EvaluatedAt.Equal(evalNow) {
		t.Errorf("evaluated_at = %v, want the service clock", a.EvaluatedAt)
	}
}

// TestCheckEligibilityFiltersByLot: a notice-level requirement is inherited by
// every lot; another lot's requirement is not this lot's problem.
func TestCheckEligibilityFiltersByLot(t *testing.T) {
	ctx := context.Background()
	reqs := &fakeReqRepo{stored: []Requirement{
		{ID: "notice", Kind: RequirementOther, Source: RequirementUserStated, Text: "Notice level"},
		{ID: "lot1", LotRef: "LOT-0001", Kind: RequirementOther, Source: RequirementUserStated, Text: "Lot 1 only"},
		{ID: "lot2", LotRef: "LOT-0002", Kind: RequirementOther, Source: RequirementUserStated, Text: "Lot 2 only"},
	}}
	s := newTestService(&fakeRepo{}, reqs, viewer(), fakeTenders{}, fakeDocuments{})

	a, err := s.CheckEligibility(ctx, testUser, testWS, 545620, "LOT-0001")
	if err != nil {
		t.Fatalf("CheckEligibility: %v", err)
	}
	if a.RequirementCount != 2 {
		t.Fatalf("requirement count = %d, want 2 (notice-level + this lot)", a.RequirementCount)
	}
	for _, g := range a.Gaps {
		if g.Requirement.LotRef == "LOT-0002" {
			t.Error("another lot's requirement leaked into this assessment")
		}
	}
	if a.LotRef != "LOT-0001" {
		t.Errorf("lot ref = %q", a.LotRef)
	}
}

// TestCheckEligibilityWithNoDossier: "we hold nothing about this company" is a
// real product answer (ask the question), not an outage.
func TestCheckEligibilityWithNoDossier(t *testing.T) {
	ctx := context.Background()
	reqs := &fakeReqRepo{stored: []Requirement{
		{ID: "r1", Kind: RequirementSOA, Source: RequirementUserStated, Blocking: true,
			Text: "SOA OG1 III", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIII}},
	}}
	s := newTestService(&fakeRepo{}, reqs, viewer(), fakeTenders{}, fakeDocuments{})

	a, err := s.CheckEligibility(ctx, testUser, testWS, 545620, "")
	if err != nil {
		t.Fatalf("CheckEligibility with an empty dossier: %v", err)
	}
	if a.Verdict != VerdictInsufficientData {
		t.Errorf("verdict = %q, want insufficient_data", a.Verdict)
	}
	if a.UnknownGapCount != 1 || a.Gaps[0].Question == nil {
		t.Error("an empty dossier must produce a question, not a guess")
	}
}

// TestCheckEligibilityPropagatesDeadlineFailure: without the deadline an
// expiring attestation reads as met, so a tender lookup failure must fail the
// read rather than degrade to nil.
func TestCheckEligibilityPropagatesDeadlineFailure(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("tender lookup failed")
	s := newTestService(&fakeRepo{}, &fakeReqRepo{}, viewer(), fakeTenders{err: boom}, fakeDocuments{})
	if _, err := s.CheckEligibility(ctx, testUser, testWS, 545620, ""); !errors.Is(err, boom) {
		t.Fatalf("CheckEligibility = %v, want the tender error", err)
	}
}

func TestCheckEligibilityPropagatesAvailabilityFailure(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("availability failed")
	s := newTestService(&fakeRepo{}, &fakeReqRepo{}, viewer(), fakeTenders{}, fakeDocuments{err: boom})
	if _, err := s.CheckEligibility(ctx, testUser, testWS, 545620, ""); !errors.Is(err, boom) {
		t.Fatalf("CheckEligibility = %v, want the availability error", err)
	}
}

func TestCheckEligibilityRejectsBadTenderID(t *testing.T) {
	ctx := context.Background()
	s := newTestService(&fakeRepo{}, &fakeReqRepo{}, viewer(), fakeTenders{}, fakeDocuments{})
	if _, err := s.CheckEligibility(ctx, testUser, testWS, 0, ""); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("CheckEligibility = %v, want ErrInvalidArgument", err)
	}
}

// ── Partial writes ──────────────────────────────────────────────────────────

// TestPutMergesWithTheRecordOnFile pins the rule that a Put asserts only what it
// carries and never retracts what it does not.
//
// The scenario is the product's own: a user types their fatturato and organico
// into settings, and weeks later answers a just-in-time headcount question on a
// scheda gara. That answer's RPC sends a year and a headcount and nothing else,
// because that is all the question asked. A replacing upsert would blank the
// turnover — and the very next eligibility check would ask for a figure the
// product already held. The same shape applies to a SOA classifica answered
// without its expiry dates, where the loss is worse: VerifiedUntil going missing
// turns a lapsed attestation back into a valid one.
func TestPutMergesWithTheRecordOnFile(t *testing.T) {
	ctx := context.Background()
	fullYear := int64(2_100_000_00)
	staff := int32(18)
	validUntil := evalNow.AddDate(3, 0, 0)
	verifiedUntil := evalNow.AddDate(1, 0, 0)

	t.Run("a headcount answer keeps the turnover already on file", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{FinancialYears: []FinancialYear{{
			Year: 2024, TurnoverMinor: &fullYear, Currency: "EUR", Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutFinancialYear(ctx, testUser, testWS, FinancialYear{Year: 2024, Headcount: &staff})
		if err != nil {
			t.Fatalf("PutFinancialYear: %v", err)
		}
		if got.TurnoverMinor == nil || *got.TurnoverMinor != fullYear {
			t.Errorf("turnover = %v, want the stored %d — a headcount answer must not blank a fatturato", got.TurnoverMinor, fullYear)
		}
		if got.Headcount == nil || *got.Headcount != staff {
			t.Errorf("headcount = %v, want %d", got.Headcount, staff)
		}
		if got.Currency != "EUR" {
			t.Errorf("currency = %q, want the stored EUR", got.Currency)
		}
	})

	t.Run("a turnover answer keeps the headcount already on file", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{FinancialYears: []FinancialYear{{
			Year: 2024, Headcount: &staff, Currency: "EUR", Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutFinancialYear(ctx, testUser, testWS, FinancialYear{Year: 2024, TurnoverMinor: &fullYear, Currency: "EUR"})
		if err != nil {
			t.Fatalf("PutFinancialYear: %v", err)
		}
		if got.Headcount == nil || *got.Headcount != staff {
			t.Errorf("headcount = %v, want the stored %d", got.Headcount, staff)
		}
	})

	t.Run("a different year is a different record and merges nothing", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{FinancialYears: []FinancialYear{{
			Year: 2024, TurnoverMinor: &fullYear, Currency: "EUR", Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutFinancialYear(ctx, testUser, testWS, FinancialYear{Year: 2023, Headcount: &staff})
		if err != nil {
			t.Fatalf("PutFinancialYear: %v", err)
		}
		if got.TurnoverMinor != nil {
			t.Errorf("turnover = %v, want nil — 2023 has no esercizio on file", *got.TurnoverMinor)
		}
	})

	t.Run("a classifica correction keeps both SOA expiry dates", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{SOA: []SOACategory{{
			Category: "OG1", Classifica: ClassificaII, IssuedBy: "SOA Rina",
			ValidUntil: &validUntil, VerifiedUntil: &verifiedUntil, Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutSOACategory(ctx, testUser, testWS, SOACategory{Category: "OG1", Classifica: ClassificaIII})
		if err != nil {
			t.Fatalf("PutSOACategory: %v", err)
		}
		if got.Classifica != ClassificaIII {
			t.Errorf("classifica = %v, want III — the corrected datum must win", got.Classifica)
		}
		if got.VerifiedUntil == nil || !got.VerifiedUntil.Equal(verifiedUntil) {
			t.Errorf("verified_until = %v, want the stored %v — losing it revives a lapsed attestation", got.VerifiedUntil, verifiedUntil)
		}
		if got.ValidUntil == nil || !got.ValidUntil.Equal(validUntil) {
			t.Errorf("valid_until = %v, want the stored %v", got.ValidUntil, validUntil)
		}
		if got.IssuedBy != "SOA Rina" {
			t.Errorf("issued_by = %q, want the stored body", got.IssuedBy)
		}
	})

	t.Run("a certificate re-answered without dates keeps them", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{Certifications: []Certification{{
			Standard: CertISO9001, Scope: "costruzioni", IssuedBy: "Bureau Veritas",
			ValidUntil: &validUntil, Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutCertification(ctx, testUser, testWS, Certification{Standard: CertISO9001, Scope: "costruzioni"})
		if err != nil {
			t.Fatalf("PutCertification: %v", err)
		}
		if got.ValidUntil == nil || !got.ValidUntil.Equal(validUntil) {
			t.Errorf("valid_until = %v, want the stored %v — an expired cert reported as open-ended is a wrong go", got.ValidUntil, validUntil)
		}
		if got.IssuedBy != "Bureau Veritas" {
			t.Errorf("issued_by = %q, want the stored body", got.IssuedBy)
		}
	})

	t.Run("a certificate under a different scope is a different record", func(t *testing.T) {
		repo := &fakeRepo{dossier: &Dossier{Certifications: []Certification{{
			Standard: CertISO9001, Scope: "costruzioni", ValidUntil: &validUntil, Attribution: stated(),
		}}}}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		got, err := s.PutCertification(ctx, testUser, testWS, Certification{Standard: CertISO9001, Scope: "impianti"})
		if err != nil {
			t.Fatalf("PutCertification: %v", err)
		}
		if got.ValidUntil != nil {
			t.Errorf("valid_until = %v, want nil — scope is part of the key, not an attribute", *got.ValidUntil)
		}
	})

	t.Run("the first answer a workspace ever gives has nothing to merge with", func(t *testing.T) {
		repo := &fakeRepo{}
		s := newTestService(repo, &fakeReqRepo{}, manager(), fakeTenders{}, fakeDocuments{})

		if _, err := s.PutFinancialYear(ctx, testUser, testWS, FinancialYear{Year: 2024, Headcount: &staff}); err != nil {
			t.Fatalf("PutFinancialYear on an absent dossier: %v", err)
		}
	})
}

// TestCheckEligibilityIncludesPublishedCriteria proves the last link of the
// Spanish chain: criteria PLACSP published reach the assessment the UI renders,
// and reach it WITHOUT changing what the engine is willing to conclude.
func TestCheckEligibilityIncludesPublishedCriteria(t *testing.T) {
	ctx := context.Background()
	deadline := evalNow.AddDate(0, 0, 20)
	repo := &fakeRepo{dossier: &Dossier{SOA: []SOACategory{
		{Category: "OG1", Classifica: ClassificaIII, Attribution: stated()},
	}}}
	// One captured requirement the dossier satisfies, so the verdict has
	// somewhere to land other than insufficient_data.
	reqs := &fakeReqRepo{stored: []Requirement{
		{ID: "r1", Kind: RequirementSOA, Source: RequirementUserStated, Blocking: true,
			Text: "SOA OG1 III", SOA: &SOARequirement{Category: "OG1", Classifica: ClassificaIII}},
	}}
	docs := fakeDocuments{availability: document.Availability{
		Coverage: document.CoverageNotice, Reason: document.ReasonBodyNotRetrieved,
		NoticeRead: true, KnownDocumentLinks: 1,
	}}
	tenders := fakeTenders{deadline: &deadline, selection: []tender.SelectionCriterion{
		{Ordinal: 0, Category: "financial", Origin: "es-placsp",
			Description: "Volumen anual de negocios igual o superior a 309.552,00 EUR."},
	}}
	s := newTestService(repo, reqs, viewer(), tenders, docs)

	a, err := s.CheckEligibility(ctx, testUser, testWS, 545620, "")
	if err != nil {
		t.Fatalf("CheckEligibility: %v", err)
	}

	var found bool
	for _, g := range a.Gaps {
		if g.Requirement.Source == RequirementNoticePublished {
			found = true
			if g.Status != GapUnknown {
				t.Errorf("published criterion status = %q, want unknown — it is never machine-decided", g.Status)
			}
			if g.Question == nil {
				t.Error("published criterion carries no capture question, so the user has no way to act on it")
			}
		}
	}
	if !found {
		t.Fatal("the published criterion never reached the assessment — the chain is not wired")
	}

	// And it did not cost the verdict: a satisfied dossier still reaches go.
	if a.Verdict != VerdictGo {
		t.Errorf("verdict = %q, want go — published prose must not degrade a verdict the dossier supports", a.Verdict)
	}
	if !a.Consistent() {
		t.Error("assessment is not self-consistent with the published criterion present")
	}
}
