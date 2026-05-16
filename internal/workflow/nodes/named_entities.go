package nodes

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type namedEntityPattern struct {
	regex *regexp.Regexp
}

var namedEntityPatterns = []namedEntityPattern{
	{regex: regexp.MustCompile(`\b[A-Z][A-Za-z0-9-]*(?:\s+[A-Z][A-Za-z0-9-]*){1,5}\b`)},
	{regex: regexp.MustCompile(`[\p{Han}]{2,24}(?:大学|学院|研究院|研究所|实验室|中心|医院|公司|集团|委员会|协会|基金会|出版社|科学院|工程院|银行|学会|规划署)`)},
}

var chineseEntitySuffixes = []string{
	"大学",
	"学院",
	"研究院",
	"研究所",
	"实验室",
	"中心",
	"医院",
	"公司",
	"集团",
	"委员会",
	"协会",
	"基金会",
	"出版社",
	"科学院",
	"工程院",
	"银行",
	"学会",
	"规划署",
}

var englishEntityHeads = map[string]struct{}{
	"agency":      {},
	"api":         {},
	"association": {},
	"bank":        {},
	"center":      {},
	"centre":      {},
	"college":     {},
	"committee":   {},
	"commission":  {},
	"company":     {},
	"conference":  {},
	"council":     {},
	"corporation": {},
	"department":  {},
	"foundation":  {},
	"hospital":    {},
	"institute":   {},
	"laboratory":  {},
	"lab":         {},
	"ministry":    {},
	"museum":      {},
	"press":       {},
	"sdk":         {},
	"school":      {},
	"society":     {},
	"university":  {},
}

func collectNamedEntities(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	var (
		entities []string
		occupied []factSpan
	)
	for _, pattern := range namedEntityPatterns {
		indices := pattern.regex.FindAllStringIndex(text, -1)
		for _, idx := range indices {
			if spansOverlap(occupied, idx[0], idx[1]) {
				continue
			}
			entity := normalizeNamedEntity(text[idx[0]:idx[1]])
			if !shouldProtectNamedEntity(entity) {
				continue
			}
			entities = append(entities, entity)
			occupied = append(occupied, factSpan{start: idx[0], end: idx[1]})
		}
	}

	sort.Strings(entities)
	return entities
}

func normalizeNamedEntity(entity string) string {
	entity = strings.TrimSpace(entity)
	entity = strings.Trim(entity, "\"'“”‘’")
	entity = strings.Join(strings.Fields(entity), " ")
	for _, prefix := range []string{"The ", "A ", "An "} {
		if strings.HasPrefix(entity, prefix) {
			entity = strings.TrimSpace(strings.TrimPrefix(entity, prefix))
			break
		}
	}
	entity = strings.TrimRight(entity, ".,;:!?)]}）】》")
	return strings.TrimSpace(entity)
}

func shouldProtectNamedEntity(entity string) bool {
	if entity == "" {
		return false
	}
	if hasChineseEntitySuffix(entity) {
		return true
	}

	fields := strings.Fields(entity)
	if len(fields) == 0 {
		return false
	}
	if isEnglishEntityArticle(fields[0]) {
		fields = fields[1:]
	}
	if len(fields) < 2 {
		return false
	}

	coreCount := 0
	hasStrongSignal := false
	for _, field := range fields {
		token := strings.Trim(field, "\"'“”‘’()[]{}.,;:!?")
		if token == "" || !looksLikeEnglishEntityToken(token) {
			return false
		}
		coreCount++
		if isUpperAcronym(token) || hasMixedCase(token) || hasDigit(token) {
			hasStrongSignal = true
		}
	}
	if coreCount < 2 {
		return false
	}
	if hasStrongSignal || len(fields) >= 3 {
		return true
	}
	_, ok := englishEntityHeads[strings.ToLower(fields[len(fields)-1])]
	return ok
}

func hasChineseEntitySuffix(entity string) bool {
	for _, suffix := range chineseEntitySuffixes {
		if strings.HasSuffix(entity, suffix) {
			return true
		}
	}
	return false
}

func isEnglishEntityArticle(token string) bool {
	switch strings.ToLower(token) {
	case "the", "a", "an":
		return true
	default:
		return false
	}
}

func looksLikeEnglishEntityToken(token string) bool {
	if token == "" || token[0] < 'A' || token[0] > 'Z' {
		return false
	}
	for _, r := range token[1:] {
		if r == '-' || r == '.' || r == '&' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
}

func diffNamedEntities(input, output []string) (missing, added []string) {
	inputCounts := make(map[string]int, len(input))
	for _, entity := range input {
		inputCounts[entity]++
	}

	outputCounts := make(map[string]int, len(output))
	for _, entity := range output {
		outputCounts[entity]++
	}

	for entity, count := range inputCounts {
		diff := count - outputCounts[entity]
		for i := 0; i < diff; i++ {
			missing = append(missing, entity)
		}
	}
	for entity, count := range outputCounts {
		diff := count - inputCounts[entity]
		for i := 0; i < diff; i++ {
			added = append(added, entity)
		}
	}

	sort.Strings(missing)
	sort.Strings(added)
	return missing, added
}

func buildNamedEntityReason(missing, added []string) string {
	parts := make([]string, 0, 2)
	if len(missing) > 0 {
		parts = append(parts, "missing from output: "+summarizeNamedEntities(missing))
	}
	if len(added) > 0 {
		parts = append(parts, "unexpected in output: "+summarizeNamedEntities(added))
	}
	return fmt.Sprintf("Named entities changed; preserve people, institutions, products, and place-like proper names exactly (%s).", strings.Join(parts, "; "))
}

func summarizeNamedEntities(entities []string) string {
	const maxEntities = 6
	if len(entities) <= maxEntities {
		return strings.Join(entities, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d more)", strings.Join(entities[:maxEntities], ", "), len(entities)-maxEntities)
}
