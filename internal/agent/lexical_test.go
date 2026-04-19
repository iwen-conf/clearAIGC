package agent

import (
	"context"
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type fakeLLMClient struct {
	output string
	in     int
	out    int
	err    error
}

func (f *fakeLLMClient) Complete(_ context.Context, _ domain.LLMRequest) (*domain.ProviderResult, error) {
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
	rewritten, tokens, err := mutator.Apply(context.Background(), "req-1", input)
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
	out, tokens, err := mutator.Apply(context.Background(), "req", input)
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
	out, tokens, err := rebuilder.Apply(context.Background(), "req", "original text")
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
