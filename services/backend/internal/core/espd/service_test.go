package espd_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/featurelayer/entitlement"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// ── Fakes ───────────────────────────────────────────────────────────────────

type fakeCompany struct {
	dossier company.Dossier
	err     error
}

func (f *fakeCompany) GetDossier(_ context.Context, workspaceID string) (company.Dossier, error) {
	if f.err != nil {
		return company.Dossier{}, f.err
	}
	d := f.dossier
	d.WorkspaceID = workspaceID
	return d, nil
}

type fakeBids struct {
	bid           bid.Bid
	data          bid.EspdData
	confirmation  *bid.DeclarationConfirmation
	findErr       error
	confirmations []bid.DeclarationConfirmation
}

func (f *fakeBids) FindBidByID(_ context.Context, workbenchID, bidID string) (bid.Bid, error) {
	if f.findErr != nil {
		return bid.Bid{}, f.findErr
	}
	if f.bid.WorkbenchID != workbenchID || f.bid.ID != bidID {
		return bid.Bid{}, bid.ErrBidNotFound
	}
	return f.bid, nil
}

func (f *fakeBids) ListEspdData(context.Context, string) (bid.EspdData, error) { return f.data, nil }

func (f *fakeBids) GetDeclarationConfirmation(_ context.Context, bidID string) (bid.DeclarationConfirmation, error) {
	if f.confirmation == nil {
		return bid.DeclarationConfirmation{}, bid.ErrConfirmationNotFound
	}
	return *f.confirmation, nil
}

func (f *fakeBids) PutDeclarationConfirmation(_ context.Context, c bid.DeclarationConfirmation) (bid.DeclarationConfirmation, error) {
	f.confirmations = append(f.confirmations, c)
	f.confirmation = &c
	return c, nil
}

type fakeRequests struct {
	req *espd.Request
	raw []byte
}

func (f *fakeRequests) Get(context.Context, string) (espd.Request, error) {
	if f.req == nil {
		return espd.Request{}, espd.ErrRequestNotFound
	}
	return *f.req, nil
}

func (f *fakeRequests) Put(_ context.Context, _ string, r espd.Request, raw []byte) error {
	f.req, f.raw = &r, raw
	return nil
}

type fakeExports struct{ rows []espd.Export }

func (f *fakeExports) Record(_ context.Context, e espd.Export) error {
	e.ID = "exp-" + e.ContentSHA256[:6]
	f.rows = append(f.rows, e)
	return nil
}

func (f *fakeExports) List(context.Context, string) ([]espd.Export, error) { return f.rows, nil }

type fakeAccess struct {
	accessErr, manageErr error
	workspace            string
}

func (f fakeAccess) CanAccessWorkbench(context.Context, string, string) error { return f.accessErr }
func (f fakeAccess) CanManageWorkbench(context.Context, string, string) error { return f.manageErr }
func (f fakeAccess) WorkspaceOf(context.Context, string) (string, error)      { return f.workspace, nil }

type fakeTenders struct{ detail tender.TenderDetail }

func (f fakeTenders) GetTender(_ context.Context, p tender.GetTenderParams) (tender.TenderDetail, error) {
	d := f.detail
	d.ID = p.ID
	return d, nil
}

// fakeSerializer records what it was handed and returns bytes derived from it,
// so a test can tell the XML and PDF paths apart by their content.
type fakeSerializer struct {
	version espd.Version
	err     error
	calls   int
}

func (f *fakeSerializer) Version() espd.Version { return f.version }

func (f *fakeSerializer) Serialize(r espd.Response) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return []byte("xml:" + string(f.version) + ":" + r.Declarations.Hash), nil
}

type fakeRenderer struct{ calls int }

func (f *fakeRenderer) Render(r espd.Response, opts espd.RenderOptions) ([]byte, error) {
	f.calls++
	return []byte("pdf:" + opts.Locale + ":" + r.Declarations.Hash), nil
}

// ── Wiring ──────────────────────────────────────────────────────────────────

