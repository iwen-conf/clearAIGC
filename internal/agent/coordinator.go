package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/domain/scorer"
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
5. Always respect the round-specific goal supplied in [ROUND_PROMPT]. If the round prompt conflicts with your default tendency, the round prompt wins.

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
	scorer   domain.AIScorer
	semantic scorer.SemanticSimilarity
}

func NewCoordinator(registry *Registry, lexical *LexicalMutator, syntax *SyntaxRebuilder) *Coordinator {
	return &Coordinator{registry: registry, lexical: lexical, syntax: syntax, scorer: scorer.NewComposite()}
}

func (c *Coordinator) SetScorer(textScorer domain.AIScorer) {
	if textScorer != nil {
		c.scorer = textScorer
	}
}

func (c *Coordinator) SetSemanticSimilarity(client scorer.SemanticSimilarity) {
	c.semantic = client
}

func (c *Coordinator) textScorer() domain.AIScorer {
	if c != nil && c.scorer != nil {
		return c.scorer
	}
	return scorer.NewComposite()
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
	if c.scorer == nil {
		c.scorer = scorer.NewComposite()
	}

	sentenceScores, err := c.scorer.ScoreSentences(ctx, chunk.Text)
	if err != nil {
		return nil, err
	}
	originalChunkScore, err := c.scorer.Score(ctx, chunk.Text)
	if err != nil {
		return nil, err
	}

	decisions := c.processSentences(ctx, requestID, sentenceScores, roundPrompt)
	finalText := applySentenceDecisions(chunk.Text, decisions)
	finalChunkScore, scoreErr := c.scorer.Score(ctx, finalText)
	if scoreErr != nil {
		finalChunkScore = originalChunkScore
	}
	if finalChunkScore.Total >= originalChunkScore.Total {
		finalText = chunk.Text
		decisions = markNoImprovement(decisions)
		finalChunkScore = originalChunkScore
	}

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
			rewritten, tokens, lerr := c.lexical.Apply(ctx, requestID, buildRewriteGuidance(target, roundPrompt))
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
			rebuilt, tokens, serr := c.syntax.Apply(ctx, requestID, buildRewriteGuidance(target, roundPrompt))
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
		return nil, ErrCoordinatorNotConfigured
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

	remoteScore, err := c.scorer.Score(ctx, payload.Output)
	if err != nil {
		remoteScore = finalChunkScore
	}
	outputText := finalText
	outputScore := finalChunkScore
	if remoteScore.Total+0.02 < finalChunkScore.Total {
		outputText = payload.Output
		outputScore = remoteScore
	}

	return &domain.ProviderResult{
		Provider:     "agent-coordinator",
		OutputText:   outputText,
		InputTokens:  0,
		OutputTokens: payload.TokenCost,
		Score:        &outputScore,
		Sentences:    decisions,
	}, nil
}

func (c *Coordinator) processSentences(ctx context.Context, requestID string, ranked []domain.SentenceScore, roundPrompt string) []domain.SentenceDecision {
	if len(ranked) == 0 {
		return nil
	}

	targetCount := int(math.Ceil(float64(len(ranked)) * 0.4))
	if targetCount < 1 {
		targetCount = 1
	}
	if targetCount > len(ranked) {
		targetCount = len(ranked)
	}

	selected := make([]domain.SentenceScore, targetCount)
	copy(selected, ranked[:targetCount])
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Index < selected[j].Index })

	decisions := make([]domain.SentenceDecision, 0, len(selected))
	for _, sentence := range selected {
		decision := c.ProcessSentence(ctx, requestID, sentence.Text, roundPrompt)
		decision.Index = sentence.Index
		decision.Start = sentence.Start
		decision.End = sentence.End
		decisions = append(decisions, decision)
	}
	return decisions
}

