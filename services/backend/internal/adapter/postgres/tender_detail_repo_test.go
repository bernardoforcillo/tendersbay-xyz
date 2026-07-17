package postgres_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

func TestFindDetailByID_ReturnsFullRowWithDocumentsLotsAndSecondaryCPV(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()

	id := insertTestTender(t, sqlDB, "detail-1")
	if _, err := sqlDB.Exec(`UPDATE tenders.ingested_tenders SET buyer_id=$2, nuts=$3, language=$4, cpv_secondary=$5 WHERE id=$1`,
		id, "ORG-1", "ITC4", "it", "{72000000,48000000}"); err != nil {
		t.Fatalf("update extras: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO tenders.ingested_tender_documents (tender_id, url, type) VALUES ($1,$2,$3)`,
		id, "https://ted.europa.eu/x.pdf", "notice"); err != nil {
		t.Fatalf("insert document: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO tenders.ingested_tender_lots (tender_id, ref, title, cpv, currency) VALUES ($1,$2,$3,$4,$5)`,
		id, "LOT-1", "First lot", "45000000", "EUR"); err != nil {
		t.Fatalf("insert lot: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_documents WHERE tender_id=$1`, id)
		_, _ = sqlDB.Exec(`DELETE FROM tenders.ingested_tender_lots WHERE tender_id=$1`, id)
	})

	got, err := repo.FindDetailByID(ctx, id)
	if err != nil {
		t.Fatalf("FindDetailByID: %v", err)
	}
	if got.BuyerID != "ORG-1" || got.NUTS != "ITC4" || got.Language != "it" {
		t.Errorf("scalar extras wrong: %+v", got)
	}
	if len(got.CPVSecondary) != 2 || got.CPVSecondary[0] != "72000000" || got.CPVSecondary[1] != "48000000" {
		t.Errorf("cpvSecondary = %v, want [72000000 48000000]", got.CPVSecondary)
	}
	if len(got.Documents) != 1 || got.Documents[0].Type != "notice" {
		t.Errorf("documents = %+v, want one notice", got.Documents)
	}
	if len(got.Lots) != 1 || got.Lots[0].Ref != "LOT-1" {
		t.Errorf("lots = %+v, want one LOT-1", got.Lots)
	}
}

func TestFindDetailByID_EmptySecondaryCPVYieldsNil(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	id := insertTestTender(t, sqlDB, "detail-empty")
	got, err := repo.FindDetailByID(context.Background(), id)
	if err != nil {
		t.Fatalf("FindDetailByID: %v", err)
	}
	if len(got.CPVSecondary) != 0 {
		t.Errorf("cpvSecondary = %v, want empty", got.CPVSecondary)
	}
}

func TestFindDetailByID_NotFoundReturnsSentinel(t *testing.T) {
	repo, _ := testTenderRepo(t)
	_, err := repo.FindDetailByID(context.Background(), 999999999)
	if err != tender.ErrTenderNotFound {
		t.Errorf("err = %v, want ErrTenderNotFound", err)
	}
}

func TestRecentTenderRefs_OrdersByPublishedAtDescAndCaps(t *testing.T) {
	repo, sqlDB := testTenderRepo(t)
	ctx := context.Background()
	_ = insertTestTender(t, sqlDB, "ref-1", withPublishedAt(time.Now().Add(-2*time.Hour)))
	newer := insertTestTender(t, sqlDB, "ref-2", withPublishedAt(time.Now().Add(-1*time.Hour)))

	refs, err := repo.RecentTenderRefs(ctx, 1)
	if err != nil {
		t.Fatalf("RecentTenderRefs: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("len(refs) = %d, want 1 (capped by limit)", len(refs))
	}
	if refs[0].ID != strconv.FormatInt(newer, 10) {
		t.Errorf("refs[0].ID = %s, want newest %d", refs[0].ID, newer)
	}
}
