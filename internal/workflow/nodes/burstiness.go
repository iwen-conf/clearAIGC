package nodes

import (
	"fmt"
	"math"
	"regexp"
	gostrconv "strconv"
	"strings"
	"unicode/utf8"
)

var sentenceBoundaryPattern = regexp.MustCompile(`[。！？!?；;\.]+`)

type sentenceRhythm struct {
	Lengths []int
	Mean    float64
	StdDev  float64
	CV      float64
	Range   int
}

func analyzeSentenceRhythm(text string) sentenceRhythm {
	parts := sentenceBoundaryPattern.Split(strings.TrimSpace(text), -1)
	lengths := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lengths = append(lengths, utf8.RuneCountInString(part))
	}
	if len(lengths) == 0 {
		return sentenceRhythm{}
	}

	minLen := lengths[0]
	maxLen := lengths[0]
	sum := 0.0
	for _, length := range lengths {
		if length < minLen {
			minLen = length
		}
		if length > maxLen {
			maxLen = length
		}
		sum += float64(length)
	}
	mean := sum / float64(len(lengths))
	var variance float64
	for _, length := range lengths {
		delta := float64(length) - mean
		variance += delta * delta
	}
	variance /= float64(len(lengths))
	stddev := math.Sqrt(variance)
	cv := 0.0
	if mean > 0 {
		cv = stddev / mean
	}

	return sentenceRhythm{
		Lengths: lengths,
		Mean:    mean,
		StdDev:  stddev,
		CV:      cv,
		Range:   maxLen - minLen,
	}
}

func shouldFlagLowBurstiness(input, output sentenceRhythm) bool {
	if len(output.Lengths) < 3 {
		return false
	}

	extremelyUniform := len(output.Lengths) >= 4 && output.CV < 0.18 && output.Range <= 12
	if extremelyUniform {
		return true
	}

	if len(input.Lengths) < 3 {
		return false
	}

	return output.CV < 0.20 &&
		output.Range <= 14 &&
		input.CV-output.CV >= 0.12 &&
		input.Range-output.Range >= 10
}

func buildLowBurstinessReason(input, output sentenceRhythm) string {
	return fmt.Sprintf(
		"Sentence rhythm became too uniform. Input lengths=%s (cv=%s), output lengths=%s (cv=%s). Mix shorter and longer sentences without changing facts.",
		formatSentenceLengths(input.Lengths),
		formatFloat(input.CV),
		formatSentenceLengths(output.Lengths),
		formatFloat(output.CV),
	)
}

func formatSentenceLengths(lengths []int) string {
	if len(lengths) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(lengths))
	for _, length := range lengths {
		parts = append(parts, gostrconv.Itoa(length))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatFloat(value float64) string {
	return gostrconv.FormatFloat(value, 'f', 2, 64)
}
