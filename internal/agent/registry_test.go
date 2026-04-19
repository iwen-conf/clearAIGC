package agent

import (
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestRegistryCredentialsGate(t *testing.T) {
	empty := domain.AgentSetting{
		Name:     domain.AgentCoordinator,
		Protocol: domain.AgentProtocolChat,
	}
	r := NewRegistry([]domain.AgentSetting{empty})
	if _, _, ok := r.Client(domain.AgentCoordinator); ok {
		t.Fatalf("expected no client when credentials are missing")
	}
	if _, ok := r.CoordinatorChatConfig(); ok {
		t.Fatalf("expected no chat config when coordinator missing credentials")
	}

	r.Update(domain.AgentSetting{
		Name:        domain.AgentCoordinator,
		DisplayName: "Rewrite Coordinator",
		Protocol:    domain.AgentProtocolChat,
		BaseURL:     "https://example.test",
		APIKey:      "sk-test",
		Model:       "gpt-test",
	})
	if _, _, ok := r.Client(domain.AgentCoordinator); !ok {
		t.Fatalf("expected client after credentials supplied")
	}
	cfg, ok := r.CoordinatorChatConfig()
	if !ok {
		t.Fatalf("expected chat config after credentials supplied")
	}
	if cfg.Model != "gpt-test" {
		t.Fatalf("unexpected chat config model: %s", cfg.Model)
	}
}

func TestRegistryProtocolSwitch(t *testing.T) {
	r := NewRegistry([]domain.AgentSetting{{
		Name:     domain.AgentLexicalMutator,
		Protocol: domain.AgentProtocolResponses,
		BaseURL:  "https://example.test",
		APIKey:   "sk-test",
		Model:    "gpt-test",
	}})
	if _, _, ok := r.Client(domain.AgentLexicalMutator); !ok {
		t.Fatalf("expected client for responses protocol")
	}
	r.Update(domain.AgentSetting{
		Name:     domain.AgentLexicalMutator,
		Protocol: domain.AgentProtocolChat,
		BaseURL:  "https://example.test",
		APIKey:   "sk-test",
		Model:    "gpt-test-chat",
	})
	entry, ok := r.Get(domain.AgentLexicalMutator)
	if !ok {
		t.Fatalf("expected entry after update")
	}
	if entry.Setting.Protocol != domain.AgentProtocolChat {
		t.Fatalf("expected protocol chat, got %s", entry.Setting.Protocol)
	}
	if entry.Setting.Model != "gpt-test-chat" {
		t.Fatalf("unexpected model after update: %s", entry.Setting.Model)
	}
}
