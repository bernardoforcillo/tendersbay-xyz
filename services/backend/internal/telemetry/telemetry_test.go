package telemetry

import (
	"context"
	"testing"
)

func TestSetupNoKeyIsNoop(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown is nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("noop shutdown returned error: %v", err)
	}
}
