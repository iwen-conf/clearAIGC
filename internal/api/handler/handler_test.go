package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	ipostgres "github.com/iwen-conf/Naturalize/internal/infra/postgres"
	"github.com/iwen-conf/Naturalize/internal/service"
	"github.com/iwen-conf/Naturalize/pkg/storage"
)

type fakeSessionRepository struct {
	session *domain.Session
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
}

func (r *fakeManifestRepository) Create(_ context.Context, manifest *domain.Manifest) error {
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
	lastText string
}

func (e *fakeExporter) Export(_ context.Context, text string, _ domain.DocumentFormat, destination string) error {
	e.lastText = text
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

type fakeHealth struct{}

func (h fakeHealth) Live(*gin.Context)   {}
func (h fakeHealth) Ready(*gin.Context)  {}
func (h fakeHealth) Health(*gin.Context) {}

type cardsHTTPResponse struct {
	Round int            `json:"round"`
	Cards []cardResponse `json:"cards"`
}

type cardHTTPResponse struct {
	Card cardResponse `json:"card"`
}

type applyAllHTTPResponse struct {
	Round   int            `json:"round"`
	Updated int            `json:"updated"`
	Cards   []cardResponse `json:"cards"`
}

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func TestListCardsReturnsCardPayload(t *testing.T) {
	t.Parallel()

	router, sessionID, _, _, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{
				ID:             "p0_c0",
				ParagraphIndex: 0,
				ChunkIndex:     0,
				Text:           "Original text",
				Output:         "Rewritten text",
				Status:         domain.ChunkPassed,
				AIRate:         0.5,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID.String()+"/cards?round=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp cardsHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Round != 1 {
		t.Fatalf("round mismatch: %d", resp.Round)
	}
	if len(resp.Cards) != 1 {
		t.Fatalf("card count mismatch: %d", len(resp.Cards))
	}
	if got := resp.Cards[0].Original; got != "Original text" {
		t.Fatalf("original mismatch: %q", got)
	}
	if got := resp.Cards[0].Rewrite; got != "Rewritten text" {
		t.Fatalf("rewrite mismatch: %q", got)
	}
	if got := resp.Cards[0].State; got != domain.ChunkReviewPending {
		t.Fatalf("state mismatch: %q", got)
	}
	if got := resp.Cards[0].Detector; got != domain.AIRateDetectorName {
		t.Fatalf("detector mismatch: %q", got)
	}
}

func TestAcceptCardReturnsUpdatedCard(t *testing.T) {
	t.Parallel()

	router, sessionID, manifestRepo, _, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Text: "Original text", Output: "Rewritten text"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID.String()+"/cards/p0_c0/accept?round=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp cardHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp.Card.State; got != domain.ChunkReviewAccepted {
		t.Fatalf("state mismatch: %q", got)
	}
	if got := manifestRepo.manifest.Chunks[0].State; got != domain.ChunkReviewAccepted {
		t.Fatalf("saved state mismatch: %q", got)
	}
}

func TestApplyAllCardsReturnsUpdatedCount(t *testing.T) {
	t.Parallel()

	router, sessionID, _, _, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Text: "A", Output: "A+", State: domain.ChunkReviewPending},
			{ID: "p0_c1", Text: "B", Output: "B+", State: domain.ChunkReviewRejected},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID.String()+"/cards/apply-all?round=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp applyAllHTTPResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("updated mismatch: %d", resp.Updated)
	}
	if got := resp.Cards[0].State; got != domain.ChunkReviewAccepted {
		t.Fatalf("pending card was not accepted: %q", got)
	}
	if got := resp.Cards[1].State; got != domain.ChunkReviewRejected {
		t.Fatalf("rejected card changed unexpectedly: %q", got)
	}
}

func TestRejectMissingCardReturnsNotFound(t *testing.T) {
	t.Parallel()

	router, sessionID, _, _, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
	})

	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID.String()+"/cards/missing/reject?round=1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != "CARD_NOT_FOUND" {
		t.Fatalf("error code mismatch: %s", resp.Error.Code)
	}
}

