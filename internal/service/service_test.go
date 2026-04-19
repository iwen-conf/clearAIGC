package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	ipostgres "github.com/iwen-conf/Naturalize/internal/infra/postgres"
	"github.com/iwen-conf/Naturalize/pkg/storage"
)

type fakeSessionRepository struct {
	session    *domain.Session
	sessions   []domain.Session
	total      int
	lastFilter domain.SessionListFilter
	deletedIDs []uuid.UUID
	deleteErr  error
	lockErr    error
}

func (r *fakeSessionRepository) Create(context.Context, *domain.Session) error {
	return nil
}

func (r *fakeSessionRepository) Get(_ context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	if r.session != nil && r.session.ID == sessionID {
		copySession := *r.session
		copySession.Rounds = append([]domain.Round(nil), r.session.Rounds...)
		return &copySession, nil
	}
	for index := range r.sessions {
		session := r.sessions[index]
		if session.ID != sessionID {
			continue
		}
		copySession := session
		copySession.Rounds = append([]domain.Round(nil), session.Rounds...)
		return &copySession, nil
	}
	return nil, ipostgres.ErrNotFound
}

func (r *fakeSessionRepository) List(_ context.Context, filter domain.SessionListFilter) ([]domain.Session, int, error) {
	r.lastFilter = filter
	items := append([]domain.Session(nil), r.sessions...)
	total := r.total
	if total == 0 {
		total = len(items)
	}
	return items, total, nil
}

func (r *fakeSessionRepository) Delete(_ context.Context, sessionID uuid.UUID) error {
	r.deletedIDs = append(r.deletedIDs, sessionID)
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if r.session != nil && r.session.ID == sessionID {
		r.session = nil
		return nil
	}
	for index := range r.sessions {
		if r.sessions[index].ID != sessionID {
			continue
		}
		r.sessions = append(r.sessions[:index], r.sessions[index+1:]...)
		return nil
	}
	return nil
}

func (r *fakeSessionRepository) UpdateStatus(context.Context, uuid.UUID, domain.SessionStatus) error {
	return nil
}

func (r *fakeSessionRepository) AcquireLock(context.Context, uuid.UUID) (func(), error) {
	if r.lockErr != nil {
		return nil, r.lockErr
	}
	return func() {}, nil
}

type fakeRoundRepository struct {
	round *domain.Round
}

func (r *fakeRoundRepository) Create(context.Context, *domain.Round) error {
	return nil
}

func (r *fakeRoundRepository) GetBySessionAndNumber(_ context.Context, sessionID uuid.UUID, number int) (*domain.Round, error) {
	if r.round == nil || r.round.SessionID != sessionID || r.round.Number != number {
		return nil, ipostgres.ErrNotFound
	}
	return r.round, nil
}

func (r *fakeRoundRepository) GetActiveBySession(context.Context, uuid.UUID) (*domain.Round, error) {
	return nil, ipostgres.ErrNotFound
}

func (r *fakeRoundRepository) Update(context.Context, *domain.Round) error {
	return nil
}

func (r *fakeRoundRepository) CompleteRound(context.Context, *domain.Session, *domain.Round, *domain.Manifest, []domain.QualityReport) error {
	return nil
}

type fakeManifestRepository struct {
	manifest *domain.Manifest
	creates  int
}

func (r *fakeManifestRepository) Create(_ context.Context, manifest *domain.Manifest) error {
	r.creates++
	r.manifest = cloneManifest(manifest)
	return nil
}

func (r *fakeManifestRepository) GetByRoundID(_ context.Context, roundID uuid.UUID) (*domain.Manifest, error) {
	if r.manifest == nil || r.manifest.RoundID != roundID {
		return nil, ipostgres.ErrNotFound
	}
	return cloneManifest(r.manifest), nil
}

type fakeExporter struct {
	lastText   string
	lastFormat domain.DocumentFormat
}

