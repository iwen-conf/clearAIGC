package domain

import "time"

// SessionListItem represents one history-list row returned by GET /sessions.
type SessionListItem struct {
	Session  *Session                 `json:"session"`
	Progress *SessionProgressSnapshot `json:"progress"`
	Metrics  SessionListMetrics       `json:"metrics"`
}

// SessionListMetrics contains derived session-level counters for history views.
type SessionListMetrics struct {
	CompletedRounds int       `json:"completedRounds"`
	TotalRounds     int       `json:"totalRounds"`
	TotalTokens     int64     `json:"totalTokens"`
	LastActivityAt  time.Time `json:"lastActivityAt"`
}

// RoundHistoryEntry represents one round plus its derived summary in history.
type RoundHistoryEntry struct {
	Round   *Round              `json:"round"`
	Summary RoundHistorySummary `json:"summary"`
}

// RoundHistorySummary contains per-round aggregate values for history details.
type RoundHistorySummary struct {
	ChunkCount      int   `json:"chunkCount"`
	PassedChunks    int   `json:"passedChunks"`
	RecoveredChunks int   `json:"recoveredChunks"`
	FailedChunks    int   `json:"failedChunks"`
	ScoreTotal      *int  `json:"scoreTotal"`
	DurationSeconds int64 `json:"durationSeconds"`
}

// SessionHistory represents the aggregated payload returned by GET /sessions/:id/history.
type SessionHistory struct {
	Session  *Session                 `json:"session"`
	Progress *SessionProgressSnapshot `json:"progress"`
	Metrics  SessionListMetrics       `json:"metrics"`
	Rounds   []RoundHistoryEntry      `json:"rounds"`
	Timeline []SessionTimelineEntry   `json:"timeline"`
}
