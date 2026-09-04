package e2e

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"connectrpc.com/connect"

	bidv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1"
	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	espdv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/espd/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// The ESPD journey, end to end over HTTP: a PMI fills its dossier, tracks a
// gara, sees what is still missing, closes the gaps, re-confirms its Part III
// declarations and exports the document in three shapes.
//
// It is one long test on purpose. The interesting assertions are about STATE
// CHANGING — readiness that goes from false to true, an export that is refused
// and then allowed — and a suite of independent tests would have to rebuild the
// world before each one, which is both slower and a weaker check.

// espdBid is a workspace, a workbench and a tracked bid, ready for a dossier.
type espdBid struct {
	workspaceID, workbenchID, bidID string
}

func (a *account) trackBid(t *testing.T, name string, tenderID string) espdBid {
	t.Helper()
	ctx := context.Background()
	ws := a.createWorkspace(t, name)
	wb := a.createWorkbench(t, ws.Id, name+" wb", workbench.VisibilityShared)
	added, err := a.bid.AddBid(ctx, authed(&bidv1.AddBidRequest{
		WorkbenchId: wb.Id, TenderId: tenderID,
	}, a.access))
	if err != nil {
		t.Fatalf("AddBid: %v", err)
	}
	return espdBid{workspaceID: ws.Id, workbenchID: wb.Id, bidID: added.Msg.Bid.Id}
}

// preview is the readiness read every step below checks.
func (a *account) preview(t *testing.T, b espdBid) *espdv1.GetResponsePreviewResponse {
	t.Helper()
	out, err := a.espd.GetResponsePreview(context.Background(), authed(&espdv1.GetResponsePreviewRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
	}, a.access))
	if err != nil {
		t.Fatalf("GetResponsePreview: %v", err)
	}
	return out.Msg
}

// fillDossier answers everything the document needs of the COMPANY: identity,
// a representative, and all 23 Part III exclusion grounds.
func (a *account) fillDossier(t *testing.T, workspaceID string) {
	t.Helper()
	ctx := context.Background()

	if _, err := a.company.UpdateCompanyIdentity(ctx, authed(&companyv1.UpdateCompanyIdentityRequest{
		WorkspaceId: workspaceID,
		Identity: &companyv1.CompanyIdentity{
			LegalName: "Acme Costruzioni Srl", VatNumber: "IT01234567890",
			FiscalCode: "01234567890", LegalForm: "srl",
			CciaaOffice: "MI", CciaaNumber: "1234567",
			Country: "IT", Nuts: "ITC4C", IsSme: true,
		},
	}, a.access)); err != nil {
		t.Fatalf("UpdateCompanyIdentity: %v", err)
	}

	if _, err := a.company.PutRepresentative(ctx, authed(&companyv1.PutRepresentativeRequest{
		WorkspaceId: workspaceID,
		Representative: &companyv1.Representative{
			Role: "legale rappresentante", GivenName: "Anna", FamilyName: "Rossi",
			BirthPlace: "Milano", Email: "anna.rossi@acme.example",
		},
	}, a.access)); err != nil {
		t.Fatalf("PutRepresentative: %v", err)
	}

	for _, k := range espd.ExclusionCriteria() {
		if _, err := a.company.PutDeclaration(ctx, authed(&companyv1.PutDeclarationRequest{
			WorkspaceId: workspaceID,
			Declaration: &companyv1.Declaration{Criterion: string(k), Answer: false},
		}, a.access)); err != nil {
			t.Fatalf("PutDeclaration(%s): %v", k, err)
		}
	}
}

