package agent

import (
	"fmt"
	"sync"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/infra/llm"
)

const defaultTimeout = 90 * time.Second

// Registry holds the live LLM wiring for the three configurable agents.
// Update() atomically swaps the entry for a single agent so in-flight work
// using a previous client can finish while new work gets the fresh config.
type Registry struct {
	mu      sync.RWMutex
	entries map[domain.AgentName]registryEntry
}

type registryEntry struct {
	Setting    domain.AgentSetting
	LLM        domain.LLMClient
	ChatConfig einoopenai.ChatModelConfig
}

func NewRegistry(settings []domain.AgentSetting) *Registry {
	r := &Registry{entries: make(map[domain.AgentName]registryEntry, len(settings))}
	for _, s := range settings {
		r.entries[s.Name] = buildEntry(s)
	}
	return r
}

func (r *Registry) Update(setting domain.AgentSetting) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[setting.Name] = buildEntry(setting)
}

func (r *Registry) Get(name domain.AgentName) (registryEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[name]
	return entry, ok
}

// Client returns the live LLMClient for an agent, or a nil client plus false
// when the agent has not been configured (missing api key, base url, or model).
func (r *Registry) Client(name domain.AgentName) (domain.LLMClient, domain.AgentSetting, bool) {
	entry, ok := r.Get(name)
	if !ok || entry.LLM == nil {
		return nil, entry.Setting, false
	}
	return entry.LLM, entry.Setting, true
}

// CoordinatorChatConfig returns the eino chat-model config for the coordinator.
// Returns false when the coordinator is not configured.
func (r *Registry) CoordinatorChatConfig() (einoopenai.ChatModelConfig, bool) {
	entry, ok := r.Get(domain.AgentCoordinator)
	if !ok || !entry.Setting.HasCredentials() {
		return einoopenai.ChatModelConfig{}, false
	}
	return entry.ChatConfig, true
}

func buildEntry(s domain.AgentSetting) registryEntry {
	entry := registryEntry{Setting: s}
	if !s.HasCredentials() {
		return entry
	}
	cfg := domain.ProviderConfig{
		Name:    fmt.Sprintf("agent-%s", s.Name),
		BaseURL: s.BaseURL,
		APIKey:  s.APIKey,
		Model:   s.Model,
		Timeout: defaultTimeout,
	}
	switch s.Protocol {
	case domain.AgentProtocolResponses:
		cfg.Mode = domain.ProviderModeResponses
		entry.LLM = llm.NewResponsesClient(cfg)
	default:
		cfg.Mode = domain.ProviderModeChat
		entry.LLM = llm.NewChatClient(cfg)
	}
	entry.ChatConfig = einoopenai.ChatModelConfig{
		APIKey:  s.APIKey,
		BaseURL: s.BaseURL,
		Model:   s.Model,
		Timeout: defaultTimeout,
	}
	return entry
}
