package domain

import "time"

type ProviderMode string

const (
	ProviderModeResponses ProviderMode = "responses"
	ProviderModeChat      ProviderMode = "chat_completions"
)

type ProviderConfig struct {
	Name         string
	Mode         ProviderMode
	BaseURL      string
	APIKey       string
	Model        string
	Organization string
	Project      string
	Timeout      time.Duration
	RPMLimit     int
	TPMLimit     int
	Store        bool
}

type ProviderResult struct {
	Provider     string
	OutputText   string
	InputTokens  int
	OutputTokens int
	RawRequestID string
	Score        *AIScore
	Sentences    []SentenceDecision
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
