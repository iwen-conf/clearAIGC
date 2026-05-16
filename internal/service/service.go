package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/domain/scorer"
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
	scorer      domain.AIScorer
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
		scorer:      scorer.NewComposite(),
	}
}

func (s *Service) SetScorer(textScorer domain.AIScorer) {
	if textScorer != nil {
		s.scorer = textScorer
	}
}

func (s *Service) textScorer() domain.AIScorer {
	if s != nil && s.scorer != nil {
		return s.scorer
	}
	return scorer.NewComposite()
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
	s.decorateSessionRoundPlan(ctx, session)
	return session, nil
}

func (s *Service) ListSessions(ctx context.Context, filter domain.SessionListFilter) ([]domain.SessionListItem, int, error) {
	sessions, total, err := s.sessions.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	progressBySession := map[uuid.UUID]*domain.SessionProgressSnapshot{}
	if s.states != nil {
		ids := make([]uuid.UUID, 0, len(sessions))
		for _, session := range sessions {
			ids = append(ids, session.ID)
		}

		progressBySession, err = s.states.ListProgressBySessionIDs(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
	}

	items := make([]domain.SessionListItem, 0, len(sessions))
	for index := range sessions {
		session := sessions[index]
		progress := progressBySession[session.ID]
		copySession := session
		s.decorateSessionRoundPlan(ctx, &copySession)
		items = append(items, domain.SessionListItem{
			Session:  &copySession,
			Progress: progress,
			Metrics:  s.buildSessionListMetrics(ctx, copySession, progress),
		})
	}

	return items, total, nil
}

func (s *Service) DeleteSession(ctx context.Context, sessionID uuid.UUID) error {
	var checkpointIDs []string
	if session, err := s.sessions.Get(ctx, sessionID); err == nil {
		checkpointIDs = collectCheckpointIDs(session.Rounds)
	} else if !errors.Is(err, ipostgres.ErrNotFound) {
		return err
	}
	if s.states != nil {
		_ = s.states.DeleteForSession(ctx, sessionID)
	}
	if s.checkpoints != nil {
		for _, checkpointID := range checkpointIDs {
			_ = s.checkpoints.Delete(ctx, checkpointID)
			_ = s.checkpoints.DeleteJSON(ctx, checkpointID)
		}
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
	totalRounds, _, canStartNext := s.resolveRoundPlan(ctx, session)
	if !canStartNext || nextRound > totalRounds || nextRound > profile.MaxRounds {
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
	s.recordRoundStart(ctx, session.ID, round.Number, session.PromptProfile)
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
	s.decorateSessionRoundPlan(ctx, session)

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

		timeline, timelineErr := s.states.ListTimeline(ctx, sessionID, 100, false)
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

func (s *Service) ReadHistory(ctx context.Context, sessionID uuid.UUID) (*domain.SessionHistory, error) {
	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	s.decorateSessionRoundPlan(ctx, session)

	history := &domain.SessionHistory{
		Session:  session,
		Metrics:  s.buildSessionListMetrics(ctx, *session, nil),
		Rounds:   make([]domain.RoundHistoryEntry, 0, len(session.Rounds)),
		Timeline: []domain.SessionTimelineEntry{},
	}

	if s.states != nil {
		progress, progressErr := s.states.GetProgress(ctx, sessionID)
		if progressErr == nil {
			history.Progress = progress
			history.Metrics = s.buildSessionListMetrics(ctx, *session, progress)
		} else if !errors.Is(progressErr, ipostgres.ErrNotFound) {
			return nil, progressErr
		}

		timeline, timelineErr := s.states.ListTimeline(ctx, sessionID, 0, true)
		if timelineErr != nil {
			return nil, timelineErr
		}
		history.Timeline = timeline
	}

	sort.Slice(session.Rounds, func(i, j int) bool {
		return session.Rounds[i].Number < session.Rounds[j].Number
	})

	for index := range session.Rounds {
		round := session.Rounds[index]
		summary := buildRoundHistorySummary(round)

		manifest, manifestErr := s.manifests.GetByRoundID(ctx, round.ID)
		switch {
		case manifestErr == nil:
			normalizeManifest(manifest)
			applyManifestSummary(&summary, manifest)
		case errors.Is(manifestErr, ipostgres.ErrNotFound):
		default:
			return nil, manifestErr
		}

		mergeLocalMetrics(&history.LocalMetrics, summary.LocalMetrics)
		if summary.QualityVerdict.Verdict != "" {
			history.QualityVerdict = summary.QualityVerdict
		}
		copyRound := round
		history.Rounds = append(history.Rounds, domain.RoundHistoryEntry{
			Round:   &copyRound,
			Summary: summary,
		})
	}

	return history, nil
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

func (s *Service) CleanupExpiredSessions(ctx context.Context, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, ErrInvalidRequest
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	sessions, err := s.sessions.ListExpired(ctx, cutoff, []domain.SessionStatus{
		domain.SessionCompleted,
		domain.SessionFailed,
		domain.SessionPaused,
	})
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, session := range sessions {
		if session.Status == domain.SessionProcessing {
			continue
		}
		if err := s.DeleteSession(ctx, session.ID); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
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
	if result.StopAfterRound && session.PromptProfile == "cn" {
		session.TotalRounds = round.Number
		session.NextRound = 0
		session.CanStartNext = false
	}

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

func (s *Service) recordRoundStart(ctx context.Context, sessionID uuid.UUID, roundNumber int, promptProfile string) {
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
	if promptProfile == "cn" && roundNumber == 1 {
		detail = "第一轮润色已开始。第二轮精修为可选。"
	}
	if roundNumber > 1 {
		title = fmt.Sprintf("第 %d 轮已开始", roundNumber)
		detail = fmt.Sprintf("第 %d 轮处理已开始。", roundNumber)
		if promptProfile == "cn" && roundNumber == 2 {
			detail = "第二轮精修已开始。"
		}
	}
	_ = s.states.AppendTimeline(ctx, &domain.SessionTimelineEntry{
		SessionID: sessionID,
		Round:     roundNumber,
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
		Round:     roundNumber,
		Tone:      domain.TimelineToneNeutral,
		Title:     "已继续处理",
		Detail:    fmt.Sprintf("第 %d 轮已恢复。", roundNumber),
	})
}

func (s *Service) buildSessionListMetrics(ctx context.Context, session domain.Session, progress *domain.SessionProgressSnapshot) domain.SessionListMetrics {
	totalRounds, nextRound, canStartNext := s.resolveRoundPlan(ctx, &session)
	metrics := domain.SessionListMetrics{
		TotalRounds:    totalRounds,
		NextRound:      nextRound,
		CanStartNext:   canStartNext,
		LastActivityAt: session.UpdatedAt,
	}

	for _, round := range session.Rounds {
		if round.Status == domain.RoundCompleted {
			metrics.CompletedRounds++
		}
		metrics.TotalTokens += round.TotalTokens
		if round.CompletedAt != nil {
			metrics.LastActivityAt = maxActivityTime(metrics.LastActivityAt, *round.CompletedAt)
		}
	}

	if progress != nil {
		metrics.LastActivityAt = maxActivityTime(metrics.LastActivityAt, progress.UpdatedAt)
	}

	return metrics
}

func buildRoundHistorySummary(round domain.Round) domain.RoundHistorySummary {
	summary := domain.RoundHistorySummary{
		ScoreTotal: round.ScoreTotal,
		LocalMetrics: domain.LocalMetrics{
			TotalTokens:      round.TotalTokens,
			LastFailureReason: strings.TrimSpace(round.RecoveryJustification),
		},
	}
	if round.StartedAt != nil && round.CompletedAt != nil && round.CompletedAt.After(*round.StartedAt) {
		summary.DurationSeconds = int64(round.CompletedAt.Sub(*round.StartedAt).Seconds())
		summary.LocalMetrics.DurationSeconds = summary.DurationSeconds
	}
	summary.QualityVerdict = defaultQualityVerdict(round.Status)
	return summary
}

func applyManifestSummary(summary *domain.RoundHistorySummary, manifest *domain.Manifest) {
	if summary == nil || manifest == nil {
		return
	}
	summary.ChunkCount = len(manifest.Chunks)
	scoreBefore := averageScore(manifest.Chunks, func(chunk domain.Chunk) *domain.AIScore { return chunk.Score })
	scoreAfter := averageScore(manifest.Chunks, func(chunk domain.Chunk) *domain.AIScore { return chunk.OutputScore })
	targetScore := averageTargetScore(manifest.Chunks)
	failedChecks := make([]domain.CheckType, 0)
	reasons := make([]string, 0)
	externalStatus := domain.ExternalDetectorUnavailable
	detector := ""

	for _, chunk := range manifest.Chunks {
		switch chunk.Status {
		case domain.ChunkRecovered:
			summary.RecoveredChunks++
		case domain.ChunkFailed:
			summary.FailedChunks++
		default:
			summary.PassedChunks++
		}
		switch normalizeChunkReviewState(chunk.State) {
		case domain.ChunkReviewAccepted:
			summary.LocalMetrics.AcceptedCards++
		case domain.ChunkReviewRejected:
			summary.LocalMetrics.RejectedCards++
		default:
			summary.LocalMetrics.PendingCards++
		}
		if chunk.OutputScore != nil {
			if detector == "" {
				detector = scoreDetectorName(chunk.OutputScore)
			}
			status := scoreExternalStatus(chunk.OutputScore)
			if status == domain.ExternalDetectorFailed {
				externalStatus = status
			} else if status == domain.ExternalDetectorPassed && externalStatus != domain.ExternalDetectorFailed {
				externalStatus = status
			}
		}
		for _, check := range chunk.Checks {
			if check.Passed {
				continue
			}
			failedChecks = appendUniqueCheck(failedChecks, check.Type)
			if strings.TrimSpace(check.Reason) != "" {
				reasons = append(reasons, check.Reason)
			}
		}
	}

	reviewedCards := summary.LocalMetrics.AcceptedCards + summary.LocalMetrics.RejectedCards
	if reviewedCards > 0 {
		summary.LocalMetrics.AcceptanceRate = float64(summary.LocalMetrics.AcceptedCards) / float64(reviewedCards)
	}
	qualityGatePassed := summary.FailedChunks == 0 && len(failedChecks) == 0
	reason := qualityVerdictReason(externalStatus, qualityGatePassed, reasons)
	summary.LocalMetrics.LastFailureReason = firstNonEmpty(summary.LocalMetrics.LastFailureReason, reason)
	summary.QualityVerdict = domain.QualityVerdict{
		Verdict:           verdictStatus(externalStatus, qualityGatePassed),
		ExternalStatus:    externalStatus,
		Detector:          detector,
		ScoreBefore:       scoreBefore,
		ScoreAfter:        scoreAfter,
		TargetScore:       targetScore,
		QualityGatePassed: qualityGatePassed,
		FailedChecks:      failedChecks,
		Reason:            reason,
	}
}

func defaultQualityVerdict(status domain.RoundStatus) domain.QualityVerdict {
	verdict := domain.QualityVerdict{
		Verdict:           domain.QualityVerdictUnverified,
		ExternalStatus:    domain.ExternalDetectorUnavailable,
		QualityGatePassed: status == domain.RoundCompleted,
		FailedChecks:      []domain.CheckType{},
		Reason:            "No external detector result is available for this round.",
	}
	if status == domain.RoundFailed {
		verdict.Verdict = domain.QualityVerdictNeedsReview
		verdict.QualityGatePassed = false
		verdict.Reason = "The round did not complete successfully."
	}
	return verdict
}

func verdictStatus(externalStatus domain.ExternalDetectorStatus, qualityGatePassed bool) domain.QualityVerdictStatus {
	switch {
	case externalStatus == domain.ExternalDetectorUnavailable:
		return domain.QualityVerdictUnverified
	case externalStatus == domain.ExternalDetectorPassed && qualityGatePassed:
		return domain.QualityVerdictAccepted
	default:
		return domain.QualityVerdictNeedsReview
	}
}

func qualityVerdictReason(externalStatus domain.ExternalDetectorStatus, qualityGatePassed bool, reasons []string) string {
	if externalStatus == domain.ExternalDetectorUnavailable {
		return "External detector was unavailable; local checks were used, so this result is unverified."
	}
	if externalStatus == domain.ExternalDetectorFailed {
		return "External detector score is above the target; review the highlighted passages."
	}
	if !qualityGatePassed {
		if len(reasons) > 0 {
			return reasons[0]
		}
		return "One or more quality checks failed and need review."
	}
	return "External detector and local quality checks passed."
}

func averageScore(chunks []domain.Chunk, selectScore func(domain.Chunk) *domain.AIScore) *float64 {
	total := 0.0
	count := 0
	for _, chunk := range chunks {
		if value, ok := domain.DecisionScore(selectScore(chunk)); ok {
			total += value
			count++
		}
	}
	if count == 0 {
		return nil
	}
	avg := total / float64(count)
	return &avg
}

func averageTargetScore(chunks []domain.Chunk) *float64 {
	total := 0.0
	count := 0
	for _, chunk := range chunks {
		if chunk.OutputScore != nil && chunk.OutputScore.Target > 0 {
			total += chunk.OutputScore.Target
			count++
			continue
		}
		if chunk.Score != nil && chunk.Score.Target > 0 {
			total += chunk.Score.Target
			count++
		}
	}
	if count == 0 {
		target := domain.DefaultRoundStopThreshold
		return &target
	}
	avg := total / float64(count)
	return &avg
}

func scoreExternalStatus(score *domain.AIScore) domain.ExternalDetectorStatus {
	if score == nil {
		return domain.ExternalDetectorUnavailable
	}
	if score.ExternalStatus != "" {
		if score.ExternalStatus == domain.ExternalDetectorPassed && meetsScoreTarget(score) {
			return domain.ExternalDetectorPassed
		}
		if score.ExternalStatus == domain.ExternalDetectorPassed {
			return domain.ExternalDetectorFailed
		}
		return score.ExternalStatus
	}
	mode := ""
	if score.Calibrated != nil {
		mode = strings.TrimSpace(strings.ToLower(score.Calibrated.Mode))
	}
	detector := strings.TrimSpace(strings.ToLower(scoreDetectorName(score)))
	if mode == "" || strings.Contains(mode, "offline") || strings.Contains(mode, "local") || strings.Contains(detector, "fallback") || strings.Contains(detector, "内部启发式") {
		return domain.ExternalDetectorUnavailable
	}
	if meetsScoreTarget(score) {
		return domain.ExternalDetectorPassed
	}
	return domain.ExternalDetectorFailed
}

func meetsScoreTarget(score *domain.AIScore) bool {
	value, ok := domain.DecisionScore(score)
	if !ok {
		return false
	}
	target := score.Target
	if target <= 0 {
		target = domain.DefaultRoundStopThreshold
	}
	return value <= target
}

func scoreDetectorName(score *domain.AIScore) string {
	if score == nil {
		return ""
	}
	if score.Calibrated != nil && strings.TrimSpace(score.Calibrated.Detector) != "" {
		return strings.TrimSpace(score.Calibrated.Detector)
	}
	return strings.TrimSpace(score.Detector)
}

func appendUniqueCheck(checks []domain.CheckType, check domain.CheckType) []domain.CheckType {
	for _, existing := range checks {
		if existing == check {
			return checks
		}
	}
	return append(checks, check)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mergeLocalMetrics(target *domain.LocalMetrics, source domain.LocalMetrics) {
	if target == nil {
		return
	}
	target.DurationSeconds += source.DurationSeconds
	target.TotalTokens += source.TotalTokens
	target.AcceptedCards += source.AcceptedCards
	target.RejectedCards += source.RejectedCards
	target.PendingCards += source.PendingCards
	target.LastFailureReason = firstNonEmpty(source.LastFailureReason, target.LastFailureReason)
	reviewedCards := target.AcceptedCards + target.RejectedCards
	if reviewedCards > 0 {
		target.AcceptanceRate = float64(target.AcceptedCards) / float64(reviewedCards)
	}
}

func collectCheckpointIDs(rounds []domain.Round) []string {
	ids := make([]string, 0, len(rounds))
	seen := map[string]struct{}{}
	for _, round := range rounds {
		checkpointID := strings.TrimSpace(round.CheckpointID)
		if checkpointID == "" {
			continue
		}
		if _, ok := seen[checkpointID]; ok {
			continue
		}
		seen[checkpointID] = struct{}{}
		ids = append(ids, checkpointID)
	}
	return ids
}

func maxActivityTime(current time.Time, candidate time.Time) time.Time {
	if current.IsZero() || candidate.After(current) {
		return candidate
	}
	return current
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
	detector := domain.AIRateDetectorName
	switch {
	case chunk.OutputScore != nil && chunk.OutputScore.Calibrated != nil && strings.TrimSpace(chunk.OutputScore.Calibrated.Detector) != "":
		detector = chunk.OutputScore.Calibrated.Detector
	case chunk.Score != nil && chunk.Score.Calibrated != nil && strings.TrimSpace(chunk.Score.Calibrated.Detector) != "":
		detector = chunk.Score.Calibrated.Detector
	}
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
		Detector:       detector,
		Score:          chunk.Score,
		OutputScore:    chunk.OutputScore,
		Sentences:      append([]domain.SentenceDecision(nil), chunk.Sentences...),
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

func (s *Service) decorateSessionRoundPlan(ctx context.Context, session *domain.Session) {
	if session == nil {
		return
	}
	totalRounds, nextRound, canStartNext := s.resolveRoundPlan(ctx, session)
	session.TotalRounds = totalRounds
	session.NextRound = nextRound
	session.CanStartNext = canStartNext
}

func (s *Service) resolveRoundPlan(ctx context.Context, session *domain.Session) (totalRounds int, nextRound int, canStartNext bool) {
	totalRounds = 1
	if session == nil {
		return totalRounds, 0, false
	}

	profile, ok := domain.Profiles[session.PromptProfile]
	if !ok || profile.MaxRounds <= 0 {
		profile.MaxRounds = 1
	}
	totalRounds = profile.MaxRounds

	completed := 0
	var latestCompleted *domain.Round
	for index := range session.Rounds {
		round := &session.Rounds[index]
		if round.Status != domain.RoundCompleted {
			continue
		}
		completed++
		if latestCompleted == nil || round.Number > latestCompleted.Number {
			latestCompleted = round
		}
	}

	if session.Status == domain.SessionProcessing || session.Status == domain.SessionPaused {
		if active, err := s.rounds.GetActiveBySession(ctx, session.ID); err == nil {
			return totalRounds, active.Number, false
		}
	}

	nextRound = len(session.Rounds) + 1
	if nextRound < 1 {
		nextRound = 1
	}

	switch session.PromptProfile {
	case "cn":
		totalRounds = 2
		if latestCompleted != nil && s.shouldStopAfterRound(ctx, session, latestCompleted) {
			totalRounds = completed
		}
	case "cn_single", "en":
		totalRounds = 1
	}

	if totalRounds < completed {
		totalRounds = completed
	}
	if totalRounds < 1 {
		totalRounds = 1
	}

	canStartNext = session.Status == domain.SessionPending && nextRound <= totalRounds
	if !canStartNext && nextRound > totalRounds {
		nextRound = 0
	}
	if session.Status == domain.SessionCompleted {
		canStartNext = false
		nextRound = 0
	}

	return totalRounds, nextRound, canStartNext
}

func (s *Service) shouldStopAfterRound(ctx context.Context, session *domain.Session, round *domain.Round) bool {
	if session == nil || round == nil {
		return false
	}
	if s == nil || s.rounds == nil || s.manifests == nil || s.layout == nil {
		return false
	}
	if session.PromptProfile != "cn" || round.Number < 1 {
		return false
	}
	text, _, err := s.ReadOutput(ctx, session.ID, round.Number)
	if err != nil {
		return false
	}
	score, err := s.textScorer().Score(ctx, text)
	if err != nil {
		return false
	}
	return domain.MeetsRoundStopTarget(&score)
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
