package connectapi_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// testCompanyHandler builds a handler over a service with nil ports. Every
// guard exercised below — requireUser, the tender_id parse, the payload and
// date validation — returns before any port is dereferenced, which is itself
// the property under test: a malformed request must never reach the domain, let
// alone the database.
func testCompanyHandler() *connectapi.CompanyHandler {
	return connectapi.NewCompanyHandler(company.NewService(nil, nil, nil, nil, nil))
}

func wantCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != want {
		t.Fatalf("err = %v, want code %v", err, want)
	}
}

func TestCompanyHandler_RejectsUnauthenticated(t *testing.T) {
	h := testCompanyHandler()
	ctx := context.Background()

	_, err := h.GetCompanyDossier(ctx, connect.NewRequest(&companyv1.GetCompanyDossierRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)

	_, err = h.CheckEligibility(ctx, connect.NewRequest(&companyv1.CheckEligibilityRequest{WorkspaceId: "ws-1", TenderId: "42"}))
	wantCode(t, err, connect.CodeUnauthenticated)

	// A write with a payload the handler would otherwise reject: authentication
	// must be answered FIRST, so an anonymous caller never learns whether its
	// arguments were well-formed.
	_, err = h.PutSoaCategory(ctx, connect.NewRequest(&companyv1.PutSoaCategoryRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)

	_, err = h.ListTenderRequirements(ctx, connect.NewRequest(&companyv1.ListTenderRequirementsRequest{WorkspaceId: "ws-1", TenderId: "42"}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

func TestCheckEligibility_RejectsUnparseableTenderID(t *testing.T) {
	h := testCompanyHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	_, err := h.CheckEligibility(ctx, connect.NewRequest(&companyv1.CheckEligibilityRequest{
		WorkspaceId: "ws-1", TenderId: "not-a-number",
	}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestListTenderRequirements_RejectsUnparseableTenderID(t *testing.T) {
	h := testCompanyHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	_, err := h.ListTenderRequirements(ctx, connect.NewRequest(&companyv1.ListTenderRequirementsRequest{
		WorkspaceId: "ws-1", TenderId: "545620-2026",
	}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

// TestPutRecords_RejectMissingPayload covers the shape a proto3 client produces
// by simply not setting the message: a nil sub-message must be an argument
// error, not a nil dereference three layers down.
func TestPutRecords_RejectMissingPayload(t *testing.T) {
	h := testCompanyHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	_, err := h.PutSoaCategory(ctx, connect.NewRequest(&companyv1.PutSoaCategoryRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)

	_, err = h.PutCertification(ctx, connect.NewRequest(&companyv1.PutCertificationRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)

	_, err = h.PutFinancialYear(ctx, connect.NewRequest(&companyv1.PutFinancialYearRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)

	_, err = h.PutPastContract(ctx, connect.NewRequest(&companyv1.PutPastContractRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)

	_, err = h.PutRegistration(ctx, connect.NewRequest(&companyv1.PutRegistrationRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestPutSoaCategory_RejectsUnknownClassifica(t *testing.T) {
	h := testCompanyHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	_, err := h.PutSoaCategory(ctx, connect.NewRequest(&companyv1.PutSoaCategoryRequest{
		WorkspaceId: "ws-1",
		Soa:         &companyv1.SoaCategory{Category: "OG1", Classifica: "terza"},
	}))
	wantCode(t, err, connect.CodeInvalidArgument)
}

// TestPutCertification_RejectsMalformedDate is the transport half of the rule
// that a stated expiry is never silently dropped: a certificate whose
// valid_until vanished reads as one that never lapses.
func TestPutCertification_RejectsMalformedDate(t *testing.T) {
	h := testCompanyHandler()
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	_, err := h.PutCertification(ctx, connect.NewRequest(&companyv1.PutCertificationRequest{
		WorkspaceId:   "ws-1",
		Certification: &companyv1.Certification{Standard: "iso_9001", ValidUntil: "2027-06-30"},
	}))
	wantCode(t, err, connect.CodeInvalidArgument)
}
