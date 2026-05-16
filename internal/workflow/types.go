package workflow

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

var ErrPauseRequested = errors.New("pause requested")

type Input struct {
	Session domain.Session
	Round   domain.Round
}

type State struct {
	Input                 Input
	ParsedText            string
	Manifest              *domain.Manifest
	RawOutputs            []string
	FinalOutputs          []string
	Reports               []domain.QualityReport
	PendingRecovery       []int
	MergedOutput          string
	ProviderUsed          string
	TotalTokens           int64
	ScoreTotal            int
	QualityStats          *domain.QualityStats
	RecoveryJustification string
	ChunkScore            *domain.AIScore
	StopAfterRound        bool
}

type ResumeState struct {
	Stage                 string                 `json:"stage"`
	NextIndex             int                    `json:"next_index"`
	RawOutputs            []string               `json:"raw_outputs,omitempty"`
	FinalOutputs          []string               `json:"final_outputs,omitempty"`
	Reports               []domain.QualityReport `json:"reports,omitempty"`
	TotalTokens           int64                  `json:"total_tokens"`
	ProviderUsed          string                 `json:"provider_used,omitempty"`
	RecoveryJustification string                 `json:"recovery_justification,omitempty"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

type PauseChecker interface {
	ShouldPause(sessionID uuid.UUID) bool
}
