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

	tenders, err := src.Fetch(fetchCtx)
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
