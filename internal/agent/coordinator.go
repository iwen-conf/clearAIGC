package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

// ErrCoordinatorNotConfigured signals the coordinator agent is missing credentials.
// The pipeline uses this as the trigger to fall back to the existing provider chain.
var ErrCoordinatorNotConfigured = errors.New("coordinator agent not configured")

const coordinatorSystemPrompt = `[Role]
You are the Workflow Controller for an academic text processing pipeline. Your ONLY objective is to analyze the provided text chunk, identify AI-generated stylistic signatures (high Perplexity/PPL and uniform Burstiness), and delegate specific modification tasks to downstream workers.

[Constraints]
1. DO NOT rewrite the text yourself.
2. You are a machine gateway. Do not output conversational text, greetings, or explanations.
3. Your output must strictly be the invocation of available tools.
4. When identifying generic vocabulary, do NOT replace them with standard academic transitions (e.g. "综合来看", "进一步来说", "意义", "价值", "层面"). The goal is to make it sound human, not artificially academic.

[Workflow]
1. Analyze the input text.
2. If vocabulary is generic (e.g., "crucial", "delve into"), invoke call_lexical_mutator.
3. If sentence lengths are overly uniform, invoke call_syntax_rebuilder.
4. If both are required, plan the execution sequentially.
5. After applying the necessary workers, invoke submit_final_chunk with the rewritten text.`

// Coordinator wires the ReAct flow that owns the rewrite decision.
// Every invocation pulls the latest coordinator config from Registry so that
// a saved settings page update takes effect immediately on the next chunk.
type Coordinator struct {
	registry *Registry
	lexical  *LexicalMutator
	syntax   *SyntaxRebuilder
}

func NewCoordinator(registry *Registry, lexical *LexicalMutator, syntax *SyntaxRebuilder) *Coordinator {
	return &Coordinator{registry: registry, lexical: lexical, syntax: syntax}
}

type lexicalArgs struct {
	TargetText string `json:"target_text"`
}

type syntaxArgs struct {
	TargetText string `json:"target_text"`
}

type submitArgs struct {
	RewrittenText     string   `json:"rewritten_text"`
	ModificationsMade []string `json:"modifications_made"`
}

type coordinatorPayload struct {
	Output    string `json:"output"`
	TokenCost int    `json:"token_cost"`
}

// ProcessChunk runs the coordinator ReAct loop for one chunk. Returns the
// rewritten text plus the aggregate token cost. Returns ErrCoordinatorNotConfigured
// when the coordinator is missing credentials so the caller can fall back.
func (c *Coordinator) ProcessChunk(ctx context.Context, requestID string, chunk domain.Chunk, roundPrompt string) (*domain.ProviderResult, error) {
	chatConfig, ok := c.registry.CoordinatorChatConfig()
	if !ok {
		return nil, ErrCoordinatorNotConfigured
	}

	cfg := chatConfig
	chatModel, err := einoopenai.NewChatModel(ctx, &cfg)
	if err != nil {
		return nil, fmt.Errorf("coordinator chat model: %w", err)
	}

	// Shared running state. The tools close over this to accumulate token cost
	// and carry the evolving rewrite draft through the ReAct steps.
	working := chunk.Text
	totalTokens := 0

	lexicalTool, err := utils.InferTool(
		"call_lexical_mutator",
		"Sends the text to the Lexical Analyst to replace high-frequency AI vocabulary with long-tail academic synonyms without altering sentence structure.",
		func(ctx context.Context, input lexicalArgs) (coordinatorPayload, error) {
			target := input.TargetText
			if target == "" {
				target = working
			}
			rewritten, tokens, lerr := c.lexical.Apply(ctx, requestID, target)
			if lerr != nil {
				return coordinatorPayload{}, lerr
			}
			working = rewritten
			totalTokens += tokens
			return coordinatorPayload{Output: rewritten, TokenCost: tokens}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	syntaxTool, err := utils.InferTool(
		"call_syntax_rebuilder",
		"Sends the text to the Structure Architect to break topological uniformity by splitting long sentences, merging short ones, or shifting active/passive voices.",
		func(ctx context.Context, input syntaxArgs) (coordinatorPayload, error) {
			target := input.TargetText
			if target == "" {
				target = working
			}
			rebuilt, tokens, serr := c.syntax.Apply(ctx, requestID, target)
			if serr != nil {
				return coordinatorPayload{}, serr
			}
			working = rebuilt
			totalTokens += tokens
			return coordinatorPayload{Output: rebuilt, TokenCost: tokens}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	submitTool, err := utils.InferTool(
		"submit_final_chunk",
		"Marks the processing for this chunk as complete and submits the rewritten text back to the main DAG pipeline.",
		func(_ context.Context, input submitArgs) (coordinatorPayload, error) {
			final := input.RewrittenText
			if final == "" {
				final = working
			}
			return coordinatorPayload{Output: final, TokenCost: totalTokens}, nil
		},
	)
	if err != nil {
		return nil, err
	}

	reactAgent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools: []tool.BaseTool{lexicalTool, syntaxTool, submitTool},
		},
		MaxStep: 6,
		ToolReturnDirectly: map[string]struct{}{
			"submit_final_chunk": {},
		},
		MessageModifier: func(_ context.Context, input []*schema.Message) []*schema.Message {
			out := make([]*schema.Message, 0, len(input)+1)
			out = append(out, schema.SystemMessage(coordinatorSystemPrompt))
			out = append(out, input...)
			return out
		},
	})
	if err != nil {
		return nil, err
	}

	msg, err := reactAgent.Generate(ctx, []*schema.Message{
		schema.UserMessage(fmt.Sprintf("[ROUND_PROMPT]\n%s\n\n[CHUNK_ID]\n%s\n\n[TEXT]\n%s", roundPrompt, chunk.ID, chunk.Text)),
	})
	if err != nil {
		return nil, err
	}

	var payload coordinatorPayload
	if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
		// The coordinator may return the final text directly when the tool
		// output is not re-wrapped. Fall back to the working draft.
		payload = coordinatorPayload{Output: working, TokenCost: totalTokens}
	}
	if payload.Output == "" {
		payload.Output = working
	}
	if payload.TokenCost == 0 {
		payload.TokenCost = totalTokens
	}

	return &domain.ProviderResult{
		Provider:     "agent-coordinator",
		OutputText:   payload.Output,
		InputTokens:  0,
		OutputTokens: payload.TokenCost,
	}, nil
}
