package nodes

import (
	"context"
	"strings"
	"testing"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/domain/scorer"
)

type fakeSemanticSimilarity struct {
	score float64
	mode  string
	err   error
}

func (f fakeSemanticSimilarity) Name() string {
	return "fake-embeddings"
}

func (f fakeSemanticSimilarity) Compare(_ context.Context, input, output string) (scorer.SimilarityResult, error) {
	if f.err != nil {
		return scorer.SimilarityResult{}, f.err
	}
	return scorer.SimilarityResult{Score: f.score, Mode: f.mode}, nil
}

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

func TestQualityGateDetectsFactInvariantDrift(t *testing.T) {
	gate := NewQualityGate()
	input := "研究于 2024年4月19日 完成，保留率为 35%，预算为 ￥1200，并引用了 [12]。"
	output := "研究于 2025年4月19日 完成，保留率为 38%，预算为 ￥1500，并引用了 [13]。"

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected fact invariant check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckFactInvariant {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected fact invariant check to fail")
		}
		if check.Reason == "" {
			t.Fatalf("expected failure reason for fact invariant drift")
		}
	}
	if !found {
		t.Fatalf("expected fact invariant check to be present")
	}
}

func TestQualityGatePassesWhenFactMarkersArePreserved(t *testing.T) {
	gate := NewQualityGate()
	input := "The 2024-05-01 report kept a 37.5% retention rate, a $4.5 budget note, and citation [12]."
	output := "Citation [12] is retained, the retention rate remains 37.5%, the budget note stays $4.5, and the report date is still 2024-05-01."

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckFactInvariant && !check.Passed {
			t.Fatalf("expected fact invariant check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateDetectsStructureBreak(t *testing.T) {
	gate := NewQualityGate()
	input := "1. Collect the source logs\n2. Verify the checksum table\n3. Export the audit trail"
	output := "First, collect the source logs, then verify the checksum table, and finally export the audit trail."

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected structure break check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckStructureBreak {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected structure break check to fail")
		}
		if !strings.Contains(check.Reason, "expected markers") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
	}
	if !found {
		t.Fatalf("expected structure break check to be present")
	}
}

func TestQualityGatePassesWhenStructureMarkersStayAligned(t *testing.T) {
	gate := NewQualityGate()
	input := "Section 1 Overview\n- Keep the original IDs\n- Preserve the archive order"
	output := "Section 1 Overview\n- Retain the original IDs\n- Preserve the archive order"

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckStructureBreak && !check.Passed {
			t.Fatalf("expected structure break check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateDetectsNamedEntityDrift(t *testing.T) {
	gate := NewQualityGate()
	input := "Researchers from New York University and the United Nations Environment Programme reviewed the archive."
	output := "Researchers from a local university and an international programme reviewed the archive."

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected named entity check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckNamedEntityDrift {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected named entity check to fail")
		}
		if !strings.Contains(check.Reason, "New York University") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
	}
	if !found {
		t.Fatalf("expected named entity check to be present")
	}
}

func TestQualityGatePassesWhenNamedEntitiesStayIntact(t *testing.T) {
	gate := NewQualityGate()
	input := "清华大学 reused the OpenAI Responses API with the United Nations Environment Programme dataset."
	output := "The United Nations Environment Programme dataset was still processed through the OpenAI Responses API, and 清华大学 remained the same source team."

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckNamedEntityDrift && !check.Passed {
			t.Fatalf("expected named entity check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateDetectsSemanticAnchorDrift(t *testing.T) {
	gate := NewQualityGate()
	input := "After the migration corrupted customer invoices and settlement mappings, manual reviewers restored the payment ledger and documented the archive repair."
	output := "Later that week, the team celebrated the release, discussed intern onboarding, and prepared a training plan for the next quarter."

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected semantic anchor check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckSemanticAnchorDrift {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected semantic anchor check to fail")
		}
		if !strings.Contains(check.Reason, "customer") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
	}
	if !found {
		t.Fatalf("expected semantic anchor check to be present")
	}
}

func TestQualityGatePassesWhenSemanticAnchorsStayAligned(t *testing.T) {
	gate := NewQualityGate()
	input := "After the migration corrupted customer invoices and settlement mappings, manual reviewers restored the payment ledger and documented the archive repair."
	output := "Manual reviewers documented the archive repair after the migration damaged customer invoices, settlement mappings, and the payment ledger."

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckSemanticAnchorDrift && !check.Passed {
			t.Fatalf("expected semantic anchor check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateUsesEmbeddingSimilarityWhenConfigured(t *testing.T) {
	gate := NewQualityGate(WithSemanticSimilarity(fakeSemanticSimilarity{
		score: 0.41,
		mode:  "openai-embeddings",
	}, 0.82))

	report := gate.Check("The migration kept the customer ledger intact.", "The migration kept the customer ledger intact.")
	for _, check := range report.Checks {
		if check.Type != domain.CheckSemanticAnchorDrift {
			continue
		}
		if check.Passed {
			t.Fatalf("expected embedding-backed semantic check to fail")
		}
		if !strings.Contains(check.Reason, "source=openai-embeddings") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
		return
	}
	t.Fatalf("expected semantic check to be present")
}

func TestQualityGateDetectsSemanticAnchorDriftInChineseFallback(t *testing.T) {
	gate := NewQualityGate()
	input := "迁移完成后，团队逐项核对订单状态、退款记录和库存日志，确认系统数据没有缺项。"
	output := "会议结束后，成员讨论了培训安排和下周值班计划，随后离开办公室。"

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type != domain.CheckSemanticAnchorDrift {
			continue
		}
		if check.Passed {
			t.Fatalf("expected chinese semantic anchor fallback to fail")
		}
		return
	}
	t.Fatalf("expected semantic anchor check to be present")
}

func TestQualityGateDetectsTerminologyDrift(t *testing.T) {
	gate := NewQualityGate()
	input := "The pipeline stores records in PostgreSQL, keeps hot keys in Redis, serves an HTTP API, and loads bge-m3 vectors from `internal/workflow/nodes/quality_gate.go`."
	output := "The pipeline stores records in MySQL, keeps hot keys in memory, serves a web interface, and loads dense vectors from the frontend helper."

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected terminology drift check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckTerminologyDrift {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected terminology drift check to fail")
		}
		if !strings.Contains(check.Reason, "PostgreSQL") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
	}
	if !found {
		t.Fatalf("expected terminology drift check to be present")
	}
}

func TestQualityGatePassesWhenProtectedTermsStayIntact(t *testing.T) {
	gate := NewQualityGate()
	input := "The adapter calls OpenAI through GPT-4o, caches metadata in Redis, and reads `internal/workflow/nodes/quality_gate.go` before shipping the API response."
	output := "Before shipping the API response, the adapter still calls OpenAI through GPT-4o, still caches metadata in Redis, and still reads internal/workflow/nodes/quality_gate.go."

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckTerminologyDrift && !check.Passed {
			t.Fatalf("expected terminology drift check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateDetectsLowBurstinessRegression(t *testing.T) {
	gate := NewQualityGate()
	input := "The archive was rebuilt overnight. Several records still needed manual review before sunrise. By noon, the team had already written a much longer incident note that explained where the imported metadata went wrong and which recovery steps were still pending. One operator simply marked the risky rows and moved on."
	output := "The archive team continued the review through the night. Each record then moved through the same routine checklist. Every operator logged the same kind of follow-up note. The shift ended with one more routine verification step."

	inputRhythm := analyzeSentenceRhythm(input)
	outputRhythm := analyzeSentenceRhythm(output)
	if !shouldFlagLowBurstiness(inputRhythm, outputRhythm) {
		t.Fatalf(
			"test setup invalid: low burstiness heuristic not triggered; input=%s cv=%s range=%d output=%s cv=%s range=%d",
			formatSentenceLengths(inputRhythm.Lengths),
			formatFloat(inputRhythm.CV),
			inputRhythm.Range,
			formatSentenceLengths(outputRhythm.Lengths),
			formatFloat(outputRhythm.CV),
			outputRhythm.Range,
		)
	}

	report := gate.Check(input, output)
	if report.AllPassed {
		t.Fatalf("expected low burstiness check to fail")
	}

	found := false
	for _, check := range report.Checks {
		if check.Type != domain.CheckLowBurstiness {
			continue
		}
		found = true
		if check.Passed {
			t.Fatalf("expected low burstiness check to fail")
		}
		if !strings.Contains(check.Reason, "Sentence rhythm became too uniform") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
	}
	if !found {
		t.Fatalf("expected low burstiness check to be present")
	}
}

func TestQualityGatePassesWhenSentenceVariationStaysHealthy(t *testing.T) {
	gate := NewQualityGate()
	input := "A short note starts the section. The second sentence is longer because it explains how the team traced the issue across several services and compared multiple snapshots before deciding what to restore. Then the report closes."
	output := "A short note opens the section. The second sentence stays much longer because it explains how the team traced the issue across several services, compared multiple snapshots, and decided what to restore before proceeding. Then the report closes."

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckLowBurstiness && !check.Passed {
			t.Fatalf("expected low burstiness check to pass, got reason: %s", check.Reason)
		}
	}
}

func TestQualityGateDetectsReadabilityRegression(t *testing.T) {
	gate := NewQualityGate()
	input := "先记房间号。随后，Mina把封条编号、取样时间和交接人写进纸质记录，再让Chen复核签名。好。19:14 再补一条备注。"
	output := "负责人在整个交接流程中持续记录房间号、封条编号、取样时间、签字人以及补充说明，并且把这些信息整理成若干句子长度几乎一致、节奏也基本一致的平直表述。负责人随后继续记录房间号、封条编号、取样时间、签字人以及补充说明，并且把这些信息整理成若干句子长度几乎一致、节奏也基本一致的平直表述。负责人最后再次记录房间号、封条编号、取样时间、签字人以及补充说明，并且把这些信息整理成若干句子长度几乎一致、节奏也基本一致的平直表述。"

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type != domain.CheckReadabilityDrift {
			continue
		}
		if check.Passed {
			t.Fatalf("expected readability check to fail")
		}
		if !strings.Contains(check.Reason, "Readability regressed") {
			t.Fatalf("unexpected reason: %s", check.Reason)
		}
		return
	}
	t.Fatalf("expected readability check to be present")
}

func TestQualityGatePassesHealthyReadability(t *testing.T) {
	gate := NewQualityGate()
	input := "先记房间号。随后，Mina把封条编号、取样时间和交接人写进纸质记录，再让Chen复核签名。好。19:14 再补一条备注。"
	output := "先记房间号。随后，Mina把封条编号、取样时间和交接人写进纸质记录，再让Chen复核签名。19:14 又补了一条简短备注。"

	report := gate.Check(input, output)
	for _, check := range report.Checks {
		if check.Type == domain.CheckReadabilityDrift && !check.Passed {
			t.Fatalf("expected readability check to pass, got reason: %s", check.Reason)
		}
	}
}