func (e *fakeExporter) Export(_ context.Context, text string, format domain.DocumentFormat, destination string) error {
	e.lastText = text
	e.lastFormat = format
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, []byte(text), 0o644)
}

type fakeStateRepository struct {
	progress           *domain.SessionProgressSnapshot
	progressBySession  map[uuid.UUID]*domain.SessionProgressSnapshot
	timeline           []domain.SessionTimelineEntry
	timelinesBySession map[uuid.UUID][]domain.SessionTimelineEntry
}

func (r *fakeStateRepository) UpsertProgress(_ context.Context, snapshot *domain.SessionProgressSnapshot) error {
	if snapshot == nil {
		return nil
	}
	copySnapshot := *snapshot
	if copySnapshot.UpdatedAt.IsZero() {
		copySnapshot.UpdatedAt = time.Now().UTC()
	}
	r.progress = &copySnapshot
	if r.progressBySession == nil {
		r.progressBySession = map[uuid.UUID]*domain.SessionProgressSnapshot{}
	}
	snapshotCopy := copySnapshot
	r.progressBySession[copySnapshot.SessionID] = &snapshotCopy
	return nil
}

func (r *fakeStateRepository) GetProgress(_ context.Context, sessionID uuid.UUID) (*domain.SessionProgressSnapshot, error) {
	if r.progressBySession != nil {
		if snapshot, ok := r.progressBySession[sessionID]; ok {
			copySnapshot := *snapshot
			return &copySnapshot, nil
		}
	}
	if r.progress == nil {
		return nil, ipostgres.ErrNotFound
	}
	copySnapshot := *r.progress
	return &copySnapshot, nil
}

func (r *fakeStateRepository) ListProgressBySessionIDs(_ context.Context, sessionIDs []uuid.UUID) (map[uuid.UUID]*domain.SessionProgressSnapshot, error) {
	items := make(map[uuid.UUID]*domain.SessionProgressSnapshot, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if snapshot, err := r.GetProgress(context.Background(), sessionID); err == nil {
			items[sessionID] = snapshot
		}
	}
	return items, nil
}

func (r *fakeStateRepository) AppendTimeline(_ context.Context, entry *domain.SessionTimelineEntry) error {
	if entry == nil {
		return nil
	}
	copyEntry := *entry
	if copyEntry.ID == "" {
		copyEntry.ID = uuid.NewString()
	}
	if copyEntry.Timestamp == 0 {
		copyEntry.Timestamp = time.Now().UnixMilli()
	}
	r.timeline = append([]domain.SessionTimelineEntry{copyEntry}, r.timeline...)
	if r.timelinesBySession == nil {
		r.timelinesBySession = map[uuid.UUID][]domain.SessionTimelineEntry{}
	}
	r.timelinesBySession[copyEntry.SessionID] = append([]domain.SessionTimelineEntry{copyEntry}, r.timelinesBySession[copyEntry.SessionID]...)
	return nil
}

