package nodes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type terminologyPattern struct {
	regex *regexp.Regexp
}

var terminologyPatterns = []terminologyPattern{
	{regex: regexp.MustCompile("`[^`\n]+`")},
	{regex: regexp.MustCompile(`(?:https?://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+|(?:\.{1,2}/|/)?[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+(?:\.[A-Za-z0-9._-]+)?)`)},
	{regex: regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:(?:[_:@.-])[A-Za-z0-9_]+)+\b`)},
}

var asciiWordPattern = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*\b`)

func collectProtectedTerms(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var (
		terms    []string
		occupied []factSpan
	)
	for _, pattern := range terminologyPatterns {
		indices := pattern.regex.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			if spansOverlap(occupied, idx[0], idx[1]) {
				continue
			}
			term := normalizeProtectedTerm(text[idx[0]:idx[1]])
			if term == "" {
				continue
			}
			terms = append(terms, term)
			occupied = append(occupied, factSpan{start: idx[0], end: idx[1]})
		}
	}

	for _, idx := range asciiWordPattern.FindAllStringIndex(text, -1) {
		if spansOverlap(occupied, idx[0], idx[1]) {
			continue
		}
		term := text[idx[0]:idx[1]]
		if !shouldProtectWord(text, idx[0], term) {
			continue
		}
		terms = append(terms, term)
		occupied = append(occupied, factSpan{start: idx[0], end: idx[1]})
	}

	sort.Strings(terms)
	return terms
}

func normalizeProtectedTerm(term string) string {
	term = strings.TrimSpace(term)
	if len(term) >= 2 && term[0] == '`' && term[len(term)-1] == '`' {
		term = term[1 : len(term)-1]
	}
	term = strings.Trim(term, "\"'“”‘’")
	term = strings.Join(strings.Fields(term), " ")
	term = strings.TrimRight(term, ".,;:!?)]}）】》")
	return strings.TrimSpace(term)
}

func shouldProtectWord(text string, start int, term string) bool {
	if len(term) < 2 {
		return false
	}
	if isUpperAcronym(term) {
		return true
	}
	if hasDigit(term) {
		return true
	}
	if hasMixedCase(term) {
		return true
	}
	return isTitleCase(term) && !isSentenceStart(text, start)
}

func isUpperAcronym(term string) bool {
	upperLetters := 0
	for _, r := range term {
		switch {
		case unicode.IsUpper(r):
			upperLetters++
		case unicode.IsDigit(r):
		default:
			return false
		}
	}
	return upperLetters >= 2
}

func hasDigit(term string) bool {
	for _, r := range term {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func hasMixedCase(term string) bool {
	if isTitleCase(term) {
		return false
	}

	hasUpper := false
	hasLower := false
	for _, r := range term {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsLower(r) {
			hasLower = true
		}
	}
	return hasUpper && hasLower
}

func isTitleCase(term string) bool {
	runes := []rune(term)
	if len(runes) < 2 || !unicode.IsUpper(runes[0]) {
		return false
	}
	for _, r := range runes[1:] {
		if !unicode.IsLower(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isSentenceStart(text string, start int) bool {
	cursor := start
	for cursor > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:cursor])
		if unicode.IsSpace(r) || isIgnorableLeadingRune(r) {
			cursor -= size
			continue
		}
		return isSentenceBoundary(r)
	}
	return true
}

func isIgnorableLeadingRune(r rune) bool {
	switch r {
	case '"', '\'', '`', '(', '[', '{', '<', '“', '”', '‘', '’', '（', '【', '《':
		return true
	default:
		return false
	}
}

func isSentenceBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '。', '！', '？', '；', '\n', '\r':
		return true
	default:
		return false
	}
}

func diffProtectedTerms(input, output []string) (missing, added []string) {
	inputCounts := make(map[string]int, len(input))
	for _, term := range input {
		inputCounts[term]++
	}

	outputCounts := make(map[string]int, len(output))
	for _, term := range output {
		outputCounts[term]++
	}

	for term, count := range inputCounts {
		diff := count - outputCounts[term]
		for i := 0; i < diff; i++ {
			missing = append(missing, term)
		}
	}
	for term, count := range outputCounts {
		diff := count - inputCounts[term]
		for i := 0; i < diff; i++ {
			added = append(added, term)
		}
	}

	sort.Strings(missing)
	sort.Strings(added)
	return missing, added
}

func buildTerminologyDriftReason(missing, added []string) string {
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing from output: "+summarizeProtectedTerms(missing))
	}
	if len(added) > 0 {
		parts = append(parts, "unexpected in output: "+summarizeProtectedTerms(added))
	}
	return fmt.Sprintf("Protected terminology changed; preserve technical terms, acronyms, identifiers, model names, and path-like tokens exactly (%s).", strings.Join(parts, "; "))
}

func summarizeProtectedTerms(terms []string) string {
	const maxTerms = 6
	if len(terms) <= maxTerms {
		return strings.Join(terms, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d more)", strings.Join(terms[:maxTerms], ", "), len(terms)-maxTerms)
}
