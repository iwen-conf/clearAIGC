package nodes

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var semanticAnchorWordPattern = regexp.MustCompile(`\b[a-z][a-z0-9-]{3,}\b`)

var semanticAnchorStopwords = map[string]struct{}{
	"about": {}, "after": {}, "along": {}, "also": {}, "although": {}, "another": {}, "because": {},
	"before": {}, "being": {}, "between": {}, "could": {}, "does": {}, "during": {}, "each": {},
	"from": {}, "have": {}, "into": {}, "itself": {}, "later": {}, "might": {}, "more": {},
	"most": {}, "other": {}, "over": {}, "same": {}, "should": {}, "since": {}, "still": {},
	"such": {}, "than": {}, "that": {}, "their": {}, "there": {}, "these": {}, "those": {},
	"through": {}, "under": {}, "until": {}, "very": {}, "were": {}, "what": {}, "when": {},
	"which": {}, "while": {}, "with": {}, "within": {}, "would": {},
}

func collectSemanticAnchors(text string) []string {
	text = strings.ToLower(text)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	seen := map[string]struct{}{}
	anchors := make([]string, 0, 16)
	for _, match := range semanticAnchorWordPattern.FindAllString(text, -1) {
		if _, skip := semanticAnchorStopwords[match]; skip {
			continue
		}
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		anchors = append(anchors, match)
	}
	sort.Strings(anchors)
	return anchors
}

func normalizeSemanticText(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))
	for _, r := range strings.ToLower(text) {
		switch {
		case unicode.Is(unicode.Han, r):
			builder.WriteRune(r)
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func collectCharShingles(text string, size int) []string {
	if size <= 0 {
		return nil
	}

	runes := []rune(normalizeSemanticText(text))
	if len(runes) < size {
		return nil
	}

	seen := map[string]struct{}{}
	shingles := make([]string, 0, len(runes)-size+1)
	for i := 0; i <= len(runes)-size; i++ {
		shingle := string(runes[i : i+size])
		if _, ok := seen[shingle]; ok {
			continue
		}
		seen[shingle] = struct{}{}
		shingles = append(shingles, shingle)
	}
	sort.Strings(shingles)
	return shingles
}

func countSharedStrings(input, output []string) int {
	if len(input) == 0 || len(output) == 0 {
		return 0
	}

	set := make(map[string]struct{}, len(output))
	for _, item := range output {
		set[item] = struct{}{}
	}

	shared := 0
	for _, item := range input {
		if _, ok := set[item]; ok {
			shared++
		}
	}
	return shared
}

func diffStringSets(input, output []string) (missing, added []string) {
	inputSet := make(map[string]struct{}, len(input))
	for _, item := range input {
		inputSet[item] = struct{}{}
	}
	outputSet := make(map[string]struct{}, len(output))
	for _, item := range output {
		outputSet[item] = struct{}{}
	}

	for _, item := range input {
		if _, ok := outputSet[item]; !ok {
			missing = append(missing, item)
		}
	}
	for _, item := range output {
		if _, ok := inputSet[item]; !ok {
			added = append(added, item)
		}
	}
	sort.Strings(missing)
	sort.Strings(added)
	return missing, added
}

func evaluateSemanticAnchorDrift(input, output string) (bool, string) {
	inputAnchors := collectSemanticAnchors(input)
	outputAnchors := collectSemanticAnchors(output)
	sharedAnchors := countSharedStrings(inputAnchors, outputAnchors)
	anchorRatio := 1.0
	if len(inputAnchors) > 0 {
		anchorRatio = float64(sharedAnchors) / float64(len(inputAnchors))
	}

	inputShingles := collectCharShingles(input, 3)
	outputShingles := collectCharShingles(output, 3)
	sharedShingles := countSharedStrings(inputShingles, outputShingles)
	shingleContainment := 1.0
	if len(inputShingles) > 0 {
		shingleContainment = float64(sharedShingles) / float64(len(inputShingles))
	}

	missingAnchors, _ := diffStringSets(inputAnchors, outputAnchors)

	anchorGateTriggered := false
	if len(inputAnchors) >= 5 {
		requiredShared := int(math.Ceil(float64(len(inputAnchors)) * 0.40))
		if requiredShared < 2 {
			requiredShared = 2
		}
		anchorGateTriggered = sharedAnchors < requiredShared && anchorRatio < 0.45
	}

	shingleGateTriggered := len(inputShingles) >= 12 && shingleContainment < 0.12

	if !anchorGateTriggered && !shingleGateTriggered {
		return false, ""
	}

	return true, buildSemanticAnchorReason(sharedAnchors, len(inputAnchors), anchorRatio, shingleContainment, missingAnchors)
}

func buildSemanticAnchorReason(shared, total int, anchorRatio, shingleContainment float64, missing []string) string {
	return fmt.Sprintf(
		"Core semantic anchors drifted; preserve the main topic words and content anchors (shared anchors=%d/%d, anchor overlap=%s, char-shingle containment=%s, missing anchors=%s).",
		shared,
		total,
		formatFloat(anchorRatio),
		formatFloat(shingleContainment),
		summarizeSemanticAnchors(missing),
	)
}

func buildSemanticSimilarityReason(score, threshold float64, mode string) string {
	if strings.TrimSpace(mode) == "" {
		mode = "embedding"
	}
	return fmt.Sprintf(
		"Semantic similarity fell below threshold; preserve meaning and topic anchors (similarity=%s, threshold=%s, source=%s).",
		formatFloat(score),
		formatFloat(threshold),
		mode,
	)
}

func summarizeSemanticAnchors(anchors []string) string {
	if len(anchors) == 0 {
		return "none"
	}
	const maxAnchors = 6
	if len(anchors) <= maxAnchors {
		return strings.Join(anchors, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d more)", strings.Join(anchors[:maxAnchors], ", "), len(anchors)-maxAnchors)
}
