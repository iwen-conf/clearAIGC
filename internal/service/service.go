package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	illm "github.com/iwen-conf/Naturalize/internal/infra/llm"
	ipostgres "github.com/iwen-conf/Naturalize/internal/infra/postgres"
	"github.com/iwen-conf/Naturalize/internal/workflow"
	"github.com/iwen-conf/Naturalize/pkg/storage"
)

const (
	exportSelectionAll      = "all"
	exportSelectionAccepted = "accepted"
)

type Service struct {
	sessions    domain.SessionRepository
	rounds      domain.RoundRepository
	manifests   domain.ManifestRepository
	quality     domain.QualityRepository
	states      domain.SessionStateRepository
	pipeline    *workflow.Pipeline
	publisher   domain.ProgressPublisher
	checkpoints domain.CheckpointStore
	exporter    domain.Exporter
	layout      *storage.Layout
	manager     *ExecutionManager
	builder     *workflow.PromptBuilder
}

type CreateSessionInput struct {
	FileName      string
	FileSize      int64
	PromptProfile string
	FileReader    io.Reader
}

func NewService(
	sessions domain.SessionRepository,
	rounds domain.RoundRepository,
	manifests domain.ManifestRepository,
	quality domain.QualityRepository,
	states domain.SessionStateRepository,
	pipeline *workflow.Pipeline,
	publisher domain.ProgressPublisher,
	checkpoints domain.CheckpointStore,
	exporter domain.Exporter,
	layout *storage.Layout,
	manager *ExecutionManager,
	builder *workflow.PromptBuilder,
) *Service {
	return &Service{
		sessions:    sessions,
		rounds:      rounds,
		manifests:   manifests,
		quality:     quality,
		states:      states,
		pipeline:    pipeline,
		publisher:   publisher,
		checkpoints: checkpoints,
		exporter:    exporter,
		layout:      layout,
		manager:     manager,
		builder:     builder,
	}
}

func (s *Service) CreateSession(ctx context.Context, input CreateSessionInput) (*domain.Session, error) {
	if input.FileSize <= 0 || input.FileSize > 50*1024*1024 {
		return nil, ErrInvalidRequest
	}
	format, ok := domain.ParseDocumentFormat(filepath.Ext(input.FileName))
	if !ok {
		return nil, ErrUnsupportedFormat
	}
	if _, ok := domain.Profiles[input.PromptProfile]; !ok {
		return nil, ErrInvalidRequest
	}

	sessionID := uuid.New()
	documentID := uuid.New()
	if err := s.layout.EnsureSession(sessionID); err != nil {
		return nil, err
	}
	originalPath := s.layout.OriginalPath(sessionID, input.FileName)
	file, err := os.Create(originalPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err := io.Copy(file, input.FileReader); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	session := &domain.Session{
		ID:            sessionID,
		DocumentID:    documentID,
		DocumentName:  filepath.Base(input.FileName),
		DocID:         documentID.String(),
		OriginPath:    originalPath,
		FileFormat:    format,
		FileSizeBytes: input.FileSize,
		PromptProfile: input.PromptProfile,
		Status:        domain.SessionPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Service) GetSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	session, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, filter domain.SessionListFilter) ([]domain.Session, int, error) {
	return s.sessions.List(ctx, filter)
}

func (s *Service) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	if s.states != nil {
		_ = s.states.DeleteForSession(ctx, sessionID)
	}
	if err := s.sessions.Delete(ctx, sessionID); err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	return os.RemoveAll(s.layout.SessionDir(sessionID))
}

