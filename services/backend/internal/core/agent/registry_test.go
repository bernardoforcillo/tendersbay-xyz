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

func TestRegisterDefaults_BaseChatHasSearchStreakHeadroom(t *testing.T) {
	r := NewRegistry("")
	r.RegisterDefaults()
	cfg, ok := r.GetConfig(AgentTypeBaseChat)
	if !ok {
		t.Fatal("AgentTypeBaseChat not registered")
	}
	if cfg.MaxTurns != 8 {
		t.Fatalf("MaxTurns = %d, want 8 (headroom for 5 empty searches + a round to call ask_choice)", cfg.MaxTurns)
	}
}
