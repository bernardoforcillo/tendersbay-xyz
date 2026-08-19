// Package ingestion is the core orchestrator: it defines the Source (input)
// and Sink (output) ports each provider/persistence adapter implements, and
// runs one ingestion cycle across every registered provider.
package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentProviders bounds how many providers run Fetch/Save at once.
const maxConcurrentProviders = 8

// providerTimeout bounds one provider's Fetch+Save. Sized well under the
// CronJob's activeDeadlineSeconds so a stalled provider fails on its own
// and the cycle still records every other provider's result.
const providerTimeout = 20 * time.Minute

// Source is the input port each provider implements. This is the
// extensibility/scaling seam: TED / national-portal connectors plug in here.
type Source interface {
	Name() string
	Fetch(ctx context.Context) ([]tender.Tender, error)
}

// DetailedSource is an OPTIONAL companion to Source, implemented by a provider
// whose SEARCH feed already carries notice-document detail. Spain's PLACSP is
// the case that forced it to exist: its ATOM entries embed the whole CODICE
// document, selection criteria included, where TED publishes only a link to a
// notice document a later pass has to fetch.
//
// It is a separate interface rather than a widened Source so the four existing
// providers keep compiling untouched — a provider that has nothing extra to
// give should not have to say so. runSource type-asserts and falls back to
// plain Fetch, which is also what keeps this seam honest: a provider either
// genuinely has the detail at fetch time or it does not, and there is no third
// state where it half-implements one.
//
// The criteria are keyed by SourceRef, not by tender ID: IDs are the sink's
// business and do not exist yet at fetch time. Providers must use the same
// SourceRef they put on the tenders they return, or the sink cannot join the
// two and the criteria are silently dropped.
type DetailedSource interface {
	Source
	FetchWithDetail(ctx context.Context) ([]tender.Tender, map[string][]tender.SelectionCriterion, error)
}

// SaveResult reports how many tender rows one Save call inserted vs updated.
type SaveResult struct {
	Inserted int
	Updated  int
}

// RunRecord is one provider's outcome for one ingestion cycle, persisted as
// an audit row.
type RunRecord struct {
	Source     string
	StartedAt  time.Time
	FinishedAt time.Time
	Fetched    int
	Inserted   int
	Updated    int
	Err        error
}

// Sink is the output port for persistence.
type Sink interface {
	Save(ctx context.Context, tenders []tender.Tender) (SaveResult, error)

	// SaveSelectionCriteria persists the selection criteria a DetailedSource
	// supplied, keyed by the SourceRef of the tender each set belongs to. It is
	// called AFTER Save, so every referenced tender row exists; a ref that
	// still resolves to nothing is skipped rather than erroring, because the
	// only way to reach that state is a provider keying its map inconsistently
	// with the tenders it returned, and failing the whole cycle over one
	// mismatched key would lose the other providers' work too.
	//
	// It must not touch enrichment bookkeeping. These criteria came from a
	// search feed, not from a notice document, and a row whose notice was never
	// read must not be recorded as read.
	SaveSelectionCriteria(ctx context.Context, source string, bySourceRef map[string][]tender.SelectionCriterion) error
	RecordRun(ctx context.Context, rec RunRecord) error
}

// ProviderReport is the outcome of running one provider for one cycle.
type ProviderReport struct {
	Provider string
	Fetched  int
	Inserted int
	Updated  int
	Err      error
}

// Report aggregates every provider's outcome for one RunOnce call.
type Report struct {
	Providers []ProviderReport
}

// Failed reports whether any provider errored during the run.
func (r Report) Failed() bool {
	for _, p := range r.Providers {
		if p.Err != nil {
			return true
		}
	}
	return false
}

// Summary renders a short line per provider for logging.
func (r Report) Summary() string {
	return fmt.Sprintf("%+v", r.Providers)
}

// Service orchestrates provider fan-out and persistence for one ingestion
// cycle.
type Service struct {
	sources []Source
	sink    Sink
}

