package domain

import "time"

type AgentName string

const (
	AgentCoordinator     AgentName = "coordinator"
	AgentLexicalMutator  AgentName = "lexical_mutator"
	AgentSyntaxRebuilder AgentName = "syntax_rebuilder"
)

func (n AgentName) Valid() bool {
	switch n {
	case AgentCoordinator, AgentLexicalMutator, AgentSyntaxRebuilder:
		return true
	}
	return false
}

type AgentProtocol string

const (
	AgentProtocolResponses AgentProtocol = "responses"
	AgentProtocolChat      AgentProtocol = "chat"
)

func (p AgentProtocol) Valid() bool {
	return p == AgentProtocolResponses || p == AgentProtocolChat
}

type AgentSetting struct {
	Name        AgentName     `json:"name"`
	DisplayName string        `json:"displayName"`
	Protocol    AgentProtocol `json:"protocol"`
	BaseURL     string        `json:"baseUrl"`
	APIKey      string        `json:"apiKey,omitempty"`
	Model       string        `json:"model"`
	UpdatedAt   time.Time     `json:"updatedAt"`
}

func (s AgentSetting) HasCredentials() bool {
	return s.APIKey != "" && s.Model != "" && s.BaseURL != ""
}
