package agent

import (
	"context"
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestCoordinatorProcessSentenceFallsBackWhenNoCandidateImproves(t *testing.T) {
	registry := NewRegistry(nil)
	coordinator := NewCoordinator(registry, NewLexicalMutator(registry), NewSyntaxRebuilder(registry))

	decision := coordinator.ProcessSentence(context.Background(), "req", "普通句子。", "保持自然")
	if decision.Accepted {
		t.Fatal("expected baseline fallback for unconfigured workers")
	}
	if decision.Output != decision.Input {
		t.Fatalf("expected unchanged output, got %q", decision.Output)
	}
	if len(decision.Candidates) == 0 {
		t.Fatal("expected at least baseline candidate")
	}
}

func TestCoordinatorProcessChunkRequiresConfiguredCoordinatorModel(t *testing.T) {
	registry := NewRegistry(nil)
	coordinator := NewCoordinator(registry, NewLexicalMutator(registry), NewSyntaxRebuilder(registry))

	_, err := coordinator.ProcessChunk(context.Background(), "req", domain.Chunk{
		ID:   "p0_c0",
		Text: "第一句比较普通。综合来看，这一方向具有现实意义和实践价值。",
	}, "降低模板化风险")
	if err != ErrCoordinatorNotConfigured {
		t.Fatalf("expected ErrCoordinatorNotConfigured, got %v", err)
	}
}