// NewService builds a Service over the given sources and sink.
func NewService(sources []Source, sink Sink) *Service {
	return &Service{sources: sources, sink: sink}
}

// RunOnce runs every registered source concurrently (bounded), saves each
// source's batch through Sink, and records a per-provider audit row. One
// source's failure does not stop the others.
func (s *Service) RunOnce(ctx context.Context) Report {
	reports := make([]ProviderReport, len(s.sources))

	var g errgroup.Group
	g.SetLimit(maxConcurrentProviders)
	for i, src := range s.sources {
		i, src := i, src
		g.Go(func() error {
			reports[i] = s.runSource(ctx, src)
			return nil
		})
	}
	_ = g.Wait()

	return Report{Providers: reports}
}

func (s *Service) runSource(ctx context.Context, src Source) ProviderReport {
	started := time.Now().UTC()
	report := ProviderReport{Provider: src.Name()}

	// Per-provider cap, not a cap on the run: Kubernetes still owns the
	// overall budget via activeDeadlineSeconds (which is why INGESTION_TIMEOUT
	// was dropped — see the design doc). What that global cap cannot do is
	// stop one unresponsive portal from consuming the entire window and
	// taking every other provider down with it, since the job is killed
	// outright rather than yielding. Bounding each provider keeps a single
	// bad upstream to its own slot.
	//
	// The audit write below deliberately keeps the *parent* context: a
	// provider that exhausts its budget leaves fetchCtx cancelled, and
	// recording the run through that context would fail exactly when the
	// failure is most worth recording.
	fetchCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	slog.InfoContext(ctx, "provider run started", "provider", src.Name())

	tenders, criteria, err := fetchFrom(fetchCtx, src)
	if err != nil {
		report.Err = err
	} else {
		report.Fetched = len(tenders)
		result, saveErr := s.sink.Save(fetchCtx, tenders)
		if saveErr != nil {
			report.Err = saveErr
		} else {
			report.Inserted = result.Inserted
			report.Updated = result.Updated

			// Counts are recorded BEFORE this can fail, so a criteria write
			// that errors still leaves an audit row saying truthfully how many
			// tenders landed. The error is reported rather than logged away:
			// criteria refill on the next cycle when the feed is re-read, so a
			// single failure is self-healing, but one that persists means a
			// provider's admissibility data never arrives and that should be
			// visible rather than quiet.
			if len(criteria) > 0 {
				if err := s.sink.SaveSelectionCriteria(fetchCtx, src.Name(), criteria); err != nil {
					report.Err = err
				}
			}
		}
	}

	rec := RunRecord{
		Source:     src.Name(),
		StartedAt:  started,
		FinishedAt: time.Now().UTC(),
		Fetched:    report.Fetched,
		Inserted:   report.Inserted,
		Updated:    report.Updated,
		Err:        report.Err,
	}
	if recErr := s.sink.RecordRun(ctx, rec); recErr != nil {
		slog.ErrorContext(ctx, "failed to record ingestion run", "provider", src.Name(), "error", recErr)
	}
	if report.Err != nil {
		slog.ErrorContext(ctx, "provider run failed",
			"provider", src.Name(),
			"duration", rec.FinishedAt.Sub(started).String(),
			"error", report.Err)
	} else {
		slog.InfoContext(ctx, "provider run complete",
			"provider", src.Name(),
			"duration", rec.FinishedAt.Sub(started).String(),
			"fetched", report.Fetched,
			"inserted", report.Inserted,
			"updated", report.Updated)
	}
	return report
}

// fetchFrom calls the richer FetchWithDetail when a provider implements
// DetailedSource, and plain Fetch otherwise. Keeping the type assertion here
// rather than in runSource leaves that function reading as one straight line of
// cycle bookkeeping, and gives the fallback exactly one place to live.
func fetchFrom(ctx context.Context, src Source) ([]tender.Tender, map[string][]tender.SelectionCriterion, error) {
	if d, ok := src.(DetailedSource); ok {
		return d.FetchWithDetail(ctx)
	}
	tenders, err := src.Fetch(ctx)
	return tenders, nil, err
}
