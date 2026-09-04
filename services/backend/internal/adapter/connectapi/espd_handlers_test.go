package connectapi_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	bidv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1"
	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
)

// TestEspdSectionRPCs_RejectUnauthenticatedFirst: the six dossier RPCs and the
// seven per-bid RPCs added for the ESPD answer authentication before they look
// at the payload, like every sibling.
func TestEspdSectionRPCs_RejectUnauthenticatedFirst(t *testing.T) {
	ctx := context.Background()
	c := testCompanyHandler()
	b := testBidHandler()

	_, err := c.PutRepresentative(ctx, connect.NewRequest(&companyv1.PutRepresentativeRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.RemoveRepresentative(ctx, connect.NewRequest(&companyv1.RemoveRepresentativeRequest{WorkspaceId: "ws-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.PutDeclaration(ctx, connect.NewRequest(&companyv1.PutDeclarationRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.RemoveDeclaration(ctx, connect.NewRequest(&companyv1.RemoveDeclarationRequest{WorkspaceId: "ws-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.PutNationalGround(ctx, connect.NewRequest(&companyv1.PutNationalGroundRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = c.RemoveNationalGround(ctx, connect.NewRequest(&companyv1.RemoveNationalGroundRequest{WorkspaceId: "ws-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)

	_, err = b.ListEspdData(ctx, connect.NewRequest(&bidv1.ListEspdDataRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.PutLot(ctx, connect.NewRequest(&bidv1.PutLotRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.RemoveLot(ctx, connect.NewRequest(&bidv1.RemoveLotRequest{WorkbenchId: "wb-1", BidId: "b-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.PutSubcontractor(ctx, connect.NewRequest(&bidv1.PutSubcontractorRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.RemoveSubcontractor(ctx, connect.NewRequest(&bidv1.RemoveSubcontractorRequest{WorkbenchId: "wb-1", BidId: "b-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.PutReliance(ctx, connect.NewRequest(&bidv1.PutRelianceRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeUnauthenticated)
	_, err = b.RemoveReliance(ctx, connect.NewRequest(&bidv1.RemoveRelianceRequest{WorkbenchId: "wb-1", BidId: "b-1", Id: "x"}))
	wantCode(t, err, connect.CodeUnauthenticated)
}

// TestEspdSectionRPCs_RejectMissingPayload: a Put without its record is an
// InvalidArgument at the transport, before the service is reached (the fakes
// here are nil, so reaching it would panic).
func TestEspdSectionRPCs_RejectMissingPayload(t *testing.T) {
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	c := testCompanyHandler()
	b := testBidHandler()

	_, err := c.PutRepresentative(ctx, connect.NewRequest(&companyv1.PutRepresentativeRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = c.PutDeclaration(ctx, connect.NewRequest(&companyv1.PutDeclarationRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = c.PutNationalGround(ctx, connect.NewRequest(&companyv1.PutNationalGroundRequest{WorkspaceId: "ws-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = c.PutRepresentative(ctx, connect.NewRequest(&companyv1.PutRepresentativeRequest{WorkspaceId: "ws-1", Representative: &companyv1.Representative{Role: "r", GivenName: "a", FamilyName: "b", BirthDate: "yesterday"}}))
	wantCode(t, err, connect.CodeInvalidArgument)

	_, err = b.PutLot(ctx, connect.NewRequest(&bidv1.PutLotRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = b.PutSubcontractor(ctx, connect.NewRequest(&bidv1.PutSubcontractorRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = b.PutReliance(ctx, connect.NewRequest(&bidv1.PutRelianceRequest{WorkbenchId: "wb-1", BidId: "b-1"}))
	wantCode(t, err, connect.CodeInvalidArgument)
}
