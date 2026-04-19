package domain

type PromptProfile struct {
	Name      string
	MaxRounds int
	Metric    ChunkMetric
	Prompts   []string
}

type OutputContract struct {
	Text               string
	DisallowedPatterns []string
}

var DisallowedPatterns = []string{
	`修改后[:：]`,
	`润色后[:：]`,
	`改写后[:：]`,
	`优化后[:：]`,
	`以下是`,
	`下面是`,
	`As an AI`,
	`I have rewritten`,
	"```",
}

var OutputContractText = `You must return only the rewritten passage.
Do not add headings, notes, preambles, markdown fences, bullet points, or explanations.
Do not add new facts, references, conclusions, or examples.
Preserve the original paragraph order, numbering, terminology, and factual meaning.`

var Profiles = map[string]PromptProfile{
	"cn": {
		Name:      "cn",
		MaxRounds: 2,
		Metric:    ChunkMetricChar,
		Prompts:   []string{"prompts/round1_cn.md", "prompts/round2_cn.md"},
	},
	"cn_single": {
		Name:      "cn_single",
		MaxRounds: 1,
		Metric:    ChunkMetricChar,
		Prompts:   []string{"prompts/round1_cn.md"},
	},
	"en": {
		Name:      "en",
		MaxRounds: 1,
		Metric:    ChunkMetricWord,
		Prompts:   []string{"prompts/round1_en.md"},
	},
}

func BuildOutputContract() OutputContract {
	return OutputContract{
		Text:               OutputContractText,
		DisallowedPatterns: DisallowedPatterns,
	}
}
