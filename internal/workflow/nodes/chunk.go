package nodes

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type Chunker struct{}

func NewChunker() *Chunker {
	return &Chunker{}
}

func (c *Chunker) BuildManifest(roundID uuid.UUID, text string, metric domain.ChunkMetric, limit int) *domain.Manifest {
	if limit <= 0 {
		limit = 850
	}
	paragraphs := splitParagraphs(text)
	mappings := make([]domain.ParagraphMapping, 0, len(paragraphs))
	chunks := make([]domain.Chunk, 0, len(paragraphs))

	for pIdx, paragraph := range paragraphs {
		segments := c.splitParagraph(paragraph, metric, limit)
		ids := make([]string, 0, len(segments))
		for sIdx, segment := range segments {
			chunkID := "p" + strconv(pIdx) + "_c" + strconv(sIdx)
			ids = append(ids, chunkID)
			chunks = append(chunks, domain.Chunk{
				ID:             chunkID,
				ParagraphIndex: pIdx,
				ChunkIndex:     sIdx,
				Text:           segment,
				MetricValue:    metricValue(metric, segment),
			})
		}
		mappings = append(mappings, domain.ParagraphMapping{
			ParagraphIndex: pIdx,
			ChunkIDs:       ids,
			Text:           paragraph,
		})
	}

	return &domain.Manifest{
		ID:             uuid.New(),
		RoundID:        roundID,
		ChunkLimit:     limit,
		ChunkMetric:    metric,
		ParagraphCount: len(paragraphs),
		ChunkCount:     len(chunks),
		Paragraphs:     mappings,
		Chunks:         chunks,
	}
}

func splitParagraphs(text string) []string {
	text = normalizeNewlines(text)
	if text == "" {
		return []string{""}
	}
	re := regexp.MustCompile(`\n\s*\n`)
	parts := re.Split(text, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func (c *Chunker) splitParagraph(paragraph string, metric domain.ChunkMetric, limit int) []string {
	if metricValue(metric, paragraph) <= limit {
		return []string{paragraph}
	}

	level2 := splitBySentence(paragraph)
	if allUnderLimit(level2, metric, limit) {
		return level2
	}

	level3 := make([]string, 0, len(level2))
	for _, sentence := range level2 {
		if metricValue(metric, sentence) <= limit {
			level3 = append(level3, sentence)
			continue
		}
		level3 = append(level3, splitByPunctuation(sentence)...)
	}
	if allUnderLimit(level3, metric, limit) {
		return compactSegments(level3)
	}

	level4 := make([]string, 0, len(level3))
	for _, segment := range level3 {
		if metricValue(metric, segment) <= limit {
			level4 = append(level4, segment)
			continue
		}
		level4 = append(level4, hardSplit(segment, metric, limit)...)
	}
	return compactSegments(level4)
}

func splitBySentence(text string) []string {
	return splitAfterAny(strings.TrimSpace(text), "。！？!?.")
}

func splitByPunctuation(text string) []string {
	return splitAfterAny(strings.TrimSpace(text), "，；、,;:")
}

func hardSplit(text string, metric domain.ChunkMetric, limit int) []string {
	if metric == domain.ChunkMetricWord {
		words := strings.Fields(text)
		if len(words) == 0 {
			return []string{text}
		}
		out := make([]string, 0, (len(words)/limit)+1)
		for len(words) > 0 {
			n := limit
			if len(words) < n {
				n = len(words)
			}
			out = append(out, strings.Join(words[:n], " "))
			words = words[n:]
		}
		return out
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return []string{text}
	}
	out := make([]string, 0, (len(runes)/limit)+1)
	for len(runes) > 0 {
		n := limit
		if len(runes) < n {
			n = len(runes)
		}
		cut := n
		for cut > 0 && cut < len(runes) && isWordRune(runes[cut-1]) && isWordRune(runes[cut]) {
			cut--
		}
		if cut == 0 {
			cut = n
		}
		out = append(out, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	return compactSegments(out)
}

func allUnderLimit(parts []string, metric domain.ChunkMetric, limit int) bool {
	for _, part := range parts {
		if metricValue(metric, part) > limit {
			return false
		}
	}
	return true
}

func compactSegments(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func metricValue(metric domain.ChunkMetric, text string) int {
	if metric == domain.ChunkMetricWord {
		return len(strings.Fields(text))
	}
	return utf8.RuneCountInString(text)
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func strconv(v int) string {
	return fmt.Sprintf("%d", v)
}

func splitAfterAny(text, delimiters string) []string {
	if text == "" {
		return []string{""}
	}
	var segments []string
	var builder strings.Builder
	for _, r := range text {
		builder.WriteRune(r)
		if strings.ContainsRune(delimiters, r) {
			segments = append(segments, strings.TrimSpace(builder.String()))
			builder.Reset()
		}
	}
	if tail := strings.TrimSpace(builder.String()); tail != "" {
		segments = append(segments, tail)
	}
	return compactSegments(segments)
}
