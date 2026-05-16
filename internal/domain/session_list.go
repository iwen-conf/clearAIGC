package domain

import "time"

// QualityVerdictStatus is the local history acceptance outcome.
type QualityVerdictStatus string

const (
	QualityVerdictAccepted    QualityVerdictStatus = "accepted"
	QualityVerdictNeedsReview QualityVerdictStatus = "needs_review"
	QualityVerdictUnverified  QualityVerdictStatus = "unverified"
)

// ExternalDetectorStatus captures whether the external detector confirmed the output.
type ExternalDetectorStatus string

const (
	ExternalDetectorPassed      ExternalDetectorStatus = "passed"
	ExternalDetectorFailed      ExternalDetectorStatus = "failed"
	ExternalDetectorUnavailable ExternalDetectorStatus = "unavailable"
)

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
	NextRound       int       `json:"nextRound,omitempty"`
	CanStartNext    bool      `json:"canStartNextRound,omitempty"`
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
	ChunkCount      int            `json:"chunkCount"`
	PassedChunks    int            `json:"passedChunks"`
	RecoveredChunks int            `json:"recoveredChunks"`
	FailedChunks    int            `json:"failedChunks"`
	ScoreTotal      *int           `json:"scoreTotal"`
	DurationSeconds int64          `json:"durationSeconds"`
	QualityVerdict  QualityVerdict `json:"qualityVerdict"`
	LocalMetrics    LocalMetrics   `json:"localMetrics"`
}

// QualityVerdict describes how the latest output should be interpreted.
type QualityVerdict struct {
	Verdict           QualityVerdictStatus   `json:"verdict"`
	ExternalStatus    ExternalDetectorStatus `json:"externalStatus"`
	Detector          string                 `json:"detector,omitempty"`
	ScoreBefore       *float64               `json:"scoreBefore"`
	ScoreAfter        *float64               `json:"scoreAfter"`
	TargetScore       *float64               `json:"targetScore"`
	QualityGatePassed bool                   `json:"qualityGatePassed"`
	FailedChecks      []CheckType            `json:"failedChecks"`
	Reason            string                 `json:"reason,omitempty"`
}

// LocalMetrics contains local-only operational counters for a history detail page.
type LocalMetrics struct {
	DurationSeconds  int64   `json:"durationSeconds"`
	TotalTokens      int64   `json:"totalTokens"`
	AcceptedCards    int     `json:"acceptedCards"`
	RejectedCards    int     `json:"rejectedCards"`
	PendingCards     int     `json:"pendingCards"`
	AcceptanceRate   float64 `json:"acceptanceRate"`
	LastFailureReason string  `json:"lastFailureReason,omitempty"`
}

// SessionHistory represents the aggregated payload returned by GET /sessions/:id/history.
type SessionHistory struct {
	Session        *Session                 `json:"session"`
	Progress       *SessionProgressSnapshot `json:"progress"`
	Metrics        SessionListMetrics       `json:"metrics"`
	QualityVerdict QualityVerdict           `json:"qualityVerdict"`
	LocalMetrics   LocalMetrics             `json:"localMetrics"`
	Rounds         []RoundHistoryEntry      `json:"rounds"`
	Timeline       []SessionTimelineEntry   `json:"timeline"`
}
