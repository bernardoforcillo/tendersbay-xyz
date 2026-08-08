package connectapi

import (
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// TestToConnectError_CompanyDomain pins every sentinel core/company owns to a
// status code. It is a table over the whole set rather than a spot check
// because toConnectError's default arm is CodeInternal: a sentinel nobody maps
// does not fail loudly, it 500s — and "the classifica you sent does not exist"
// reported as an outage is a bug the caller cannot act on.
func TestToConnectError_CompanyDomain(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want connect.Code
	}{
		{"dossier not found", company.ErrDossierNotFound, connect.CodeNotFound},
		{"record not found", company.ErrRecordNotFound, connect.CodeNotFound},
		{"invalid argument", company.ErrInvalidArgument, connect.CodeInvalidArgument},
		{"unknown field key", company.ErrUnknownFieldKey, connect.CodeInvalidArgument},
		{"invalid soa category", company.ErrInvalidSOACategory, connect.CodeInvalidArgument},
		{"invalid classifica", company.ErrInvalidClassifica, connect.CodeInvalidArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mapped := toConnectError(c.err)
			var ce *connect.Error
			if !errors.As(mapped, &ce) || ce.Code() != c.want {
				t.Fatalf("toConnectError(%v) = %v, want code %v", c.err, mapped, c.want)
			}
		})
	}
}

// TestToConnectError_CompanyWrappedSentinel guards the wrapping the domain
// actually uses. Every company validator returns
// fmt.Errorf("%w: <what was wrong>", ErrInvalidArgument), so a mapping written
// with == instead of errors.Is would pass the table above and still 500 on
// every real request.
func TestToConnectError_CompanyWrappedSentinel(t *testing.T) {
	wrapped := fmt.Errorf("%w: unknown legal form %q", company.ErrInvalidArgument, "spqr")
	mapped := toConnectError(wrapped)
	var ce *connect.Error
	if !errors.As(mapped, &ce) || ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("toConnectError(wrapped ErrInvalidArgument) = %v, want CodeInvalidArgument", mapped)
	}
}