func (r *fakeStateRepository) ListTimeline(_ context.Context, sessionID uuid.UUID, limit int, ascending bool) ([]domain.SessionTimelineEntry, error) {
	entries := append([]domain.SessionTimelineEntry(nil), r.timeline...)
	if r.timelinesBySession != nil {
		entries = append([]domain.SessionTimelineEntry(nil), r.timelinesBySession[sessionID]...)
	}
	if ascending {
		for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
			entries[left], entries[right] = entries[right], entries[left]
		}
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (r *fakeStateRepository) DeleteForSession(_ context.Context, sessionID uuid.UUID) error {
	if r.progress != nil && r.progress.SessionID == sessionID {
		r.progress = nil
	}
	if r.progressBySession != nil {
		delete(r.progressBySession, sessionID)
	}
	if len(r.timeline) > 0 {
		filtered := r.timeline[:0]
		for _, entry := range r.timeline {
			if entry.SessionID != sessionID {
				filtered = append(filtered, entry)
			}
		}
		r.timeline = filtered
	}
	if r.timelinesBySession != nil {
		delete(r.timelinesBySession, sessionID)
	}
	return nil
}

func TestReadDiffBuildsChunkComparison(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{
				ID:             "p0_c0",
				ParagraphIndex: 0,
				ChunkIndex:     0,
				Text:           "在当前数字化转型不断深入推进的大背景下，企业知识管理工作正在呈现出快速发展、持续演进以及多元融合的总体趋势。与此同时，越来越多的团队开始认识到，仅仅依靠传统的文档整理方式已经较难满足现实需求。综合来看，这一方向具有现实意义和实践价值。",
				Output:         "在当前数字化转型不断深入推进的大背景下，企业知识管理工作正在呈现出快速发展、持续演进以及多元融合的总体趋势。与此同时，越来越多的团队开始认识到，仅仅依靠传统的文档整理方式已经较难满足现实需求。综合来看，这一方向具有现实意义和实践价值。a",
				Status:         domain.ChunkRecovered,
				Checks: []domain.CheckResult{
					{Type: domain.CheckDisallowedPattern, Passed: true},
				},
			},
		},
	})

	diff, err := service.ReadDiff(context.Background(), service.rounds.(*fakeRoundRepository).round.SessionID, 1)
	if err != nil {
		t.Fatalf("ReadDiff returned error: %v", err)
	}

	if diff.Round != 1 {
		t.Fatalf("round mismatch: %d", diff.Round)
	}
	if len(diff.Chunks) != 1 {
		t.Fatalf("chunk count mismatch: %d", len(diff.Chunks))
	}
	if got := diff.Chunks[0].Status; got != domain.ChunkRecovered {
		t.Fatalf("chunk status mismatch: %q", got)
	}
	if got := diff.Chunks[0].CharDelta; got != 1 {
		t.Fatalf("char delta mismatch: %d", got)
	}
	if got := diff.Chunks[0].Output; got != "在当前数字化转型不断深入推进的大背景下，企业知识管理工作正在呈现出快速发展、持续演进以及多元融合的总体趋势。与此同时，越来越多的团队开始认识到，仅仅依靠传统的文档整理方式已经较难满足现实需求。综合来看，这一方向具有现实意义和实践价值。a" {
		t.Fatalf("chunk output mismatch: %q", got)
	}
	if got := diff.Chunks[0].AIRate; got < 0.45 {
		t.Fatalf("chunk aiRate too low: %v", got)
	}
	if got := diff.Chunks[0].OutputAIRate; got < 0.45 {
		t.Fatalf("chunk output aiRate too low: %v", got)
	}
	if got := diff.Chunks[0].Detector; got != domain.AIRateDetectorName {
		t.Fatalf("chunk detector mismatch: %q", got)
	}
	if got := diff.Chunks[0].State; got != domain.ChunkReviewPending {
		t.Fatalf("chunk state mismatch: %q", got)
	}
}

func TestUpdateCardStatePersistsManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		update   func(*Service, context.Context, uuid.UUID, int, string) (*domain.ChunkDiff, error)
		expected domain.ChunkReviewState
	}{
		{
			name:     "accept",
			update:   (*Service).AcceptCard,
			expected: domain.ChunkReviewAccepted,
		},
		{
			name:     "reject",
			update:   (*Service).RejectCard,
			expected: domain.ChunkReviewRejected,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, &domain.Manifest{
				RoundID: uuid.New(),
				Chunks: []domain.Chunk{
					{
						ID:             "p0_c0",
						ParagraphIndex: 0,
						ChunkIndex:     0,
						Text:           "Original text",
						Output:         "Rewritten text",
						Status:         domain.ChunkPassed,
					},
				},
			})
			sessionID := service.rounds.(*fakeRoundRepository).round.SessionID

			card, err := tt.update(service, context.Background(), sessionID, 1, "p0_c0")
			if err != nil {
				t.Fatalf("update card returned error: %v", err)
			}
			if got := card.State; got != tt.expected {
				t.Fatalf("card state mismatch: %q", got)
			}

			manifestRepo := service.manifests.(*fakeManifestRepository)
			if manifestRepo.creates != 1 {
				t.Fatalf("manifest save count mismatch: %d", manifestRepo.creates)
			}
			if got := manifestRepo.manifest.Chunks[0].State; got != tt.expected {
				t.Fatalf("saved state mismatch: %q", got)
			}
		})
	}
}