func (s *Service) StartNextRound(ctx context.Context, sessionID uuid.UUID, chunkLimit int) (*domain.Round, error) {
	unlock, err := s.sessions.AcquireLock(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrSessionLocked) {
			return nil, ErrSessionLocked
		}
		return nil, err
	}
	defer unlock()

	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if s.manager.IsRunning(sessionID) {
		return nil, ErrSessionLocked
	}

	profile := domain.Profiles[session.PromptProfile]
	nextRound := len(session.Rounds) + 1
	if nextRound > profile.MaxRounds {
		return nil, ErrAllRoundsCompleted
	}

	inputPath := session.OriginPath
	if nextRound > 1 {
		inputPath = session.Rounds[nextRound-2].OutputPath
	}
	if err := s.layout.EnsureSession(sessionID, nextRound); err != nil {
		return nil, err
	}
	prompt, err := s.builder.LoadRoundPrompt(session.PromptProfile, nextRound)
	if err != nil {
		return nil, err
	}

	if chunkLimit <= 0 {
		chunkLimit = 850
	}
	now := time.Now().UTC()
	round := &domain.Round{
		ID:            uuid.New(),
		SessionID:     session.ID,
		Number:        nextRound,
		Prompt:        prompt,
		PromptProfile: session.PromptProfile,
		InputPath:     inputPath,
		OutputPath:    s.layout.RoundOutputPath(session.ID, nextRound),
		ChunkLimit:    chunkLimit,
		CheckpointID:  fmt.Sprintf("session-%s-round-%d", session.ID, nextRound),
		Status:        domain.RoundProcessing,
		StartedAt:     &now,
		CreatedAt:     now,
	}
	if err := s.rounds.Create(ctx, round); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateStatus(ctx, session.ID, domain.SessionProcessing); err != nil {
		return nil, err
	}

	session.Rounds = append(session.Rounds, *round)
	s.recordRoundStart(ctx, session.ID, round.Number)
	s.manager.Register(session.ID, round.ID)
	go s.runRound(context.Background(), session, round)

	return round, nil
}

func (s *Service) PauseRound(ctx context.Context, sessionID uuid.UUID) (*domain.Round, error) {
	round, err := s.rounds.GetActiveBySession(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return nil, ErrNoActiveRound
		}
		return nil, err
	}

	if !s.manager.RequestPause(sessionID) {
		return nil, ErrNoActiveRound
	}

	round.Status = domain.RoundPaused
	if err := s.rounds.Update(ctx, round); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateStatus(ctx, sessionID, domain.SessionPaused); err != nil {
		return nil, err
	}
	return round, nil
}

func (s *Service) ResumeRound(ctx context.Context, sessionID uuid.UUID) (*domain.Round, error) {
	unlock, err := s.sessions.AcquireLock(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrSessionLocked) {
			return nil, ErrSessionLocked
		}
		return nil, err
	}
	defer unlock()

	if s.manager.IsRunning(sessionID) {
		return nil, ErrSessionLocked
	}
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	var round *domain.Round
	for i := range session.Rounds {
		if session.Rounds[i].Status == domain.RoundPaused {
			copyRound := session.Rounds[i]
			round = &copyRound
		}
	}
	if round == nil {
		return nil, ErrNoPausedRound
	}

	now := time.Now().UTC()
	round.Status = domain.RoundProcessing
	round.StartedAt = &now
	if err := s.rounds.Update(ctx, round); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateStatus(ctx, sessionID, domain.SessionProcessing); err != nil {
		return nil, err
	}

	s.recordRoundResume(ctx, session.ID, round.Number)
	s.manager.Register(session.ID, round.ID)
	go s.runRound(context.Background(), session, round)
	return round, nil
}

func (s *Service) GetRound(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*domain.Round, error) {
	round, err := s.rounds.GetBySessionAndNumber(ctx, sessionID, roundNumber)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return nil, ErrRoundNotFound
		}
		return nil, err
	}
	return round, nil
}

func (s *Service) ReadOutput(ctx context.Context, sessionID uuid.UUID, roundNumber int) (string, *domain.Manifest, error) {
	round, err := s.GetRound(ctx, sessionID, roundNumber)
	if err != nil {
		return "", nil, err
	}
	data, err := os.ReadFile(round.OutputPath)
	if err != nil {
		return "", nil, err
	}
	manifest, err := s.manifests.GetByRoundID(ctx, round.ID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return string(data), nil, nil
		}
		return "", nil, err
	}
	return string(data), manifest, nil
}

func (s *Service) ReadDiff(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*domain.RoundDiff, error) {
	_, manifest, err := s.getRoundManifest(ctx, sessionID, roundNumber)
	if err != nil {
		return nil, err
	}

	return &domain.RoundDiff{
		Round:  roundNumber,
		Chunks: buildChunkDiffs(manifest.Chunks),
	}, nil
}

