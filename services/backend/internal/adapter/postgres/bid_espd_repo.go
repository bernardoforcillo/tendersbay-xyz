package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// The per-bid ESPD data (migration 0016). Every method is bid-scoped; the
// service has already resolved the bid through its workbench, so a bid id here
// is one the caller may touch.

// ── Lots ────────────────────────────────────────────────────────────────────

// PutLot upserts on (bid_id, lot_ref).
func (r *BidRepo) PutLot(ctx context.Context, bidID string, l bid.Lot) (bid.Lot, error) {
	var row DBLot
	err := r.db.Insert(BidLots).
		Row(LotBidID.Val(bidID), LotRef.Val(l.LotRef), LotPosition.Val(l.Position)).
		OnConflictUpdate(LotBidID, LotRef).
		Set(LotPosition.Val(l.Position)).
		Done().
		Returning(LotID, LotBidID, LotRef, LotPosition).
		One(ctx, &row)
	if err != nil {
		return bid.Lot{}, err
	}
	return bid.Lot{ID: row.ID, BidID: row.BidID, LotRef: row.LotRef, Position: row.Position}, nil
}

func (r *BidRepo) DeleteLot(ctx context.Context, bidID, id string) error {
	res, err := r.db.Delete(BidLots).Where(LotBidID.Eq(bidID), LotID.Eq(id)).Exec(ctx)
	return bidDeletedOne(res, err)
}

// ── Subcontractors ──────────────────────────────────────────────────────────

// PutSubcontractor upserts on (bid_id, vat).
func (r *BidRepo) PutSubcontractor(ctx context.Context, bidID string, s bid.Subcontractor) (bid.Subcontractor, error) {
	set := []pg.ColumnValue{SubName.Val(s.Name), SubCountry.Val(s.Country), nullableShare(s.Share)}
	var row DBSubcontractor
	err := r.db.Insert(BidSubcontractors).
		Row(append([]pg.ColumnValue{SubBidID.Val(bidID), SubVAT.Val(s.VAT)}, set...)...).
		OnConflictUpdate(SubBidID, SubVAT).
		Set(set...).
		Done().
		Returning(SubID, SubBidID, SubName, SubVAT, SubCountry, SubShare).
		One(ctx, &row)
	if err != nil {
		return bid.Subcontractor{}, err
	}
	return dbSubToDomain(row), nil
}

func (r *BidRepo) DeleteSubcontractor(ctx context.Context, bidID, id string) error {
	res, err := r.db.Delete(BidSubcontractors).Where(SubBidID.Eq(bidID), SubID.Eq(id)).Exec(ctx)
	return bidDeletedOne(res, err)
}

func nullableShare(v *int32) pg.ColumnValue {
	if v == nil {
		return SubShare.SetDefault()
	}
	return SubShare.Val(int16(*v))
}

func dbSubToDomain(row DBSubcontractor) bid.Subcontractor {
	out := bid.Subcontractor{ID: row.ID, BidID: row.BidID, Name: row.Name, VAT: row.VAT, Country: row.Country}
	if row.Share != nil {
		share := int32(*row.Share)
		out.Share = &share
	}
	return out
}

// ── Reliances ───────────────────────────────────────────────────────────────

// PutReliance upserts on (bid_id, vat, criterion).
func (r *BidRepo) PutReliance(ctx context.Context, bidID string, rel bid.Reliance) (bid.Reliance, error) {
	var row DBReliance
	err := r.db.Insert(BidReliances).
		Row(RelBidID.Val(bidID), RelVAT.Val(rel.VAT), RelCriterion.Val(rel.Criterion), RelEntityName.Val(rel.EntityName)).
		OnConflictUpdate(RelBidID, RelVAT, RelCriterion).
		Set(RelEntityName.Val(rel.EntityName)).
		Done().
		Returning(RelID, RelBidID, RelEntityName, RelVAT, RelCriterion).
		One(ctx, &row)
	if err != nil {
		return bid.Reliance{}, err
	}
	return bid.Reliance{ID: row.ID, BidID: row.BidID, EntityName: row.EntityName, VAT: row.VAT, Criterion: row.Criterion}, nil
}

