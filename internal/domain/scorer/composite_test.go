package scorer

import (
	"context"
	"testing"
)

func TestCompositeScoreReturnsSignalsAndCalibration(t *testing.T) {
	s := NewComposite()
	score, err := s.Score(context.Background(), "在当前数字化协作的背景下，综合来看，这项工作具有现实意义。")
	if err != nil {
		t.Fatalf("Score returned error: %v", err)
	}
	if score.Total <= 0 {
		t.Fatalf("expected positive score, got %v", score.Total)
	}
	if len(score.Signals) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(score.Signals))
	}
	if score.Calibrated == nil {
		t.Fatal("expected calibrated score")
	}
	if score.Target <= 0 || score.Target >= score.Total {
		t.Fatalf("expected target to be below total, got total=%v target=%v", score.Total, score.Target)
	}
}

func TestCompositeSentenceScoringRanksRiskierSentenceFirst(t *testing.T) {
	s := NewComposite()
	scores, err := s.ScoreSentences(context.Background(), "普通句子更简洁。综合来看，这一方向具有现实意义和实践价值。")
	if err != nil {
		t.Fatalf("ScoreSentences returned error: %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("expected 2 sentence scores, got %d", len(scores))
	}
	if scores[0].Index != 1 {
		t.Fatalf("expected second sentence to rank first, got index=%d", scores[0].Index)
	}
}