func (s *Service) ReadState(ctx context.Context, sessionID uuid.UUID) (*domain.SessionState, error) {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	state := &domain.SessionState{
		Session:  session,
		Timeline: []domain.SessionTimelineEntry{},
	}

	if s.states != nil {
		progress, progressErr := s.states.GetProgress(ctx, sessionID)
		if progressErr == nil {
			state.Progress = progress
		} else if !errors.Is(progressErr, ipostgres.ErrNotFound) {
			return nil, progressErr
		}

		timeline, timelineErr := s.states.ListTimeline(ctx, sessionID, 100)
		if timelineErr != nil {
			return nil, timelineErr
		}
		state.Timeline = timeline
	}

	round := latestCompletedRound(session)
	if round == nil {
		return state, nil
	}

	text, manifest, err := s.ReadOutput(ctx, sessionID, round.Number)
	if err != nil {
		return nil, err
	}
	diff, err := s.ReadDiff(ctx, sessionID, round.Number)
	if err != nil {
		return nil, err
	}

	state.Preview = &domain.OutputPreview{Text: text}
	if manifest != nil {
		state.Preview.SegmentCount = manifest.ChunkCount
		state.Preview.ParagraphCount = manifest.ParagraphCount
	}
	state.Comparison = diff
	return state, nil
}

func (s *Service) ListCards(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*domain.RoundDiff, error) {
	return s.ReadDiff(ctx, sessionID, roundNumber)
}

func (s *Service) AcceptCard(ctx context.Context, sessionID uuid.UUID, roundNumber int, cardID string) (*domain.ChunkDiff, error) {
	return s.updateCardState(ctx, sessionID, roundNumber, cardID, domain.ChunkReviewAccepted)
}

func (s *Service) RejectCard(ctx context.Context, sessionID uuid.UUID, roundNumber int, cardID string) (*domain.ChunkDiff, error) {
	return s.updateCardState(ctx, sessionID, roundNumber, cardID, domain.ChunkReviewRejected)
}

func (s *Service) ApplyAllCards(ctx context.Context, sessionID uuid.UUID, roundNumber int) ([]domain.ChunkDiff, int, error) {
	unlock, err := s.sessions.AcquireLock(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrSessionLocked) {
			return nil, 0, ErrSessionLocked
		}
		return nil, 0, err
	}
	defer unlock()

	_, manifest, err := s.getRoundManifest(ctx, sessionID, roundNumber)
	if err != nil {
		return nil, 0, err
	}

	updated := 0
	for index := range manifest.Chunks {
		if manifest.Chunks[index].State == domain.ChunkReviewPending {
			manifest.Chunks[index].State = domain.ChunkReviewAccepted
			updated++
		}
	}
	if updated > 0 {
		if err := s.manifests.Create(ctx, manifest); err != nil {
			return nil, 0, err
		}
	}

	return buildChunkDiffs(manifest.Chunks), updated, nil
}

func (s *Service) Export(ctx context.Context, sessionID uuid.UUID, roundNumber int, format domain.DocumentFormat, selection string) (string, error) {
	round, err := s.GetRound(ctx, sessionID, roundNumber)
	if err != nil {
		return "", err
	}

	selection, err = normalizeExportSelection(selection)
	if err != nil {
		return "", err
	}

	text, err := s.exportText(ctx, round, selection)
	if err != nil {
		return "", err
	}

	path := s.layout.RoundExportPath(sessionID, roundNumber, string(format))
	if err := s.exporter.Export(ctx, text, format, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) getRoundManifest(ctx context.Context, sessionID uuid.UUID, roundNumber int) (*domain.Round, *domain.Manifest, error) {
	round, err := s.GetRound(ctx, sessionID, roundNumber)
	if err != nil {
		return nil, nil, err
	}

	manifest, err := s.manifests.GetByRoundID(ctx, round.ID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			return nil, nil, ErrRoundNotFound
		}
		return nil, nil, err
	}

	normalizeManifest(manifest)
	return round, manifest, nil
}

