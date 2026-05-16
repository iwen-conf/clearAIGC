package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/workflow"
)

type Supervisor struct {
	modelConfig einoopenai.ChatModelConfig
	llm         domain.LLMClient
	builder     *workflow.PromptBuilder
}

func NewSupervisor(modelConfig einoopenai.ChatModelConfig, llm domain.LLMClient, builder *workflow.PromptBuilder) *Supervisor {
	return &Supervisor{
		modelConfig: modelConfig,
		llm:         llm,
		builder:     builder,
	}
}

type retryArgs struct {
	Instruction string `json:"instruction,omitempty"`
}

type splitArgs struct {
	Parts int `json:"parts,omitempty"`
}

type acceptArgs struct {
	Justification string `json:"justification"`
}

type recoveryPayload struct {
	Output        string `json:"output"`
	Accepted      bool   `json:"accepted"`
	Justification string `json:"justification,omitempty"`
	Method        string `json:"method"`
	Steps         int    `json:"steps"`
	TokenCost     int    `json:"token_cost"`
}

func (s *Supervisor) Recover(ctx context.Context, request domain.RecoveryRequest) (*domain.RecoveryDecision, error) {
	chatModel, err := einoopenai.NewChatModel(ctx, &s.modelConfig)
	if err != nil {
		return nil, err
	}

	retryTool, err := utils.InferTool("retry_with_strict_prompt", "Retry the rewrite with stricter constraints.", func(ctx context.Context, input retryArgs) (recoveryPayload, error) {
		prompt := s.builder.BuildStrictRecoveryPrompt(request)
		if input.Instruction != "" {
			prompt += "\n\n[ADDITIONAL INSTRUCTION]\n" + input.Instruction
		}
		result, err := s.llm.Complete(ctx, domain.LLMRequest{
			RequestID: request.RequestID + "-recover-retry",
			Prompt:    prompt,
		})
		if err != nil {
			return recoveryPayload{}, err
		}
		return recoveryPayload{
			Output:    result.OutputText,
			Method:    "retry_with_strict_prompt",
			Steps:     1,
			TokenCost: result.InputTokens + result.OutputTokens,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	splitTool, err := utils.InferTool("split_and_rewrite", "Split the source passage into smaller parts and rewrite each part.", func(ctx context.Context, input splitArgs) (recoveryPayload, error) {
		parts := input.Parts
		if parts < 2 {
			parts = 2
		}
		segments := splitText(request.OriginalInput, parts)
		outputs := make([]string, 0, len(segments))
		tokenCost := 0
		for index, segment := range segments {
			chunk := request.Chunk
			chunk.Text = segment
			chunk.ID = fmt.Sprintf("%s_part_%d", request.Chunk.ID, index)
			prompt, err := s.builder.BuildChunkPrompt(profileFromRequestID(request.RequestID), 1, chunk)
			if err != nil {
				return recoveryPayload{}, err
			}
			result, err := s.llm.Complete(ctx, domain.LLMRequest{
				RequestID: fmt.Sprintf("%s-split-%d", request.RequestID, index),
				Prompt:    prompt,
			})
			if err != nil {
				return recoveryPayload{}, err
			}
			tokenCost += result.InputTokens + result.OutputTokens
			outputs = append(outputs, result.OutputText)
		}
		return recoveryPayload{
			Output:    strings.Join(outputs, joinSeparator(request.OriginalInput)),
			Method:    "split_and_rewrite",
			Steps:     len(segments),
			TokenCost: tokenCost,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	acceptTool, err := utils.InferTool("accept_as_is", "Accept the current output only when it is still usable.", func(_ context.Context, input acceptArgs) (recoveryPayload, error) {
		return recoveryPayload{
			Output:        request.CurrentOutput,
			Accepted:      true,
			Justification: input.Justification,
			Method:        "accept_as_is",
			Steps:         1,
		}, nil
	})
	if err != nil {
		return nil, err
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{retryTool, splitTool, acceptTool},
		},
		MaxStep: 10,
		ToolReturnDirectly: map[string]struct{}{
			"retry_with_strict_prompt": {},
			"split_and_rewrite":        {},
			"accept_as_is":             {},
		},
		MessageModifier: func(_ context.Context, input []*schema.Message) []*schema.Message {
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, schema.SystemMessage(`You are a recovery agent for document rewriting quality failures.
Choose exactly one tool.
Use retry_with_strict_prompt for template wording or formatting problems.
Use retry_with_strict_prompt for factual invariant problems (numbers, dates, percentages, citations) as well.
Use retry_with_strict_prompt for structure problems (numbering, bullets, explicit headings, line structure) as well.
Use retry_with_strict_prompt for named entity problems (people, institutions, products, proper nouns, place names) as well.
Use retry_with_strict_prompt for semantic similarity problems (embedding similarity dropped or core topic anchors collapsed) as well.
Use retry_with_strict_prompt for terminology drift problems (technical terms, acronyms, identifiers, model names, file paths) as well.
Use retry_with_strict_prompt for overly uniform sentence rhythm as well.
Use retry_with_strict_prompt for readability regressions (overly dense, overly flat, or choppy sentence rhythm) as well.
Use split_and_rewrite when the output expanded too much.
Use accept_as_is only if the current output is still safe to keep.
Do not answer with free text.`))
			out = append(out, input...)
			return out
		},
	})
	if err != nil {
		return nil, err
	}

	msg, err := agent.Generate(ctx, []*schema.Message{
		schema.UserMessage(buildRecoveryPrompt(request)),
	})
	if err != nil {
		return nil, err
	}

	var payload recoveryPayload
	if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
		return nil, fmt.Errorf("parse recovery payload: %w", err)
	}

	return &domain.RecoveryDecision{
		Output:        payload.Output,
		Accepted:      payload.Accepted,
		Justification: payload.Justification,
		Method:        payload.Method,
		Steps:         payload.Steps,
		TokenCost:     payload.TokenCost,
	}, nil
}

func buildRecoveryPrompt(request domain.RecoveryRequest) string {
	lines := make([]string, 0, len(request.FailedChecks))
	for index, check := range request.FailedChecks {
		reason := ""
		if index < len(request.Reasons) {
			reason = request.Reasons[index]
		}
		if reason == "" {
			lines = append(lines, "- "+string(check))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", check, reason))
	}
	return fmt.Sprintf(
		"[SOURCE]\n%s\n\n[CURRENT OUTPUT]\n%s\n\n[FAILED CHECKS]\n%s\n\nChoose the safest tool to make this passage acceptable.",
		request.OriginalInput,
		request.CurrentOutput,
		strings.Join(lines, "\n"),
	)
}

func splitText(text string, parts int) []string {
	words := strings.Fields(text)
	if len(words) >= parts*8 {
		size := len(words) / parts
		if size == 0 {
			size = len(words)
		}
		out := make([]string, 0, parts)
		for len(words) > 0 {
			n := size
			if len(out) == parts-1 || len(words) < n {
				n = len(words)
			}
			out = append(out, strings.Join(words[:n], " "))
			words = words[n:]
		}
		return out
	}

	runes := []rune(text)
	size := len(runes) / parts
	if size == 0 {
		return []string{text}
	}
	out := make([]string, 0, parts)
	for len(runes) > 0 {
		n := size
		if len(out) == parts-1 || len(runes) < n {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}

func joinSeparator(text string) string {
	if len(strings.Fields(text)) > 8 {
		return " "
	}
	return ""
}

func profileFromRequestID(requestID string) string {
	if strings.Contains(requestID, "-en-") {
		return "en"
	}
	return "cn"
}