// TestEspdJourney is the whole path from an empty dossier to three exported
// files.
func TestEspdJourney(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	a := s.signUp(t, "espd")
	b := a.trackBid(t, "espd", "424242")

	// ── 1. An empty dossier composes to a document of gaps ──────────────────
	//
	// Not an error, and not an empty response: the gaps ARE the answer to
	// "what do I still need", which is the product's whole proposition.
	first := a.preview(t, b)
	if first.Ready {
		t.Fatal("an empty dossier must not be ready to export")
	}
	if first.MissingCount == 0 {
		t.Fatal("an empty dossier must report gaps")
	}
	if first.Declarations == nil || first.Declarations.Complete {
		t.Error("no declaration has been made yet")
	}
	// Every gap must say WHERE it is fixed, or the UI cannot route it.
	for _, g := range first.Gaps {
		if g.Scope != "company" && g.Scope != "bid" {
			t.Errorf("gap %s/%s has no usable scope %q", g.Criterion, g.Field, g.Scope)
		}
		if g.Reason == "" {
			t.Errorf("gap %s/%s has no reason", g.Criterion, g.Field)
		}
	}

	// ── 2. On the free plan the PLAN answers first ──────────────────────────
	//
	// The document is also incomplete here, and the caller is told neither
	// thing: an unentitled caller must not learn whether its document happens
	// to be ready. The order of the two gates is the assertion.
	_, err := a.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID, Version: "edm_2_1_1", Format: "xml",
	}, a.access))
	if codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("export on the free plan = %v, want permission_denied", err)
	}
	// And the preview stays free: seeing the document is not what is sold.
	if p := a.preview(t, b); p.MissingCount == 0 {
		t.Error("the preview must keep working for a workspace that cannot export")
	}

	// ── 3. Filling the dossier closes the company-scoped gaps ───────────────
	a.fillDossier(t, b.workspaceID)
	filled := a.preview(t, b)
	if filled.ReadyCount <= first.ReadyCount {
		t.Errorf("filling the dossier did not add filled fields (%d -> %d)", first.ReadyCount, filled.ReadyCount)
	}
	if !filled.Declarations.Complete {
		t.Fatal("every exclusion ground was answered, so the set must be complete")
	}
	if filled.Declarations.Confirmed {
		t.Error("answering is not confirming: the set must still need a signature for this gara")
	}
	if filled.Ready {
		t.Error("a set that has never been confirmed for this bid must not be ready")
	}
	// The one gap left must be the confirmation, and it must be bid-scoped —
	// the fix belongs to this gara, not to the dossier.
	stale := 0
	for _, g := range filled.Gaps {
		if g.Reason == "stale" {
			stale++
			if g.Scope != "bid" {
				t.Errorf("a stale confirmation is fixed on the bid, not in the dossier: %+v", g)
			}
		}
	}
	if stale != 1 {
		t.Errorf("%d stale gaps, want exactly the confirmation", stale)
	}

	// ── 4. On Pro, the DOCUMENT answers ─────────────────────────────────────
	//
	// Same call, same incomplete-in-one-respect document, different refusal:
	// now that the plan carries the export, the missing confirmation is what
	// stands in the way, and the client is told so.
	s.upgradeToPro(t, b.workspaceID)
	_, err = a.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID, Version: "edm_2_1_1", Format: "xml",
	}, a.access))
	if codeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("export with unconfirmed declarations = %v, want failed_precondition", err)
	}

	// ── 5. Re-confirming for this gara makes it ready ───────────────────────
	confirmed, err := a.espd.ConfirmDeclarations(ctx, authed(&espdv1.ConfirmDeclarationsRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
	}, a.access))
	if err != nil {
		t.Fatalf("ConfirmDeclarations: %v", err)
	}
	if !confirmed.Msg.Declarations.Confirmed || confirmed.Msg.Declarations.ConfirmedBy != a.userID {
		t.Errorf("the confirmation must record who signed: %+v", confirmed.Msg.Declarations)
	}
	ready := a.preview(t, b)
	if !ready.Ready {
		t.Fatalf("the document should be ready; %d gaps remain: %+v", ready.MissingCount, ready.Gaps)
	}

	// ── 6. Export, in all three shapes ──────────────────────────────────────
	var exported []*espdv1.ExportResponseResponse
	for _, tc := range []struct{ version, format, wantMIME, wantExt string }{
		{"edm_2_1_1", "xml", "application/xml", ".xml"},
		{"edm_4", "xml", "application/xml", ".xml"},
		{"edm_2_1_1", "pdf", "application/pdf", ".pdf"},
	} {
		out, err := a.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
			WorkbenchId: b.workbenchID, BidId: b.bidID,
			Version: tc.version, Format: tc.format, Locale: "it-it",
		}, a.access))
		if err != nil {
			t.Fatalf("export %s/%s: %v", tc.version, tc.format, err)
		}
		if out.Msg.MimeType != tc.wantMIME || !strings.HasSuffix(out.Msg.Filename, tc.wantExt) {
			t.Errorf("%s/%s: mime %q file %q", tc.version, tc.format, out.Msg.MimeType, out.Msg.Filename)
		}
		if len(out.Msg.Content) == 0 {
			t.Fatalf("%s/%s produced no bytes", tc.version, tc.format)
		}
		if out.Msg.Export == nil || out.Msg.Export.ContentSha256 == "" || out.Msg.Export.DeclarationsConfirmedAt == "" {
			t.Errorf("%s/%s: the audit row is incomplete: %+v", tc.version, tc.format, out.Msg.Export)
		}
		exported = append(exported, out.Msg)
	}

	// ── 7. The bytes are the documents they claim to be ─────────────────────
	for i, want := range []string{"2.1.1", "4.1.0"} {
		var doc struct {
			XMLName   xml.Name
			VersionID string `xml:"VersionID"`
		}
		if err := xml.Unmarshal(exported[i].Content, &doc); err != nil {
			t.Fatalf("export %d is not well-formed XML: %v", i, err)
		}
		if doc.XMLName.Local != "QualificationApplicationResponse" || doc.VersionID != want {
			t.Errorf("export %d = <%s> version %q, want a %s response", i, doc.XMLName.Local, doc.VersionID, want)
		}
		if !strings.Contains(string(exported[i].Content), "Acme Costruzioni Srl") {
			t.Errorf("export %d does not carry the operator's name", i)
		}
	}
	if !strings.HasPrefix(string(exported[2].Content), "%PDF-1.") {
		t.Error("the PDF export is not a PDF")
	}
	if !strings.Contains(string(exported[2].Content), "Documento di gara unico europeo") {
		t.Error("the Italian PDF is not in Italian")
	}

	// The two XML versions must differ — that is the entire reason there are
	// two serializers — while describing the same company.
	if string(exported[0].Content) == string(exported[1].Content) {
		t.Error("the 2.1.1 and 4.x exports are byte-identical")
	}

	// ── 8. The audit lists all three, bytes never stored ────────────────────
	list, err := a.espd.ListExports(ctx, authed(&espdv1.ListExportsRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
	}, a.access))
	if err != nil {
		t.Fatalf("ListExports: %v", err)
	}
	if len(list.Msg.Exports) != 3 {
		t.Fatalf("%d audit rows for 3 exports", len(list.Msg.Exports))
	}
	seen := map[string]bool{}
	for _, e := range list.Msg.Exports {
		if e.UserId != a.userID || e.BidId != b.bidID {
			t.Errorf("audit row does not name who exported what: %+v", e)
		}
		seen[e.Version+"/"+e.Format] = true
	}
	for _, want := range []string{"edm_2_1_1/xml", "edm_4/xml", "edm_2_1_1/pdf"} {
		if !seen[want] {
			t.Errorf("the audit is missing %s", want)
		}
	}

	// ── 9. Changing a declaration invalidates the confirmation ──────────────
	//
	// No cron, no flag: the confirmation binds to a content hash, so the next
	// read simply reports it as stale.
	if _, err := a.company.PutDeclaration(ctx, authed(&companyv1.PutDeclarationRequest{
		WorkspaceId: b.workspaceID,
		Declaration: &companyv1.Declaration{
			Criterion: string(espd.CritPaymentTaxes), Answer: true,
			SelfCleaning: "Rateizzazione concessa; pagamenti in corso regolarmente.",
		},
	}, a.access)); err != nil {
		t.Fatalf("PutDeclaration: %v", err)
	}
	afterChange := a.preview(t, b)
	if afterChange.Ready {
		t.Error("a changed declaration must invalidate the confirmation")
	}
	if !afterChange.Declarations.Complete || afterChange.Declarations.Confirmed {
		t.Errorf("the set is still complete but no longer confirmed: %+v", afterChange.Declarations)
	}
	_, err = a.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID, Version: "edm_2_1_1", Format: "xml",
	}, a.access))
	if codeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("export with a stale confirmation = %v, want failed_precondition", err)
	}

	// Re-confirming re-opens it, and the new document carries the new answer.
	if _, err := a.espd.ConfirmDeclarations(ctx, authed(&espdv1.ConfirmDeclarationsRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
	}, a.access)); err != nil {
		t.Fatalf("re-confirm: %v", err)
	}
	after, err := a.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID, Version: "edm_2_1_1", Format: "pdf", Locale: "it-it",
	}, a.access))
	if err != nil {
		t.Fatalf("export after re-confirming: %v", err)
	}
	if !strings.Contains(string(after.Msg.Content), "Rateizzazione") {
		t.Error("the re-exported document does not carry the changed declaration")
	}
	if after.Msg.Export.ContentSha256 == exported[2].Export.ContentSha256 {
		t.Error("a changed document produced the same content hash")
	}
}

