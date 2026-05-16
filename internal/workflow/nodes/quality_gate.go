package nodes

import (
	"context"
	"regexp"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/domain/scorer"
)

type QualityGateOption func(*QualityGate)

type QualityGate struct {
	disallowed          []*regexp.Regexp
	semantic            scorer.SemanticSimilarity
	semanticThreshold   float64
	readabilityMaxScore float64
}

func NewQualityGate(opts ...QualityGateOption) *QualityGate {
	patterns := make([]*regexp.Regexp, 0, len(domain.DisallowedPatterns))
	for _, pattern := range domain.DisallowedPatterns {
		patterns = append(patterns, regexp.MustCompile(pattern))
	}
	gate := &QualityGate{
		disallowed:          patterns,
		semanticThreshold:   0.82,
		readabilityMaxScore: 0.24,
	}
	for _, opt := range opts {
		opt(gate)
	}
	return gate
}

func (g *QualityGate) Check(input, output string) domain.QualityReport {
	return g.CheckWithContext(context.Background(), input, output)
}

func WithSemanticSimilarity(client scorer.SemanticSimilarity, threshold float64) QualityGateOption {
	return func(g *QualityGate) {
		g.semantic = client
		if threshold > 0 {
			g.semanticThreshold = threshold
		}
	}
}

func WithReadabilityMaxScore(maxScore float64) QualityGateOption {
	return func(g *QualityGate) {
		if maxScore > 0 {
			g.readabilityMaxScore = maxScore
		}
	}
}

func (g *QualityGate) CheckWithContext(ctx context.Context, input, output string) domain.QualityReport {
	checks := []domain.CheckResult{
		g.checkEmpty(output),
		g.checkDisallowed(output),
		g.checkMarkdown(output),
		g.checkExpansion(input, output),
		g.checkFactInvariants(input, output),
		g.checkStructureBreak(input, output),
		g.checkNamedEntityDrift(input, output),
		g.checkSemanticAnchorDrift(ctx, input, output),
		g.checkTerminologyDrift(input, output),
		g.checkLowBurstiness(input, output),
		g.checkReadabilityDrift(input, output),
		g.checkAIRateElevated(input, output),
	}

	report := domain.QualityReport{
		Checks:       checks,
		AllPassed:    true,
		FailedChecks: make([]domain.CheckType, 0, 2),
	}
	for _, check := range checks {
		if check.Passed {
			continue
		}
		report.AllPassed = false
		report.FailedChecks = append(report.FailedChecks, check.Type)
	}
	return report
}

func (g *QualityGate) Stats(reports []domain.QualityReport) *domain.QualityStats {
	stats := &domain.QualityStats{
		TotalChunks: len(reports),
		ByCheckType: map[domain.CheckType]int{},
	}
	for _, report := range reports {
		switch {
		case report.AllPassed && report.Recovered:
			stats.RecoveredChunks++
		case report.AllPassed:
			stats.PassedChunks++
		default:
			stats.FailedChunks++
		}
		for _, check := range report.Checks {
			if !check.Passed {
				stats.ByCheckType[check.Type]++
			}
		}
	}
	if stats.TotalChunks > 0 {
		stats.PassRate = float64(stats.PassedChunks+stats.RecoveredChunks) / float64(stats.TotalChunks)
		if stats.RecoveredChunks+stats.FailedChunks > 0 {
			stats.RecoveryRate = float64(stats.RecoveredChunks) / float64(stats.RecoveredChunks+stats.FailedChunks)
		}
	}
	return stats
}

func (g *QualityGate) Score(stats *domain.QualityStats) int {
	if stats == nil || stats.TotalChunks == 0 {
		return 0
	}
	score := 70
	score -= stats.FailedChunks * 12
	score -= stats.RecoveredChunks * 4
	if score < 0 {
		return 0
	}
	return score
}

func (g *QualityGate) checkEmpty(output string) domain.CheckResult {
	output = strings.TrimSpace(output)
	if output == "" {
		return domain.CheckResult{Type: domain.CheckEmpty, Passed: false, Reason: "The model returned no text."}
	}
	return domain.CheckResult{Type: domain.CheckEmpty, Passed: true}
}

func (g *QualityGate) checkDisallowed(output string) domain.CheckResult {
	for _, pattern := range g.disallowed {
		if pattern.MatchString(output) {
			return domain.CheckResult{Type: domain.CheckDisallowedPattern, Passed: false, Reason: "The rewritten text still contains template wording."}
		}
	}
	return domain.CheckResult{Type: domain.CheckDisallowedPattern, Passed: true}
}

func (g *QualityGate) checkMarkdown(output string) domain.CheckResult {
	if strings.Contains(output, "```") || regexp.MustCompile(`(?m)^\s*#{1,6}\s+`).MatchString(output) {
		return domain.CheckResult{Type: domain.CheckMarkdownInjection, Passed: false, Reason: "The model added formatting instead of plain text."}
	}
	return domain.CheckResult{Type: domain.CheckMarkdownInjection, Passed: true}
}

