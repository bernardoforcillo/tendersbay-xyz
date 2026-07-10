package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	bagent "github.com/buildwithgo/berrygem/agent"
)

func TestConsumeStream_ReturnsContentThenDone(t *testing.T) {
	content := make(chan string, 2)
	errs := make(chan error, 1)
	done := make(chan *bagent.RunResult, 1)

	content <- "Hel"
	content <- "lo"
	close(content)
	close(errs)
	done <- &bagent.RunResult{Content: "Hello"}
	close(done)

	var got string
	svc := &Service{}
	fullContent, result, err := svc.consumeStream(context.Background(), content, errs, done, func(tok string) error {
		got += tok
		return nil
	})
	if err != nil {
		t.Fatalf("consumeStream: %v", err)
	}
	if fullContent != "Hello" || got != "Hello" {
		t.Fatalf("fullContent = %q, sendToken saw %q, want both \"Hello\"", fullContent, got)
	}
	if result == nil || result.Content != "Hello" {
		t.Fatalf("result = %+v, want a RunResult with Content=Hello", result)
	}
}

func TestConsumeStream_ReturnsErrorFromErrChannel(t *testing.T) {
	content := make(chan string)
	errs := make(chan error, 1)
	done := make(chan *bagent.RunResult)

	close(content)
	errs <- errors.New("boom")
	close(errs)
	close(done)

	svc := &Service{}
	_, _, err := svc.consumeStream(context.Background(), content, errs, done, func(string) error { return nil })
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v, want \"boom\"", err)
	}
}

func TestConsumeStream_AllChannelsClosedWithoutResultReturnsExplicitError(t *testing.T) {
	// Reproduces berrygem's own contract (verified against its source): all
	// three channels are ALWAYS closed together when the stream loop
	// returns. If none of them ever carried a real value — a state that
	// shouldn't happen given berrygem's contract, but the loop must not
	// spin or hang if it somehow does — consumeStream must return a clear
	// error rather than looping on permanently-ready-but-empty channels.
	content := make(chan string)
	errs := make(chan error)
	done := make(chan *bagent.RunResult)
	close(content)
	close(errs)
	close(done)

	doneCh := make(chan struct{})
	var err error
	svc := &Service{}
	go func() {
		_, _, err = svc.consumeStream(context.Background(), content, errs, done, func(string) error { return nil })
		close(doneCh)
	}()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("consumeStream did not return within 2s — busy-spin or hang regression")
	}
	if err == nil {
		t.Fatal("err = nil, want an explicit \"stream ended without a result\" error")
	}
}

func TestConsumeStream_ReturnsPromptlyOnContextCancellation(t *testing.T) {
	content := make(chan string)
	errs := make(chan error)
	done := make(chan *bagent.RunResult)
	// Deliberately never closed/written — only ctx cancellation should
	// unblock consumeStream in this test.
	defer close(content)
	defer close(errs)
	defer close(done)

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan struct{})
	var err error
	svc := &Service{}
	go func() {
		_, _, err = svc.consumeStream(ctx, content, errs, done, func(string) error { return nil })
		close(doneCh)
	}()

	cancel()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("consumeStream did not return promptly after ctx cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
