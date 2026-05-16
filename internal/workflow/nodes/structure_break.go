package nodes

import (
	"fmt"
	"regexp"
	"strings"
)

type structureKind string

const (
	structureKindOrdered structureKind = "ordered"
	structureKindBullet  structureKind = "bullet"
	structureKindHeading structureKind = "heading"
)

type structureLine struct {
	Kind      structureKind
	Signature string
	Preview   string
}

type structureProfile struct {
	LineCount  int
	Structured []structureLine
}

var (
	structureBulletPattern        = regexp.MustCompile(`^\s*([\-*+•▪◦●○■□◆◇])\s+`)
	structureOrderedArabicPattern = regexp.MustCompile(`^\s*(\d{1,3})[.)]\s+`)
	structureOrderedNestedPattern = regexp.MustCompile(`^\s*(\d{1,3}(?:\.\d{1,3}){1,4})\s+`)
	structureOrderedAlphaPattern  = regexp.MustCompile(`^\s*([A-Za-z])[.)]\s+`)
	structureOrderedRomanPattern  = regexp.MustCompile(`^\s*([IVXLCDMivxlcdm]+)[.)]\s+`)
	structureOrderedHanParen      = regexp.MustCompile(`^\s*[（(]([一二三四五六七八九十百千]+)[)）]\s*`)
	structureOrderedHanComma      = regexp.MustCompile(`^\s*([一二三四五六七八九十百千]+)、\s*`)
	structureHeadingZHPattern     = regexp.MustCompile(`^\s*(第[一二三四五六七八九十百千0-9]+[章节部分篇条款](?:\s*[-:：]\s*.*|\s+.*)?)\s*$`)
	structureHeadingENPattern     = regexp.MustCompile(`^\s*((?:Chapter|Section|Part)\s+[A-Za-z0-9IVXLCDMivxlcdm.-]+(?:\s+[A-Za-z0-9/&-]+){0,6})\s*$`)
)

func evaluateStructureBreak(input, output string) (bool, string) {
	inputProfile := collectStructureProfile(input)
	if !inputProfile.requiresProtection() {
		return false, ""
	}

	outputProfile := collectStructureProfile(output)
	if len(outputProfile.Structured) != len(inputProfile.Structured) {
		return true, buildStructureBreakReason(inputProfile, outputProfile)
	}

	for index := range inputProfile.Structured {
		expected := inputProfile.Structured[index]
		actual := outputProfile.Structured[index]
		if expected.Kind != actual.Kind || expected.Signature != actual.Signature {
			return true, buildStructureBreakReason(inputProfile, outputProfile)
		}
	}

	return false, ""
}

func collectStructureProfile(text string) structureProfile {
	lines := strings.Split(normalizeNewlines(text), "\n")
	profile := structureProfile{
		Structured: make([]structureLine, 0, len(lines)),
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		profile.LineCount++
		if structured, ok := detectStructuredLine(trimmed); ok {
			profile.Structured = append(profile.Structured, structured)
		}
	}

	return profile
}

func (p structureProfile) requiresProtection() bool {
	if len(p.Structured) == 0 {
		return false
	}

	for _, line := range p.Structured {
		if line.Kind == structureKindHeading {
			return true
		}
	}

	return len(p.Structured) >= 2
}

func detectStructuredLine(line string) (structureLine, bool) {
	switch {
	case structureHeadingZHPattern.MatchString(line):
		match := structureHeadingZHPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindHeading,
			Signature: "heading:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindHeading, match[1]),
		}, true
	case structureHeadingENPattern.MatchString(line):
		match := structureHeadingENPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindHeading,
			Signature: "heading:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindHeading, match[1]),
		}, true
	case structureBulletPattern.MatchString(line):
		match := structureBulletPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindBullet,
			Signature: "bullet",
			Preview:   previewStructureLabel(structureKindBullet, match[1]),
		}, true
	case structureOrderedHanParen.MatchString(line):
		match := structureOrderedHanParen.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:han:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	case structureOrderedHanComma.MatchString(line):
		match := structureOrderedHanComma.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:han:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	case structureOrderedNestedPattern.MatchString(line):
		match := structureOrderedNestedPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:nested:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	case structureOrderedArabicPattern.MatchString(line):
		match := structureOrderedArabicPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:arabic:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	case structureOrderedAlphaPattern.MatchString(line):
		match := structureOrderedAlphaPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:alpha:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	case structureOrderedRomanPattern.MatchString(line):
		match := structureOrderedRomanPattern.FindStringSubmatch(line)
		return structureLine{
			Kind:      structureKindOrdered,
			Signature: "ordered:roman:" + normalizeStructureToken(match[1]),
			Preview:   previewStructureLabel(structureKindOrdered, match[1]),
		}, true
	default:
		return structureLine{}, false
	}
}

func normalizeStructureToken(token string) string {
	replacer := strings.NewReplacer(
		"（", "(",
		"）", ")",
		"：", ":",
		"\t", " ",
	)
	token = replacer.Replace(strings.TrimSpace(token))
	token = strings.ToLower(token)
	token = strings.Join(strings.Fields(token), " ")
	return strings.TrimSpace(token)
}

func previewStructureLabel(kind structureKind, marker string) string {
	marker = strings.TrimSpace(marker)
	switch kind {
	case structureKindBullet:
		return "bullet item"
	case structureKindHeading:
		return `heading "` + clipStructureText(marker) + `"`
	default:
		return `numbered item "` + clipStructureText(marker) + `"`
	}
}

func clipStructureText(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= 18 {
		return string(runes)
	}
	return string(runes[:18]) + "..."
}

func buildStructureBreakReason(input, output structureProfile) string {
	return fmt.Sprintf(
		"Document structure drifted; preserve numbering, bullet lists, and explicit headings (expected markers=%s, observed markers=%s).",
		summarizeStructureMarkers(input.Structured),
		summarizeStructureMarkers(output.Structured),
	)
}

func summarizeStructureMarkers(lines []structureLine) string {
	if len(lines) == 0 {
		return "none"
	}
	const maxLines = 5
	parts := make([]string, 0, maxLines)
	for index, line := range lines {
		if index == maxLines {
			return fmt.Sprintf("%s, ... (+%d more)", strings.Join(parts, ", "), len(lines)-maxLines)
		}
		parts = append(parts, line.Preview)
	}
	return strings.Join(parts, ", ")
}