func (c *Coordinator) ProcessSentence(ctx context.Context, requestID string, sentence string, roundPrompt string) domain.SentenceDecision {
	guidance := buildRewriteGuidanceWithScorer(c.textScorer(), sentence, roundPrompt)

	base := domain.RewriteCandidate{
		ID:             "c4",
		Label:          "original",
		Worker:         "baseline",
		Output:         sentence,
		Accepted:       true,
		Reason:         "baseline",
		Score:          guidance.Score,
		Similarity:     1,
		SimilarityMode: "identity",
	}

	candidates := []domain.RewriteCandidate{base}

	if rewritten, tokens, err := c.lexical.Apply(ctx, requestID+"-sentence", guidance); err == nil && strings.TrimSpace(rewritten) != "" {
		score, _ := scoreCandidate(c.textScorer(), ctx, rewritten)
		similarity, mode := c.similarityScore(ctx, sentence, rewritten)
		candidates = append(candidates, domain.RewriteCandidate{
			ID:             "c1",
			Label:          "lexical",
			Worker:         "lexical_mutator",
			Output:         rewritten,
			TokenCost:      tokens,
			Score:          score,
			Similarity:     similarity,
			SimilarityMode: mode,
		})
	}
	if rebuilt, tokens, err := c.syntax.Apply(ctx, requestID+"-sentence", guidance); err == nil && strings.TrimSpace(rebuilt) != "" {
		score, _ := scoreCandidate(c.textScorer(), ctx, rebuilt)
		similarity, mode := c.similarityScore(ctx, sentence, rebuilt)
		candidates = append(candidates, domain.RewriteCandidate{
			ID:             "c2",
			Label:          "syntax",
			Worker:         "syntax_rebuilder",
			Output:         rebuilt,
			TokenCost:      tokens,
			Score:          score,
			Similarity:     similarity,
			SimilarityMode: mode,
		})
	}

	if len(candidates) >= 3 {
		hybridGuidance := buildRewriteGuidanceWithScorer(c.textScorer(), candidates[1].Output, roundPrompt)
		if rebuilt, tokens, err := c.syntax.Apply(ctx, requestID+"-sentence-hybrid", hybridGuidance); err == nil && strings.TrimSpace(rebuilt) != "" {
			score, _ := scoreCandidate(c.textScorer(), ctx, rebuilt)
			similarity, mode := c.similarityScore(ctx, sentence, rebuilt)
			candidates = append(candidates, domain.RewriteCandidate{
				ID:             "c3",
				Label:          "hybrid",
				Worker:         "lexical_mutator+syntax_rebuilder",
				Output:         rebuilt,
				TokenCost:      tokens,
				Score:          score,
				Similarity:     similarity,
				SimilarityMode: mode,
			})
		}
	}

	bestIndex := 0
	bestLoss := computeLoss(guidance.Score.Total, sentence, candidates[0])
	candidates[0].Loss = bestLoss
	for i := 1; i < len(candidates); i++ {
		loss := computeLoss(guidance.Score.Total, sentence, candidates[i])
		candidates[i].Loss = loss
		if loss < bestLoss {
			bestLoss = loss
			bestIndex = i
		}
	}

	best := candidates[bestIndex]
	accepted := bestIndex != 0 && best.Score.Total+0.03 < guidance.Score.Total
	reason := "no_improvement"
	output := sentence
	finalScore := guidance.Score
	if accepted {
		reason = "accepted_best_of_n"
		output = best.Output
		finalScore = best.Score
	}

	for idx := range candidates {
		candidates[idx].Accepted = idx == bestIndex && accepted
		if candidates[idx].Reason == "" {
			if idx == bestIndex && accepted {
				candidates[idx].Reason = "selected"
			} else if idx == 0 {
				candidates[idx].Reason = "fallback"
			} else {
				candidates[idx].Reason = "not_selected"
			}
		}
	}

	return domain.SentenceDecision{
		Input:      sentence,
		Output:     output,
		Start:      0,
		End:        0,
		Original:   guidance.Score,
		Final:      finalScore,
		Candidates: candidates,
		Accepted:   accepted,
		Reason:     reason,
	}
}

func (c *Coordinator) similarityScore(ctx context.Context, input, output string) (float64, string) {
	if c.semantic != nil {
		if result, err := c.semantic.Compare(ctx, input, output); err == nil {
			return result.Score, result.Mode
		}
	}
	return 1 - similarityPenalty(input, output), "prefix_fallback"
}

func computeLoss(originalScore float64, input string, candidate domain.RewriteCandidate) float64 {
	similarity := candidate.Similarity
	if strings.TrimSpace(candidate.SimilarityMode) == "" && strings.TrimSpace(candidate.Output) != "" {
		similarity = 1 - similarityPenalty(input, candidate.Output)
	}
	distance := 1 - similarity
	readability := scorer.EvaluateReadability(candidate.Output).Penalty
	candidateScore := candidate.Score.Total
	return 0.55*candidateScore + 0.30*distance + 0.15*readability - (0.10 * (originalScore - candidateScore))
}

func similarityPenalty(input, output string) float64 {
	inputRunes := []rune(strings.TrimSpace(input))
	outputRunes := []rune(strings.TrimSpace(output))
	if len(inputRunes) == 0 {
		return 0
	}
	shared := longestCommonPrefix(inputRunes, outputRunes)
	ratio := float64(shared) / float64(len(inputRunes))
	return 1 - ratio
}

func longestCommonPrefix(a, b []rune) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	count := 0
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			break
		}
		count++
	}
	return count
}

func applySentenceDecisions(original string, decisions []domain.SentenceDecision) string {
	if len(decisions) == 0 {
		return original
	}

	out := original
	for _, decision := range decisions {
		if !decision.Accepted || strings.TrimSpace(decision.Output) == "" {
			continue
		}
		out = strings.Replace(out, decision.Input, decision.Output, 1)
	}
	return out
}

func markNoImprovement(decisions []domain.SentenceDecision) []domain.SentenceDecision {
	if len(decisions) == 0 {
		return decisions
	}
	cloned := make([]domain.SentenceDecision, len(decisions))
	copy(cloned, decisions)
	for idx := range cloned {
		cloned[idx].Accepted = false
		cloned[idx].Output = cloned[idx].Input
		cloned[idx].Final = cloned[idx].Original
		cloned[idx].Reason = "chunk_no_improvement"
		for candidateIndex := range cloned[idx].Candidates {
			cloned[idx].Candidates[candidateIndex].Accepted = false
		}
	}
	return cloned
}

func scoreCandidate(textScorer domain.AIScorer, ctx context.Context, text string) (domain.AIScore, error) {
	if local, ok := textScorer.(localSentenceScorer); ok {
		return local.ScoreLocal(ctx, text)
	}
	return textScorer.Score(ctx, text)
}
