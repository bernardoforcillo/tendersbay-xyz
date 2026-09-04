package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

func testEspdRepos(t *testing.T) (*postgres.CompanyRepo, *postgres.BidRepo, *postgres.EspdStore, *sql.DB) {
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
	return postgres.NewCompanyRepo(db), postgres.NewBidRepo(db), postgres.NewEspdStore(db), sqlDB
}

// seedBidForEspd creates user → workspace → workbench → bid and returns the
// ids. Everything cascades from the workspace, which the cleanup removes.
func seedBidForEspd(t *testing.T, sqlDB *sql.DB, slug string) (workspaceID, userID, bidID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = seedWorkspaceForCompany(t, sqlDB, slug)
	if err := sqlDB.QueryRowContext(ctx, `SELECT owner_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&userID); err != nil {
		t.Fatalf("owner: %v", err)
	}
	var workbenchID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workbenches (workspace_id, name, owner_id) VALUES ($1, 'ESPD WB', $2) RETURNING id`,
		workspaceID, userID).Scan(&workbenchID); err != nil {
		t.Fatalf("seed workbench: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO bids (workbench_id, tender_id, created_by) VALUES ($1, 424242, $2) RETURNING id`,
		workbenchID, userID).Scan(&bidID); err != nil {
		t.Fatalf("seed bid: %v", err)
	}
	return workspaceID, userID, bidID
}

func TestCompanyRepo_EspdSectionsRoundTripAndUpsert(t *testing.T) {
	companies, _, _, sqlDB := testEspdRepos(t)
	ctx := context.Background()
	wsID, userID, _ := seedBidForEspd(t, sqlDB, "espd-sections")
	attr := company.Attribution{Provenance: company.ProvenanceUserStated, StatedBy: userID, StatedAt: time.Now().Add(-time.Minute), PromptedBy: company.PromptOnboarding}

	birth := time.Date(1980, 5, 4, 0, 0, 0, 0, time.UTC)
	rep, err := companies.PutRepresentative(ctx, wsID, company.Representative{Role: "legale_rappresentante", GivenName: "Anna", FamilyName: "Rossi", BirthDate: &birth, Email: "anna@acme.example", Attribution: attr})
	if err != nil {
		t.Fatalf("PutRepresentative: %v", err)
	}
	rep.PowerOfAttorney = true
	if _, err := companies.PutRepresentative(ctx, wsID, rep); err != nil {
		t.Fatalf("update representative: %v", err)
	}

	dec, err := companies.PutDeclaration(ctx, wsID, company.Declaration{Criterion: string(espd.CritFraud), Answer: false, Attribution: attr})
	if err != nil {
		t.Fatalf("PutDeclaration: %v", err)
	}
	again, err := companies.PutDeclaration(ctx, wsID, company.Declaration{Criterion: string(espd.CritFraud), Answer: true, SelfCleaning: "misure", Attribution: attr})
	if err != nil {
		t.Fatalf("re-answer: %v", err)
	}
	if again.ID != dec.ID || !again.Answer {
		t.Errorf("re-answer must update the same row: %+v vs %+v", dec, again)
	}

	ng, err := companies.PutNationalGround(ctx, wsID, company.NationalGround{Country: "IT", Criterion: "art94.c1", Answer: false, Attribution: attr})
	if err != nil {
		t.Fatalf("PutNationalGround: %v", err)
	}
	if _, err := companies.PutNationalGround(ctx, wsID, company.NationalGround{Country: "IT", Criterion: "art94.c1", Answer: true, Note: "n", Attribution: attr}); err != nil {
		t.Fatalf("re-answer national ground: %v", err)
	}

	d, err := companies.GetDossier(ctx, wsID)
	if err != nil {
		t.Fatalf("GetDossier: %v", err)
	}
	if len(d.Representatives) != 1 || !d.Representatives[0].PowerOfAttorney || d.Representatives[0].BirthDate == nil || !d.Representatives[0].BirthDate.Equal(birth) {
		t.Errorf("representatives = %+v", d.Representatives)
	}
	if len(d.Declarations) != 1 || d.Declarations[0].StatedBy != userID || d.Declarations[0].SelfCleaning != "misure" {
		t.Errorf("declarations = %+v", d.Declarations)
	}
	if len(d.NationalGrounds) != 1 || d.NationalGrounds[0].ID != ng.ID || !d.NationalGrounds[0].Answer {
		t.Errorf("national grounds = %+v", d.NationalGrounds)
	}

	// A dossier whose only content is a declaration is a dossier, not
	// ErrDossierNotFound.
	if err := companies.DeleteRepresentative(ctx, wsID, rep.ID); err != nil {
		t.Fatalf("DeleteRepresentative: %v", err)
	}
	if err := companies.DeleteNationalGround(ctx, wsID, ng.ID); err != nil {
		t.Fatalf("DeleteNationalGround: %v", err)
	}
	if _, err := companies.GetDossier(ctx, wsID); err != nil {
		t.Fatalf("GetDossier with one declaration: %v", err)
	}
	if err := companies.DeleteDeclaration(ctx, wsID, dec.ID); err != nil {
		t.Fatalf("DeleteDeclaration: %v", err)
	}
	if _, err := companies.GetDossier(ctx, wsID); !errors.Is(err, company.ErrDossierNotFound) {
		t.Errorf("empty dossier = %v, want ErrDossierNotFound", err)
	}
	if err := companies.DeleteDeclaration(ctx, wsID, dec.ID); !errors.Is(err, company.ErrRecordNotFound) {
		t.Errorf("second delete = %v, want ErrRecordNotFound", err)
	}
}

// TestCompanyRepo_DeclarationsRefuseNonAuthoritativeProvenance: the adapter
// re-checks the Part III provenance wall, so a caller that bypassed the
// service still cannot write an inferred declaration.
func TestCompanyRepo_DeclarationsRefuseNonAuthoritativeProvenance(t *testing.T) {
	companies, _, _, sqlDB := testEspdRepos(t)
	ctx := context.Background()
	wsID, _, _ := seedBidForEspd(t, sqlDB, "espd-wall")
	inferred := company.Attribution{Provenance: company.ProvenanceAgentInferred, Confidence: ptr(0.9), StatedAt: time.Now()}
	if _, err := companies.PutDeclaration(ctx, wsID, company.Declaration{Criterion: string(espd.CritFraud), Attribution: inferred}); !errors.Is(err, company.ErrDeclarationNotAuthoritative) {
		t.Errorf("PutDeclaration(inferred) = %v", err)
	}
	if _, err := companies.PutNationalGround(ctx, wsID, company.NationalGround{Country: "IT", Criterion: "x", Attribution: inferred}); !errors.Is(err, company.ErrDeclarationNotAuthoritative) {
		t.Errorf("PutNationalGround(inferred) = %v", err)
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM company_declarations WHERE workspace_id = $1`, wsID).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows written = %d, %v", n, err)
	}
}

func TestCompanyRepo_IsSMERoundTrips(t *testing.T) {
	companies, _, _, sqlDB := testEspdRepos(t)
	ctx := context.Background()
	wsID, userID, _ := seedBidForEspd(t, sqlDB, "espd-sme")
	attr := company.Attribution{Provenance: company.ProvenanceUserStated, StatedBy: userID, StatedAt: time.Now(), PromptedBy: company.PromptOnboarding}
	id, err := companies.UpsertIdentity(ctx, wsID, company.Identity{LegalName: "Acme", IsSME: true, Attribution: map[company.FieldKey]company.Attribution{company.FieldLegalName: attr, company.FieldIsSME: attr}})
	if err != nil {
		t.Fatalf("UpsertIdentity: %v", err)
	}
	if !id.IsSME || id.Attribution[company.FieldIsSME].Provenance != company.ProvenanceUserStated {
		t.Errorf("identity = %+v", id)
	}
	id, err = companies.UpsertIdentity(ctx, wsID, company.Identity{LegalName: "Acme", IsSME: false, Attribution: id.Attribution})
	if err != nil || id.IsSME {
		t.Errorf("full replace did not clear is_sme: %+v %v", id, err)
	}
}

func TestBidRepo_EspdDataUpsertsOnNaturalKeysAndCascades(t *testing.T) {
	_, bids, _, sqlDB := testEspdRepos(t)
	ctx := context.Background()
	_, userID, bidID := seedBidForEspd(t, sqlDB, "espd-bid")

	lot, err := bids.PutLot(ctx, bidID, bid.Lot{LotRef: "LOT-0002", Position: 2})
	if err != nil {
		t.Fatalf("PutLot: %v", err)
	}
	if _, err := bids.PutLot(ctx, bidID, bid.Lot{LotRef: "LOT-0001", Position: 1}); err != nil {
		t.Fatalf("PutLot: %v", err)
	}
	lot2, err := bids.PutLot(ctx, bidID, bid.Lot{LotRef: "LOT-0002", Position: 9})
	if err != nil || lot2.ID != lot.ID || lot2.Position != 9 {
		t.Errorf("lot upsert: %+v %+v %v", lot, lot2, err)
	}

	share := int32(25)
	sub, err := bids.PutSubcontractor(ctx, bidID, bid.Subcontractor{Name: "Beta", VAT: "IT001", Country: "IT", Share: &share})
	if err != nil {
		t.Fatalf("PutSubcontractor: %v", err)
	}
	sub2, err := bids.PutSubcontractor(ctx, bidID, bid.Subcontractor{Name: "Beta Srl", VAT: "IT001"})
	if err != nil || sub2.ID != sub.ID || sub2.Share != nil || sub2.Name != "Beta Srl" {
		t.Errorf("subcontractor upsert: %+v %+v %v", sub, sub2, err)
	}

	rel, err := bids.PutReliance(ctx, bidID, bid.Reliance{EntityName: "Gamma", VAT: "IT002", Criterion: string(espd.CritGeneralTurnover)})
	if err != nil {
		t.Fatalf("PutReliance: %v", err)
	}
	if _, err := bids.PutReliance(ctx, bidID, bid.Reliance{EntityName: "Gamma", VAT: "IT002", Criterion: string(espd.CritSpecificTurnover)}); err != nil {
		t.Fatalf("second criterion, same entity: %v", err)
	}

	data, err := bids.ListEspdData(ctx, bidID)
	if err != nil {
		t.Fatalf("ListEspdData: %v", err)
	}
	if len(data.Lots) != 2 || data.Lots[0].LotRef != "LOT-0001" || len(data.Subcontractors) != 1 || len(data.Reliances) != 2 {
		t.Errorf("data = %+v", data)
	}

	if _, err := bids.GetDeclarationConfirmation(ctx, bidID); !errors.Is(err, bid.ErrConfirmationNotFound) {
		t.Errorf("fresh bid confirmation = %v, want ErrConfirmationNotFound", err)
	}
	c, err := bids.PutDeclarationConfirmation(ctx, bid.DeclarationConfirmation{BidID: bidID, UserID: userID, DeclarationsHash: "h1"})
	if err != nil || c.ConfirmedAt.IsZero() {
		t.Fatalf("PutDeclarationConfirmation: %+v %v", c, err)
	}
	c2, err := bids.PutDeclarationConfirmation(ctx, bid.DeclarationConfirmation{BidID: bidID, UserID: userID, DeclarationsHash: "h2"})
	if err != nil || c2.DeclarationsHash != "h2" {
		t.Fatalf("second confirmation: %+v %v", c2, err)
	}
	got, err := bids.GetDeclarationConfirmation(ctx, bidID)
	if err != nil || got.DeclarationsHash != "h2" {
		t.Errorf("latest confirmation = %+v %v", got, err)
	}

	if err := bids.DeleteReliance(ctx, bidID, rel.ID); err != nil {
		t.Fatalf("DeleteReliance: %v", err)
	}
	if err := bids.DeleteReliance(ctx, bidID, rel.ID); !errors.Is(err, bid.ErrInvalidArgument) {
		t.Errorf("second delete = %v", err)
	}
	if err := bids.DeleteLot(ctx, "00000000-0000-0000-0000-000000000000", lot.ID); !errors.Is(err, bid.ErrInvalidArgument) {
		t.Errorf("delete through another bid id = %v, want not-found", err)
	}

	// Everything hangs off the bid.
	if err := bids.DeleteBid(ctx, bidID); err != nil {
		t.Fatalf("DeleteBid: %v", err)
	}
	for _, table := range []string{"bid_lots", "bid_subcontractors", "bid_reliances", "bid_declaration_confirmations"} {
		var n int
		if err := sqlDB.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE bid_id = $1`, bidID).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s rows after DeleteBid = %d, %v", table, n, err)
		}
	}
}

