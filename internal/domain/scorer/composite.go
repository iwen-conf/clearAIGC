package scorer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type Composite struct {
	calibrator *Calibrator
	external   ExternalDetector
}

type CompositeOption func(*Composite)

func NewComposite(opts ...CompositeOption) *Composite {
	c := &Composite{
		calibrator: NewCalibrator(),
		external:   ExternalFallback{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Composite) Score(ctx context.Context, text string) (domain.AIScore, error) {
	return c.scoreText(ctx, text, true)
}

func (c *Composite) ScoreLocal(ctx context.Context, text string) (domain.AIScore, error) {
	return c.scoreText(ctx, text, false)
}

func WithCalibrator(calibrator *Calibrator) CompositeOption {
	return func(c *Composite) {
		if calibrator != nil {
			c.calibrator = calibrator
		}
	}
}

func WithExternalDetector(detector ExternalDetector) CompositeOption {
	return func(c *Composite) {
		if detector != nil {
			c.external = detector
		}
	}
}

func (c *Composite) scoreText(ctx context.Context, text string, useExternal bool) (domain.AIScore, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.AIScore{
			Detector:       domain.AIRateDetectorName,
			Signals:        []domain.ScoreSignal{},
			Features:       []string{},
			Forbidden:      []string{},
			ThresholdGreen: 0.33,
			ThresholdRed:   0.66,
		}, nil
	}

	lengths := sentenceLengths(text)
	burstinessRaw := 0.0
	if len(lengths) > 1 {
		avg := mean(lengths)
		if avg > 0 {
			burstinessRaw = math.Sqrt(variance(lengths)) / avg
		}
	}
	pplLikeRaw := estimatePerplexityLike(text)
	ngramRaw := estimateNGramAnomaly(text)
	internalTotal := math.Max(0, math.Min(1,
		0.34*normalizeClamp(1/pplLikeRaw, 0.5)+
			0.33*(1-normalizeClamp(burstinessRaw, 0.65))+
			0.33*ngramRaw,
	))

	externalRaw := domain.EstimateAIRate(text)
	scoreMode := "local_internal"
	externalStatus := domain.ExternalDetectorUnavailable
	if useExternal && c.external != nil {
		detectorMode := inferDetectorMode(c.external.Name())
		scoreMode = "offline_fallback"
		if score, err := c.external.Score(ctx, text); err == nil {
			externalRaw = score
			if c.calibrator != nil && detectorMode != "offline_fallback" && detectorMode != "offline" {
				c.calibrator.Observe(internalTotal, externalRaw, c.external.Name())
			}
			if c.calibrator != nil {
				scoreMode = c.calibrator.Mode()
			} else {
				scoreMode = "online_calibrated"
			}
			if detectorMode != "offline_fallback" && detectorMode != "offline" {
				externalStatus = domain.ExternalDetectorPassed
			}
		} else {
			scoreMode = "offline_fallback"
		}
	}

	signals := []domain.ScoreSignal{
		{
			Name:        "perplexity",
			Label:       "Perplexity",
			Value:       pplLikeRaw,
			Normalized:  normalizeClamp(1/pplLikeRaw, 0.5),
			Description: "Lower pseudo-perplexity implies more templated text.",
		},
		{
			Name:        "burstiness",
			Label:       "Burstiness",
			Value:       burstinessRaw,
			Normalized:  1 - normalizeClamp(burstinessRaw, 0.65),
			Description: "Low sentence-length variance is treated as higher risk.",
		},
		{
			Name:        "ngram",
			Label:       "n-gram anomaly",
			Value:       ngramRaw,
			Normalized:  ngramRaw,
			Description: "High-frequency template phrases and abstract terms.",
		},
		{
			Name:        "external",
			Label:       "Detector reference",
			Value:       externalRaw,
			Normalized:  externalRaw,
			Description: "Offline-aligned detector proxy.",
		},
	}

	total := 0.75*internalTotal + 0.25*signals[3].Normalized
	total = math.Max(0, math.Min(1, total))
	green := 0.33
	red := 0.66
	var calibrated *domain.CalibratedScore
	if c.calibrator != nil {
		green, red = c.calibrator.Thresholds()
		calibrated = c.calibrator.WithScoreMode(c.calibrator.Apply(internalTotal), scoreMode)
	}

	return domain.AIScore{
		Total:          total,
		Target:         deriveTarget(total),
		Detector:       domain.AIRateDetectorName,
		ExternalStatus: externalStatus,
		Signals:        signals,
		Features:       domain.IdentifyAIFeatures(text),
		Forbidden:      domain.ForbiddenAIPhrases(text),
		Calibrated:     calibrated,
		ThresholdGreen: green,
		ThresholdRed:   red,
	}, nil
}

func (c *Composite) ScoreSentences(ctx context.Context, text string) ([]domain.SentenceScore, error) {
	segments := splitSentences(text)
	if len(segments) == 0 {
		return nil, nil
	}

	out := make([]domain.SentenceScore, 0, len(segments))
	for _, segment := range segments {
		score, err := c.ScoreLocal(ctx, segment.Text)
		if err != nil {
			return nil, err
		}
		importance := score.Total * (1 + normalizeClamp(float64(len(segment.Text)), 120))
		out = append(out, domain.SentenceScore{
			Index:      segment.Index,
			Text:       segment.Text,
			Start:      segment.Start,
			End:        segment.End,
			Total:      score.Total,
			Importance: importance,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Importance == out[j].Importance {
			return out[i].Index < out[j].Index
		}
		return out[i].Importance > out[j].Importance
	})
	return out, nil
}

func estimatePerplexityLike(text string) float64 {
	risk := domain.EstimateAIRate(text)
	return math.Max(1.2, 5.8-(risk*4.2))
}

func estimateNGramAnomaly(text string) float64 {
	terms := collectNGramTerms(text)
	if len(terms) == 0 {
		return 0
	}
	unique := map[string]struct{}{}
	for _, term := range terms {
		unique[term] = struct{}{}
	}
	return normalizeClamp(float64(len(unique)), 10)
}

func deriveTarget(total float64) float64 {
	switch {
	case total <= 0:
		return 0
	case total <= 0.2:
		return math.Max(0, total-0.04)
	default:
		return math.Max(0.18, total*0.72)
	}
}

func BuildSignalSummary(score domain.AIScore) string {
	if len(score.Signals) == 0 {
		return "no signals"
	}
	parts := make([]string, 0, len(score.Signals))
	for _, signal := range score.Signals {
		parts = append(parts, fmt.Sprintf("%s=%.2f", signal.Name, signal.Value))
	}
	return strings.Join(parts, ", ")
}