const (
	testUser   = "11111111-1111-1111-1111-111111111111"
	testWS     = "22222222-2222-2222-2222-222222222222"
	testWB     = "33333333-3333-3333-3333-333333333333"
	testBidID  = "44444444-4444-4444-4444-444444444444"
	testTender = int64(42)
)

type harness struct {
	svc       *espd.Service
	companies *fakeCompany
	bids      *fakeBids
	requests  *fakeRequests
	exports   *fakeExports
	edm211    *fakeSerializer
	edm4      *fakeSerializer
	renderer  *fakeRenderer
}

// newHarness wires the service over fakes, with a subscription on the plan the
// test names. A nil plan means no subscription at all — the shape a workspace
// has before anything was ever seeded.
func newHarness(t *testing.T, plan entitlement.PlanID, access fakeAccess) *harness {
	t.Helper()
	subs := entitlement.NewMemSubscriptions()
	if plan != "" {
		subs.Set(entitlement.Subscription{TenantID: testWS, Plan: plan, BillingAnchor: now})
	}
	engine, err := features.New(subs, entitlement.NewMemUsage())
	if err != nil {
		t.Fatalf("features.New: %v", err)
	}

	d := readyDossier()
	h := &harness{
		companies: &fakeCompany{dossier: d},
		bids: &fakeBids{
			bid:  bid.Bid{ID: testBidID, WorkbenchID: testWB, TenderID: testTender},
			data: readyData(),
			confirmation: &bid.DeclarationConfirmation{
				BidID: testBidID, UserID: testUser, ConfirmedAt: now.Add(-time.Hour),
				DeclarationsHash: espd.HashDeclarations(d.Declarations),
			},
		},
		requests: &fakeRequests{},
		exports:  &fakeExports{},
		edm211:   &fakeSerializer{version: espd.EDM211},
		edm4:     &fakeSerializer{version: espd.EDM4},
		renderer: &fakeRenderer{},
	}
	if access.workspace == "" {
		access.workspace = testWS
	}
	svc, err := espd.NewService(
		h.companies, h.bids, h.requests, h.exports, access,
		fakeTenders{detail: tender.TenderDetail{
			BuyerName: "Comune di Milano", Title: "Manutenzione strade",
			SourceRef: "A0B1C2D3E4", PublicationNumber: "2026/S 100-123456", Country: "IT",
		}},
		engine,
		[]espd.Serializer{h.edm211, h.edm4}, h.renderer,
	)
	if err != nil {
		t.Fatalf("espd.NewService: %v", err)
	}
	h.svc = svc
	return h
}

// ── Constructor ─────────────────────────────────────────────────────────────

func TestNewServiceRejectsDuplicateAndUnknownSerializers(t *testing.T) {
	engine, err := features.New(entitlement.NewMemSubscriptions(), entitlement.NewMemUsage())
	if err != nil {
		t.Fatal(err)
	}
	build := func(sers ...espd.Serializer) error {
		_, err := espd.NewService(&fakeCompany{}, &fakeBids{}, &fakeRequests{}, &fakeExports{},
			fakeAccess{workspace: testWS}, fakeTenders{}, engine, sers, &fakeRenderer{})
		return err
	}
	if err := build(&fakeSerializer{version: espd.EDM211}, &fakeSerializer{version: espd.EDM211}); err == nil {
		t.Error("two serializers for one version must not silently overwrite one another")
	}
	if err := build(&fakeSerializer{version: espd.Version("edm_9")}); err == nil {
		t.Error("a serializer for an unknown version must be rejected")
	}
}

// ── Authorization ───────────────────────────────────────────────────────────

