package workflow

import (
	"fmt"
	"os"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

func (b *PromptBuilder) LoadRoundPrompt(profile string, round int) (string, error) {
	p, ok := domain.Profiles[profile]
	if !ok {
		return "", fmt.Errorf("unknown prompt profile %q", profile)
	}
	if round < 1 || round > len(p.Prompts) {
		return "", fmt.Errorf("round %d is not defined for profile %s", round, profile)
	}
	data, err := os.ReadFile(p.Prompts[round-1])
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (b *PromptBuilder) BuildChunkPrompt(profile string, round int, chunk domain.Chunk) (string, error) {
	roundPrompt, err := b.LoadRoundPrompt(profile, round)
	if err != nil {
		return "", err
	}
	contract := domain.BuildOutputContract()
	return fmt.Sprintf(
		"[ROUND %d]\n[CHUNK %s]\n\n%s\n\n[OUTPUT CONTRACT]\n%s\n\n[DISALLOWED PATTERNS]\n- %s\n\n[INPUT TEXT]\n%s",
		round,
		chunk.ID,
		roundPrompt,
		contract.Text,
		strings.Join(contract.DisallowedPatterns, "\n- "),
		chunk.Text,
	), nil
}

func (b *PromptBuilder) BuildStrictRecoveryPrompt(request domain.RecoveryRequest) string {
	failures := make([]string, 0, len(request.FailedChecks))
	for idx, check := range request.FailedChecks {
		reason := ""
		if idx < len(request.Reasons) {
			reason = request.Reasons[idx]
		}
		if reason != "" {
			failures = append(failures, fmt.Sprintf("%s: %s", check, reason))
			continue
		}
		failures = append(failures, string(check))
	}
	return fmt.Sprintf(
		"%s\n\n[RECOVERY MODE]\nThe previous output failed these checks:\n- %s\n\nRewrite the passage again so it satisfies every rule. Return only the corrected passage.\n\n[CURRENT OUTPUT]\n%s",
		mustBuildChunkPrompt(b, request),
		strings.Join(failures, "\n- "),
		request.CurrentOutput,
	)
}

func mustBuildChunkPrompt(b *PromptBuilder, request domain.RecoveryRequest) string {
	prompt, err := b.LoadRoundPrompt(profileFromChunkRequest(request), 1)
	if err != nil {
		return "[INPUT TEXT]\n" + request.OriginalInput
	}
	contract := domain.BuildOutputContract()
	return fmt.Sprintf(
		"%s\n\n[OUTPUT CONTRACT]\n%s\n\n[INPUT TEXT]\n%s",
		prompt,
		contract.Text,
		request.OriginalInput,
	)
}

func profileFromChunkRequest(request domain.RecoveryRequest) string {
	if strings.Contains(request.RequestID, "-en-") {
		return "en"
	}
	return "cn"
}