func TestApplyAllCardsAcceptsOnlyPending(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Text: "A", Output: "A+", State: domain.ChunkReviewPending},
			{ID: "p0_c1", Text: "B", Output: "B+", State: domain.ChunkReviewAccepted},
			{ID: "p0_c2", Text: "C", Output: "C+", State: domain.ChunkReviewRejected},
		},
	})

	cards, updated, err := service.ApplyAllCards(context.Background(), service.rounds.(*fakeRoundRepository).round.SessionID, 1)
	if err != nil {
		t.Fatalf("ApplyAllCards returned error: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated count mismatch: %d", updated)
	}
	if got := cards[0].State; got != domain.ChunkReviewAccepted {
		t.Fatalf("pending card was not accepted: %q", got)
	}
	if got := cards[1].State; got != domain.ChunkReviewAccepted {
		t.Fatalf("accepted card changed unexpectedly: %q", got)
	}
	if got := cards[2].State; got != domain.ChunkReviewRejected {
		t.Fatalf("rejected card changed unexpectedly: %q", got)
	}
}

func TestReadStateReturnsPersistedProgressAndTimeline(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{
				ID:             "p0_c0",
				ParagraphIndex: 0,
				ChunkIndex:     0,
				Text:           "Original text",
				Output:         "Rewritten text",
				Status:         domain.ChunkPassed,
			},
		},
	})
	sessionID := service.rounds.(*fakeRoundRepository).round.SessionID
	stateRepo := service.states.(*fakeStateRepository)
	stateRepo.progress = &domain.SessionProgressSnapshot{
		SessionID:       sessionID,
		Round:           1,
		Phase:           "chunk-complete",
		CompletedChunks: 1,
		TotalChunks:     3,
		Percent:         33.3,
		ProviderUsed:    "openai-responses",
		UpdatedAt:       time.Now().UTC(),
	}
	stateRepo.timeline = []domain.SessionTimelineEntry{
		{
			ID:        "1",
			SessionID: sessionID,
			Tone:      domain.TimelineToneNeutral,
			Title:     "文档已提交",
			Detail:    "第一轮润色已开始。",
			Timestamp: time.Now().UnixMilli(),
		},
	}

	state, err := service.ReadState(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state.Session == nil || state.Session.ID != sessionID {
		t.Fatalf("session mismatch: %+v", state.Session)
	}
	if state.Progress == nil || state.Progress.Phase != "chunk-complete" {
		t.Fatalf("progress mismatch: %+v", state.Progress)
	}
	if len(state.Timeline) != 1 || state.Timeline[0].Title != "文档已提交" {
		t.Fatalf("timeline mismatch: %+v", state.Timeline)
	}
	if state.Preview == nil || state.Preview.Text != "legacy output" {
		t.Fatalf("preview mismatch: %+v", state.Preview)
	}
	if state.Comparison == nil || len(state.Comparison.Chunks) != 1 {
		t.Fatalf("comparison mismatch: %+v", state.Comparison)
	}
}