// TestAuthorizationFollowsTheWorkbench pins the read/write split and, more
// importantly, that this domain never invents its own gate: a preview needs
// workbench ACCESS, everything that writes or produces a file needs MANAGE.
func TestAuthorizationFollowsTheWorkbench(t *testing.T) {
	ctx := context.Background()
	denied := errors.New("forbidden")

	viewer := newHarness(t, features.PlanPro, fakeAccess{manageErr: denied})
	if _, err := viewer.svc.Preview(ctx, testUser, testWB, testBidID); err != nil {
		t.Errorf("a viewer must be able to preview: %v", err)
	}
	for name, call := range map[string]func() error{
		"confirm": func() error {
			_, err := viewer.svc.ConfirmDeclarations(ctx, testUser, testWB, testBidID)
			return err
		},
		"export": func() error {
			_, err := viewer.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it")
			return err
		},
		"import": func() error {
			_, err := viewer.svc.ImportRequest(ctx, testUser, testWB, testBidID, []byte("<x/>"))
			return err
		},
	} {
		if err := call(); !errors.Is(err, denied) {
			t.Errorf("%s by a viewer = %v, want the workbench's refusal", name, err)
		}
	}

	stranger := newHarness(t, features.PlanPro, fakeAccess{accessErr: denied, manageErr: denied})
	if _, err := stranger.svc.Preview(ctx, testUser, testWB, testBidID); !errors.Is(err, denied) {
		t.Errorf("preview by a stranger = %v, want the workbench's refusal", err)
	}
	if _, err := stranger.svc.ListExports(ctx, testUser, testWB, testBidID); !errors.Is(err, denied) {
		t.Errorf("list by a stranger = %v", err)
	}
}

// ── Entitlement ─────────────────────────────────────────────────────────────

// TestExportIsPlanGatedAndPreviewIsNot is the product decision made mechanical:
// a free workspace sees its whole document and its gaps, and cannot download
// the artefact.
func TestExportIsPlanGatedAndPreviewIsNot(t *testing.T) {
	ctx := context.Background()

	free := newHarness(t, features.PlanFree, fakeAccess{})
	resp, err := free.svc.Preview(ctx, testUser, testWB, testBidID)
	if err != nil {
		t.Fatalf("a free workspace must be able to preview: %v", err)
	}
	if !resp.Ready() {
		t.Fatalf("fixture not ready: %+v", resp.Gaps)
	}
	_, err = free.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it")
	if !errors.Is(err, espd.ErrNotEntitled) {
		t.Fatalf("free export = %v, want ErrNotEntitled", err)
	}
	var notEntitled *espd.NotEntitledError
	if !errors.As(err, &notEntitled) || notEntitled.Reason == "" {
		t.Errorf("the refusal must carry featurelayer's reason, got %v", err)
	}
	if len(free.exports.rows) != 0 || free.edm211.calls != 0 {
		t.Error("a refused export must neither serialize nor write an audit row")
	}

	pro := newHarness(t, features.PlanPro, fakeAccess{})
	if _, err := pro.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it"); err != nil {
		t.Fatalf("pro export: %v", err)
	}
}

// TestExportWithoutASubscriptionIsRefused: a workspace whose subscription row
// was never seeded must not fall through to "allowed".
func TestExportWithoutASubscriptionIsRefused(t *testing.T) {
	h := newHarness(t, "", fakeAccess{})
	_, err := h.svc.Export(context.Background(), testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it")
	if !errors.Is(err, espd.ErrNotEntitled) {
		t.Fatalf("export with no subscription = %v, want ErrNotEntitled", err)
	}
}

// ── Export ──────────────────────────────────────────────────────────────────

// TestExportRefusesAnIncompleteDocument: an ESPD with a blank field is not a
// draft, it is an incomplete legal declaration.
func TestExportRefusesAnIncompleteDocument(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, features.PlanPro, fakeAccess{})

	// Drop one declaration: the set is no longer complete.
	h.companies.dossier.Declarations = h.companies.dossier.Declarations[1:]
	_, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it")
	if !errors.Is(err, espd.ErrNotReady) {
		t.Fatalf("export = %v, want ErrNotReady", err)
	}
	var notReady *espd.NotReadyError
	if !errors.As(err, &notReady) || len(notReady.Gaps) == 0 {
		t.Errorf("the refusal must carry the gaps, got %v", err)
	}
	if len(h.exports.rows) != 0 {
		t.Error("a refused export must not write an audit row")
	}
}