func (g *QualityGate) checkExpansion(input, output string) domain.CheckResult {
	in := len([]rune(strings.TrimSpace(input)))
	out := len([]rune(strings.TrimSpace(output)))
	if in == 0 {
		return domain.CheckResult{Type: domain.CheckAbnormalExpansion, Passed: true}
	}
	if out > in*2+200 {
		return domain.CheckResult{Type: domain.CheckAbnormalExpansion, Passed: false, Reason: "The rewritten text expanded far beyond the source passage."}
	}
	return domain.CheckResult{Type: domain.CheckAbnormalExpansion, Passed: true}
}

func (g *QualityGate) checkFactInvariants(input, output string) domain.CheckResult {
	missing, added := diffFactTokens(collectFactTokens(input), collectFactTokens(output))
	if len(missing) == 0 && len(added) == 0 {
		return domain.CheckResult{Type: domain.CheckFactInvariant, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckFactInvariant,
		Passed: false,
		Reason: buildFactInvariantReason(missing, added),
	}
}

func (g *QualityGate) checkStructureBreak(input, output string) domain.CheckResult {
	flagged, reason := evaluateStructureBreak(input, output)
	if !flagged {
		return domain.CheckResult{Type: domain.CheckStructureBreak, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckStructureBreak,
		Passed: false,
		Reason: reason,
	}
}

func (g *QualityGate) checkNamedEntityDrift(input, output string) domain.CheckResult {
	missing, added := diffNamedEntities(collectNamedEntities(input), collectNamedEntities(output))
	if len(missing) == 0 && len(added) == 0 {
		return domain.CheckResult{Type: domain.CheckNamedEntityDrift, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckNamedEntityDrift,
		Passed: false,
		Reason: buildNamedEntityReason(missing, added),
	}
}

func (g *QualityGate) checkSemanticAnchorDrift(ctx context.Context, input, output string) domain.CheckResult {
	if g.semantic != nil {
		if result, err := g.semantic.Compare(ctx, input, output); err == nil {
			if result.Score >= g.semanticThreshold {
				return domain.CheckResult{Type: domain.CheckSemanticAnchorDrift, Passed: true}
			}
			return domain.CheckResult{
				Type:   domain.CheckSemanticAnchorDrift,
				Passed: false,
				Reason: buildSemanticSimilarityReason(result.Score, g.semanticThreshold, result.Mode),
			}
		}
	}

	flagged, reason := evaluateSemanticAnchorDrift(input, output)
	if !flagged {
		return domain.CheckResult{Type: domain.CheckSemanticAnchorDrift, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckSemanticAnchorDrift,
		Passed: false,
		Reason: reason,
	}
}

func (g *QualityGate) checkTerminologyDrift(input, output string) domain.CheckResult {
	missing, added := diffProtectedTerms(collectProtectedTerms(input), collectProtectedTerms(output))
	if len(missing) == 0 && len(added) == 0 {
		return domain.CheckResult{Type: domain.CheckTerminologyDrift, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckTerminologyDrift,
		Passed: false,
		Reason: buildTerminologyDriftReason(missing, added),
	}
}

func (g *QualityGate) checkLowBurstiness(input, output string) domain.CheckResult {
	inputRhythm := analyzeSentenceRhythm(input)
	outputRhythm := analyzeSentenceRhythm(output)
	if !shouldFlagLowBurstiness(inputRhythm, outputRhythm) {
		return domain.CheckResult{Type: domain.CheckLowBurstiness, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckLowBurstiness,
		Passed: false,
		Reason: buildLowBurstinessReason(inputRhythm, outputRhythm),
	}
}

func (g *QualityGate) checkReadabilityDrift(input, output string) domain.CheckResult {
	inputMetrics := scorer.EvaluateReadability(input)
	outputMetrics := scorer.EvaluateReadability(output)
	if !scorer.ShouldFlagReadabilityRegression(inputMetrics, outputMetrics, g.readabilityMaxScore) {
		return domain.CheckResult{Type: domain.CheckReadabilityDrift, Passed: true}
	}
	return domain.CheckResult{
		Type:   domain.CheckReadabilityDrift,
		Passed: false,
		Reason: scorer.BuildReadabilityReason(inputMetrics, outputMetrics),
	}
}

func (g *QualityGate) checkAIRateElevated(input, output string) domain.CheckResult {
	inRate := domain.EstimateAIRate(input)
	outRate := domain.EstimateAIRate(output)

	if outRate > inRate || outRate > 0.5 {
		features := domain.IdentifyAIFeatures(output)
		if len(features) > 0 {
			reason := "High AI-likeness score. MUST REMOVE these specific patterns: " + strings.Join(features, ", ")
			return domain.CheckResult{Type: domain.CheckAIRateElevated, Passed: false, Reason: reason}
		}
	}
	return domain.CheckResult{Type: domain.CheckAIRateElevated, Passed: true}
}
