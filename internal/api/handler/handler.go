package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
	"github.com/iwen-conf/Naturalize/internal/service"
)

type Handler struct {
	service *service.Service
	agents  *service.AgentSettingsService
	health  HealthChecker
}

type HealthChecker interface {
	Live(*gin.Context)
	Ready(*gin.Context)
	Health(*gin.Context)
}

type cardResponse struct {
	ID             string                  `json:"id"`
	ParagraphIndex int                     `json:"paragraphIndex"`
	ChunkIndex     int                     `json:"chunkIndex"`
	Original       string                  `json:"original"`
	Rewrite        string                  `json:"rewrite"`
	AIRate         float64                 `json:"aiRate"`
	OutputAIRate   float64                 `json:"outputAiRate"`
	Detector       string                  `json:"detector"`
	State          domain.ChunkReviewState `json:"state"`
	Status         domain.ChunkStatus      `json:"status"`
	Checks         []domain.CheckResult    `json:"checks"`
}

func New(svc *service.Service, agents *service.AgentSettingsService, health HealthChecker) *Handler {
	return &Handler{service: svc, agents: agents, health: health}
}

func (h *Handler) CreateSession(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		writeError(c, service.ErrInvalidRequest)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		writeError(c, err)
		return
	}
	defer file.Close()

	session, err := h.service.CreateSession(c.Request.Context(), service.CreateSessionInput{
		FileName:      fileHeader.Filename,
		FileSize:      fileHeader.Size,
		PromptProfile: c.DefaultPostForm("promptProfile", "cn"),
		FileReader:    file,
	})
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":            session.ID,
		"docId":         session.DocID,
		"status":        session.Status,
		"promptProfile": session.PromptProfile,
	})
}

func (h *Handler) CreateSessionBatch(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		writeError(c, service.ErrInvalidRequest)
		return
	}

	fileHeaders := form.File["files"]
	if len(fileHeaders) == 0 {
		writeError(c, service.ErrInvalidRequest)
		return
	}

	promptProfile := c.DefaultPostForm("promptProfile", "cn")
	created := make([]*domain.Session, 0, len(fileHeaders))
	for _, fileHeader := range fileHeaders {
		file, err := fileHeader.Open()
		if err != nil {
			writeError(c, err)
			return
		}

		session, createErr := h.service.CreateSession(c.Request.Context(), service.CreateSessionInput{
			FileName:      fileHeader.Filename,
			FileSize:      fileHeader.Size,
			PromptProfile: promptProfile,
			FileReader:    file,
		})
		_ = file.Close()
		if createErr != nil {
			for _, existing := range created {
				_ = h.service.DeleteSession(c.Request.Context(), existing.ID)
			}
			writeError(c, createErr)
			return
		}

		created = append(created, session)
	}

	sessions := make([]gin.H, 0, len(created))
	for _, session := range created {
		sessions = append(sessions, gin.H{
			"id":            session.ID,
			"docId":         session.DocID,
			"status":        session.Status,
			"promptProfile": session.PromptProfile,
		})
	}

	c.JSON(http.StatusCreated, gin.H{"sessions": sessions})
}

func (h *Handler) ListSessions(c *gin.Context) {
	page, err := parsePositiveQuery(c, "page", 1)
	if err != nil {
		writeError(c, service.ErrInvalidRequest)
		return
	}
	size, err := parsePositiveQuery(c, "size", 20)
	if err != nil {
		writeError(c, service.ErrInvalidRequest)
		return
	}
	if size > 100 {
		size = 100
	}

	query := strings.TrimSpace(c.Query("q"))
	if utf8.RuneCountInString(query) > 128 {
		writeError(c, service.ErrInvalidRequest)
		return
	}

	items, total, err := h.service.ListSessions(c.Request.Context(), domain.SessionListFilter{
		Page:   page,
		Size:   size,
		Status: domain.SessionStatus(strings.TrimSpace(c.Query("status"))),
		Query:  query,
		Sort:   strings.TrimSpace(c.Query("sort")),
	})
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": coalesceSessionListItems(items),
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func (h *Handler) GetSession(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	session, err := h.service.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *Handler) GetSessionState(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	state, err := h.service.ReadState(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *Handler) GetSessionHistory(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	history, err := h.service.ReadHistory(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, history)
}

func (h *Handler) DeleteSession(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.service.DeleteSession(c.Request.Context(), sessionID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) StartRound(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	var body struct {
		ChunkLimit int `json:"chunkLimit"`
	}
	_ = c.ShouldBindJSON(&body)
	round, err := h.service.StartNextRound(c.Request.Context(), sessionID, body.ChunkLimit)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"roundId":     round.ID,
		"roundNumber": round.Number,
		"status":      round.Status,
	})
}

func (h *Handler) PauseRound(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	round, err := h.service.PauseRound(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "paused",
		"checkpointId": round.CheckpointID,
	})
}

func (h *Handler) ResumeRound(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	round, err := h.service.ResumeRound(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"roundId": round.ID,
		"status":  round.Status,
	})
}

