package domain

type CheckType string

const (
	CheckEmpty             CheckType = "empty"
	CheckDisallowedPattern CheckType = "disallowed_pattern"
	CheckMarkdownInjection CheckType = "markdown_injection"
	CheckAbnormalExpansion CheckType = "abnormal_expansion"
	CheckAIRateElevated    CheckType = "airate_elevated"
)

type CheckResult struct {
	Type   CheckType `json:"type"`
	Passed bool      `json:"passed"`
	Reason string    `json:"reason,omitempty"`
}

type QualityReport struct {
	ChunkID        string
	Checks         []CheckResult
	AllPassed      bool
	FailedChecks   []CheckType
	Recovered      bool
	RecoveryMethod string
	RecoverySteps  int
	TokenCost      int
}

type QualityStats struct {
	TotalChunks     int               `json:"totalChunks"`
	PassedChunks    int               `json:"passedChunks"`
	FailedChunks    int               `json:"failedChunks"`
	RecoveredChunks int               `json:"recoveredChunks"`
	PassRate        float64           `json:"passRate"`
	RecoveryRate    float64           `json:"recoveryRate"`
	ByCheckType     map[CheckType]int `json:"byCheckType,omitempty"`
}

type RecoveryDecision struct {
	Output        string
	Accepted      bool
	Justification string
	Method        string
	Steps         int
	TokenCost     int
}
