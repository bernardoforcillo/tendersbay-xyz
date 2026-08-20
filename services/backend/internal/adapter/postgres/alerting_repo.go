package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/alerting"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// AlertingRepo implements alerting.Repo.
//
// The interface is declared in core and satisfied here, never the other way
// round — the same mechanical statement of the dependency rule the other
// adapters make.
type AlertingRepo struct {
	db *pg.DB
}

// NewAlertingRepo builds the repository.
func NewAlertingRepo(db *pg.DB) *AlertingRepo { return &AlertingRepo{db: db} }

var _ alerting.Repo = (*AlertingRepo)(nil)

// dueCandidatesSQL is a COARSE filter on purpose. alerting.Due owns the rule
// about who deserves a mail; this query's only job is to avoid dragging the
// whole bid table into memory, so it excludes the two states that can never
// become due again (closed, submitted) and nothing else. Encoding the bucket
// arithmetic here would put the rule in two places, and SQL is the copy nobody
// would think to check when the thresholds change.
//
// The 30-day horizon is deliberately wider than the widest bucket (14): a bid
// crossing into range between two runs must already be in the result set, and a
// margin costs one comparison on an indexed column.
//
// tender_id has no foreign key — tenders.ingested_tenders is owned by
// services/ingestion — so this is a plain join and a bid whose tender row is
// missing simply does not appear. That is the correct outcome: without a
// deadline there is nothing to count down to.
const dueCandidatesSQL = `
SELECT b.id, w.workspace_id, b.workbench_id, b.tender_id,
       COALESCE(t.title, ''), t.deadline,
       b.go_no_go, b.stage, COALESCE(b.outcome, ''), b.last_reminded_bucket,
       (SELECT count(*) FROM tender_requirements r
         WHERE r.workspace_id = w.workspace_id
           AND r.tender_id = b.tender_id
           AND r.confirmed_by IS NULL) AS unconfirmed
FROM bids b
JOIN workbenches w ON w.id = b.workbench_id
JOIN tenders.ingested_tenders t ON t.id = b.tender_id
WHERE b.outcome IS NULL
  AND b.stage <> 'submitted'
  AND t.deadline IS NOT NULL
  AND t.deadline > now()
  AND t.deadline < now() + interval '30 days'`

// ListDueCandidates satisfies alerting.Repo.
func (r *AlertingRepo) ListDueCandidates(ctx context.Context) ([]alerting.Candidate, error) {
	rows, err := r.db.Query(ctx, dueCandidatesSQL)
	if err != nil {
		return nil, fmt.Errorf("postgres: list reminder candidates: %w", err)
	}
	defer rows.Close()

	var out []alerting.Candidate
	for rows.Next() {
		var c alerting.Candidate
		var decision, stage, outcome string
		if err := rows.Scan(&c.BidID, &c.WorkspaceID, &c.WorkbenchID, &c.TenderID,
			&c.TenderTitle, &c.Deadline, &decision, &stage, &outcome,
			&c.LastRemindedBucket, &c.UnconfirmedRequirements); err != nil {
			return nil, fmt.Errorf("postgres: scan reminder candidate: %w", err)
		}
		c.Decision, c.Stage, c.Outcome = bid.GoNoGo(decision), bid.Stage(stage), bid.Outcome(outcome)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list reminder candidates: %w", err)
	}
	return out, nil
}

// recipientsSQL returns the workbench's members who have not opted out, with
// their unsubscribe token.
//
// The LEFT JOIN plus "opted_out_at IS NULL" is what makes a member with NO
// preferences row a subscriber: they have never been asked, and the first
// reminder is where they are given the chance to say no. An INNER JOIN would
// have silently excluded everyone until a row existed, which reads as "nobody
// wants reminders" and is impossible to distinguish from a broken query.
//
// Members whose token has not been minted yet come back with an empty one and
// are filtered out by the caller — mintTokens runs first, so in practice this
// only happens in the race where a member is added between the two statements,
// and losing one reminder to that is better than mailing a message with no way
// out.
const recipientsSQL = `
SELECT u.id, u.email, u.display_name, u.locale, COALESCE(p.unsubscribe_token, '')
FROM workbench_members m
JOIN users u ON u.id = m.user_id
LEFT JOIN reminder_preferences p ON p.user_id = u.id
WHERE m.workbench_id = $1
  AND (p.opted_out_at IS NULL)
  AND u.email_verified_at IS NOT NULL`