// TestEspdImportRequestNarrowsTheDocument: importing the buyer's request fills
// Part I from the buyer's own words and reports which criteria they asked for.
func TestEspdImportRequest(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	a := s.signUp(t, "espd-import")
	b := a.trackBid(t, "espd-import", "515151")

	raw := []byte(`<QualificationApplicationRequest xmlns="urn:oasis:names:specification:ubl:schema:xsd:QualificationApplicationRequest-2"
  xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:VersionID>2.1.1</cbc:VersionID>
  <cbc:ContractFolderID>CIG-IMPORTED-1</cbc:ContractFolderID>
  <cac:ContractingParty><cac:Party>
    <cac:PartyName><cbc:Name>Provincia di Lodi</cbc:Name></cac:PartyName>
    <cac:PostalAddress><cac:Country><cbc:IdentificationCode>IT</cbc:IdentificationCode></cac:Country></cac:PostalAddress>
  </cac:Party></cac:ContractingParty>
  <cac:ProcurementProject><cbc:Name>Servizi di pulizia</cbc:Name></cac:ProcurementProject>
  <cac:ProcurementProjectLot><cbc:ID>LOT-0001</cbc:ID></cac:ProcurementProjectLot>
  <cac:TenderingCriterion>
    <cbc:ID>297d2323-3ede-424e-94bc-a91561e6f320</cbc:ID>
    <cbc:CriterionTypeCode>CRITERION.EXCLUSION.CONVICTIONS.FRAUD</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
  <cac:TenderingCriterion>
    <cbc:ID>9999</cbc:ID>
    <cbc:CriterionTypeCode>CRITERION.SELECTION.SOMETHING.WE_DO_NOT_KNOW</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
</QualificationApplicationRequest>`)

	imported, err := a.espd.ImportRequest(ctx, authed(&espdv1.ImportRequestRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID, Xml: raw,
	}, a.access))
	if err != nil {
		t.Fatalf("ImportRequest: %v", err)
	}
	got := imported.Msg.Request
	if got.BuyerName != "Provincia di Lodi" || got.ProcedureReference != "CIG-IMPORTED-1" {
		t.Errorf("Part I was not read from the request: %+v", got)
	}
	if got.ImportedBy != a.userID || got.ImportedAt == "" {
		t.Errorf("the import must be stamped: %+v", got)
	}
	if len(got.Criteria) != 1 || got.Criteria[0] != string(espd.CritFraud) {
		t.Errorf("mapped criteria = %v", got.Criteria)
	}
	// A criterion we cannot map is REPORTED, never dropped: a buyer asking for
	// something we do not model must be visible, not invisible.
	if len(got.UnmappedCriteria) != 1 {
		t.Errorf("unmapped criteria = %v", got.UnmappedCriteria)
	}

	// The preview now composes against the request: Part I comes from the
	// buyer, and the lot the buyer declared is a gap until the bid names it.
	p := a.preview(t, b)
	if !p.RequestKnown || p.Request.ProcedureReference != "CIG-IMPORTED-1" {
		t.Fatalf("the imported request did not reach the preview: %+v", p.Request)
	}
	lotGap := false
	for _, g := range p.Gaps {
		if g.Criterion == string(espd.CritLots) {
			lotGap = true
			if g.Scope != "bid" {
				t.Errorf("a lot gap is fixed on the bid: %+v", g)
			}
		}
	}
	if !lotGap {
		t.Error("a request that declares lots and a bid that names none must gap")
	}

	// Naming the lot closes it.
	if _, err := a.bid.PutLot(ctx, authed(&bidv1.PutLotRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
		Lot: &bidv1.Lot{LotRef: "LOT-0001", Position: 1},
	}, a.access)); err != nil {
		t.Fatalf("PutLot: %v", err)
	}
	for _, g := range a.preview(t, b).Gaps {
		if g.Criterion == string(espd.CritLots) {
			t.Errorf("the lot gap survived naming the lot: %+v", g)
		}
	}
}