// TestExportRefusesAStaleConfirmation: changing an answer after confirming
// must block the export until someone re-reads and re-confirms.
func TestExportRefusesAStaleConfirmation(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, features.PlanPro, fakeAccess{})
	h.companies.dossier.Declarations[2].Answer = true
	h.companies.dossier.Declarations[2].SelfCleaning = "misure adottate"

	if _, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it"); !errors.Is(err, espd.ErrNotReady) {
		t.Fatalf("export after a change = %v, want ErrNotReady", err)
	}

	// Re-confirming rebinds to the new content and unblocks it.
	if _, err := h.svc.ConfirmDeclarations(ctx, testUser, testWB, testBidID); err != nil {
		t.Fatalf("ConfirmDeclarations: %v", err)
	}
	if _, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it"); err != nil {
		t.Fatalf("export after re-confirming: %v", err)
	}
}

// TestExportRoutesToTheRightWriterAndRecordsTheFact.
func TestExportRoutesToTheRightWriterAndRecordsTheFact(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, features.PlanPro, fakeAccess{})

	xml211, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatXML, "it")
	if err != nil {
		t.Fatalf("export 2.1.1: %v", err)
	}
	if !strings.HasPrefix(string(xml211.Content), "xml:edm_2_1_1:") || h.edm4.calls != 0 {
		t.Errorf("the 2.1.1 request reached the wrong writer: %q", xml211.Content)
	}
	if xml211.MIMEType != "application/xml" || !strings.HasSuffix(xml211.Filename, "-edm2.1.1.xml") {
		t.Errorf("filename/mime = %q %q", xml211.Filename, xml211.MIMEType)
	}
	// The file is named after the PROCEDURE, which is how a person finds it
	// again next to the rest of that gara's paperwork.
	if !strings.Contains(xml211.Filename, "a0b1c2d3e4") {
		t.Errorf("filename does not carry the procedure reference: %q", xml211.Filename)
	}

	if _, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM4, espd.FormatXML, "it"); err != nil {
		t.Fatalf("export 4.x: %v", err)
	}
	pdfOut, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.FormatPDF, "it-it")
	if err != nil {
		t.Fatalf("export pdf: %v", err)
	}
	if !strings.HasPrefix(string(pdfOut.Content), "pdf:it-it:") || pdfOut.MIMEType != "application/pdf" {
		t.Errorf("pdf export = %q / %q", pdfOut.Content, pdfOut.MIMEType)
	}

	if len(h.exports.rows) != 3 {
		t.Fatalf("%d audit rows for 3 exports", len(h.exports.rows))
	}
	for _, row := range h.exports.rows {
		if row.ContentSHA256 == "" || row.UserID != testUser || row.BidID != testBidID {
			t.Errorf("audit row is incomplete: %+v", row)
		}
		if row.DeclarationsConfirmedAt.IsZero() {
			t.Error("an export must record the confirmation it rested on")
		}
	}
	// Different documents, different hashes — the property the audit rests on.
	if h.exports.rows[0].ContentSHA256 == h.exports.rows[2].ContentSHA256 {
		t.Error("an XML and a PDF export recorded the same content hash")
	}
}

func TestExportRejectsUnknownVersionAndFormat(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, features.PlanPro, fakeAccess{})
	if _, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.Version("edm_9"), espd.FormatXML, ""); !errors.Is(err, espd.ErrInvalidArgument) {
		t.Errorf("unknown version = %v", err)
	}
	if _, err := h.svc.Export(ctx, testUser, testWB, testBidID, espd.EDM211, espd.Format("docx"), ""); !errors.Is(err, espd.ErrInvalidArgument) {
		t.Errorf("unknown format = %v", err)
	}
}

// ── Confirmation ────────────────────────────────────────────────────────────

// TestConfirmDeclarationsRefusesAnIncompleteSet: confirming answers that do not
// exist would produce a signed statement about unanswered questions.
func TestConfirmDeclarationsRefusesAnIncompleteSet(t *testing.T) {
	h := newHarness(t, features.PlanPro, fakeAccess{})
	h.companies.dossier.Declarations = nil
	_, err := h.svc.ConfirmDeclarations(context.Background(), testUser, testWB, testBidID)
	if !errors.Is(err, espd.ErrInvalidArgument) {
		t.Fatalf("confirm with no answers = %v, want ErrInvalidArgument", err)
	}
	if len(h.bids.confirmations) != 0 {
		t.Error("a refused confirmation must not be written")
	}
}

