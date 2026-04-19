package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/infra/llm"
	"github.com/iwen-conf/Naturalize/internal/workflow/nodes"
)

type Pipeline struct {
	builder     *PromptBuilder
	parser      *nodes.Parser
	chunker     *nodes.Chunker
	gate        *nodes.QualityGate
	exporter    *nodes.Exporter
	providers   *llm.ProviderChain
	rewriter    domain.Rewriter
	agent       domain.RecoveryAgent
	checkpoints domain.CheckpointStore
	publisher   domain.ProgressPublisher
	pauses      PauseChecker
	runner      compose.Runnable[*State, *State]
}

func NewPipeline(
	builder *PromptBuilder,
	parser *nodes.Parser,
	chunker *nodes.Chunker,
	gate *nodes.QualityGate,
	exporter *nodes.Exporter,
	providers *llm.ProviderChain,
	rewriter domain.Rewriter,
	agent domain.RecoveryAgent,
	checkpoints domain.CheckpointStore,
	publisher domain.ProgressPublisher,
	pauses PauseChecker,
) (*Pipeline, error) {
	p := &Pipeline{
		builder:     builder,
		parser:      parser,
		chunker:     chunker,
		gate:        gate,
		exporter:    exporter,
		providers:   providers,
		rewriter:    rewriter,
		agent:       agent,
		checkpoints: checkpoints,
		publisher:   publisher,
		pauses:      pauses,
	}

	if err := p.compile(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Pipeline) Execute(ctx context.Context, input Input) (*State, error) {
	state := &State{Input: input}
	out, err := p.runner.Invoke(ctx, state, compose.WithCheckPointID(input.Round.CheckpointID))
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (p *Pipeline) compile() error {
	graph := compose.NewGraph[*State, *State]()
	_ = graph.AddLambdaNode("parse", compose.InvokableLambda(p.parse))
	_ = graph.AddLambdaNode("chunk", compose.InvokableLambda(p.chunk))
	_ = graph.AddLambdaNode("batch_llm", compose.InvokableLambda(p.batchLLM))
	_ = graph.AddLambdaNode("quality_gate", compose.InvokableLambda(p.qualityGate))
	_ = graph.AddLambdaNode("react_recovery", compose.InvokableLambda(p.reactRecovery))
	_ = graph.AddLambdaNode("merge", compose.InvokableLambda(p.merge))
	_ = graph.AddLambdaNode("export", compose.InvokableLambda(p.export))

	_ = graph.AddEdge(compose.START, "parse")
	_ = graph.AddEdge("parse", "chunk")
	_ = graph.AddEdge("chunk", "batch_llm")
	_ = graph.AddEdge("batch_llm", "quality_gate")
	graph.AddBranch("quality_gate", compose.NewGraphBranch(func(_ context.Context, state *State) (string, error) {
		if len(state.PendingRecovery) > 0 {
			return "react_recovery", nil
		}
		return "merge", nil
	}, map[string]bool{"react_recovery": true, "merge": true}))
	_ = graph.AddEdge("react_recovery", "merge")
	_ = graph.AddEdge("merge", "export")
	_ = graph.AddEdge("export", compose.END)

	runner, err := graph.Compile(context.Background(),
		compose.WithCheckPointStore(p.checkpoints),
		compose.WithGraphName("naturalize-pipeline"),
	)
	if err != nil {
		return err
	}
	p.runner = runner
	return nil
}

func (p *Pipeline) parse(ctx context.Context, state *State) (*State, error) {
	text, err := p.parser.Parse(ctx, state.Input.Round.InputPath, state.Input.Session.FileFormat)
	if err != nil {
		return nil, err
	}
	state.ParsedText = text
	return state, nil
}

func (p *Pipeline) chunk(_ context.Context, state *State) (*State, error) {
	profile := domain.Profiles[state.Input.Session.PromptProfile]
	state.Manifest = p.chunker.BuildManifest(state.Input.Round.ID, state.ParsedText, profile.Metric, state.Input.Round.ChunkLimit)
	return state, nil
}

func (p *Pipeline) batchLLM(ctx context.Context, state *State) (*State, error) {
	resume := ResumeState{}
	if loaded, ok, err := p.loadResumeState(ctx, state.Input.Round.CheckpointID); err != nil {
		return nil, err
	} else if ok {
		resume = loaded
	}

	chunkCount := len(state.Manifest.Chunks)
	if len(state.RawOutputs) != chunkCount {
		state.RawOutputs = make([]string, chunkCount)
	}
	if len(resume.RawOutputs) == chunkCount {
		copy(state.RawOutputs, resume.RawOutputs)
		state.TotalTokens = resume.TotalTokens
		state.ProviderUsed = resume.ProviderUsed
	}
	if resume.Stage == "react_recovery" && len(state.RawOutputs) == chunkCount && filled(state.RawOutputs) {
		return state, nil
	}

	start := resume.NextIndex
	for index := start; index < chunkCount; index++ {
		if p.shouldPause(state.Input.Session.ID) {
			if err := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
				Stage:        "batch_llm",
				NextIndex:    index,
				RawOutputs:   state.RawOutputs,
				TotalTokens:  state.TotalTokens,
				ProviderUsed: state.ProviderUsed,
				UpdatedAt:    time.Now().UTC(),
			}); err != nil {
				return nil, err
			}
			return nil, ErrPauseRequested
		}

		chunk := state.Manifest.Chunks[index]
		prompt, err := p.builder.BuildChunkPrompt(state.Input.Session.PromptProfile, state.Input.Round.Number, chunk)
		if err != nil {
			return nil, err
		}

		requestID := buildRequestID(state.Input.Session, state.Input.Round, chunk.ID)

		var result *domain.ProviderResult
		if p.rewriter != nil {
			result, err = p.rewriter.ProcessChunk(ctx, requestID, chunk, prompt)
			if err != nil {
				result = nil
			}
		}
		if result == nil {
			result, err = p.providers.Complete(ctx, domain.LLMRequest{
				RequestID: requestID,
				Prompt:    prompt,
			})
		}
		if err != nil {
			if errors.Is(err, llm.ErrAllProvidersUnavailable) {
				if saveErr := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
					Stage:        "batch_llm",
					NextIndex:    index,
					RawOutputs:   state.RawOutputs,
					TotalTokens:  state.TotalTokens,
					ProviderUsed: state.ProviderUsed,
					UpdatedAt:    time.Now().UTC(),
				}); saveErr != nil {
					return nil, saveErr
				}
			}
			return nil, err
		}

		state.RawOutputs[index] = result.OutputText
		state.ProviderUsed = result.Provider
		state.TotalTokens += int64(result.InputTokens + result.OutputTokens)

		if err := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
			Stage:        "batch_llm",
			NextIndex:    index + 1,
			RawOutputs:   state.RawOutputs,
			TotalTokens:  state.TotalTokens,
			ProviderUsed: state.ProviderUsed,
			UpdatedAt:    time.Now().UTC(),
		}); err != nil {
			return nil, err
		}

		_ = p.publisher.Publish(ctx, state.Input.Session.ID, domain.ProgressEvent{
			Type: "progress",
			Data: map[string]any{
				"sessionId":       state.Input.Session.ID,
				"round":           state.Input.Round.Number,
				"phase":           "chunk-complete",
				"completedChunks": index + 1,
				"totalChunks":     chunkCount,
				"percent":         percent(index+1, chunkCount),
				"chunkId":         chunk.ID,
				"paragraphIndex":  chunk.ParagraphIndex,
				"chunkIndex":      chunk.ChunkIndex,
				"providerUsed":    result.Provider,
			},
		})
	}

	return state, p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
		Stage:        "react_recovery",
		NextIndex:    0,
		RawOutputs:   state.RawOutputs,
		TotalTokens:  state.TotalTokens,
		ProviderUsed: state.ProviderUsed,
		UpdatedAt:    time.Now().UTC(),
	})
}