func TestListSessionsMetrics(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	sessionID := uuid.New()
	secondSessionID := uuid.New()
	repo := &fakeSessionRepository{
		sessions: []domain.Session{
			{
				ID:            sessionID,
				DocumentID:    uuid.New(),
				DocumentName:  "alpha.txt",
				DocID:         uuid.NewString(),
				PromptProfile: "cn",
				Status:        domain.SessionPending,
				CreatedAt:     now.Add(-2 * time.Hour),
				UpdatedAt:     now.Add(-20 * time.Minute),
				Rounds: []domain.Round{
					{ID: uuid.New(), SessionID: sessionID, Number: 1, Status: domain.RoundCompleted, TotalTokens: 120, CompletedAt: ptrTime(now.Add(-90 * time.Minute))},
					{ID: uuid.New(), SessionID: sessionID, Number: 2, Status: domain.RoundProcessing, TotalTokens: 30},
				},
			},
			{
				ID:            secondSessionID,
				DocumentID:    uuid.New(),
				DocumentName:  "beta.txt",
				DocID:         uuid.NewString(),
				PromptProfile: "en",
				Status:        domain.SessionCompleted,
				CreatedAt:     now.Add(-3 * time.Hour),
				UpdatedAt:     now.Add(-10 * time.Minute),
				Rounds: []domain.Round{
					{ID: uuid.New(), SessionID: secondSessionID, Number: 1, Status: domain.RoundCompleted, TotalTokens: 88, CompletedAt: ptrTime(now.Add(-5 * time.Minute))},
				},
			},
		},
		total: 2,
	}
	states := &fakeStateRepository{
		progressBySession: map[uuid.UUID]*domain.SessionProgressSnapshot{
			sessionID: {
				SessionID:    sessionID,
				Round:        2,
				Phase:        "chunk-complete",
				ProviderUsed: "gpt-test",
				UpdatedAt:    now.Add(-2 * time.Minute),
			},
		},
	}
	service := &Service{
		sessions: repo,
		states:   states,
	}

	items, total, err := service.ListSessions(context.Background(), domain.SessionListFilter{
		Page:   1,
		Size:   20,
		Query:  "alpha",
		Sort:   "-created_at",
		Status: domain.SessionPending,
	})
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if total != 2 {
		t.Fatalf("total mismatch: %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("item count mismatch: %d", len(items))
	}
	if repo.lastFilter.Query != "alpha" || repo.lastFilter.Sort != "-created_at" || repo.lastFilter.Status != domain.SessionPending {
		t.Fatalf("filter mismatch: %+v", repo.lastFilter)
	}
	if items[0].Progress == nil || items[0].Progress.ProviderUsed != "gpt-test" {
		t.Fatalf("progress mismatch: %+v", items[0].Progress)
	}
	if items[0].Metrics.CompletedRounds != 1 || items[0].Metrics.TotalRounds != 2 || items[0].Metrics.TotalTokens != 150 {
		t.Fatalf("metrics mismatch: %+v", items[0].Metrics)
	}
	if !items[0].Metrics.LastActivityAt.Equal(now.Add(-2 * time.Minute)) {
		t.Fatalf("last activity mismatch: %s", items[0].Metrics.LastActivityAt)
	}
	if items[1].Metrics.TotalRounds != 1 || items[1].Metrics.TotalTokens != 88 {
		t.Fatalf("second metrics mismatch: %+v", items[1].Metrics)
	}
}

func TestReadHistoryReturnsAggregatedRoundsAndTimeline(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{
		RoundID:        uuid.New(),
		ParagraphCount: 3,
		ChunkCount:     3,
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Status: domain.ChunkPassed},
			{ID: "p1_c0", Status: domain.ChunkRecovered},
			{ID: "p2_c0", Status: domain.ChunkFailed},
		},
	})
	sessionID := service.rounds.(*fakeRoundRepository).round.SessionID
	stateRepo := service.states.(*fakeStateRepository)
	stateRepo.progress = &domain.SessionProgressSnapshot{
		SessionID:    sessionID,
		Round:        1,
		Phase:        "complete",
		ProviderUsed: "gpt-4.1-mini",
		UpdatedAt:    time.Now().UTC(),
	}
	stateRepo.timeline = []domain.SessionTimelineEntry{
		{ID: "2", SessionID: sessionID, Round: 1, Title: "第 1 轮完成", Timestamp: 200},
		{ID: "1", SessionID: sessionID, Round: 1, Title: "文档已提交", Timestamp: 100},
	}
	roundRepo := service.rounds.(*fakeRoundRepository)
	startedAt := time.Now().Add(-45 * time.Second).UTC()
	completedAt := time.Now().UTC()
	score := 68
	roundRepo.round.StartedAt = &startedAt
	roundRepo.round.CompletedAt = &completedAt
	roundRepo.round.ScoreTotal = &score
	service.sessions.(*fakeSessionRepository).session.Rounds[0] = *roundRepo.round

	history, err := service.ReadHistory(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("ReadHistory returned error: %v", err)
	}
	if history.Progress == nil || history.Progress.Phase != "complete" {
		t.Fatalf("progress mismatch: %+v", history.Progress)
	}
	if len(history.Timeline) != 2 || history.Timeline[0].Title != "文档已提交" || history.Timeline[1].Title != "第 1 轮完成" {
		t.Fatalf("timeline order mismatch: %+v", history.Timeline)
	}
	if len(history.Rounds) != 1 {
		t.Fatalf("round count mismatch: %d", len(history.Rounds))
	}
	summary := history.Rounds[0].Summary
	if summary.ChunkCount != 3 || summary.PassedChunks != 1 || summary.RecoveredChunks != 1 || summary.FailedChunks != 1 {
		t.Fatalf("summary mismatch: %+v", summary)
	}
	if summary.ScoreTotal == nil || *summary.ScoreTotal != 68 {
		t.Fatalf("score mismatch: %+v", summary.ScoreTotal)
	}
	if summary.DurationSeconds <= 0 {
		t.Fatalf("duration mismatch: %d", summary.DurationSeconds)
	}
}

