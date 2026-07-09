package agent

import (
	"sync"

	"github.com/buildwithgo/berrygem/agent"
	"github.com/buildwithgo/berrygem/chat"
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

type Registry struct {
	mu           sync.RWMutex
	configs      map[AgentType]AgentConfig
	apiKey       string
	chatSessions map[string]*chat.Chat
}

func NewRegistry(apiKey string) *Registry {
	return &Registry{
		configs:      make(map[AgentType]AgentConfig),
		apiKey:       apiKey,
		chatSessions: make(map[string]*chat.Chat),
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

func (r *Registry) GetOrCreateChat(sessionID string, ag *agent.Agent) (*chat.Chat, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.chatSessions[sessionID]; ok {
		return c, false
	}
	c := chat.New(ag)
	r.chatSessions[sessionID] = c
	return c, true
}

func (r *Registry) RemoveChat(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.chatSessions, sessionID)
}

// RegisterDefaults sets up the built-in agent configurations.
// API key resolution: if empty, Berrygem reads from the env.
func (r *Registry) RegisterDefaults() {
	r.Register(AgentConfig{
		Type:  AgentTypeBaseChat,
		Model: "accounts/fireworks/models/glm-5p2",
		Instructions: "Sei un assistente esperto di bandi pubblici europei. Rispondi in modo conciso e " +
			"professionale in italiano. Se l'utente ti chiede di creare un workbench, deduci nome e " +
			"visibilità (privato o condiviso) dalla conversazione e usa il tool ask_choice per farteli " +
			"confermare o correggere dall'utente PRIMA di chiamare create_workbench. Non chiamare mai " +
			"create_workbench senza aver prima ottenuto una conferma esplicita tramite ask_choice.",
		MaxTurns: 5,
	})
}
