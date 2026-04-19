package nodes

import (
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestQualityGateDetectsTemplateWording(t *testing.T) {
	gate := NewQualityGate()
	report := gate.Check("原文内容", "修改后：这里是结果")
	if report.AllPassed {
		t.Fatalf("expected report to fail")
	}
	if len(report.FailedChecks) == 0 {
		t.Fatalf("expected failed checks")
	}
}

func TestQualityGateScore(t *testing.T) {
	gate := NewQualityGate()
	stats := gate.Stats([]domain.QualityReport{
		{AllPassed: true},
		{AllPassed: true, Recovered: true},
		{AllPassed: false},
	})
	if stats.TotalChunks != 3 {
		t.Fatalf("unexpected total chunks: %d", stats.TotalChunks)
	}
	score := gate.Score(stats)
	if score <= 0 || score >= 70 {
		t.Fatalf("unexpected score: %d", score)
	}
}