func TestConfirmDeclarationsBindsToTheCurrentContent(t *testing.T) {
	h := newHarness(t, features.PlanPro, fakeAccess{})
	h.bids.confirmation = nil
	resp, err := h.svc.ConfirmDeclarations(context.Background(), testUser, testWB, testBidID)
	if err != nil {
		t.Fatalf("ConfirmDeclarations: %v", err)
	}
	if !resp.Declarations.Confirmed() {
		t.Error("the returned response must show the set as confirmed")
	}
	if got := h.bids.confirmations[0].DeclarationsHash; got != espd.HashDeclarations(h.companies.dossier.Declarations) {
		t.Errorf("the confirmation bound to %q, not to the current declarations", got)
	}
	if h.bids.confirmations[0].UserID != testUser {
		t.Error("the confirmation must record who confirmed")
	}
}

// ── Import ──────────────────────────────────────────────────────────────────

func TestImportRequestStoresTheRawBytesAndStamps(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, features.PlanPro, fakeAccess{})
	raw := []byte(`<QualificationApplicationRequest xmlns="urn:x"><VersionID>2.1.1</VersionID>` +
		`<ContractFolderID>CIG123</ContractFolderID>` +
		`<ContractingParty><Party><PartyName><Name>Comune</Name></PartyName></Party></ContractingParty>` +
		`</QualificationApplicationRequest>`)

	req, err := h.svc.ImportRequest(ctx, testUser, testWB, testBidID, raw)
	if err != nil {
		t.Fatalf("ImportRequest: %v", err)
	}
	if req.ImportedBy != testUser || req.ImportedAt.IsZero() {
		t.Errorf("the import must be stamped: %+v", req)
	}
	if string(h.requests.raw) != string(raw) {
		t.Error("the RAW bytes must be stored, not just the parsed struct")
	}

	// And the preview now composes against it.
	resp, err := h.svc.Preview(ctx, testUser, testWB, testBidID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if resp.Request == nil || resp.Request.ProcedureReference != "CIG123" {
		t.Errorf("the stored request did not reach the preview: %+v", resp.Request)
	}
}

func TestImportRequestRejectsRubbish(t *testing.T) {
	h := newHarness(t, features.PlanPro, fakeAccess{})
	if _, err := h.svc.ImportRequest(context.Background(), testUser, testWB, testBidID, []byte("not xml")); err == nil {
		t.Fatal("an unparseable request must be refused")
	}
	if h.requests.req != nil {
		t.Error("a refused import must not be stored")
	}
}

// ── Preview ─────────────────────────────────────────────────────────────────

// TestPreviewSurvivesAMissingDossierAndTender: a workspace that has asserted
// nothing still gets a document full of gaps rather than an error, because the
// gaps ARE the answer to "what do I still need".
func TestPreviewSurvivesAMissingDossier(t *testing.T) {
	h := newHarness(t, features.PlanPro, fakeAccess{})
	h.companies.err = company.ErrDossierNotFound
	resp, err := h.svc.Preview(context.Background(), testUser, testWB, testBidID)
	if err != nil {
		t.Fatalf("Preview with no dossier: %v", err)
	}
	if resp.Ready() || len(resp.Gaps) == 0 {
		t.Error("an empty dossier must compose to a document of gaps")
	}
}

func TestPreviewPropagatesAMissingBid(t *testing.T) {
	h := newHarness(t, features.PlanPro, fakeAccess{})
	h.bids.findErr = bid.ErrBidNotFound
	if _, err := h.svc.Preview(context.Background(), testUser, testWB, testBidID); !errors.Is(err, bid.ErrBidNotFound) {
		t.Fatalf("preview of a missing bid = %v", err)
	}
}
