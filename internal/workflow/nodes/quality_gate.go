package nodes

import (
	"regexp"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type QualityGate struct {
	disallowed []*regexp.Regexp
}

func NewQualityGate() *QualityGate {
	patterns := make([]*regexp.Regexp, 0, len(domain.DisallowedPatterns))
	for _, pattern := range domain.DisallowedPatterns {
		patterns = append(patterns, regexp.MustCompile(pattern))
	}
	return &QualityGate{disallowed: patterns}
}

func (g *QualityGate) Check(input, output string) domain.QualityReport {
	checks := []domain.CheckResult{
		g.checkEmpty(output),
		g.checkDisallowed(output),
		g.checkMarkdown(output),
		g.checkExpansion(input, output),
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