func (h *Handler) GetRound(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Param("num"))
	round, err := h.service.GetRound(c.Request.Context(), sessionID, number)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, round)
}

func (h *Handler) ReadOutput(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	text, manifest, err := h.service.ReadOutput(c.Request.Context(), sessionID, number)
	if err != nil {
		writeError(c, err)
		return
	}

	resp := gin.H{"text": text}
	if manifest != nil {
		resp["segmentCount"] = manifest.ChunkCount
		resp["paragraphCount"] = manifest.ParagraphCount
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ReadDiff(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	diff, err := h.service.ReadDiff(c.Request.Context(), sessionID, number)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, diff)
}

func (h *Handler) ListCards(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	diff, err := h.service.ListCards(c.Request.Context(), sessionID, number)
	if err != nil {
		writeError(c, err)
		return
	}

	cards := make([]cardResponse, 0, len(diff.Chunks))
	for _, chunk := range diff.Chunks {
		cards = append(cards, cardFromDiff(chunk))
	}
	c.JSON(http.StatusOK, gin.H{
		"round": diff.Round,
		"cards": cards,
	})
}

func (h *Handler) AcceptCard(c *gin.Context) {
	h.updateCard(c, h.service.AcceptCard)
}

func (h *Handler) RejectCard(c *gin.Context) {
	h.updateCard(c, h.service.RejectCard)
}

func (h *Handler) ApplyAllCards(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	cards, updated, err := h.service.ApplyAllCards(c.Request.Context(), sessionID, number)
	if err != nil {
		writeError(c, err)
		return
	}

	resp := make([]cardResponse, 0, len(cards))
	for _, chunk := range cards {
		resp = append(resp, cardFromDiff(chunk))
	}
	c.JSON(http.StatusOK, gin.H{
		"round":   number,
		"updated": updated,
		"cards":   resp,
	})
}

func (h *Handler) Export(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	format, ok := domain.ParseDocumentFormat(c.DefaultQuery("format", "txt"))
	if !ok {
		writeError(c, service.ErrUnsupportedFormat)
		return
	}
	path, err := h.service.Export(c.Request.Context(), sessionID, number, format, c.DefaultQuery("selection", "all"))
	if err != nil {
		writeError(c, err)
		return
	}
	filename := "rewritten." + string(format)
	c.FileAttachment(path, filename)
}

func (h *Handler) Stream(c *gin.Context) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}

	events, cancel, err := h.service.Publisher().Subscribe(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, err)
		return
	}
	defer cancel()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeError(c, service.ErrInvalidRequest)
		return
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			_, _ = c.Writer.Write([]byte(": heartbeat\n\n"))
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, _ := json.Marshal(event.Data)
			_, _ = c.Writer.Write([]byte("event: " + event.Type + "\n"))
			_, _ = c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
			flusher.Flush()
		}
	}
}

func (h *Handler) Live(c *gin.Context)   { h.health.Live(c) }
func (h *Handler) Ready(c *gin.Context)  { h.health.Ready(c) }
func (h *Handler) Health(c *gin.Context) { h.health.Health(c) }

func parseUUIDParam(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil {
		writeError(c, service.ErrInvalidRequest)
		return uuid.Nil, false
	}
	return id, true
}

func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch service.ErrorCode(err) {
	case "INVALID_REQUEST", "INVALID_FILE_FORMAT":
		status = http.StatusBadRequest
	case "SESSION_NOT_FOUND", "ROUND_NOT_FOUND", "CARD_NOT_FOUND":
		status = http.StatusNotFound
	case "SESSION_LOCKED":
		status = http.StatusConflict
	case "ALL_ROUNDS_COMPLETED", "ROUND_STATE_INVALID":
		status = http.StatusUnprocessableEntity
	case "AGENT_NOT_FOUND":
		status = http.StatusNotFound
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    service.ErrorCode(err),
			"message": service.ErrorMessage(err),
			"details": gin.H{},
		},
	})
}

func parsePositiveQuery(c *gin.Context, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, service.ErrInvalidRequest
	}
	return value, nil
}

func coalesceSessionListItems(items []domain.SessionListItem) []domain.SessionListItem {
	if items == nil {
		return []domain.SessionListItem{}
	}
	return items
}

func (h *Handler) updateCard(c *gin.Context, fn func(context.Context, uuid.UUID, int, string) (*domain.ChunkDiff, error)) {
	sessionID, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	number, _ := strconv.Atoi(c.Query("round"))
	card, err := fn(c.Request.Context(), sessionID, number, c.Param("cardId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"card": cardFromDiff(*card)})
}

func cardFromDiff(chunk domain.ChunkDiff) cardResponse {
	return cardResponse{
		ID:             chunk.ID,
		ParagraphIndex: chunk.ParagraphIndex,
		ChunkIndex:     chunk.ChunkIndex,
		Original:       chunk.Input,
		Rewrite:        chunk.Output,
		AIRate:         chunk.AIRate,
		OutputAIRate:   chunk.OutputAIRate,
		Detector:       chunk.Detector,
		State:          chunk.State,
		Status:         chunk.Status,
		Checks:         chunk.Checks,
	}
}
