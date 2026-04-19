package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

const syntaxSystemPrompt = `[Role]
You are a Structure Architect. Your objective is to inject burstiness (variance in sentence length and topological complexity) into the provided text to bypass AI stylistic detection.

[Directives]
1. Analyze the rhythm of the text. AI writes in uniform, balanced, multi-clause sentences.
2. Disrupt this uniformity: forcefully split long compound sentences into punchy short sentences.
3. Consolidate fragmented short sentences into complex, asymmetrical structures using semicolons or em-dashes.
4. Invert grammatical structures (e.g., shift passive voice to active, use fronted adverbials).
5. CRITICAL BOUNDARY: strictly preserve the logical sequence and academic validity of the arguments. Do not alter facts.

[Output Format]
Respond with JSON only, no prose, no code fences, schema:
{"rebuilt_text":"string","burstiness_metrics":{"longest_sentence_word_count":0,"shortest_sentence_word_count":0}}`

type SyntaxRebuilder struct {
	registry *Registry
}

func NewSyntaxRebuilder(registry *Registry) *SyntaxRebuilder {
	return &SyntaxRebuilder{registry: registry}
}

type syntaxResponse struct {
	RebuiltText string `json:"rebuilt_text"`
}

// Apply returns the restructured text and the total tokens consumed.
// When the agent is not configured, it returns the original text unchanged and zero tokens.
func (s *SyntaxRebuilder) Apply(ctx context.Context, requestID, text string) (string, int, error) {
	client, _, ok := s.registry.Client(domain.AgentSyntaxRebuilder)
	if !ok {
		return text, 0, nil
	}

	prompt := fmt.Sprintf("%s\n\n[INPUT TEXT]\n%s", syntaxSystemPrompt, text)
	result, err := client.Complete(ctx, domain.LLMRequest{
		RequestID: requestID + "-syntax",
		Prompt:    prompt,
	})
	if err != nil {
		return "", 0, fmt.Errorf("syntax rebuilder: %w", err)
	}

	payload := extractJSON(result.OutputText)
	var parsed syntaxResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return "", result.InputTokens + result.OutputTokens, fmt.Errorf("parse syntax output: %w", err)
	}
	rebuilt := strings.TrimSpace(parsed.RebuiltText)
	if rebuilt == "" {
		return "", result.InputTokens + result.OutputTokens, fmt.Errorf("syntax rebuilder returned empty text")
	}
	return rebuilt, result.InputTokens + result.OutputTokens, nil
}
