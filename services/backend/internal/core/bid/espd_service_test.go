package bid

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

func TestEspdData_WritesRequireManageAndUpsertOnNaturalKeys(t *testing.T) {
	svc, repo, access, _, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1"}
	ctx := context.Background()

	lot, err := svc.PutLot(ctx, "u1", "wb1", "b1", Lot{LotRef: " LOT-0001 ", Position: 1})
	if err != nil || lot.LotRef != "LOT-0001" {
		t.Fatalf("PutLot: %+v %v", lot, err)
	}
	again, _ := svc.PutLot(ctx, "u1", "wb1", "b1", Lot{LotRef: "LOT-0001", Position: 2})
	if again.ID != lot.ID {
		t.Errorf("re-put of the same lot_ref created a second row")
	}

	share := int32(30)
	sub, err := svc.PutSubcontractor(ctx, "u1", "wb1", "b1", Subcontractor{Name: "Beta Srl", VAT: "IT01234567890", Country: "it", Share: &share})
	if err != nil || sub.Country != "IT" {
		t.Fatalf("PutSubcontractor: %+v %v", sub, err)
	}
	rel, err := svc.PutReliance(ctx, "u1", "wb1", "b1", Reliance{EntityName: "Gamma Spa", VAT: "IT09876543210", Criterion: "iv.b.general_turnover"})
	if err != nil {
		t.Fatalf("PutReliance: %v", err)
	}

	data, err := svc.ListEspdData(ctx, "u1", "wb1", "b1")
	if err != nil || len(data.Lots) != 1 || len(data.Subcontractors) != 1 || len(data.Reliances) != 1 {
		t.Fatalf("ListEspdData: %+v %v", data, err)
	}

	if err := svc.DeleteReliance(ctx, "u1", "wb1", "b1", rel.ID); err != nil {
		t.Fatalf("DeleteReliance: %v", err)
	}
	if err := svc.DeleteSubcontractor(ctx, "u1", "wb1", "b1", sub.ID); err != nil {
		t.Fatalf("DeleteSubcontractor: %v", err)
	}
	if err := svc.DeleteLot(ctx, "u1", "wb1", "b1", lot.ID); err != nil {
		t.Fatalf("DeleteLot: %v", err)
	}
	data, _ = svc.ListEspdData(ctx, "u1", "wb1", "b1")
	if len(data.Lots)+len(data.Subcontractors)+len(data.Reliances) != 0 {
		t.Errorf("deletes did not empty the data: %+v", data)
	}

	// A viewer reads but does not write.
	access.manageErr["wb1"] = workbench.ErrForbidden
	if _, err := svc.PutLot(ctx, "u1", "wb1", "b1", Lot{LotRef: "LOT-0002"}); !errors.Is(err, workbench.ErrForbidden) {
		t.Errorf("viewer PutLot = %v, want ErrForbidden", err)
	}
	if _, err := svc.ListEspdData(ctx, "u1", "wb1", "b1"); err != nil {
		t.Errorf("viewer ListEspdData = %v, want nil", err)
	}
	// And a bid from another workbench is not reachable through this one.
	access.manageErr["wb1"] = nil
	if _, err := svc.PutLot(ctx, "u1", "wb2", "b1", Lot{LotRef: "LOT-0002"}); !errors.Is(err, ErrBidNotFound) {
		t.Errorf("cross-workbench PutLot = %v, want ErrBidNotFound", err)
	}
}

func TestEspdData_Validation(t *testing.T) {
	svc, repo, _, _, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1"}
	ctx := context.Background()
	over := int32(101)
	cases := map[string]func() error{
		"empty lot ref": func() error { _, err := svc.PutLot(ctx, "u1", "wb1", "b1", Lot{LotRef: "  "}); return err },
		"sub without vat": func() error {
			_, err := svc.PutSubcontractor(ctx, "u1", "wb1", "b1", Subcontractor{Name: "x"})
			return err
		},
		"sub share > 100": func() error {
			_, err := svc.PutSubcontractor(ctx, "u1", "wb1", "b1", Subcontractor{Name: "x", VAT: "1", Share: &over})
			return err
		},
		"sub bad country": func() error {
			_, err := svc.PutSubcontractor(ctx, "u1", "wb1", "b1", Subcontractor{Name: "x", VAT: "1", Country: "Italy"})
			return err
		},
		"reliance no crit": func() error {
			_, err := svc.PutReliance(ctx, "u1", "wb1", "b1", Reliance{EntityName: "x", VAT: "1"})
			return err
		},
		"delete empty id": func() error { return svc.DeleteLot(ctx, "u1", "wb1", "b1", "") },
	}
	for name, run := range cases {
		if err := run(); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}
