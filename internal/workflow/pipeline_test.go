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

func TestMergeRollsBackWholeRoundWhenMergedRiskGetsWorse(t *testing.T) {
	t.Parallel()

	originalLeft := "The review team first mapped the sample boundaries and then checked the field definitions before writing the owner and approval result into one operating checklist for the next shift."
	originalRight := "The implementation note also asked each operator to keep the timestamp and approval record for every handoff so a later audit could reconstruct the full sequence without guesswork."
	rewrittenLeft := "at the same time " + originalLeft
	rewrittenRight := "at the same time " + originalRight

	pipeline := &Pipeline{gate: nodes.NewQualityGate()}
	state := &State{
		ParsedText: originalLeft + originalRight,
		Manifest: &domain.Manifest{
			ID:             uuid.New(),
			RoundID:        uuid.New(),
			ChunkMetric:    domain.ChunkMetricChar,
			ParagraphCount: 1,
			ChunkCount:     2,
			Paragraphs: []domain.ParagraphMapping{
				{ParagraphIndex: 0, ChunkIDs: []string{"p0_c0", "p0_c1"}, Text: originalLeft + originalRight},
			},
			Chunks: []domain.Chunk{
				{ID: "p0_c0", ParagraphIndex: 0, ChunkIndex: 0, Text: originalLeft},
				{ID: "p0_c1", ParagraphIndex: 0, ChunkIndex: 1, Text: originalRight},
			},
		},
		FinalOutputs: []string{rewrittenLeft, rewrittenRight},
		Reports: []domain.QualityReport{
			{ChunkID: "p0_c0", AllPassed: true, Checks: []domain.CheckResult{{Type: domain.CheckEmpty, Passed: true}}},
			{ChunkID: "p0_c1", AllPassed: true, Checks: []domain.CheckResult{{Type: domain.CheckEmpty, Passed: true}}},
		},
	}

	if domain.EstimateAIRate(rewrittenLeft) > domain.EstimateAIRate(originalLeft) {
		t.Fatalf("test setup invalid: first chunk risk unexpectedly increased")
	}
	if domain.EstimateAIRate(rewrittenRight) > domain.EstimateAIRate(originalRight) {
		t.Fatalf("test setup invalid: second chunk risk unexpectedly increased")
	}
	if domain.EstimateAIRate(rewrittenLeft+rewrittenRight) <= domain.EstimateAIRate(state.ParsedText) {
		t.Fatalf("test setup invalid: merged output did not increase overall risk")
	}

	next, err := pipeline.merge(context.Background(), state)
	if err != nil {
		t.Fatalf("merge returned error: %v", err)
	}

	if got := next.Manifest.Chunks[0].Output; got != originalLeft {
		t.Fatalf("first chunk should have rolled back to input: %q", got)
	}
	if got := next.Manifest.Chunks[1].Output; got != originalRight {
		t.Fatalf("second chunk should have rolled back to input: %q", got)
	}
	if got := next.MergedOutput; got != state.ParsedText {
		t.Fatalf("merged output should match original text after rollback: %q", got)
	}
	if next.RecoveryJustification == "" {
		t.Fatal("expected rollback justification to be recorded")
	}
}

func TestMergeEnablesEarlyStopWhenCalibratedScoreMeetsThreshold(t *testing.T) {
	t.Parallel()

	rewritten := "先记房间号。随后，Mina把封条编号、取样时间和交接人写进纸质记录，再让Chen复核签名。好。19:14 再补一条备注。"
	pipeline := &Pipeline{gate: nodes.NewQualityGate()}
	state := &State{
		Input:      workflowInputForTest(1),
		ParsedText: rewritten,
		Manifest: &domain.Manifest{
			ID:             uuid.New(),
			RoundID:        uuid.New(),
			ChunkMetric:    domain.ChunkMetricChar,
			ParagraphCount: 1,
			ChunkCount:     1,
			Paragraphs: []domain.ParagraphMapping{
				{ParagraphIndex: 0, ChunkIDs: []string{"p0_c0"}, Text: rewritten},
			},
			Chunks: []domain.Chunk{
				{ID: "p0_c0", ParagraphIndex: 0, ChunkIndex: 0, Text: rewritten},
			},
		},
		FinalOutputs: []string{rewritten},
		Reports: []domain.QualityReport{
			{ChunkID: "p0_c0", AllPassed: true, Checks: []domain.CheckResult{{Type: domain.CheckEmpty, Passed: true}}},
		},
	}

	next, err := pipeline.merge(context.Background(), state)
	if err != nil {
		t.Fatalf("merge returned error: %v", err)
	}
	if next.ChunkScore == nil {
		t.Fatal("expected merged score to be stored")
	}
	if !domain.MeetsRoundStopTarget(next.ChunkScore) {
		t.Fatalf("test setup invalid: expected score to meet stop target, got %+v", next.ChunkScore)
	}
	if !next.StopAfterRound {
		t.Fatal("expected stop-after-round flag to be enabled")
	}
}

func workflowInputForTest(round int) Input {
	return Input{
		Session: domain.Session{ID: uuid.New(), PromptProfile: "cn"},
		Round:   domain.Round{ID: uuid.New(), Number: round},
	}
}
