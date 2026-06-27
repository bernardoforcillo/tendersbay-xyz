package health

import (
	"context"
	"errors"
	"testing"
)

type stubProbe struct {
	name string
	err  error
}

func (p stubProbe) Name() string                { return p.name }
func (p stubProbe) Check(context.Context) error { return p.err }

func TestReadyAllPass(t *testing.T) {
	svc := New(stubProbe{name: "a"}, stubProbe{name: "b"})
	st := svc.Ready(context.Background())
	if !st.OK {
		t.Fatalf("OK = false, want true; checks=%v", st.Checks)
	}
	if len(st.Checks) != 0 {
		t.Fatalf("Checks = %v, want empty", st.Checks)
	}
}

func TestReadyOneFails(t *testing.T) {
	svc := New(
		stubProbe{name: "a"},
		stubProbe{name: "db", err: errors.New("connection refused")},
	)
	st := svc.Ready(context.Background())
	if st.OK {
		t.Fatal("OK = true, want false")
	}
	if st.Checks["db"] != "connection refused" {
		t.Fatalf("Checks[db] = %q, want connection refused", st.Checks["db"])
	}
}

func TestReadyNoProbes(t *testing.T) {
	st := New().Ready(context.Background())
	if !st.OK {
		t.Fatal("OK = false with no probes, want true")
	}
}
