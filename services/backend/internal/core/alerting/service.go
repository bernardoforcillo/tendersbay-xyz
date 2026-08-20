package alerting

import (
	"context"
	"log/slog"
	"time"
)

// Service runs one reminder pass.
type Service struct {
	repo   Repo
	mailer Mailer
	now    func() time.Time
}

// NewService builds a Service over its ports.
func NewService(repo Repo, mailer Mailer) *Service {
	return &Service{repo: repo, mailer: mailer, now: func() time.Time { return time.Now().UTC() }}
}

// Report is one pass's outcome, for the job's exit status and its log line.
type Report struct {
	Considered int
	Due        int
	Sent       int
	// Suppressed counts reminders where nobody was left to mail — every member
	// had opted out. Not a failure, and counted separately from Sent so a pass
	// that mails nobody is distinguishable from one that had nothing to say.
	Suppressed int
	// Failed counts reminders where every send errored and the watermark was
	// therefore NOT advanced, so the next pass retries them.
	Failed int
}

// Run performs one pass: read candidates, decide, mail, advance watermarks.
//
// One bid's failure never stops the pass. A reminder job that aborts halfway
// leaves a silent, partial send that nobody can tell from a quiet day, so
// errors are counted and logged and the loop continues.
func (s *Service) Run(ctx context.Context) (Report, error) {
	candidates, err := s.repo.ListDueCandidates(ctx)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Considered: len(candidates)}

	due := Due(candidates, s.now())
	rep.Due = len(due)

	for _, r := range due {
		recipients, err := s.repo.RecipientsFor(ctx, r.Candidate.WorkbenchID)
		if err != nil {
			rep.Failed++
			slog.ErrorContext(ctx, "reminder recipients unavailable", "bucket", r.Bucket, "error", err)
			continue
		}

		var sent, attempted int
		for _, to := range recipients {
			// A recipient without an unsubscribe token cannot be mailed. The
			// alternative is sending a marketing-adjacent message with no way
			// out, which is both a compliance failure and the fastest way to
			// have password-reset mail treated as spam.
			if to.UnsubscribeToken == "" {
				slog.ErrorContext(ctx, "reminder skipped: recipient has no unsubscribe token")
				continue
			}
			attempted++
			if err := s.mailer.SendReminder(ctx, to, r); err != nil {
				slog.ErrorContext(ctx, "reminder send failed", "bucket", r.Bucket, "error", err)
				continue
			}
			sent++
		}

		// Nobody to tell: advance anyway. Every member opted out, and leaving
		// the watermark would make the job reconsider this bid every hour until
		// its deadline, for no one.
		if attempted == 0 {
			rep.Suppressed++
			s.mark(ctx, r)
			continue
		}
		// Every send failed: do NOT advance, so the next pass retries. This is
		// the one case where a duplicate is the lesser risk — the alternative
		// is a bucket silently lost to a transient outage.
		if sent == 0 {
			rep.Failed++
			continue
		}
		rep.Sent += sent
		s.mark(ctx, r)
	}

	// No workspace id, no bid id, no address: this line is a product metric and
	// travels to OTLP, so it carries counts only.
	slog.InfoContext(ctx, "reminder pass complete",
		"considered", rep.Considered, "due", rep.Due,
		"sent", rep.Sent, "suppressed", rep.Suppressed, "failed", rep.Failed)
	return rep, nil
}

// mark advances the watermark, logging rather than failing the pass. A bid that
// was mailed but whose watermark did not advance will be mailed again next
// pass — annoying, and strictly better than aborting a run that has already
// delivered mail.
func (s *Service) mark(ctx context.Context, r Reminder) {
	if err := s.repo.MarkReminded(ctx, r.Candidate.BidID, r.Bucket); err != nil {
		slog.ErrorContext(ctx, "reminder watermark not advanced", "bucket", r.Bucket, "error", err)
	}
}

// Unsubscribe records an opt-out for the holder of token.
//
// It returns nil for an unknown token, deliberately. The caller renders one
// page either way: a reader who mistyped a link and a reader whose token was
// revoked both deserve "you will not get these any more", and distinguishing
// them would turn the endpoint into a check for which tokens are live. A miss
// is logged, because a rash of them is worth seeing.
func (s *Service) Unsubscribe(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	found, err := s.repo.OptOut(ctx, token)
	if err != nil {
		return err
	}
	if !found {
		slog.InfoContext(ctx, "unsubscribe token did not match")
	}
	return nil
}
