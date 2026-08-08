package agent

import "testing"

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
