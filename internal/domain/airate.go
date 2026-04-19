package domain

import (
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const AIRateDetectorName = "启发式规则"

var aiStrongPatterns = []*regexp.Regexp{
	regexp.MustCompile(`在当前[^。；\n]{0,24}背景下`),
	regexp.MustCompile(`从(?:实际情况|方法层面|整体上)来看`),
	regexp.MustCompile(`如果从[^，。]{0,18}(?:层面|角度|维度)进行分析`),
	regexp.MustCompile(`进一步(?:来说|提出|分析)`),
	regexp.MustCompile(`基于以上认识`),
	regexp.MustCompile(`由此可见`),
	regexp.MustCompile(`综合来看`),
	regexp.MustCompile(`总而言之`),
	regexp.MustCompile(`换句话说`),
	regexp.MustCompile(`具有(?:现实意义|实践价值)`),
	regexp.MustCompile(`值得在后续实践中持续探索`),
	regexp.MustCompile(`(?i)\bin the context of\b`),
	regexp.MustCompile(`(?i)\bfrom the perspective of\b`),
	regexp.MustCompile(`(?i)\bit is worth noting that\b`),
	regexp.MustCompile(`(?i)\bin conclusion\b`),
	regexp.MustCompile(`(?i)\bin summary\b`),
}

var aiTransitionPhrases = []string{
	"与此同时",
	"进一步来说",
	"进一步提出",
	"由此可见",
	"综合来看",
	"总而言之",
	"换句话说",
	"基于以上认识",
	"首先",
	"其次",
	"最后",
	"当然",
	"in conclusion",
	"in summary",
	"at the same time",
	"therefore",
	"moreover",
}

var aiAbstractTerms = []string{
	"意义",
	"价值",
	"趋势",
	"机制",
	"课题",
	"层面",
	"环节",
	"效果",
	"前景",
	"任务",
	"路径",
	"维度",
	"能力",
	"形态",
	"总体趋势",
	"实践价值",
	"现实意义",
}

// EstimateAIRate returns a heuristic 0..1 AI-likeness score for an input chunk.
func EstimateAIRate(text string) float64 {
	text = strings.TrimSpace(normalizeAIRateText(text))
	if text == "" {
		return 0
	}

	score := 0.0

	patternHits := 0
	for _, pattern := range aiStrongPatterns {
		if pattern.MatchString(text) {
			patternHits++
		}
	}
	score += math.Min(0.56, float64(patternHits)*0.14)

	transitionHits := 0
	lowerText := strings.ToLower(text)
	for _, phrase := range aiTransitionPhrases {
		if isASCIIPhrase(phrase) {
			transitionHits += strings.Count(lowerText, phrase)
			continue
		}
		transitionHits += strings.Count(text, phrase)
	}
	if transitionHits >= 2 {
		score += math.Min(0.18, float64(transitionHits-1)*0.04)
	}

	abstractHits := 0
	for _, term := range aiAbstractTerms {
		abstractHits += strings.Count(text, term)
	}
	if abstractHits >= 4 {
		score += math.Min(0.16, float64(abstractHits-3)*0.02)
	}

	moreHits := strings.Count(text, "更加")
	if moreHits >= 2 {
		score += math.Min(0.09, float64(moreHits-1)*0.03)
	}

	if hasLongEnumerativeSentence(text) {
		score += 0.08
	}

	if utf8.RuneCountInString(text) >= 180 && transitionHits >= 2 {
		score += 0.05
	}

	return math.Max(0, math.Min(1, score))
}

func normalizeAIRateText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

func isASCIIPhrase(phrase string) bool {
	for _, r := range phrase {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func hasLongEnumerativeSentence(text string) bool {
	parts := regexp.MustCompile(`[。！？!?;\n]+`).Split(text, -1)
	if len(parts) == 0 {
		return false
	}

	totalRunes := 0
	longSentences := 0
	commaHeavySentences := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		length := utf8.RuneCountInString(part)
		totalRunes += length
		if length >= 42 {
			longSentences++
		}
		commaCount := strings.Count(part, "，") + strings.Count(part, ",")
		if commaCount >= 3 {
			commaHeavySentences++
		}
	}
	if totalRunes == 0 {
		return false
	}
	return longSentences > 0 && commaHeavySentences > 0
}

// IdentifyAIFeatures extracts specific substrings and structural patterns that contribute to a high AI rate.
func IdentifyAIFeatures(text string) []string {
	text = strings.TrimSpace(normalizeAIRateText(text))
	if text == "" {
		return nil
	}

	var features []string

	for _, pattern := range aiStrongPatterns {
		if match := pattern.FindString(text); match != "" {
			features = append(features, match)
		}
	}

	lowerText := strings.ToLower(text)
	for _, phrase := range aiTransitionPhrases {
		target := text
		if isASCIIPhrase(phrase) {
			target = lowerText
		}
		if strings.Contains(target, phrase) {
			features = append(features, phrase)
		}
	}

	for _, term := range aiAbstractTerms {
		if strings.Contains(text, term) {
			features = append(features, term)
		}
	}

	if strings.Contains(text, "更加") {
		features = append(features, "更加")
	}

	if hasLongEnumerativeSentence(text) {
		features = append(features, "句式单一/包含过多逗号的长列举句")
	}

	return features
}