// TestEspdRejectsAnUnreadableRequest: a request we cannot parse is refused with
// a bad-input code, not stored and not half-read.
func TestEspdRejectsAnUnreadableRequest(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "espd-bad-request")
	b := a.trackBid(t, "espd-bad-request", "626262")

	for name, raw := range map[string][]byte{
		"not xml":    []byte("{}"),
		"ESPD-EDM 1": []byte(`<ESPDRequest><VersionID>1.0.2</VersionID></ESPDRequest>`),
		"wrong root": []byte(`<QualificationApplicationResponse><VersionID>2.1.1</VersionID></QualificationApplicationResponse>`),
	} {
		_, err := a.espd.ImportRequest(context.Background(), authed(&espdv1.ImportRequestRequest{
			WorkbenchId: b.workbenchID, BidId: b.bidID, Xml: raw,
		}, a.access))
		if codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want invalid_argument", name, err)
		}
	}
	if p := a.preview(t, b); p.RequestKnown {
		t.Error("a refused import must not be stored")
	}
}

// TestEspdIsWorkbenchScoped: the document belongs to a bid, which lives in a
// workbench, so a stranger gets the workbench's refusal and learns nothing
// about the dossier behind it.
func TestEspdIsWorkbenchScoped(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	owner := s.signUp(t, "espd-owner")
	stranger := s.signUp(t, "espd-stranger")
	b := owner.trackBid(t, "espd-owner", "737373")
	owner.fillDossier(t, b.workspaceID)

	for name, call := range map[string]func() error{
		"preview": func() error {
			_, err := stranger.espd.GetResponsePreview(ctx, authed(&espdv1.GetResponsePreviewRequest{
				WorkbenchId: b.workbenchID, BidId: b.bidID,
			}, stranger.access))
			return err
		},
		"confirm": func() error {
			_, err := stranger.espd.ConfirmDeclarations(ctx, authed(&espdv1.ConfirmDeclarationsRequest{
				WorkbenchId: b.workbenchID, BidId: b.bidID,
			}, stranger.access))
			return err
		},
		"export": func() error {
			_, err := stranger.espd.ExportResponse(ctx, authed(&espdv1.ExportResponseRequest{
				WorkbenchId: b.workbenchID, BidId: b.bidID, Version: "edm_2_1_1", Format: "xml",
			}, stranger.access))
			return err
		},
		"list": func() error {
			_, err := stranger.espd.ListExports(ctx, authed(&espdv1.ListExportsRequest{
				WorkbenchId: b.workbenchID, BidId: b.bidID,
			}, stranger.access))
			return err
		},
	} {
		err := call()
		if code := codeOf(err); code != connect.CodeNotFound && code != connect.CodePermissionDenied {
			t.Errorf("%s by a stranger = %v (code %v), want not_found or permission_denied", name, err, code)
		}
	}

	// Anonymous callers get nowhere at all.
	if _, err := s.anon.espd.GetResponsePreview(ctx, connect.NewRequest(&espdv1.GetResponsePreviewRequest{
		WorkbenchId: b.workbenchID, BidId: b.bidID,
	})); codeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("anonymous preview = %v, want unauthenticated", err)
	}
}

// TestEspdDeclarationsCannotBeImported pins the provenance wall at the
// TRANSPORT: whatever a client claims about where a Part III answer came from,
// the server records it as stated by that user.
func TestEspdDeclarationsCannotBeImported(t *testing.T) {
	s := newStack(t)
	a := s.signUp(t, "espd-provenance")
	b := a.trackBid(t, "espd-provenance", "848484")

	out, err := a.company.PutDeclaration(context.Background(), authed(&companyv1.PutDeclarationRequest{
		WorkspaceId: b.workspaceID,
		Declaration: &companyv1.Declaration{
			Criterion: string(espd.CritFraud),
			Attribution: &companyv1.Attribution{
				Provenance: "agent_inferred", Confidence: 0.99, ConfidenceSet: true,
				StatedBy: "somebody-else",
			},
		},
	}, a.access))
	if err != nil {
		t.Fatalf("PutDeclaration: %v", err)
	}
	got := out.Msg.Declaration.Attribution
	if got.Provenance != "user_stated" || got.StatedBy != a.userID || got.ConfidenceSet {
		t.Errorf("a forged provenance survived the write: %+v", got)
	}
}