func (s *Service) updateCardState(ctx context.Context, sessionID uuid.UUID, roundNumber int, cardID string, state domain.ChunkReviewState) (*domain.ChunkDiff, error) {
	unlock, err := s.sessions.AcquireLock(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrSessionLocked) {
			return nil, ErrSessionLocked
		}
		return nil, err
	}
	defer unlock()

	_, manifest, err := s.getRoundManifest(ctx, sessionID, roundNumber)
	if err != nil {
		return nil, err
	}

	for index := range manifest.Chunks {
		if manifest.Chunks[index].ID != cardID {
			continue
		}
		manifest.Chunks[index].State = state
		if err := s.manifests.Create(ctx, manifest); err != nil {
			return nil, err
		}
		diff := buildChunkDiff(manifest.Chunks[index])
		return &diff, nil
	}

	return nil, errCardNotFound
}

func (s *Service) exportText(ctx context.Context, round *domain.Round, selection string) (string, error) {
	manifest, err := s.manifests.GetByRoundID(ctx, round.ID)
	if err != nil {
		if errors.Is(err, ipostgres.ErrNotFound) {
			data, readErr := os.ReadFile(round.OutputPath)
			if readErr != nil {
				return "", readErr
			}
			return string(data), nil
		}
		return "", err
	}

	normalizeManifest(manifest)
	return renderExportText(manifest, selection), nil
}

func (s *Service) runRound(ctx context.Context, session *domain.Session, round *domain.Round) {
	defer s.manager.Unregister(session.ID)

	result, err := s.pipeline.Execute(ctx, workflow.Input{
		Session: *session,
		Round:   *round,
	})
	if err != nil {
		log.Printf("round execution error session=%s round=%d: %v", session.ID, round.Number, err)
		switch {
		case errors.Is(err, workflow.ErrPauseRequested), errors.Is(err, illm.ErrAllProvidersUnavailable):
			round.Status = domain.RoundPaused
			if updateErr := s.rounds.Update(ctx, round); updateErr == nil {
				_ = s.sessions.UpdateStatus(ctx, session.ID, domain.SessionPaused)
			}
			reason := "user_requested"
			if errors.Is(err, illm.ErrAllProvidersUnavailable) {
				reason = "provider_unavailable"
			}
			_ = s.publisher.Publish(ctx, session.ID, domain.ProgressEvent{
				Type: "paused",
				Data: map[string]any{
					"sessionId":       session.ID,
					"round":           round.Number,
					"completedChunks": 0,
					"totalChunks":     0,
					"checkpointId":    round.CheckpointID,
					"reason":          reason,
				},
			})
		default:
			round.Status = domain.RoundFailed
			now := time.Now().UTC()
			round.CompletedAt = &now
			_ = s.rounds.Update(ctx, round)
			_ = s.sessions.UpdateStatus(ctx, session.ID, domain.SessionFailed)
			_ = s.publisher.Publish(ctx, session.ID, domain.ProgressEvent{
				Type: "error",
				Data: map[string]any{
					"message":     "The document could not be processed.",
					"recoverable": false,
					"code":        "PROCESSING_FAILED",
				},
			})
		}
		return
	}

	now := time.Now().UTC()
	round.Status = domain.RoundCompleted
	round.CompletedAt = &now
	round.OutputPath = s.layout.RoundOutputPath(session.ID, round.Number)
	round.ProviderUsed = result.ProviderUsed
	round.TotalTokens = result.TotalTokens
	round.InputSegmentCount = result.Manifest.ChunkCount
	round.OutputSegmentCount = result.Manifest.ChunkCount
	round.RecoveryJustification = result.RecoveryJustification
	score := result.ScoreTotal
	round.ScoreTotal = &score

	replaced := false
	for i := range session.Rounds {
		if session.Rounds[i].ID == round.ID {
			session.Rounds[i] = *round
			replaced = true
		}
	}
	if !replaced {
		session.Rounds = append(session.Rounds, *round)
	}

	if err := s.rounds.CompleteRound(ctx, session, round, result.Manifest, result.Reports); err != nil {
		_ = s.sessions.UpdateStatus(ctx, session.ID, domain.SessionFailed)
		return
	}

	_ = s.publisher.Publish(ctx, session.ID, domain.ProgressEvent{
		Type: "complete",
		Data: map[string]any{
			"sessionId":       session.ID,
			"round":           round.Number,
			"scoreTotal":      result.ScoreTotal,
			"chunkCount":      result.Manifest.ChunkCount,
			"passedChunks":    result.QualityStats.PassedChunks,
			"recoveredChunks": result.QualityStats.RecoveredChunks,
			"failedChunks":    result.QualityStats.FailedChunks,
			"totalTokens":     result.TotalTokens,
			"providerUsed":    result.ProviderUsed,
			"downloadUrl":     fmt.Sprintf("/api/v1/sessions/%s/export?round=%d&format=txt", session.ID, round.Number),
		},
	})
}

