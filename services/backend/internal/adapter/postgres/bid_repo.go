package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

type BidRepo struct{ db *pg.DB }

func NewBidRepo(db *pg.DB) *BidRepo { return &BidRepo{db: db} }

var _ bid.Repository = (*BidRepo)(nil)

// bidColumns is the RETURNING projection every write shares. It is one list and
// not four literals because the four had already drifted apart once by omission:
// a column added to the table but forgotten in one RETURNING scans as its zero
// value, and for the decision record that zero value ("no recommendation
// existed", not overridden) is indistinguishable from a real answer.
func bidColumns() []drops.Expression {
	return []drops.Expression{
		BidID, BidWorkbenchID, BidTenderID, BidGoNoGo, BidStage, BidOutcome,
		BidDecisionRecommendation, BidDecisionOverridden, BidDecisionBlockingGaps, BidDecisionRecordedAt,
		BidCreatedBy, BidCreatedAt, BidUpdatedAt,
	}
}

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
		Returning(bidColumns()...).
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

// UpdateGoNoGo writes the decision and the recommendation it was taken against
// in ONE statement, per the port's contract: the assessment is never persisted,
// so a baseline lost to a second failed statement is lost for good.
func (r *BidRepo) UpdateGoNoGo(ctx context.Context, bidID string, d bid.GoNoGo, rec bid.DecisionRecord) (bid.Bid, error) {
	var row DBBid
	err := r.db.Update(Bids).
		Set(
			BidGoNoGo.Val(string(d)),
			BidDecisionRecommendation.Val(string(rec.Recommendation)),
			BidDecisionOverridden.Val(rec.Overridden),
			BidDecisionBlockingGaps.Val(int64(rec.BlockingGapCount)),
			bidDecisionRecordedAtCol(rec.RecordedAt),
			BidUpdatedAt.Val(time.Now()),
		).
		Where(BidID.Eq(bidID)).
		Returning(bidColumns()...).
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
		Returning(bidColumns()...).
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
		Returning(bidColumns()...).
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

// bidDecisionRecordedAtCol writes the decision timestamp, or the SQL DEFAULT
// keyword (→ NULL) when there is none. Same construction and same reason as
// bidOutcomeCol above: (*Col[time.Time]).Val cannot express a NULL, and NULL is
// the honest value for "we do not know when this was decided" on a row written
// before the decision record existed.
func bidDecisionRecordedAtCol(at *time.Time) pg.ColumnValue {
	if at == nil {
		return BidDecisionRecordedAt.SetDefault()
	}
	return BidDecisionRecordedAt.Val(*at)
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
		Decision: bid.DecisionRecord{
			Recommendation:   company.Verdict(row.DecisionRecommendation),
			Overridden:       row.DecisionOverridden,
			BlockingGapCount: int(row.DecisionBlockingGaps),
			RecordedAt:       row.DecisionRecordedAt,
		},
		CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (r *BidRepo) SeedChecklist(ctx context.Context, bidID string, seeds []bid.ChecklistItemSeed) error {
	if len(seeds) == 0 {
		return nil
	}
	rows := make([][]pg.ColumnValue, 0, len(seeds))
	for _, s := range seeds {
		rows = append(rows, []pg.ColumnValue{
			BCIBidID.Val(bidID),
			BCISectionCode.Val(s.SectionCode),
			BCIItemCode.Val(s.ItemCode),
			BCIRequired.Val(s.Required),
			BCIPosition.Val(int64(s.Position)),
		})
	}
	// status ('pending'), note (''), id (gen_random_uuid()) and updated_at
	// (now()) fall to their column defaults.
	_, err := r.db.Insert(BidChecklistItems).Rows(rows...).Exec(ctx)
	return err
}

func (r *BidRepo) ListChecklistItems(ctx context.Context, bidID string) ([]bid.ChecklistItem, error) {
	var rows []DBChecklistItem
	err := r.db.Select().From(BidChecklistItems).
		Where(BCIBidID.Eq(bidID)).
		OrderBy(BCIPosition.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]bid.ChecklistItem, len(rows))
	for i, row := range rows {
		out[i] = dbChecklistToDomain(row)
	}
	return out, nil
}

func (r *BidRepo) UpsertChecklistItem(ctx context.Context, bidID, itemCode, status, note string) (bid.ChecklistItem, error) {
	now := time.Now()
	var row DBChecklistItem
	// The (bid_id, item_code) unique arbiters the upsert. section_code is
	// NOT NULL with no column default, so the INSERT branch binds "" to stay
	// valid; the conflict (UPDATE) branch — the real path, since AddBid always
	// seeds the item — leaves section_code/required/position untouched and
	// RETURNING reflects the seeded values.
	err := r.db.Insert(BidChecklistItems).
		Row(
			BCIBidID.Val(bidID),
			BCIItemCode.Val(itemCode),
			BCISectionCode.Val(""),
			BCIStatus.Val(status),
			BCINote.Val(note),
			BCIUpdatedAt.Val(now),
		).
		OnConflictUpdate(BCIBidID, BCIItemCode).
		Set(BCIStatus.Val(status), BCINote.Val(note), BCIUpdatedAt.Val(now)).
		Done().
		Returning(BCIID, BCIBidID, BCISectionCode, BCIItemCode, BCIStatus, BCINote, BCIRequired, BCIPosition, BCIUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return bid.ChecklistItem{}, err
	}
	return dbChecklistToDomain(row), nil
}

func dbChecklistToDomain(row DBChecklistItem) bid.ChecklistItem {
	return bid.ChecklistItem{
		ID:          row.ID,
		BidID:       row.BidID,
		SectionCode: row.SectionCode,
		ItemCode:    row.ItemCode,
		Status:      row.Status,
		Note:        row.Note,
		Required:    row.Required,
		Position:    int(row.Position),
		UpdatedAt:   row.UpdatedAt,
	}
}
