package nodes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type factPattern struct {
	name  string
	regex *regexp.Regexp
}

type factSpan struct {
	start int
	end   int
}

var factPatterns = []factPattern{
	{name: "square_citation", regex: regexp.MustCompile(`\[[0-9,\-\s]+\]`)},
	{name: "year_citation", regex: regexp.MustCompile(`[\(（][^()（）\n]{0,80}(?:19|20)\d{2}[a-zA-Z]?[^()（）\n]{0,80}[\)）]`)},
	{name: "iso_date", regex: regexp.MustCompile(`\b\d{4}[-/]\d{1,2}[-/]\d{1,2}\b`)},
	{name: "cn_date", regex: regexp.MustCompile(`\d{4}年\d{1,2}月(?:\d{1,2}日)?`)},
	{name: "currency_symbol", regex: regexp.MustCompile(`(?:[$¥￥€£]\s?\d+(?:[.,]\d+)?(?:[kKmM]|万|亿)?)`)},
	{name: "currency_word", regex: regexp.MustCompile(`\b\d+(?:[.,]\d+)?\s?(?:USD|RMB|CNY|EUR|GBP)\b|\d+(?:[.,]\d+)?\s?(?:美元|元|万元|亿元|人民币|欧元|英镑)`)},
	{name: "percentage", regex: regexp.MustCompile(`\b\d+(?:[.,]\d+)?\s?(?:%|％|percent)\b`)},
	{name: "year", regex: regexp.MustCompile(`\b(?:19|20)\d{2}\b`)},
	{name: "number", regex: regexp.MustCompile(`\b\d+(?:[.,]\d+)?\b`)},
}

func collectFactTokens(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var (
		tokens   []string
		occupied []factSpan
	)
	for _, pattern := range factPatterns {
		indices := pattern.regex.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			if spansOverlap(occupied, idx[0], idx[1]) {
				continue
			}
			token := normalizeFactToken(text[idx[0]:idx[1]])
			if token == "" {
				continue
			}
			tokens = append(tokens, token)
			occupied = append(occupied, factSpan{start: idx[0], end: idx[1]})
		}
	}
	sort.Strings(tokens)
	return tokens
}

func normalizeFactToken(token string) string {
	replacer := strings.NewReplacer(
		"％", "%",
		"￥", "¥",
		"（", "(",
		"）", ")",
		"，", ",",
	)
	token = replacer.Replace(token)
	token = strings.ToLower(token)
	token = strings.Join(strings.Fields(token), " ")
	return strings.TrimSpace(token)
}

func spansOverlap(occupied []factSpan, start, end int) bool {
	for _, span := range occupied {
		if start < span.end && end > span.start {
			return true
		}
	}
	return false
}

func diffFactTokens(input, output []string) (missing, added []string) {
	inputCounts := make(map[string]int, len(input))
	for _, token := range input {
		inputCounts[token]++
	}

	outputCounts := make(map[string]int, len(output))
	for _, token := range output {
		outputCounts[token]++
	}

	for token, count := range inputCounts {
		diff := count - outputCounts[token]
		for i := 0; i < diff; i++ {
			missing = append(missing, token)
		}
	}
	for token, count := range outputCounts {
		diff := count - inputCounts[token]
		for i := 0; i < diff; i++ {
			added = append(added, token)
		}
	}

	sort.Strings(missing)
	sort.Strings(added)
	return missing, added
}

func buildFactInvariantReason(missing, added []string) string {
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing from output: "+summarizeFactTokens(missing))
	}
	if len(added) > 0 {
		parts = append(parts, "unexpected in output: "+summarizeFactTokens(added))
	}
	return fmt.Sprintf("Critical factual markers changed; preserve numbers, dates, percentages, currencies, and citations exactly (%s).", strings.Join(parts, "; "))
}

func summarizeFactTokens(tokens []string) string {
	const maxTokens = 6
	if len(tokens) <= maxTokens {
		return strings.Join(tokens, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d more)", strings.Join(tokens[:maxTokens], ", "), len(tokens)-maxTokens)
}