func TestGetSessionStateReturnsPersistedState(t *testing.T) {
	t.Parallel()

	router, sessionID, _, _, stateRepo := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Text: "Original text", Output: "Rewritten text", Status: domain.ChunkPassed},
		},
	})
	stateRepo.progress = &domain.SessionProgressSnapshot{
		SessionID:       sessionID,
		Round:           1,
		Phase:           "chunk-complete",
		CompletedChunks: 1,
		TotalChunks:     2,
		Percent:         50,
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

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID.String()+"/state", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
		Preview *struct {
			Text string `json:"text"`
		} `json:"preview"`
		Progress *struct {
			Phase string `json:"phase"`
		} `json:"progress"`
		Timeline []struct {
			Title string `json:"title"`
		} `json:"timeline"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Session.ID != sessionID.String() {
		t.Fatalf("session id mismatch: %s", resp.Session.ID)
	}
	if resp.Preview == nil || resp.Preview.Text != "legacy output" {
		t.Fatalf("preview mismatch: %+v", resp.Preview)
	}
	if resp.Progress == nil || resp.Progress.Phase != "chunk-complete" {
		t.Fatalf("progress mismatch: %+v", resp.Progress)
	}
	if len(resp.Timeline) != 1 || resp.Timeline[0].Title != "文档已提交" {
		t.Fatalf("timeline mismatch: %+v", resp.Timeline)
	}
}

func TestExportPassesSelectionToService(t *testing.T) {
	t.Parallel()

	router, sessionID, _, exporter, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID:        uuid.New(),
		ChunkMetric:    domain.ChunkMetricChar,
		ParagraphCount: 2,
		ChunkCount:     2,
		Paragraphs: []domain.ParagraphMapping{
			{ParagraphIndex: 0, ChunkIDs: []string{"p0_c0"}},
			{ParagraphIndex: 1, ChunkIDs: []string{"p1_c0"}},
		},
		Chunks: []domain.Chunk{
			{ID: "p0_c0", Text: "A", Output: "A+", State: domain.ChunkReviewAccepted},
			{ID: "p1_c0", Text: "B", Output: "B+", State: domain.ChunkReviewPending},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID.String()+"/export?round=1&format=txt&selection=accepted", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if exporter.lastText != "A+\n\nB" {
		t.Fatalf("export text mismatch: %q", exporter.lastText)
	}
}

func TestExportRejectsInvalidSelection(t *testing.T) {
	t.Parallel()

	router, sessionID, _, _, _ := newHandlerHarness(t, &domain.Manifest{
		RoundID: uuid.New(),
	})

	req := httptest.NewRequest(http.MethodGet, "/sessions/"+sessionID.String()+"/export?round=1&format=txt&selection=invalid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var resp errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error.Code != "INVALID_REQUEST" {
		t.Fatalf("error code mismatch: %s", resp.Error.Code)
	}
}

func newHandlerHarness(t *testing.T, manifest *domain.Manifest) (*gin.Engine, uuid.UUID, *fakeManifestRepository, *fakeExporter, *fakeStateRepository) {
	t.Helper()

	gin.SetMode(gin.TestMode)

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

	manifestRepo := &fakeManifestRepository{manifest: cloneManifest(manifest)}
	exporter := &fakeExporter{}
	stateRepo := &fakeStateRepository{}
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
	svc := service.NewService(
		&fakeSessionRepository{session: session},
		&fakeRoundRepository{round: round},
		manifestRepo,
		nil,
		stateRepo,
		nil,
		nil,
		nil,
		exporter,
		layout,
		nil,
		nil,
	)

	h := New(svc, nil, fakeHealth{})
	router := gin.New()
	router.GET("/sessions/:id/state", h.GetSessionState)
	router.GET("/sessions/:id/cards", h.ListCards)
	router.POST("/sessions/:id/cards/apply-all", h.ApplyAllCards)
	router.POST("/sessions/:id/cards/:cardId/accept", h.AcceptCard)
	router.POST("/sessions/:id/cards/:cardId/reject", h.RejectCard)
	router.GET("/sessions/:id/export", h.Export)

	return router, sessionID, manifestRepo, exporter, stateRepo
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
			paragraphClone.ChunkIDs = append([]string(nil), paragraph.ChunkIDs...)
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
