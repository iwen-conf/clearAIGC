package scorer

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

var sentenceSplitPattern = regexp.MustCompile(`(?m)([^。！？!?;\n]+[。！？!?;]?|[^\n]+$)`)

type sentenceSegment struct {
	Index int
	Text  string
	Start int
	End   int
}

func splitSentences(text string) []sentenceSegment {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	matches := sentenceSplitPattern.FindAllStringIndex(text, -1)
	out := make([]sentenceSegment, 0, len(matches))
	for idx, match := range matches {
		segment := strings.TrimSpace(text[match[0]:match[1]])
		if segment == "" {
			continue
		}
		out = append(out, sentenceSegment{
			Index: idx,
			Text:  segment,
			Start: match[0],
			End:   match[1],
		})
	}
	return out
}

func sentenceLengths(text string) []float64 {
	segments := splitSentences(text)
	lengths := make([]float64, 0, len(segments))
	for _, segment := range segments {
		lengths = append(lengths, float64(utf8.RuneCountInString(segment.Text)))
	}
	return lengths
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func variance(values []float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	avg := mean(values)
	sum := 0.0
	for _, value := range values {
		diff := value - avg
		sum += diff * diff
	}
	return sum / float64(len(values))
}

func normalizeClamp(value, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return math.Max(0, math.Min(1, value/max))
}

func collectNGramTerms(text string) []string {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return nil
	}

	terms := make([]string, 0, len(strings.Fields(text)))
	for _, term := range domain.IdentifyAIFeatures(text) {
		normalized := strings.TrimSpace(strings.ToLower(term))
		if normalized != "" {
			terms = append(terms, normalized)
		}
	}
	sort.Strings(terms)
	return terms
}

type ReadabilityMetrics struct {
	SentenceCount         int
	AverageSentenceLength float64
	SentenceStdDev        float64
	ShortSentenceRatio    float64
	LongSentenceRatio     float64
	Penalty               float64
}

func EvaluateReadability(text string) ReadabilityMetrics {
	lengths := sentenceLengths(text)
	if len(lengths) == 0 {
		return ReadabilityMetrics{}
	}

	avg := mean(lengths)
	varianceValue := variance(lengths)
	stddev := math.Sqrt(varianceValue)
	shortCount := 0.0
	longCount := 0.0
	for _, length := range lengths {
		switch {
		case length <= 8:
			shortCount++
		case length >= 42:
			longCount++
		}
	}

	penalty := 0.0
	switch {
	case avg > 42:
		penalty += 0.18
	}
	if varianceValue < 18 && len(lengths) >= 3 {
		penalty += 0.12
	}
	if avg < 6 && len(lengths) >= 4 {
		penalty += 0.08
	}
	shortRatio := shortCount / float64(len(lengths))
	longRatio := longCount / float64(len(lengths))
	if shortRatio > 0.60 && len(lengths) >= 4 {
		penalty += 0.10
	}
	if longRatio > 0.40 && len(lengths) >= 3 {
		penalty += 0.12
	}

	return ReadabilityMetrics{
		SentenceCount:         len(lengths),
		AverageSentenceLength: avg,
		SentenceStdDev:        stddev,
		ShortSentenceRatio:    shortRatio,
		LongSentenceRatio:     longRatio,
		Penalty:               math.Max(0, math.Min(1, penalty)),
	}
}

func readabilityPenalty(text string) float64 {
	return EvaluateReadability(text).Penalty
}

func ShouldFlagReadabilityRegression(input, output ReadabilityMetrics, threshold float64) bool {
	if output.SentenceCount < 2 {
		return false
	}
	if output.Penalty >= 0.70 {
		return true
	}
	if threshold <= 0 {
		threshold = 0.24
	}
	if output.Penalty < threshold {
		return false
	}
	if input.SentenceCount < 2 {
		return true
	}
	return output.Penalty-input.Penalty >= 0.08
}

func BuildReadabilityReason(input, output ReadabilityMetrics) string {
	return fmt.Sprintf(
		"Readability regressed; input(avg=%s, std=%s, short=%s, long=%s, penalty=%s), output(avg=%s, std=%s, short=%s, long=%s, penalty=%s). Restore more natural sentence rhythm without changing facts.",
		formatMetricFloat(input.AverageSentenceLength),
		formatMetricFloat(input.SentenceStdDev),
		formatMetricFloat(input.ShortSentenceRatio),
		formatMetricFloat(input.LongSentenceRatio),
		formatMetricFloat(input.Penalty),
		formatMetricFloat(output.AverageSentenceLength),
		formatMetricFloat(output.SentenceStdDev),
		formatMetricFloat(output.ShortSentenceRatio),
		formatMetricFloat(output.LongSentenceRatio),
		formatMetricFloat(output.Penalty),
	)
}

func formatMetricFloat(value float64) string {
	return fmt.Sprintf("%.2f", value)
}