func (s *Service) Publisher() domain.ProgressPublisher {
	return s.publisher
}

func latestCompletedRound(session *domain.Session) *domain.Round {
	var latest *domain.Round
	for index := range session.Rounds {
		round := &session.Rounds[index]
		if round.Status != domain.RoundCompleted {
			continue
		}
		if latest == nil || round.Number > latest.Number {
			latest = round
		}
	}
	return latest
}

func (s *Service) recordRoundStart(ctx context.Context, sessionID uuid.UUID, roundNumber int) {
	if s.states == nil {
		return
	}

	_ = s.states.UpsertProgress(ctx, &domain.SessionProgressSnapshot{
		SessionID: sessionID,
		Round:     roundNumber,
		Phase:     "queued",
	})

	title := "文档已提交"
	detail := "第一轮润色已开始。"
	if roundNumber > 1 {
		title = fmt.Sprintf("第 %d 轮已开始", roundNumber)
		detail = fmt.Sprintf("第 %d 轮处理已开始。", roundNumber)
	}
	_ = s.states.AppendTimeline(ctx, &domain.SessionTimelineEntry{
		SessionID: sessionID,
		Tone:      domain.TimelineToneNeutral,
		Title:     title,
		Detail:    detail,
	})
}

func (s *Service) recordRoundResume(ctx context.Context, sessionID uuid.UUID, roundNumber int) {
	if s.states == nil {
		return
	}

	snapshot := &domain.SessionProgressSnapshot{
		SessionID: sessionID,
		Round:     roundNumber,
		Phase:     "resumed",
	}
	if existing, err := s.states.GetProgress(ctx, sessionID); err == nil {
		snapshot.CompletedChunks = existing.CompletedChunks
		snapshot.TotalChunks = existing.TotalChunks
		snapshot.Percent = existing.Percent
		snapshot.ChunkID = existing.ChunkID
		snapshot.ParagraphIndex = existing.ParagraphIndex
		snapshot.ChunkIndex = existing.ChunkIndex
		snapshot.ProviderUsed = existing.ProviderUsed
	}
	_ = s.states.UpsertProgress(ctx, snapshot)
	_ = s.states.AppendTimeline(ctx, &domain.SessionTimelineEntry{
		SessionID: sessionID,
		Tone:      domain.TimelineToneNeutral,
		Title:     "已继续处理",
		Detail:    fmt.Sprintf("第 %d 轮已恢复。", roundNumber),
	})
}

func normalizeManifest(manifest *domain.Manifest) {
	for index := range manifest.Chunks {
		manifest.Chunks[index].State = normalizeChunkReviewState(manifest.Chunks[index].State)
	}
}

func normalizeChunkReviewState(state domain.ChunkReviewState) domain.ChunkReviewState {
	switch state {
	case domain.ChunkReviewAccepted, domain.ChunkReviewRejected:
		return state
	default:
		return domain.ChunkReviewPending
	}
}

func normalizeExportSelection(selection string) (string, error) {
	switch selection {
	case "", exportSelectionAll:
		return exportSelectionAll, nil
	case exportSelectionAccepted:
		return exportSelectionAccepted, nil
	default:
		return "", ErrInvalidRequest
	}
}

func buildChunkDiffs(chunks []domain.Chunk) []domain.ChunkDiff {
	diffs := make([]domain.ChunkDiff, 0, len(chunks))
	for _, chunk := range chunks {
		diffs = append(diffs, buildChunkDiff(chunk))
	}
	return diffs
}

func buildChunkDiff(chunk domain.Chunk) domain.ChunkDiff {
	charDelta := 0
	if chunk.Output != "" {
		charDelta = utf8.RuneCountInString(chunk.Output) - utf8.RuneCountInString(chunk.Text)
	}
	inputAIRate := domain.EstimateAIRate(chunk.Text)
	outputAIRate := domain.EstimateAIRate(chunk.Output)
	return domain.ChunkDiff{
		ID:             chunk.ID,
		ParagraphIndex: chunk.ParagraphIndex,
		ChunkIndex:     chunk.ChunkIndex,
		Input:          chunk.Text,
		Output:         chunk.Output,
		Status:         chunk.Status,
		CharDelta:      charDelta,
		AIRate:         inputAIRate,
		OutputAIRate:   outputAIRate,
		Detector:       domain.AIRateDetectorName,
		State:          normalizeChunkReviewState(chunk.State),
		Checks:         append([]domain.CheckResult(nil), chunk.Checks...),
	}
}

