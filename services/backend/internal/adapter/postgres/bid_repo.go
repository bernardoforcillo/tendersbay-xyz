package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

type BidRepo struct{ db *pg.DB }

func NewBidRepo(db *pg.DB) *BidRepo { return &BidRepo{db: db} }

func (r *BidRepo) CreateBid(ctx context.Context, b bid.Bid) (bid.Bid, error) {
	var row DBBid
	err := r.db.Insert(Bids).
		Row(
			BidWorkbenchID.Val(b.WorkbenchID),
			BidTenderID.Val(b.TenderID),
			BidGoNoGo.Val(string(b.GoNoGo)),
			BidStage.Val(string(b.Stage)),
			bidOutcomeCol(b.Outcome),
			BidCreatedBy.Val(b.CreatedBy),
		).
		Returning(BidID, BidWorkbenchID, BidTenderID, BidGoNoGo, BidStage, BidOutcome, BidCreatedBy, BidCreatedAt, BidUpdatedAt).
		One(ctx, &row)
	if errors.Is(err, pg.ErrUniqueViolation) {
		return bid.Bid{}, bid.ErrBidExists
	}
	if err != nil {
		return bid.Bid{}, err
	}
	return dbBidToDomain(row), nil
}

func (r *BidRepo) FindBidByID(ctx context.Context, workbenchID, bidID string) (bid.Bid, error) {
	var row DBBid
	err := r.db.Select().From(Bids).
		Where(BidID.Eq(bidID), BidWorkbenchID.Eq(workbenchID)).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return bid.Bid{}, bid.ErrBidNotFound
	}
	if err != nil {
		return bid.Bid{}, err
	}
	return dbBidToDomain(row), nil
}

func (r *BidRepo) ListBidsByWorkbench(ctx context.Context, workbenchID string) ([]bid.Bid, error) {
	var rows []DBBid
	err := r.db.Select().From(Bids).
		Where(BidWorkbenchID.Eq(workbenchID)).
		OrderBy(BidCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]bid.Bid, len(rows))
	for i, row := range rows {
		out[i] = dbBidToDomain(row)
	}
	return out, nil
}

func (r *BidRepo) UpdateGoNoGo(ctx context.Context, bidID string, d bid.GoNoGo) (bid.Bid, error) {
	var row DBBid
	err := r.db.Update(Bids).
		Set(BidGoNoGo.Val(string(d)), BidUpdatedAt.Val(time.Now())).
		Where(BidID.Eq(bidID)).
		Returning(BidID, BidWorkbenchID, BidTenderID, BidGoNoGo, BidStage, BidOutcome, BidCreatedBy, BidCreatedAt, BidUpdatedAt).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return bid.Bid{}, bid.ErrBidNotFound
	}
	if err != nil {
		return bid.Bid{}, err
	}
	return dbBidToDomain(row), nil
}

func (r *BidRepo) UpdateStage(ctx context.Context, bidID string, s bid.Stage) (bid.Bid, error) {
	var row DBBid
	err := r.db.Update(Bids).
		Set(BidStage.Val(string(s)), BidUpdatedAt.Val(time.Now())).
		Where(BidID.Eq(bidID)).
		Returning(BidID, BidWorkbenchID, BidTenderID, BidGoNoGo, BidStage, BidOutcome, BidCreatedBy, BidCreatedAt, BidUpdatedAt).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return bid.Bid{}, bid.ErrBidNotFound
	}
	if err != nil {
		return bid.Bid{}, err
	}
	return dbBidToDomain(row), nil
}

func (r *BidRepo) UpdateOutcome(ctx context.Context, bidID string, o bid.Outcome) (bid.Bid, error) {
	var row DBBid
	err := r.db.Update(Bids).
		Set(bidOutcomeCol(o), BidUpdatedAt.Val(time.Now())).
		Where(BidID.Eq(bidID)).
		Returning(BidID, BidWorkbenchID, BidTenderID, BidGoNoGo, BidStage, BidOutcome, BidCreatedBy, BidCreatedAt, BidUpdatedAt).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return bid.Bid{}, bid.ErrBidNotFound
	}
	if err != nil {
		return bid.Bid{}, err
	}
	return dbBidToDomain(row), nil
}

func (r *BidRepo) DeleteBid(ctx context.Context, bidID string) error {
	_, err := r.db.Delete(Bids).Where(BidID.Eq(bidID)).Exec(ctx)
	return err
}

// bidOutcomeCol writes the outcome text, or the SQL DEFAULT keyword (→ NULL,
// the bids.outcome column carries no explicit .Default) when the bid is open
// (Outcome == ""). Mirrors valueMinCol/valueMaxCol in client_profile_repo.go:
// (*Col[string]).Val cannot express a NULL, so SetDefault is the clear path.
func bidOutcomeCol(o bid.Outcome) pg.ColumnValue {
	if o == "" {
		return BidOutcome.SetDefault()
	}
	return BidOutcome.Val(string(o))
}

func dbBidToDomain(row DBBid) bid.Bid {
	outcome := ""
	if row.Outcome != nil {
		outcome = *row.Outcome
	}
	return bid.Bid{
		ID:          row.ID,
		WorkbenchID: row.WorkbenchID,
		TenderID:    row.TenderID,
		GoNoGo:      bid.GoNoGo(row.GoNoGo),
		Stage:       bid.Stage(row.Stage),
		Outcome:     bid.Outcome(outcome),
		CreatedBy:   row.CreatedBy,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