func TestDeleteSessionRemovesHistory(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{RoundID: uuid.New()})
	sessionID := service.rounds.(*fakeRoundRepository).round.SessionID
	stateRepo := service.states.(*fakeStateRepository)
	stateRepo.progress = &domain.SessionProgressSnapshot{SessionID: sessionID, Round: 1}
	stateRepo.timeline = []domain.SessionTimelineEntry{{ID: "1", SessionID: sessionID, Title: "文档已提交"}}

	if err := service.DeleteSession(context.Background(), sessionID); err != nil {
		t.Fatalf("DeleteSession returned error: %v", err)
	}
	if _, err := stateRepo.GetProgress(context.Background(), sessionID); !errors.Is(err, ipostgres.ErrNotFound) {
		t.Fatalf("expected progress to be deleted, got: %v", err)
	}
	if timeline, err := stateRepo.ListTimeline(context.Background(), sessionID, 0, true); err != nil || len(timeline) != 0 {
		t.Fatalf("expected timeline to be deleted, got timeline=%+v err=%v", timeline, err)
	}
	if len(service.sessions.(*fakeSessionRepository).deletedIDs) != 1 {
		t.Fatalf("delete was not propagated to session repo")
	}
}

func TestExportHonorsSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		selection string
		expected  string
	}{
		{
			name:      "all",
			selection: "all",
			expected:  "A+\n\nB+\n\nC\n\nD",
		},
		{
			name:      "accepted",
			selection: "accepted",
			expected:  "A+\n\nB\n\nC\n\nD",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, &domain.Manifest{
				RoundID:        uuid.New(),
				ChunkMetric:    domain.ChunkMetricChar,
				ParagraphCount: 4,
				ChunkCount:     4,
				Paragraphs: []domain.ParagraphMapping{
					{ParagraphIndex: 0, ChunkIDs: []string{"p0_c0"}},
					{ParagraphIndex: 1, ChunkIDs: []string{"p1_c0"}},
					{ParagraphIndex: 2, ChunkIDs: []string{"p2_c0"}},
					{ParagraphIndex: 3, ChunkIDs: []string{"p3_c0"}},
				},
				Chunks: []domain.Chunk{
					{ID: "p0_c0", Text: "A", Output: "A+", State: domain.ChunkReviewAccepted},
					{ID: "p1_c0", Text: "B", Output: "B+", State: domain.ChunkReviewPending},
					{ID: "p2_c0", Text: "C", Output: "C+", State: domain.ChunkReviewRejected},
					{ID: "p3_c0", Text: "D", Output: "", State: domain.ChunkReviewAccepted},
				},
			})
			sessionID := service.rounds.(*fakeRoundRepository).round.SessionID

			path, err := service.Export(context.Background(), sessionID, 1, domain.FormatTXT, tt.selection)
			if err != nil {
				t.Fatalf("Export returned error: %v", err)
			}

			exporter := service.exporter.(*fakeExporter)
			if exporter.lastText != tt.expected {
				t.Fatalf("export text mismatch: %q", exporter.lastText)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read export file: %v", err)
			}
			if string(data) != tt.expected {
				t.Fatalf("export file content mismatch: %q", string(data))
			}
		})
	}
}

