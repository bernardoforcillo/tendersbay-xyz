package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
)

func ptr[T any](v T) *T { return &v }

func testCompanyRepos(t *testing.T) (*postgres.CompanyRepo, *postgres.RequirementRepo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return postgres.NewCompanyRepo(db), postgres.NewRequirementRepo(db), sqlDB
}

// seedWorkspaceForCompany creates a throwaway user + workspace. The slug is
// per-test so parallel packages don't collide on the workspaces.slug unique.
func seedWorkspaceForCompany(t *testing.T, sqlDB *sql.DB, slug string) string {
	t.Helper()
	ctx := context.Background()

	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, 'x', 'Company Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
		"company-test-"+slug+"@example.com",
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Company Test WS', $1, $2) RETURNING id`,
		"company-test-"+slug, ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })
	return workspaceID
}

func TestCompanyRepo_GetDossier_NotFoundWhenNothingAsserted(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "empty")

	_, err := repo.GetDossier(context.Background(), ws)
	if !errors.Is(err, company.ErrDossierNotFound) {
		t.Fatalf("GetDossier = %v, want ErrDossierNotFound", err)
	}
}

// TestCompanyRepo_OneCompanyPerWorkspace is the schema-level invariant: the
// primary key of workspace_companies IS the foreign key to workspaces.id, so a
// second company row for the same workspace cannot be inserted at all.
func TestCompanyRepo_OneCompanyPerWorkspace(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "unique")
	ctx := context.Background()

	if _, err := repo.UpsertIdentity(ctx, ws, company.Identity{
		LegalName:   "Acme Costruzioni Srl",
		VATNumber:   "12345678901",
		Attribution: map[company.FieldKey]company.Attribution{company.FieldLegalName: {Provenance: company.ProvenanceUserStated, StatedAt: time.Now()}},
	}); err != nil {
		t.Fatalf("UpsertIdentity (first): %v", err)
	}

	// A raw second INSERT must be rejected by the primary key.
	_, err := sqlDB.ExecContext(ctx,
		`INSERT INTO workspace_companies (workspace_id, legal_name) VALUES ($1, 'Second Company Srl')`, ws)
	if err == nil {
		t.Fatal("a second company row for one workspace must be rejected by the schema")
	}

	// The repository's own second write is an UPDATE, not a second row.
	if _, err := repo.UpsertIdentity(ctx, ws, company.Identity{LegalName: "Acme Costruzioni SpA"}); err != nil {
		t.Fatalf("UpsertIdentity (second): %v", err)
	}
	var count int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM workspace_companies WHERE workspace_id = $1`, ws).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("workspace_companies rows = %d, want exactly 1", count)
	}
}

// TestCompanyRepo_UpsertIdentity_FullReplaceAndProvenanceRoundTrip covers the
// two things the identity row must not lose: a cleared nullable must actually
// clear, and the per-field provenance map must survive a round trip byte for
// byte — provenance is what decides whether a fact may block a bid.
func TestCompanyRepo_UpsertIdentity_FullReplaceAndProvenanceRoundTrip(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "identity")
	ctx := context.Background()

	statedAt := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	tenderID := int64(545620)
	created, err := repo.UpsertIdentity(ctx, ws, company.Identity{
		LegalName:   "Acme Costruzioni Srl",
		VATNumber:   "12345678901",
		FiscalCode:  "12345678901",
		LegalForm:   company.LegalFormSRL,
		CCIAAOffice: "MI",
		CCIAANumber: "1234567",
		Country:     "IT",
		NUTS:        "ITC4C",
		FoundedYear: ptr(int32(1998)),
		Attribution: map[company.FieldKey]company.Attribution{
			company.FieldLegalName: {
				Provenance: company.ProvenanceUserStated,
				StatedBy:   "00000000-0000-0000-0000-000000000001",
				StatedAt:   statedAt,
				PromptedBy: company.PromptOnboarding,
			},
			company.FieldVATNumber: {
				Provenance:         company.ProvenanceAgentInferred,
				Confidence:         ptr(0.72),
				StatedAt:           statedAt,
				PromptedBy:         company.PromptTender,
				PromptedByTenderID: &tenderID,
				SourceNote:         "gara 545620-2026, disciplinare §7.2",
			},
		},
	})
	if err != nil {
		t.Fatalf("UpsertIdentity: %v", err)
	}
	if created.FoundedYear == nil || *created.FoundedYear != 1998 {
		t.Fatalf("founded year = %v", created.FoundedYear)
	}

	got, err := repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}
	legal := got.Identity.Attribution[company.FieldLegalName]
	if legal.Provenance != company.ProvenanceUserStated || legal.Confidence != nil {
		t.Errorf("legal_name attribution = %+v, want user_stated with no confidence", legal)
	}
	if !legal.StatedAt.Equal(statedAt) {
		t.Errorf("legal_name stated_at = %v, want %v", legal.StatedAt, statedAt)
	}
	vat := got.Identity.Attribution[company.FieldVATNumber]
	if vat.Provenance != company.ProvenanceAgentInferred {
		t.Errorf("vat provenance = %q, want agent_inferred", vat.Provenance)
	}
	// The confidence is the whole reason an inferred fact is distinguishable
	// from a stated one — losing it would silently promote a guess.
	if vat.Confidence == nil || *vat.Confidence != 0.72 {
		t.Errorf("vat confidence = %v, want 0.72", vat.Confidence)
	}
	if vat.PromptedBy != company.PromptTender || vat.PromptedByTenderID == nil || *vat.PromptedByTenderID != tenderID {
		t.Errorf("vat prompt provenance = %q / %v", vat.PromptedBy, vat.PromptedByTenderID)
	}
	if vat.SourceNote != "gara 545620-2026, disciplinare §7.2" {
		t.Errorf("vat source note = %q", vat.SourceNote)
	}

	// A second write is a FULL REPLACE: the cleared founded_year must go back to
	// NULL and the dropped attribution entry must disappear.
	updated, err := repo.UpsertIdentity(ctx, ws, company.Identity{
		LegalName: "Acme Costruzioni SpA",
		LegalForm: company.LegalFormSPA,
		Attribution: map[company.FieldKey]company.Attribution{
			company.FieldLegalName: {Provenance: company.ProvenanceUserStated, StatedAt: statedAt},
		},
	})
	if err != nil {
		t.Fatalf("UpsertIdentity (replace): %v", err)
	}
	if updated.FoundedYear != nil {
		t.Errorf("founded year = %v, want nil (cleared)", *updated.FoundedYear)
	}
	if updated.VATNumber != "" {
		t.Errorf("vat number = %q, want cleared", updated.VATNumber)
	}
	if _, ok := updated.Attribution[company.FieldVATNumber]; ok {
		t.Error("a dropped attribution entry must not survive a full replace")
	}
}

// TestCompanyRepo_SOA_UniquePerCategoryAndProvenanceRoundTrip covers the
// modelling claim behind the (workspace_id, category) unique: a re-answered
// question is a correction, not a second contradictory attestation.
func TestCompanyRepo_SOA_UniquePerCategoryAndProvenanceRoundTrip(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "soa")
	ctx := context.Background()

	validUntil := time.Date(2029, 6, 30, 0, 0, 0, 0, time.UTC)
	verifiedUntil := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	first, err := repo.PutSOACategory(ctx, ws, company.SOACategory{
		Category:      "OG1",
		Classifica:    company.ClassificaI,
		IssuedBy:      "SOA Nord Ovest",
		ValidUntil:    &validUntil,
		VerifiedUntil: &verifiedUntil,
		Attribution: company.Attribution{
			Provenance: company.ProvenanceUserConfirmed,
			StatedBy:   "00000000-0000-0000-0000-000000000001",
			StatedAt:   time.Now().UTC().Truncate(time.Millisecond),
			PromptedBy: company.PromptTender,
		},
	})
	if err != nil {
		t.Fatalf("PutSOACategory: %v", err)
	}

	// A correction of the classifica must land on the SAME row.
	second, err := repo.PutSOACategory(ctx, ws, company.SOACategory{
		Category:   "OG1",
		Classifica: company.ClassificaIIIBis,
		Attribution: company.Attribution{
			Provenance: company.ProvenanceUserStated,
			StatedBy:   "00000000-0000-0000-0000-000000000001",
			StatedAt:   time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("PutSOACategory (correction): %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("a correction created a new row: %q vs %q", second.ID, first.ID)
	}

	got, err := repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}
	if len(got.SOA) != 1 {
		t.Fatalf("SOA rows = %d, want 1", len(got.SOA))
	}
	line := got.SOA[0]
	if line.Classifica != company.ClassificaIIIBis {
		t.Errorf("classifica = %v, want III-bis", line.Classifica)
	}
	if line.Provenance != company.ProvenanceUserStated {
		t.Errorf("provenance = %q, want the corrected user_stated", line.Provenance)
	}
	// A "bis" tier must survive the smallint round trip as itself, not collapse
	// onto its neighbour — the whole SOA comparison is an ordering question.
	if line.Classifica.String() != "III-bis" {
		t.Errorf("classifica string = %q", line.Classifica.String())
	}
	// The correction cleared the dates; a full-column write must reflect that.
	if line.ValidUntil != nil || line.VerifiedUntil != nil {
		t.Errorf("dates = %v / %v, want both cleared", line.ValidUntil, line.VerifiedUntil)
	}

	// A DIFFERENT category is a different row.
	if _, err := repo.PutSOACategory(ctx, ws, company.SOACategory{
		Category: "OS30", Classifica: company.ClassificaII,
		Attribution: company.Attribution{Provenance: company.ProvenanceUserStated, StatedAt: time.Now()},
	}); err != nil {
		t.Fatalf("PutSOACategory (OS30): %v", err)
	}
	got, err = repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}
	if len(got.SOA) != 2 {
		t.Fatalf("SOA rows = %d, want 2", len(got.SOA))
	}

	if err := repo.DeleteSOACategory(ctx, ws, first.ID); err != nil {
		t.Fatalf("DeleteSOACategory: %v", err)
	}
	if err := repo.DeleteSOACategory(ctx, ws, first.ID); !errors.Is(err, company.ErrRecordNotFound) {
		t.Fatalf("second delete = %v, want ErrRecordNotFound", err)
	}
}

// TestCompanyRepo_AgentInferredConfidenceSurvives is the storage half of the
// engine's asymmetry: if the adapter dropped confidence or flattened provenance,
// an agent guess would become indistinguishable from a legal representative's
// statement and could produce a blocking "do not bid".
func TestCompanyRepo_AgentInferredConfidenceSurvives(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "provenance")
	ctx := context.Background()

	tenderID := int64(545620)
	inferred := company.Attribution{
		Provenance:         company.ProvenanceAgentInferred,
		Confidence:         ptr(0.61),
		StatedAt:           time.Now().UTC().Truncate(time.Millisecond),
		PromptedBy:         company.PromptExtraction,
		PromptedByTenderID: &tenderID,
		SourceNote:         "estratto dal disciplinare",
	}
	statedByHuman := company.Attribution{
		Provenance: company.ProvenanceUserStated,
		StatedBy:   "00000000-0000-0000-0000-000000000009",
		StatedAt:   time.Now().UTC().Truncate(time.Millisecond),
		PromptedBy: company.PromptOnboarding,
	}

	if _, err := repo.PutCertification(ctx, ws, company.Certification{
		Standard: company.CertISO9001, Scope: "Costruzioni", Attribution: inferred,
	}); err != nil {
		t.Fatalf("PutCertification: %v", err)
	}
	if _, err := repo.PutFinancialYear(ctx, ws, company.FinancialYear{
		Year: 2025, TurnoverMinor: ptr(int64(2_100_000_00)), Currency: "EUR",
		Headcount: ptr(int32(18)), Attribution: statedByHuman,
	}); err != nil {
		t.Fatalf("PutFinancialYear: %v", err)
	}
	if _, err := repo.PutPastContract(ctx, ws, company.PastContract{
		Description: "Manutenzione stradale", BuyerName: "Comune di Milano", CPV: "45233",
		ValueMinor: ptr(int64(5_000_000_00)), Currency: "EUR", Role: company.RoleRTIMandante,
		SharePct: ptr(20.0), EndedOn: ptr(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)),
		Attribution: inferred,
	}); err != nil {
		t.Fatalf("PutPastContract: %v", err)
	}
	if _, err := repo.PutRegistration(ctx, ws, company.Registration{
		Kind: company.RegWhiteList, Authority: "Prefettura di Milano", Identifier: "WL-2024-000123",
		Section: "Trasporto rifiuti", Attribution: statedByHuman,
	}); err != nil {
		t.Fatalf("PutRegistration: %v", err)
	}

	got, err := repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}

	cert := got.Certifications[0]
	if cert.Provenance != company.ProvenanceAgentInferred || cert.Authoritative() {
		t.Errorf("certification provenance = %q (authoritative %v)", cert.Provenance, cert.Authoritative())
	}
	if cert.Confidence == nil || *cert.Confidence != 0.61 {
		t.Errorf("certification confidence = %v, want 0.61", cert.Confidence)
	}
	// No human was involved, so stated_by must come back empty rather than as a
	// zero uuid that looks like a real user.
	if cert.StatedBy != "" {
		t.Errorf("certification stated_by = %q, want empty", cert.StatedBy)
	}
	if cert.PromptedByTenderID == nil || *cert.PromptedByTenderID != tenderID {
		t.Errorf("certification prompted_by_tender_id = %v", cert.PromptedByTenderID)
	}

	fy := got.FinancialYears[0]
	if fy.Year != 2025 || fy.TurnoverMinor == nil || *fy.TurnoverMinor != 2_100_000_00 {
		t.Errorf("financial year = %+v", fy)
	}
	if fy.Headcount == nil || *fy.Headcount != 18 {
		t.Errorf("headcount = %v", fy.Headcount)
	}
	if !fy.Authoritative() || fy.Confidence != nil || fy.StatedBy == "" {
		t.Errorf("financial year attribution = %+v, want authoritative with a stated_by and no confidence", fy.Attribution)
	}

	pc := got.PastContracts[0]
	if pc.SharePct == nil || *pc.SharePct != 20 {
		t.Errorf("share_pct = %v, want 20", pc.SharePct)
	}
	if eff := pc.EffectiveValueMinor(); eff == nil || *eff != 1_000_000_00 {
		t.Errorf("effective value = %v, want 100000000", eff)
	}

	reg := got.Registrations[0]
	if reg.Identifier != "WL-2024-000123" || reg.Section != "Trasporto rifiuti" {
		t.Errorf("registration = %+v", reg)
	}
}

// TestCompanyRepo_DossierExistsWithoutIdentity: under just-in-time capture the
// first thing ever written may be a SOA line answered in chat, months before
// anyone types a VAT number. That is a real dossier, not a not-found.
func TestCompanyRepo_DossierExistsWithoutIdentity(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "jit")
	ctx := context.Background()

	if _, err := repo.PutSOACategory(ctx, ws, company.SOACategory{
		Category: "OG1", Classifica: company.ClassificaIII,
		Attribution: company.Attribution{Provenance: company.ProvenanceUserConfirmed, StatedAt: time.Now()},
	}); err != nil {
		t.Fatalf("PutSOACategory: %v", err)
	}

	got, err := repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier = %v, want a dossier with no identity row", err)
	}
	if got.Identity.LegalName != "" {
		t.Errorf("identity should be empty, got %q", got.Identity.LegalName)
	}
	if len(got.SOA) != 1 {
		t.Fatalf("SOA rows = %d, want 1", len(got.SOA))
	}
	if got.Empty() {
		t.Error("a dossier with one captured fact must not report as empty")
	}
}

// TestCompanyRepo_FinancialYearsNewestFirst: the turnover window reads
// FinancialYears[0].Year as the newest esercizio on file, so the repository's
// ordering is load-bearing rather than cosmetic.
func TestCompanyRepo_FinancialYearsNewestFirst(t *testing.T) {
	repo, _, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "years")
	ctx := context.Background()

	attr := company.Attribution{Provenance: company.ProvenanceUserStated, StatedAt: time.Now()}
	for _, year := range []int32{2023, 2025, 2024} {
		if _, err := repo.PutFinancialYear(ctx, ws, company.FinancialYear{
			Year: year, TurnoverMinor: ptr(int64(1_000_000_00)), Currency: "EUR", Attribution: attr,
		}); err != nil {
			t.Fatalf("PutFinancialYear(%d): %v", year, err)
		}
	}
	// A re-answer for the same year is an UPDATE, not a fourth row.
	if _, err := repo.PutFinancialYear(ctx, ws, company.FinancialYear{
		Year: 2024, TurnoverMinor: ptr(int64(1_500_000_00)), Currency: "EUR", Attribution: attr,
	}); err != nil {
		t.Fatalf("PutFinancialYear (re-answer): %v", err)
	}

	got, err := repo.GetDossier(ctx, ws)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}
	if len(got.FinancialYears) != 3 {
		t.Fatalf("financial years = %d, want 3", len(got.FinancialYears))
	}
	want := []int32{2025, 2024, 2023}
	for i, y := range want {
		if got.FinancialYears[i].Year != y {
			t.Fatalf("financial years out of order: %d at index %d, want %d", got.FinancialYears[i].Year, i, y)
		}
	}
	if v := got.FinancialYears[1].TurnoverMinor; v == nil || *v != 1_500_000_00 {
		t.Errorf("2024 turnover = %v, want the re-answered 150000000", v)
	}

	if err := repo.DeleteFinancialYear(ctx, ws, 2023); err != nil {
		t.Fatalf("DeleteFinancialYear: %v", err)
	}
	if err := repo.DeleteFinancialYear(ctx, ws, 2023); !errors.Is(err, company.ErrRecordNotFound) {
		t.Fatalf("second delete = %v, want ErrRecordNotFound", err)
	}
}

// ── Requirements ────────────────────────────────────────────────────────────

// TestRequirementRepo_PutIsIdempotentOnDedupeKey is the reason the unique exists:
// an agent that re-extracts on every turn must update rows, not multiply them.
func TestRequirementRepo_PutIsIdempotentOnDedupeKey(t *testing.T) {
	_, reqs, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "req-dedupe")
	ctx := context.Background()
	const tenderID = int64(545620)

	batch := []company.Requirement{{
		Kind: company.RequirementSOA, Source: company.RequirementDocumentExcerpt, Blocking: true,
		Text: "Attestazione SOA OG1 classifica III",
		SOA:  &company.SOARequirement{Category: "OG1", Classifica: company.ClassificaIII, Prevalente: true},
		Citation: &document.Citation{
			DocumentURL: "https://example.test/disciplinare.pdf", DocumentType: "spec",
			PartIndex: 12, PageStart: ptr(14), PageEnd: ptr(16),
			SectionPath: []string{"7", "7.2"}, SectionTitle: "Requisiti di capacità tecnica",
		},
		Excerpt: "…attestazione SOA nella categoria OG1 classifica III…",
	}}

	first, err := reqs.PutRequirements(ctx, ws, tenderID, batch)
	if err != nil {
		t.Fatalf("PutRequirements: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("returned %d requirements, want 1", len(first))
	}

	// The same extraction re-run with a CORRECTED classifica must update the
	// same row: the dedupe key deliberately omits the classifica.
	corrected := batch
	corrected[0].SOA = &company.SOARequirement{Category: "OG1", Classifica: company.ClassificaIV, Prevalente: true}
	corrected[0].Text = "Attestazione SOA OG1 classifica IV"
	second, err := reqs.PutRequirements(ctx, ws, tenderID, corrected)
	if err != nil {
		t.Fatalf("PutRequirements (re-extract): %v", err)
	}
	if second[0].ID != first[0].ID {
		t.Fatalf("a re-extraction created a new row: %q vs %q", second[0].ID, first[0].ID)
	}

	stored, err := reqs.ListRequirements(ctx, ws, tenderID)
	if err != nil {
		t.Fatalf("ListRequirements: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("stored requirements = %d, want 1", len(stored))
	}
	got := stored[0]
	if got.SOA == nil || got.SOA.Classifica != company.ClassificaIV || !got.SOA.Prevalente {
		t.Errorf("payload did not round trip: %+v", got.SOA)
	}
	// The citation is the verification affordance — a claim the user can check
	// without leaving the product — so every part of it must survive.
	if got.Citation == nil {
		t.Fatal("citation lost")
	}
	if got.Citation.DocumentURL != "https://example.test/disciplinare.pdf" ||
		got.Citation.PartIndex != 12 ||
		got.Citation.PageStart == nil || *got.Citation.PageStart != 14 ||
		len(got.Citation.SectionPath) != 2 || got.Citation.SectionTitle != "Requisiti di capacità tecnica" {
		t.Errorf("citation did not round trip: %+v", got.Citation)
	}
	if got.Excerpt == "" {
		t.Error("excerpt lost")
	}
}

// TestRequirementRepo_NoticeLevelRowsDoNotDuplicate: lot_ref is NOT NULL
// DEFAULT the empty string precisely so notice-level rows conflict with each other. A nullable
// column would let unlimited duplicates through, since PostgreSQL NULLs never
// conflict in a unique key.
func TestRequirementRepo_NoticeLevelRowsDoNotDuplicate(t *testing.T) {
	_, reqs, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "req-lot")
	ctx := context.Background()
	const tenderID = int64(545621)

	noticeLevel := company.Requirement{
		Kind: company.RequirementOther, Source: company.RequirementUserStated, Blocking: true,
		Text: "Polizza assicurativa RCT/RCO",
	}
	lotScoped := noticeLevel
	lotScoped.LotRef = "LOT-0001"

	for range 3 {
		if _, err := reqs.PutRequirements(ctx, ws, tenderID, []company.Requirement{noticeLevel, lotScoped}); err != nil {
			t.Fatalf("PutRequirements: %v", err)
		}
	}

	stored, err := reqs.ListRequirements(ctx, ws, tenderID)
	if err != nil {
		t.Fatalf("ListRequirements: %v", err)
	}
	if len(stored) != 2 {
		t.Fatalf("stored requirements = %d, want 2 (one notice-level, one lot-scoped)", len(stored))
	}
}

// TestRequirementRepo_ConfirmIsTheOnlyPathToBinding covers the ConfirmedBy gate:
// a re-extraction must neither grant nor revoke a human's confirmation.
func TestRequirementRepo_ConfirmIsTheOnlyPathToBinding(t *testing.T) {
	_, reqs, sqlDB := testCompanyRepos(t)
	ws := seedWorkspaceForCompany(t, sqlDB, "req-confirm")
	ctx := context.Background()
	const tenderID = int64(545622)

	var userID string
	if err := sqlDB.QueryRowContext(ctx, `SELECT owner_id FROM workspaces WHERE id = $1`, ws).Scan(&userID); err != nil {
		t.Fatalf("load owner: %v", err)
	}

	batch := []company.Requirement{{
		Kind: company.RequirementOther, Source: company.RequirementDocumentExcerpt, Blocking: true,
		Text: "Iscrizione alla white list prefettizia",
	}}
	created, err := reqs.PutRequirements(ctx, ws, tenderID, batch)
	if err != nil {
		t.Fatalf("PutRequirements: %v", err)
	}
	if created[0].Confirmed() || created[0].CanBlock() {
		t.Fatal("an extracted requirement must arrive unconfirmed and unable to block")
	}

	confirmed, err := reqs.ConfirmRequirement(ctx, ws, created[0].ID, userID)
	if err != nil {
		t.Fatalf("ConfirmRequirement: %v", err)
	}
	if !confirmed.Confirmed() || !confirmed.CanBlock() {
		t.Fatal("a confirmed extraction must be able to block")
	}
	if confirmed.ConfirmedAt == nil {
		t.Fatal("confirmed_at not recorded")
	}

	// Re-extracting the same requirement must not revoke the confirmation.
	if _, err := reqs.PutRequirements(ctx, ws, tenderID, batch); err != nil {
		t.Fatalf("PutRequirements (re-extract): %v", err)
	}
	stored, err := reqs.ListRequirements(ctx, ws, tenderID)
	if err != nil {
		t.Fatalf("ListRequirements: %v", err)
	}
	if len(stored) != 1 || !stored[0].Confirmed() {
		t.Fatalf("a re-extraction revoked the confirmation: %+v", stored)
	}

	if err := reqs.DeleteRequirement(ctx, ws, created[0].ID); err != nil {
		t.Fatalf("DeleteRequirement: %v", err)
	}
	if err := reqs.DeleteRequirement(ctx, ws, created[0].ID); !errors.Is(err, company.ErrRecordNotFound) {
		t.Fatalf("second delete = %v, want ErrRecordNotFound", err)
	}
}

// TestRequirementRepo_TenantIsolation: every method is workspace-scoped as its
// first argument, so a requirement id leaked into another tenant is inert there.
func TestRequirementRepo_TenantIsolation(t *testing.T) {
	_, reqs, sqlDB := testCompanyRepos(t)
	mine := seedWorkspaceForCompany(t, sqlDB, "req-mine")
	theirs := seedWorkspaceForCompany(t, sqlDB, "req-theirs")
	ctx := context.Background()
	const tenderID = int64(545623)

	created, err := reqs.PutRequirements(ctx, mine, tenderID, []company.Requirement{{
		Kind: company.RequirementOther, Source: company.RequirementUserStated, Blocking: true,
		Text: "Polizza RCT",
	}})
	if err != nil {
		t.Fatalf("PutRequirements: %v", err)
	}

	if got, err := reqs.ListRequirements(ctx, theirs, tenderID); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant list = %v (%d rows), want empty", err, len(got))
	}
	if _, err := reqs.ConfirmRequirement(ctx, theirs, created[0].ID, "00000000-0000-0000-0000-000000000001"); !errors.Is(err, company.ErrRecordNotFound) {
		t.Fatalf("cross-tenant confirm = %v, want ErrRecordNotFound", err)
	}
	if err := reqs.DeleteRequirement(ctx, theirs, created[0].ID); !errors.Is(err, company.ErrRecordNotFound) {
		t.Fatalf("cross-tenant delete = %v, want ErrRecordNotFound", err)
	}
}
