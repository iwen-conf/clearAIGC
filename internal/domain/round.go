package domain

import (
	"time"

	"github.com/google/uuid"
)

type SessionStatus string

const (
	SessionPending    SessionStatus = "pending"
	SessionProcessing SessionStatus = "processing"
	SessionPaused     SessionStatus = "paused"
	SessionCompleted  SessionStatus = "completed"
	SessionFailed     SessionStatus = "failed"
)

type RoundStatus string

const (
	RoundPending    RoundStatus = "pending"
	RoundProcessing RoundStatus = "processing"
	RoundPaused     RoundStatus = "paused"
	RoundCompleted  RoundStatus = "completed"
	RoundFailed     RoundStatus = "failed"
)

type Session struct {
	ID            uuid.UUID      `json:"id"`
	DocumentID    uuid.UUID      `json:"documentId"`
	DocumentName  string         `json:"documentName"`
	DocID         string         `json:"docId"`
	OriginPath    string         `json:"originPath"`
	FileFormat    DocumentFormat `json:"fileFormat"`
	FileSizeBytes int64          `json:"fileSizeBytes"`
	PromptProfile string         `json:"promptProfile"`
	Status        SessionStatus  `json:"status"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	Rounds        []Round        `json:"rounds"`
}

type Round struct {
	ID                    uuid.UUID   `json:"id"`
	SessionID             uuid.UUID   `json:"sessionId"`
	Number                int         `json:"number"`
	Prompt                string      `json:"prompt"`
	PromptProfile         string      `json:"promptProfile"`
	InputPath             string      `json:"inputPath"`
	OutputPath            string      `json:"outputPath"`
	ScoreTotal            *int        `json:"scoreTotal"`
	ChunkLimit            int         `json:"chunkLimit"`
	InputSegmentCount     int         `json:"inputSegmentCount"`
	OutputSegmentCount    int         `json:"outputSegmentCount"`
	CheckpointID          string      `json:"checkpointId"`
	ProviderUsed          string      `json:"providerUsed"`
	TotalTokens           int64       `json:"totalTokens"`
	Status                RoundStatus `json:"status"`
	StartedAt             *time.Time  `json:"startedAt"`
	CompletedAt           *time.Time  `json:"completedAt"`
	CreatedAt             time.Time   `json:"createdAt"`
	RecoveryJustification string      `json:"recoveryJustification,omitempty"`
}

func DeriveSessionStatus(session *Session) SessionStatus {
	if len(session.Rounds) == 0 {
		return SessionPending
	}

	last := session.Rounds[len(session.Rounds)-1]
	switch last.Status {
	case RoundProcessing:
		return SessionProcessing
	case RoundPaused:
		return SessionPaused
	case RoundFailed:
		return SessionFailed
	case RoundCompleted:
		profile, ok := Profiles[session.PromptProfile]
		if ok && last.Number >= profile.MaxRounds {
			return SessionCompleted
		}
		return SessionPending
	default:
		return SessionPending
	}
}
