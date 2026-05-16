package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type fakeLLMClient struct {
	output string
	in     int
	out    int
	err    error
	last   domain.LLMRequest
}

func (f *fakeLLMClient) Complete(_ context.Context, request domain.LLMRequest) (*domain.ProviderResult, error) {
	f.last = request
	if f.err != nil {
		return nil, f.err
	}
	return &domain.ProviderResult{
		Provider:     "fake",
		OutputText:   f.output,
		InputTokens:  f.in,
		OutputTokens: f.out,
	}, nil
}

func newRegistryWithClient(name domain.AgentName, client domain.LLMClient) *Registry {
	r := &Registry{entries: map[domain.AgentName]registryEntry{}}
	r.entries[name] = registryEntry{
		Setting: domain.AgentSetting{
			Name:     name,
			Protocol: domain.AgentProtocolChat,
			BaseURL:  "https://example.test",
			APIKey:   "sk-test",
			Model:    "gpt-test",
		},
		LLM: client,
	}
	return r
}

func TestLexicalMutatorAppliesPatches(t *testing.T) {
	client := &fakeLLMClient{
		output: `{"patches":[{"original_word":"crucial","replacement_word":"pivotal"},{"original_word":"delve into","replacement_word":"examine"}]}`,
		in:     10,
		out:    20,
	}
	r := newRegistryWithClient(domain.AgentLexicalMutator, client)
	mutator := NewLexicalMutator(r)

	input := "It is crucial to delve into the dataset."
	rewritten, tokens, err := mutator.Apply(context.Background(), "req-1", RewriteGuidance{Text: input})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "It is pivotal to examine the dataset."
	if rewritten != want {
		t.Fatalf("expected %q, got %q", want, rewritten)
	}
	if tokens != 30 {
		t.Fatalf("expected 30 tokens, got %d", tokens)
	}
}

func TestLexicalMutatorUnconfiguredReturnsOriginal(t *testing.T) {
	r := NewRegistry([]domain.AgentSetting{{Name: domain.AgentLexicalMutator, Protocol: domain.AgentProtocolChat}})
	mutator := NewLexicalMutator(r)
	input := "unchanged text"
	out, tokens, err := mutator.Apply(context.Background(), "req", RewriteGuidance{Text: input})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != input || tokens != 0 {
		t.Fatalf("expected pass-through; got %q tokens=%d", out, tokens)
	}
}

func TestExtractJSONStripsFences(t *testing.T) {
	raw := "```json\n{\"patches\":[]}\n```"
	got := extractJSON(raw)
	want := `{"patches":[]}`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSyntaxRebuilderParsesOutput(t *testing.T) {
	client := &fakeLLMClient{
		output: `{"rebuilt_text":"Short sentence. Longer, reshaped sentence follows."}`,
		in:     5,
		out:    15,
	}
	r := newRegistryWithClient(domain.AgentSyntaxRebuilder, client)
	rebuilder := NewSyntaxRebuilder(r)
	out, tokens, err := rebuilder.Apply(context.Background(), "req", RewriteGuidance{Text: "original text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Short sentence. Longer, reshaped sentence follows." {
		t.Fatalf("unexpected output: %q", out)
	}
	if tokens != 20 {
		t.Fatalf("unexpected tokens: %d", tokens)
	}
}

func TestBuildRewriteGuidanceDerivesTargetAndForbiddenPhrases(t *testing.T) {
	guidance := buildRewriteGuidance(
		"在当前数字化协作的背景下，综合来看，这项工作具有现实意义。",
		"第二轮进一步去模板化",
	)

	if guidance.TargetRiskScore <= 0 || guidance.TargetRiskScore >= guidance.RiskScore {
		t.Fatalf("expected target risk score below current risk, got current=%.2f target=%.2f", guidance.RiskScore, guidance.TargetRiskScore)
	}
	if len(guidance.ForbiddenPhrases) == 0 {
		t.Fatalf("expected forbidden phrases to be populated")
	}
}

func TestLexicalMutatorPromptIncludesTargetRiskAndForbiddenPhrases(t *testing.T) {
	client := &fakeLLMClient{output: `{"patches":[]}`}
	r := newRegistryWithClient(domain.AgentLexicalMutator, client)
	mutator := NewLexicalMutator(r)

	guidance := RewriteGuidance{
		Text:             "在当前数字化协作的背景下，综合来看，这项工作具有现实意义。",
		RoundPrompt:      "第二轮进一步去模板化",
		RiskScore:        0.58,
		TargetRiskScore:  0.40,
		RiskFeatures:     []string{"综合来看", "现实意义"},
		ForbiddenPhrases: []string{"综合来看", "意义"},
	}

	if _, _, err := mutator.Apply(context.Background(), "req-prompt", guidance); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prompt := client.last.Prompt
	if !strings.Contains(prompt, "Target risk score: <= 0.40") {
		t.Fatalf("expected target risk score in prompt, got %q", prompt)
	}
	if !strings.Contains(prompt, "[FORBIDDEN PHRASES]") || !strings.Contains(prompt, "- 综合来看") {
		t.Fatalf("expected forbidden phrase list in prompt, got %q", prompt)
	}
}
