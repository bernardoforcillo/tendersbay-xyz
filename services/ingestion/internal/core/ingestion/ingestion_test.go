package ingestion_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

type fakeSource struct {
	name    string
	tenders []tender.Tender
	err     error
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Fetch(context.Context) ([]tender.Tender, error) {
	return f.tenders, f.err
}

type blockingSource struct {
	name    string
	started chan struct{}
}

func (b *blockingSource) Name() string { return b.name }
func (b *blockingSource) Fetch(ctx context.Context) ([]tender.Tender, error) {
	close(b.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

type fakeSink struct {
	mu      sync.Mutex
	saved   [][]tender.Tender
	runs    []ingestion.RunRecord
	saveErr error
}

func (f *fakeSink) Save(_ context.Context, tenders []tender.Tender) (ingestion.SaveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return ingestion.SaveResult{}, f.saveErr
	}
	f.saved = append(f.saved, tenders)
	return ingestion.SaveResult{Inserted: len(tenders)}, nil
}

func (f *fakeSink) RecordRun(_ context.Context, rec ingestion.RunRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, rec)
	return nil
}

func (f *fakeSink) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

func TestRunOnce_AggregatesAcrossProviders(t *testing.T) {
	a := &fakeSource{name: "a", tenders: []tender.Tender{{SourceRef: "a-1"}, {SourceRef: "a-2"}}}
	b := &fakeSource{name: "b", tenders: []tender.Tender{{SourceRef: "b-1"}}}
	sink := &fakeSink{}

	svc := ingestion.NewService([]ingestion.Source{a, b}, sink)
	report := svc.RunOnce(context.Background())

	if report.Failed() {
		t.Fatalf("Failed() = true, want false: %+v", report.Providers)
	}
	if len(report.Providers) != 2 {
		t.Fatalf("len(Providers) = %d, want 2", len(report.Providers))
	}

	byProvider := map[string]ingestion.ProviderReport{}
	for _, p := range report.Providers {
		byProvider[p.Provider] = p
	}
	if byProvider["a"].Fetched != 2 || byProvider["a"].Inserted != 2 {
		t.Errorf("provider a report = %+v, want Fetched=2 Inserted=2", byProvider["a"])
	}
	if byProvider["b"].Fetched != 1 || byProvider["b"].Inserted != 1 {
		t.Errorf("provider b report = %+v, want Fetched=1 Inserted=1", byProvider["b"])
	}
	if sink.runCount() != 2 {
		t.Errorf("sink recorded %d runs, want 2 (one per provider)", sink.runCount())
	}
}

func TestRunOnce_OneProviderFailingDoesNotStopOthers(t *testing.T) {
	failing := &fakeSource{name: "failing", err: errors.New("fetch boom")}
	ok := &fakeSource{name: "ok", tenders: []tender.Tender{{SourceRef: "ok-1"}}}
	sink := &fakeSink{}

	svc := ingestion.NewService([]ingestion.Source{failing, ok}, sink)
	report := svc.RunOnce(context.Background())

	if !report.Failed() {
		t.Fatal("Failed() = false, want true")
	}

	byProvider := map[string]ingestion.ProviderReport{}
	for _, p := range report.Providers {
		byProvider[p.Provider] = p
	}
	if byProvider["failing"].Err == nil {
		t.Error("failing provider report has nil Err, want fetch error")
	}
	if byProvider["ok"].Err != nil || byProvider["ok"].Inserted != 1 {
		t.Errorf("ok provider report = %+v, want no error and Inserted=1", byProvider["ok"])
	}
}

func TestRunOnce_StopsFetchWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	blocking := &blockingSource{name: "blocking", started: started}
	sink := &fakeSink{}

	svc := ingestion.NewService([]ingestion.Source{blocking}, sink)

	done := make(chan ingestion.Report, 1)
	go func() { done <- svc.RunOnce(ctx) }()

	<-started
	cancel()

	report := <-done
	if !report.Failed() {
		t.Fatal("Failed() = false, want true after ctx cancellation")
	}
}
