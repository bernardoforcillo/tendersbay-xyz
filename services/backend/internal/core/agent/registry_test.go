package agent

import "testing"

func TestGetOrCreateChat_ReportsWasCreated(t *testing.T) {
	r := NewRegistry("")
	r.RegisterDefaults()
	cfg, _ := r.GetConfig(AgentTypeBaseChat)
	ag, err := r.BuildAgent(cfg)
	if err != nil {
		t.Fatalf("BuildAgent: %v", err)
	}

	_, wasCreated := r.GetOrCreateChat("session-1", ag)
	if !wasCreated {
		t.Fatal("first GetOrCreateChat for a new session: wasCreated = false, want true")
	}

	_, wasCreated = r.GetOrCreateChat("session-1", ag)
	if wasCreated {
		t.Fatal("second GetOrCreateChat for the same session: wasCreated = true, want false")
	}

	r.RemoveChat("session-1")
	_, wasCreated = r.GetOrCreateChat("session-1", ag)
	if !wasCreated {
		t.Fatal("GetOrCreateChat after RemoveChat: wasCreated = false, want true (evicted entries rebuild fresh)")
	}
}
