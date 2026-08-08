package agent

import (
	"sync"

	"github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/providers/fireworks"
)

type AgentType string

const AgentTypeBaseChat AgentType = "base-chat"

type AgentConfig struct {
	Type         AgentType
	Model        string
	Instructions string
	MaxTurns     int
}

// Registry holds the agent configurations and knows how to build a berrygem
// agent from one. It deliberately holds NO per-session state: the conversation
// lives in Postgres and nowhere else, so every pod assembles the same context
// for a session regardless of which pod served the previous turn. Keeping a
// map[sessionID]*chat.Chat here is what made the service stateful in
// contradiction of its own readiness probe (probe.Ready's "a stateless service
// is ready as soon as it can serve") and made a session's context depend on
// which replica Traefik happened to route the request to.
type Registry struct {
	mu      sync.RWMutex
	configs map[AgentType]AgentConfig
	apiKey  string
}

func NewRegistry(apiKey string) *Registry {
	return &Registry{
		configs: make(map[AgentType]AgentConfig),
		apiKey:  apiKey,
	}
}

func (r *Registry) Register(cfg AgentConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[cfg.Type] = cfg
}

func (r *Registry) GetConfig(agentType AgentType) (AgentConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.configs[agentType]
	return cfg, ok
}

func (r *Registry) BuildAgent(cfg AgentConfig, tools ...agent.Option) (*agent.Agent, error) {
	opts := []agent.Option{
		agent.WithProvider(fireworks.New(r.apiKey, cfg.Model)),
		agent.WithModel(cfg.Model),
		agent.WithName(string(cfg.Type)),
	}
	if cfg.Instructions != "" {
		opts = append(opts, agent.WithInstructions(cfg.Instructions))
	}
	if cfg.MaxTurns > 0 {
		opts = append(opts, agent.WithMaxTurns(cfg.MaxTurns))
	}
	opts = append(opts, tools...)
	return agent.New(opts...)
}

// RegisterDefaults sets up the built-in agent configurations.
// API key resolution: if empty, Berrygem reads from the env.
func (r *Registry) RegisterDefaults() {
	r.Register(AgentConfig{
		Type:  AgentTypeBaseChat,
		Model: "accounts/fireworks/models/deepseek-v4-flash",
		Instructions: "Sei un assistente esperto di bandi pubblici europei. Rispondi in modo conciso e " +
			"professionale in italiano. Se l'utente ti chiede di creare un workbench, deduci nome e " +
			"visibilità (privato o condiviso) dalla conversazione e usa il tool ask_choice per farteli " +
			"confermare o correggere dall'utente PRIMA di chiamare create_workbench. Non chiamare mai " +
			"create_workbench senza aver prima ottenuto una conferma esplicita tramite ask_choice. " +
			// A standing prior, not a duplicate of get_tender_criteria's own
			// per-call notice: that notice only reaches the model AFTER it has
			// decided to call the tool, and the failure being prevented here is
			// the model answering "come viene valutata?" straight from
			// search_tenders' eight scalars, which contain no criterion at all.
			"Non affermare mai come viene valutata una gara senza aver prima chiamato get_tender_criteria " +
			"per quella gara. Se non ci sono pesi pubblicati, dillo: non stimarli mai.",
		MaxTurns: 8,
	})
}
