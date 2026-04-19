package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

const lexicalSystemPrompt = `[Role]
You are a Lexical Analyst operating as a headless microservice. Your task is to dilute the probability distribution (PPL) of the provided academic text.

[Directives]
1. Scan the text for overused AI-generated words (e.g., "Moreover", "It is worth noting", "Comprehensive", "Pivotal").
2. Replace them with precise, human-like, long-tail academic terminology specific to the context.
3. CRITICAL BOUNDARY: DO NOT add, delete, or move any commas, periods, or clauses. Maintain the exact original structure of the sentence.
4. Ignore words wrapped in __TERM__ markers.
5. If the text is Chinese, strictly AVOID typical AI transition phrases and abstract words like "在当前背景下", "综合来看", "进一步来说", "意义", "价值", "维度", "层面", "机制", "路径", etc.

[Output Format]
You must output a precise Diff-Patch array in JSON format. Do not return the full text.
Respond with JSON only, no prose, no code fences, schema:
{"patches":[{"original_word":"string","replacement_word":"string","reasoning":"string"}]}
If no changes are required, respond with {"patches":[]}.`

type LexicalMutator struct {
	registry *Registry
}

func NewLexicalMutator(registry *Registry) *LexicalMutator {
	return &LexicalMutator{registry: registry}
}

type lexicalPatch struct {
	Original    string `json:"original_word"`
	Replacement string `json:"replacement_word"`
	Reasoning   string `json:"reasoning,omitempty"`
}

type lexicalResponse struct {
	Patches []lexicalPatch `json:"patches"`
}

// Apply returns the rewritten text after applying lexical patches and the total tokens consumed.
// When the agent is not configured, it returns the original text unchanged and zero tokens.
func (m *LexicalMutator) Apply(ctx context.Context, requestID, text string) (string, int, error) {
	client, _, ok := m.registry.Client(domain.AgentLexicalMutator)
	if !ok {
		return text, 0, nil
	}

	prompt := fmt.Sprintf("%s\n\n[INPUT TEXT]\n%s", lexicalSystemPrompt, text)
	result, err := client.Complete(ctx, domain.LLMRequest{
		RequestID: requestID + "-lexical",
		Prompt:    prompt,
	})
	if err != nil {
		return "", 0, fmt.Errorf("lexical mutator: %w", err)
	}

	payload := extractJSON(result.OutputText)
	var parsed lexicalResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return "", result.InputTokens + result.OutputTokens, fmt.Errorf("parse lexical patches: %w", err)
	}

	rewritten := text
	for _, patch := range parsed.Patches {
		if patch.Original == "" || patch.Replacement == "" {
			continue
		}
		rewritten = strings.ReplaceAll(rewritten, patch.Original, patch.Replacement)
	}
	return rewritten, result.InputTokens + result.OutputTokens, nil
}

func extractJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}
