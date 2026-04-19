package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/workflow/nodes"
)

func TestMergeStoresChunkComparisonData(t *testing.T) {
	t.Parallel()

	roundID := uuid.New()
	pipeline := &Pipeline{gate: nodes.NewQualityGate()}
	state := &State{
		Manifest: &domain.Manifest{
			ID:             uuid.New(),
			RoundID:        roundID,
			ChunkMetric:    domain.ChunkMetricChar,
			ParagraphCount: 1,
			ChunkCount:     2,
			Paragraphs: []domain.ParagraphMapping{
				{ParagraphIndex: 0, ChunkIDs: []string{"p0_c0", "p0_c1"}, Text: "Original paragraph"},
			},
			Chunks: []domain.Chunk{
				{ID: "p0_c0", ParagraphIndex: 0, ChunkIndex: 0, Text: "Original first half"},
				{ID: "p0_c1", ParagraphIndex: 0, ChunkIndex: 1, Text: "Original second half"},
			},
		},
		FinalOutputs: []string{"Rewritten first half", "Rewritten second half"},
		Reports: []domain.QualityReport{
			{
				ChunkID:      "p0_c0",
				AllPassed:    true,
				Recovered:    false,
				FailedChecks: nil,
				Checks: []domain.CheckResult{
					{Type: domain.CheckEmpty, Passed: true},
				},
			},
			{
				ChunkID:   "p0_c1",
				AllPassed: true,
				Recovered: true,
				Checks: []domain.CheckResult{
					{Type: domain.CheckDisallowedPattern, Passed: true},
				},
			},
		},
	}

	next, err := pipeline.merge(context.Background(), state)
	if err != nil {
		t.Fatalf("merge returned error: %v", err)
	}

	if got := next.Manifest.Chunks[0].Output; got != "Rewritten first half" {
		t.Fatalf("first chunk output mismatch: %q", got)
	}
	if got := next.Manifest.Chunks[0].Status; got != domain.ChunkPassed {
		t.Fatalf("first chunk status mismatch: %q", got)
	}
	if got := next.Manifest.Chunks[1].Status; got != domain.ChunkRecovered {
		t.Fatalf("second chunk status mismatch: %q", got)
	}
	if len(next.Manifest.Chunks[1].Checks) != 1 || next.Manifest.Chunks[1].Checks[0].Type != domain.CheckDisallowedPattern {
		t.Fatalf("second chunk checks were not stored: %+v", next.Manifest.Chunks[1].Checks)
	}
	if got := next.MergedOutput; got != "Rewritten first halfRewritten second half" {
		t.Fatalf("merged output mismatch: %q", got)
	}
}