func (r *BidRepo) DeleteReliance(ctx context.Context, bidID, id string) error {
	res, err := r.db.Delete(BidReliances).Where(RelBidID.Eq(bidID), RelID.Eq(id)).Exec(ctx)
	return bidDeletedOne(res, err)
}

// ── The read ────────────────────────────────────────────────────────────────

// ListEspdData loads the three collections for one bid, lots in position
// order, parties by name.
func (r *BidRepo) ListEspdData(ctx context.Context, bidID string) (bid.EspdData, error) {
	var out bid.EspdData

	var lots []DBLot
	if err := r.db.Select().From(BidLots).Where(LotBidID.Eq(bidID)).
		OrderBy(LotPosition.Asc(), LotRef.Asc()).All(ctx, &lots); err != nil {
		return bid.EspdData{}, err
	}
	for _, row := range lots {
		out.Lots = append(out.Lots, bid.Lot{ID: row.ID, BidID: row.BidID, LotRef: row.LotRef, Position: row.Position})
	}

	var subs []DBSubcontractor
	if err := r.db.Select().From(BidSubcontractors).Where(SubBidID.Eq(bidID)).
		OrderBy(SubName.Asc(), SubVAT.Asc()).All(ctx, &subs); err != nil {
		return bid.EspdData{}, err
	}
	for _, row := range subs {
		out.Subcontractors = append(out.Subcontractors, dbSubToDomain(row))
	}

	var rels []DBReliance
	if err := r.db.Select().From(BidReliances).Where(RelBidID.Eq(bidID)).
		OrderBy(RelEntityName.Asc(), RelCriterion.Asc()).All(ctx, &rels); err != nil {
		return bid.EspdData{}, err
	}
	for _, row := range rels {
		out.Reliances = append(out.Reliances, bid.Reliance{ID: row.ID, BidID: row.BidID, EntityName: row.EntityName, VAT: row.VAT, Criterion: row.Criterion})
	}
	return out, nil
}

// ── Declaration confirmation ────────────────────────────────────────────────

// PutDeclarationConfirmation upserts on bid_id: the latest confirmation wins.
func (r *BidRepo) PutDeclarationConfirmation(ctx context.Context, c bid.DeclarationConfirmation) (bid.DeclarationConfirmation, error) {
	set := []pg.ColumnValue{DCUserID.Val(c.UserID), DCConfirmedAt.Val(statedAtOrNow(c.ConfirmedAt)), DCDeclarationsHash.Val(c.DeclarationsHash)}
	var row DBDeclarationConfirmation
	err := r.db.Insert(BidDeclarationConfirmations).
		Row(append([]pg.ColumnValue{DCBidID.Val(c.BidID)}, set...)...).
		OnConflictUpdate(DCBidID).
		Set(set...).
		Done().
		Returning(DCBidID, DCUserID, DCConfirmedAt, DCDeclarationsHash).
		One(ctx, &row)
	if err != nil {
		return bid.DeclarationConfirmation{}, err
	}
	return bid.DeclarationConfirmation{BidID: row.BidID, UserID: row.UserID, ConfirmedAt: row.ConfirmedAt, DeclarationsHash: row.DeclarationsHash}, nil
}

func (r *BidRepo) GetDeclarationConfirmation(ctx context.Context, bidID string) (bid.DeclarationConfirmation, error) {
	var row DBDeclarationConfirmation
	err := r.db.Select().From(BidDeclarationConfirmations).Where(DCBidID.Eq(bidID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return bid.DeclarationConfirmation{}, bid.ErrConfirmationNotFound
	}
	if err != nil {
		return bid.DeclarationConfirmation{}, err
	}
	return bid.DeclarationConfirmation{BidID: row.BidID, UserID: row.UserID, ConfirmedAt: row.ConfirmedAt, DeclarationsHash: row.DeclarationsHash}, nil
}

// bidDeletedOne is deletedOne with the bid domain's sentinel: a zero-row DELETE
// on per-bid data means the record was not there, which the transport answers
// as not-found rather than as success.
func bidDeletedOne(res drops.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return bid.ErrInvalidArgument
	}
	return nil
}