func (p *Pipeline) qualityGate(_ context.Context, state *State) (*State, error) {
	state.Reports = make([]domain.QualityReport, 0, len(state.Manifest.Chunks))
	state.PendingRecovery = state.PendingRecovery[:0]
	for index, chunk := range state.Manifest.Chunks {
		report := p.gate.Check(chunk.Text, state.RawOutputs[index])
		report.ChunkID = chunk.ID
		state.Reports = append(state.Reports, report)
		if !report.AllPassed {
			state.PendingRecovery = append(state.PendingRecovery, index)
		}
	}
	return state, nil
}

func (p *Pipeline) reactRecovery(ctx context.Context, state *State) (*State, error) {
	if len(state.FinalOutputs) != len(state.Manifest.Chunks) {
		state.FinalOutputs = make([]string, len(state.Manifest.Chunks))
	}
	copy(state.FinalOutputs, state.RawOutputs)

	if len(state.PendingRecovery) == 0 {
		return state, nil
	}

	resume := ResumeState{}
	if loaded, ok, err := p.loadResumeState(ctx, state.Input.Round.CheckpointID); err != nil {
		return nil, err
	} else if ok && loaded.Stage == "react_recovery" {
		resume = loaded
		if len(resume.FinalOutputs) == len(state.FinalOutputs) {
			copy(state.FinalOutputs, resume.FinalOutputs)
		}
		if len(resume.Reports) == len(state.Reports) {
			state.Reports = resume.Reports
		}
		state.RecoveryJustification = resume.RecoveryJustification
	}

	for resume.NextIndex < len(state.PendingRecovery) {
		if p.shouldPause(state.Input.Session.ID) {
			if err := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
				Stage:                 "react_recovery",
				NextIndex:             resume.NextIndex,
				RawOutputs:            state.RawOutputs,
				FinalOutputs:          state.FinalOutputs,
				Reports:               state.Reports,
				TotalTokens:           state.TotalTokens,
				ProviderUsed:          state.ProviderUsed,
				RecoveryJustification: state.RecoveryJustification,
				UpdatedAt:             time.Now().UTC(),
			}); err != nil {
				return nil, err
			}
			return nil, ErrPauseRequested
		}

		chunkIndex := state.PendingRecovery[resume.NextIndex]
		chunk := state.Manifest.Chunks[chunkIndex]
		report := state.Reports[chunkIndex]
		reasons := failedReasons(report.Checks)

		_ = p.publisher.Publish(ctx, state.Input.Session.ID, domain.ProgressEvent{
			Type: "quality_alert",
			Data: map[string]any{
				"chunkId":      chunk.ID,
				"checkType":    report.FailedChecks[0],
				"reason":       reasons[0],
				"action":       "invoking_recovery",
				"recoveryStep": resume.NextIndex + 1,
			},
		})

		decision, err := p.agent.Recover(ctx, domain.RecoveryRequest{
			RequestID:     buildRequestID(state.Input.Session, state.Input.Round, chunk.ID) + "-recover",
			Chunk:         chunk,
			OriginalInput: chunk.Text,
			CurrentOutput: state.RawOutputs[chunkIndex],
			FailedChecks:  report.FailedChecks,
			Reasons:       reasons,
		})
		if err != nil {
			if errors.Is(err, llm.ErrAllProvidersUnavailable) {
				if saveErr := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
					Stage:                 "react_recovery",
					NextIndex:             resume.NextIndex,
					RawOutputs:            state.RawOutputs,
					FinalOutputs:          state.FinalOutputs,
					Reports:               state.Reports,
					TotalTokens:           state.TotalTokens,
					ProviderUsed:          state.ProviderUsed,
					RecoveryJustification: state.RecoveryJustification,
					UpdatedAt:             time.Now().UTC(),
				}); saveErr != nil {
					return nil, saveErr
				}
			}
			return nil, err
		}

		state.TotalTokens += int64(decision.TokenCost)
		state.FinalOutputs[chunkIndex] = decision.Output
		rechecked := p.gate.Check(chunk.Text, decision.Output)
		rechecked.ChunkID = chunk.ID
		rechecked.Recovered = decision.Accepted || rechecked.AllPassed
		rechecked.RecoveryMethod = decision.Method
		rechecked.RecoverySteps = decision.Steps
		rechecked.TokenCost = decision.TokenCost
		if decision.Accepted && !rechecked.AllPassed {
			rechecked.AllPassed = true
		}
		state.Reports[chunkIndex] = rechecked

		if decision.Accepted && decision.Justification != "" {
			if state.RecoveryJustification != "" {
				state.RecoveryJustification += "\n"
			}
			state.RecoveryJustification += fmt.Sprintf("%s: %s", chunk.ID, decision.Justification)
		}

		_ = p.publisher.Publish(ctx, state.Input.Session.ID, domain.ProgressEvent{
			Type: "recovery",
			Data: map[string]any{
				"chunkId":   chunk.ID,
				"success":   rechecked.AllPassed,
				"method":    decision.Method,
				"steps":     decision.Steps,
				"tokenCost": decision.TokenCost,
			},
		})

		resume.NextIndex++
		if err := p.saveResumeState(ctx, state.Input.Round.CheckpointID, ResumeState{
			Stage:                 "react_recovery",
			NextIndex:             resume.NextIndex,
			RawOutputs:            state.RawOutputs,
			FinalOutputs:          state.FinalOutputs,
			Reports:               state.Reports,
			TotalTokens:           state.TotalTokens,
			ProviderUsed:          state.ProviderUsed,
			RecoveryJustification: state.RecoveryJustification,
			UpdatedAt:             time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (p *Pipeline) merge(_ context.Context, state *State) (*State, error) {
	if len(state.FinalOutputs) != len(state.Manifest.Chunks) {
		state.FinalOutputs = make([]string, len(state.Manifest.Chunks))
		copy(state.FinalOutputs, state.RawOutputs)
	}

	for index := range state.Manifest.Chunks {
		oldRate := domain.EstimateAIRate(state.Manifest.Chunks[index].Text)
		newRate := domain.EstimateAIRate(state.FinalOutputs[index])
		if newRate > oldRate {
			state.FinalOutputs[index] = state.Manifest.Chunks[index].Text
		}
	}

	chunkMap := make(map[string]string, len(state.Manifest.Chunks))
	for index := range state.Manifest.Chunks {
		state.Manifest.Chunks[index].Output = state.FinalOutputs[index]
		state.Manifest.Chunks[index].Status = domain.ChunkStatusFromReport(state.Reports[index])
		state.Manifest.Chunks[index].Checks = append([]domain.CheckResult(nil), state.Reports[index].Checks...)
		state.Manifest.Chunks[index].AIRate = domain.EstimateAIRate(state.Manifest.Chunks[index].Text)
		state.Manifest.Chunks[index].State = domain.ChunkReviewPending
		chunkMap[state.Manifest.Chunks[index].ID] = state.FinalOutputs[index]
	}
	paragraphs := make([]string, 0, len(state.Manifest.Paragraphs))
	separator := ""
	if state.Manifest.ChunkMetric == domain.ChunkMetricWord {
		separator = " "
	}
	for _, mapping := range state.Manifest.Paragraphs {
		parts := make([]string, 0, len(mapping.ChunkIDs))
		for _, chunkID := range mapping.ChunkIDs {
			parts = append(parts, strings.TrimSpace(chunkMap[chunkID]))
		}
		paragraphs = append(paragraphs, strings.TrimSpace(strings.Join(parts, separator)))
	}
	state.MergedOutput = strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
	state.QualityStats = p.gate.Stats(state.Reports)
	state.ScoreTotal = p.gate.Score(state.QualityStats)
	return state, nil
}

func (p *Pipeline) export(ctx context.Context, state *State) (*State, error) {
	if err := p.exporter.Export(ctx, state.MergedOutput, domain.FormatTXT, state.Input.Round.OutputPath); err != nil {
		return nil, err
	}
	_ = p.checkpoints.DeleteJSON(ctx, state.Input.Round.CheckpointID)
	return state, nil
}

func (p *Pipeline) shouldPause(sessionID uuid.UUID) bool {
	return p.pauses != nil && p.pauses.ShouldPause(sessionID)
}

func (p *Pipeline) loadResumeState(ctx context.Context, checkpointID string) (ResumeState, bool, error) {
	data, ok, err := p.checkpoints.LoadJSON(ctx, checkpointID)
	if err != nil || !ok {
		return ResumeState{}, ok, err
	}
	var state ResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return ResumeState{}, false, err
	}
	return state, true, nil
}

func (p *Pipeline) saveResumeState(ctx context.Context, checkpointID string, resume ResumeState) error {
	payload, err := json.Marshal(resume)
	if err != nil {
		return err
	}
	return p.checkpoints.SaveJSON(ctx, checkpointID, payload)
}

func buildRequestID(session domain.Session, round domain.Round, chunkID string) string {
	return fmt.Sprintf("%s-r%d-%s-%s", session.ID.String(), round.Number, session.PromptProfile, chunkID)
}

func filled(items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == "" {
			return false
		}
	}
	return true
}

func percent(done, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(done) / float64(total) * 100
}

func failedReasons(checks []domain.CheckResult) []string {
	out := make([]string, 0, len(checks))
	for _, check := range checks {
		if !check.Passed {
			out = append(out, check.Reason)
		}
	}
	if len(out) == 0 {
		return []string{"The output did not pass quality review."}
	}
	return out
}