func TestEspdStore_RequestAndExportLog(t *testing.T) {
	_, _, store, sqlDB := testEspdRepos(t)
	ctx := context.Background()
	_, userID, bidID := seedBidForEspd(t, sqlDB, "espd-store")

	if _, err := store.Get(ctx, bidID); !errors.Is(err, espd.ErrRequestNotFound) {
		t.Errorf("Get on a fresh bid = %v", err)
	}
	raw := []byte(strings.ReplaceAll(`<QualificationApplicationRequest xmlns="urn:x"><VersionID>2.1.1</VersionID><ContractFolderID>CIG1</ContractFolderID>
<ContractingParty><Party><PartyName><Name>Comune</Name></PartyName></Party></ContractingParty></QualificationApplicationRequest>`, "\n", ""))
	req, err := espd.ParseRequest(raw)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	req.ImportedBy = userID
	if err := store.Put(ctx, bidID, req, raw); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := store.Get(ctx, bidID)
	if err != nil || got.ProcedureReference != "CIG1" || got.SHA256 != req.SHA256 || got.ImportedBy != userID || got.ImportedAt.IsZero() {
		t.Errorf("Get = %+v %v", got, err)
	}
	// Re-import replaces.
	raw2 := []byte(strings.Replace(string(raw), "CIG1", "CIG2", 1))
	req2, _ := espd.ParseRequest(raw2)
	req2.ImportedBy = userID
	if err := store.Put(ctx, bidID, req2, raw2); err != nil {
		t.Fatalf("Put again: %v", err)
	}
	if got, _ := store.Get(ctx, bidID); got.ProcedureReference != "CIG2" {
		t.Errorf("re-import did not replace: %+v", got)
	}

	confirmedAt := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	for _, f := range []espd.Format{espd.FormatPDF, espd.FormatXML} {
		if err := store.Record(ctx, espd.Export{BidID: bidID, UserID: userID, Version: espd.EDM211, Format: f, ContentSHA256: "sha-" + string(f), DeclarationsConfirmedAt: confirmedAt}); err != nil {
			t.Fatalf("Record(%s): %v", f, err)
		}
	}
	exports, err := store.List(ctx, bidID)
	if err != nil || len(exports) != 2 {
		t.Fatalf("List = %d %v", len(exports), err)
	}
	if exports[0].ID == "" || exports[0].UserID != userID || exports[0].Version != espd.EDM211 || !exports[0].DeclarationsConfirmedAt.Equal(confirmedAt) {
		t.Errorf("export = %+v", exports[0])
	}
}
