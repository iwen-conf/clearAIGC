package domain

import (
	"time"

	"github.com/google/uuid"
)

type TimelineTone string

const (
	TimelineToneNeutral TimelineTone = "neutral"
	TimelineToneSuccess TimelineTone = "success"
	TimelineToneWarning TimelineTone = "warning"
	TimelineToneError   TimelineTone = "error"
)

type SessionProgressSnapshot struct {
	SessionID       uuid.UUID `json:"sessionId"`
	Round           int       `json:"round"`
	Phase           string    `json:"phase"`
	CompletedChunks int       `json:"completedChunks"`
	TotalChunks     int       `json:"totalChunks"`
	Percent         float64   `json:"percent"`
	ChunkID         string    `json:"chunkId"`
	ParagraphIndex  int       `json:"paragraphIndex"`
	ChunkIndex      int       `json:"chunkIndex"`
	ProviderUsed    string    `json:"providerUsed"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type SessionTimelineEntry struct {
	ID        string       `json:"id"`
	SessionID uuid.UUID    `json:"-"`
	Tone      TimelineTone `json:"tone"`
	Title     string       `json:"title"`
	Detail    string       `json:"detail"`
	Timestamp int64        `json:"timestamp"`
}

type OutputPreview struct {
	Text           string `json:"text"`
	SegmentCount   int    `json:"segmentCount,omitempty"`
	ParagraphCount int    `json:"paragraphCount,omitempty"`
}

type SessionState struct {
	Session    *Session                 `json:"session"`
	Preview    *OutputPreview           `json:"preview,omitempty"`
	Comparison *RoundDiff               `json:"comparison,omitempty"`
	Progress   *SessionProgressSnapshot `json:"progress,omitempty"`
	Timeline   []SessionTimelineEntry   `json:"timeline"`
}
