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

	tenders, err := src.Fetch(ctx)
	if err != nil {
		report.Err = err
	} else {
		report.Fetched = len(tenders)
		result, saveErr := s.sink.Save(ctx, tenders)
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
		slog.ErrorContext(ctx, "provider run failed", "provider", src.Name(), "error", report.Err)
	}
	return report
}
