package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/domain/scorer"
)

type RewriteGuidance struct {
	Text             string
	RoundPrompt      string
	RiskScore        float64
	TargetRiskScore  float64
	SignalSummary    string
	Score            domain.AIScore
	RiskFeatures     []string
	ForbiddenPhrases []string
}

func buildRewriteGuidance(text, roundPrompt string) RewriteGuidance {
	return buildRewriteGuidanceWithScorer(nil, text, roundPrompt)
}

type localSentenceScorer interface {
	ScoreLocal(ctx context.Context, text string) (domain.AIScore, error)
}

func buildRewriteGuidanceWithScorer(textScorer domain.AIScorer, text, roundPrompt string) RewriteGuidance {
	if textScorer == nil {
		textScorer = scorer.NewComposite()
	}
	score, err := scoreForGuidance(textScorer, text)
	if err != nil {
		score = domain.AIScore{
			Total:          domain.EstimateAIRate(text),
			Detector:       domain.AIRateDetectorName,
			Features:       domain.IdentifyAIFeatures(text),
			Forbidden:      domain.ForbiddenAIPhrases(text),
			ThresholdGreen: 0.33,
			ThresholdRed:   0.66,
		}
	}
	return RewriteGuidance{
		Text:             text,
		RoundPrompt:      strings.TrimSpace(roundPrompt),
		RiskScore:        score.Total,
		TargetRiskScore:  score.Target,
		SignalSummary:    scorer.BuildSignalSummary(score),
		Score:            score,
		RiskFeatures:     score.Features,
		ForbiddenPhrases: score.Forbidden,
	}
}

func scoreForGuidance(textScorer domain.AIScorer, text string) (domain.AIScore, error) {
	if local, ok := textScorer.(localSentenceScorer); ok {
		return local.ScoreLocal(context.Background(), text)
	}
	return textScorer.Score(context.Background(), text)
}

func formatRewritePrompt(basePrompt string, guidance RewriteGuidance) string {
	sections := []string{basePrompt}
	if guidance.RoundPrompt != "" {
		sections = append(sections, "[ROUND OBJECTIVE]\n"+guidance.RoundPrompt)
	}
	sections = append(sections, fmt.Sprintf(
		"[HEURISTIC RISK CONTEXT]\n- Detector: %s\n- Current risk score: %.2f\n- Target risk score: <= %.2f\n- Signal summary: %s\n- Remove these patterns first: %s",
		domain.AIRateDetectorName,
		guidance.RiskScore,
		guidance.TargetRiskScore,
		guidance.SignalSummary,
		formatRiskFeatures(guidance.RiskFeatures),
	))
	if len(guidance.ForbiddenPhrases) > 0 {
		sections = append(sections, "[FORBIDDEN PHRASES]\n"+formatBulletList(guidance.ForbiddenPhrases))
	}
	sections = append(sections, "[INPUT TEXT]\n"+guidance.Text)
	return strings.Join(sections, "\n\n")
}

func formatRiskFeatures(features []string) string {
	if len(features) == 0 {
		return "none detected"
	}
	return strings.Join(features, ", ")
}

func formatBulletList(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}