func renderExportText(manifest *domain.Manifest, selection string) string {
	chunkMap := make(map[string]domain.Chunk, len(manifest.Chunks))
	for _, chunk := range manifest.Chunks {
		chunkMap[chunk.ID] = chunk
	}

	separator := ""
	if manifest.ChunkMetric == domain.ChunkMetricWord {
		separator = " "
	}

	paragraphs := make([]string, 0, len(manifest.Paragraphs))
	for _, mapping := range manifest.Paragraphs {
		parts := make([]string, 0, len(mapping.ChunkIDs))
		for _, chunkID := range mapping.ChunkIDs {
			chunk, ok := chunkMap[chunkID]
			if !ok {
				continue
			}
			parts = append(parts, strings.TrimSpace(selectChunkExportText(chunk, selection)))
		}
		paragraphs = append(paragraphs, strings.TrimSpace(strings.Join(parts, separator)))
	}

	return strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
}

func selectChunkExportText(chunk domain.Chunk, selection string) string {
	if strings.TrimSpace(chunk.Output) == "" {
		return chunk.Text
	}

	state := normalizeChunkReviewState(chunk.State)
	if selection == exportSelectionAccepted && state != domain.ChunkReviewAccepted {
		return chunk.Text
	}
	if selection == exportSelectionAll && state == domain.ChunkReviewRejected {
		return chunk.Text
	}
	return chunk.Output
}

func IsCustomerVisibleError(err error) bool {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrSessionNotFound),
		errors.Is(err, ErrRoundNotFound),
		errors.Is(err, errCardNotFound),
		errors.Is(err, ErrSessionLocked),
		errors.Is(err, ErrAllRoundsCompleted),
		errors.Is(err, ErrNoActiveRound),
		errors.Is(err, ErrNoPausedRound),
		errors.Is(err, ErrUnsupportedFormat):
		return true
	default:
		return false
	}
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return "INVALID_REQUEST"
	case errors.Is(err, ErrUnsupportedFormat):
		return "INVALID_FILE_FORMAT"
	case errors.Is(err, ErrSessionNotFound):
		return "SESSION_NOT_FOUND"
	case errors.Is(err, ErrRoundNotFound):
		return "ROUND_NOT_FOUND"
	case errors.Is(err, errCardNotFound):
		return "CARD_NOT_FOUND"
	case errors.Is(err, ErrSessionLocked):
		return "SESSION_LOCKED"
	case errors.Is(err, ErrAllRoundsCompleted):
		return "ALL_ROUNDS_COMPLETED"
	case errors.Is(err, ErrNoActiveRound), errors.Is(err, ErrNoPausedRound):
		return "ROUND_STATE_INVALID"
	case errors.Is(err, ErrAgentNotFound):
		return "AGENT_NOT_FOUND"
	default:
		return "INTERNAL_ERROR"
	}
}

func ErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return "The request is incomplete or contains unsupported values."
	case errors.Is(err, ErrUnsupportedFormat):
		return "This file type is not supported. Upload a .txt or .docx document."
	case errors.Is(err, ErrSessionNotFound):
		return "The document session could not be found."
	case errors.Is(err, ErrRoundNotFound):
		return "The requested version could not be found."
	case errors.Is(err, errCardNotFound):
		return "The requested review card could not be found."
	case errors.Is(err, ErrSessionLocked):
		return "This document is already being processed."
	case errors.Is(err, ErrAllRoundsCompleted):
		return "This document has already completed all available rewrite rounds."
	case errors.Is(err, ErrNoActiveRound):
		return "There is no active processing task to pause."
	case errors.Is(err, ErrNoPausedRound):
		return "There is no paused processing task to resume."
	case errors.Is(err, ErrAgentNotFound):
		return "The requested agent does not exist."
	default:
		return "The request could not be completed right now."
	}
}