func TestExportRejectsInvalidSelection(t *testing.T) {
	t.Parallel()

	service := newTestService(t, &domain.Manifest{
		RoundID: uuid.New(),
	})

	_, err := service.Export(context.Background(), service.rounds.(*fakeRoundRepository).round.SessionID, 1, domain.FormatTXT, "invalid")
	if err == nil {
		t.Fatal("expected invalid selection error")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTestService(t *testing.T, manifest *domain.Manifest) *Service {
	t.Helper()

	sessionID := uuid.New()
	if manifest.RoundID == uuid.Nil {
		manifest.RoundID = uuid.New()
	}

	layout := storage.NewLayout(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatalf("ensure layout: %v", err)
	}
	if err := layout.EnsureSession(sessionID, 1); err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	round := &domain.Round{
		ID:         manifest.RoundID,
		SessionID:  sessionID,
		Number:     1,
		Status:     domain.RoundCompleted,
		OutputPath: layout.RoundOutputPath(sessionID, 1),
	}
	if err := os.WriteFile(round.OutputPath, []byte("legacy output"), 0o644); err != nil {
		t.Fatalf("write round output: %v", err)
	}

	session := &domain.Session{
		ID:            sessionID,
		DocumentID:    uuid.New(),
		DocumentName:  "test.txt",
		DocID:         uuid.NewString(),
		OriginPath:    layout.OriginalPath(sessionID, "test.txt"),
		FileFormat:    domain.FormatTXT,
		FileSizeBytes: 10,
		PromptProfile: "cn",
		Status:        domain.SessionCompleted,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		Rounds:        []domain.Round{*round},
	}

	return &Service{
		sessions:  &fakeSessionRepository{session: session},
		rounds:    &fakeRoundRepository{round: round},
		manifests: &fakeManifestRepository{manifest: cloneManifest(manifest)},
		states:    &fakeStateRepository{},
		exporter:  &fakeExporter{},
		layout:    layout,
	}
}

func cloneManifest(manifest *domain.Manifest) *domain.Manifest {
	if manifest == nil {
		return nil
	}

	clone := *manifest
	if manifest.Paragraphs != nil {
		clone.Paragraphs = make([]domain.ParagraphMapping, len(manifest.Paragraphs))
		for index, paragraph := range manifest.Paragraphs {
			paragraphClone := paragraph
			if paragraph.ChunkIDs != nil {
				paragraphClone.ChunkIDs = append([]string(nil), paragraph.ChunkIDs...)
			}
			clone.Paragraphs[index] = paragraphClone
		}
	}
	if manifest.Chunks != nil {
		clone.Chunks = make([]domain.Chunk, len(manifest.Chunks))
		for index, chunk := range manifest.Chunks {
			chunkClone := chunk
			chunkClone.Checks = append([]domain.CheckResult(nil), chunk.Checks...)
			clone.Chunks[index] = chunkClone
		}
	}
	return &clone
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
