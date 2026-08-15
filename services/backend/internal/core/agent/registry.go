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
		Model: "accounts/fireworks/models/deepseek-v4-flash-0731",
		Instructions: "You are an expert assistant on European public tenders. Answer concisely and " +
			"professionally in Italian. If the user asks you to create a workbench, infer its name and " +
			"visibility (private or shared) from the conversation and use the ask_choice tool to have the " +
			"user confirm or correct them BEFORE calling create_workbench. Never call create_workbench " +
			"without having first obtained an explicit confirmation through ask_choice. " +
			// A standing prior, not a duplicate of get_tender_criteria's own
			// per-call notice: that notice only reaches the model AFTER it has
			// decided to call the tool, and the failure being prevented here is
			// the model answering "come viene valutata?" straight from
			// search_tenders' eight scalars, which contain no criterion at all.
			"Never state how a tender is evaluated without having first called get_tender_criteria for " +
			"that tender. If no weights are published, say so: never estimate them.",
		MaxTurns: 8,
	})
}