// RecipientsFor satisfies alerting.Repo.
func (r *AlertingRepo) RecipientsFor(ctx context.Context, workbenchID string) ([]alerting.Recipient, error) {
	// Mint one token per member that lacks one. A separate token per user is
	// minted per call rather than in bulk because each needs its own random
	// value — a shared one would let any recipient unsubscribe every other.
	if err := r.mintTokens(ctx, workbenchID); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, recipientsSQL, workbenchID)
	if err != nil {
		return nil, fmt.Errorf("postgres: reminder recipients: %w", err)
	}
	defer rows.Close()

	var out []alerting.Recipient
	for rows.Next() {
		var id string
		var rec alerting.Recipient
		if err := rows.Scan(&id, &rec.Email, &rec.DisplayName, &rec.Locale, &rec.UnsubscribeToken); err != nil {
			return nil, fmt.Errorf("postgres: scan reminder recipient: %w", err)
		}
		// Locale comes from users.locale, captured at signup from the browser's
		// Accept-Language and changeable by the user. It is '' for anyone who
		// signed up before that existed, and the sender falls back to English
		// for those — which is the honest outcome, because nobody told us.
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: reminder recipients: %w", err)
	}
	return out, nil
}

func (r *AlertingRepo) mintTokens(ctx context.Context, workbenchID string) error {
	// One statement per missing member would be a query per user per pass. One
	// statement with a single generated token would hand every member the same
	// one. So: read who is missing, then insert each with its own value.
	rows, err := r.db.Query(ctx,
		`SELECT m.user_id FROM workbench_members m
		 WHERE m.workbench_id = $1
		   AND NOT EXISTS (SELECT 1 FROM reminder_preferences p WHERE p.user_id = m.user_id)`,
		workbenchID)
	if err != nil {
		return fmt.Errorf("postgres: find members without reminder preferences: %w", err)
	}
	var missing []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("postgres: scan member without preferences: %w", err)
		}
		missing = append(missing, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("postgres: find members without reminder preferences: %w", err)
	}

	for _, userID := range missing {
		token, err := newUnsubscribeToken()
		if err != nil {
			return err
		}
		if _, err := r.db.Exec(ctx,
			`INSERT INTO reminder_preferences (user_id, unsubscribe_token)
			 VALUES ($1, $2) ON CONFLICT (user_id) DO NOTHING`,
			userID, token); err != nil {
			return fmt.Errorf("postgres: mint unsubscribe token: %w", err)
		}
	}
	return nil
}

// newUnsubscribeToken returns 32 bytes of crypto/rand, URL-safe. Long enough
// that guessing one to silence a stranger's reminders is not a realistic
// attack, and URL-safe so it survives the query string unescaped.
func newUnsubscribeToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("postgres: generate unsubscribe token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// MarkReminded satisfies alerting.Repo.
func (r *AlertingRepo) MarkReminded(ctx context.Context, bidID string, bucket int) error {
	// LEAST is what makes a re-run idempotent in the right direction: the
	// watermark only ever moves toward the deadline, so an out-of-order pass
	// cannot widen it back and re-arm a bucket that was already sent.
	if _, err := r.db.Exec(ctx,
		`UPDATE bids
		    SET last_reminded_bucket = CASE
		          WHEN last_reminded_bucket = 0 THEN $2
		          ELSE LEAST(last_reminded_bucket, $2) END
		  WHERE id = $1`,
		bidID, bucket); err != nil {
		return fmt.Errorf("postgres: advance reminder watermark: %w", err)
	}
	return nil
}

// OptOut satisfies alerting.Repo.
//
// The WHERE deliberately does NOT filter on opted_out_at IS NULL. A second
// unsubscribe from an already-unsubscribed reader must still report a match, or
// the endpoint would answer "no such token" to someone holding a perfectly valid
// one — and the caller would log a miss that is really a duplicate click.
// COALESCE keeps the original timestamp, so the suppression list records when
// they FIRST said no, which is the date a deliverability complaint is answered
// with.
func (r *AlertingRepo) OptOut(ctx context.Context, token string) (bool, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE reminder_preferences
		    SET opted_out_at = COALESCE(opted_out_at, now())
		  WHERE unsubscribe_token = $1
		  RETURNING user_id`, token)
	if err != nil {
		return false, fmt.Errorf("postgres: record unsubscribe: %w", err)
	}
	defer rows.Close()
	found := rows.Next()
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("postgres: record unsubscribe: %w", err)
	}
	return found, nil
}
