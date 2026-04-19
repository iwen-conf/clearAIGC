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
	session *domain.Session
	lockErr error
}

func (r *fakeSessionRepository) Create(context.Context, *domain.Session) error {
	return nil
}

func (r *fakeSessionRepository) Get(context.Context, uuid.UUID) (*domain.Session, error) {
	if r.session != nil {
		copySession := *r.session
		copySession.Rounds = append([]domain.Round(nil), r.session.Rounds...)
		return &copySession, nil
	}
	return nil, ipostgres.ErrNotFound
}

func (r *fakeSessionRepository) List(context.Context, domain.SessionListFilter) ([]domain.Session, int, error) {
	return nil, 0, nil
}

func (r *fakeSessionRepository) Delete(context.Context, uuid.UUID) error {
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
	progress *domain.SessionProgressSnapshot
	timeline []domain.SessionTimelineEntry
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
	return nil
}

func (r *fakeStateRepository) GetProgress(context.Context, uuid.UUID) (*domain.SessionProgressSnapshot, error) {
	if r.progress == nil {
		return nil, ipostgres.ErrNotFound
	}
	copySnapshot := *r.progress
	return &copySnapshot, nil
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
	return nil
}

func (r *fakeStateRepository) ListTimeline(context.Context, uuid.UUID, int) ([]domain.SessionTimelineEntry, error) {
	return append([]domain.SessionTimelineEntry(nil), r.timeline...), nil
}

func (r *fakeStateRepository) DeleteForSession(_ context.Context, _ uuid.UUID) error {
	r.progress = nil
	r.timeline = nil
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
