package domain

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
)

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID uuid.UUID) (*Session, error)
	List(ctx context.Context, filter SessionListFilter) ([]Session, int, error)
	Delete(ctx context.Context, sessionID uuid.UUID) error
	UpdateStatus(ctx context.Context, sessionID uuid.UUID, status SessionStatus) error
	AcquireLock(ctx context.Context, sessionID uuid.UUID) (func(), error)
}

type RoundRepository interface {
	Create(ctx context.Context, round *Round) error
	GetBySessionAndNumber(ctx context.Context, sessionID uuid.UUID, number int) (*Round, error)
	GetActiveBySession(ctx context.Context, sessionID uuid.UUID) (*Round, error)
	Update(ctx context.Context, round *Round) error
	CompleteRound(ctx context.Context, session *Session, round *Round, manifest *Manifest, reports []QualityReport) error
}

type ManifestRepository interface {
	Create(ctx context.Context, manifest *Manifest) error
	GetByRoundID(ctx context.Context, roundID uuid.UUID) (*Manifest, error)
}

type QualityRepository interface {
	ReplaceForRound(ctx context.Context, roundID uuid.UUID, reports []QualityReport) error
	GetStatsByRound(ctx context.Context, roundID uuid.UUID) (*QualityStats, error)
}

type CheckpointStore interface {
	compose.CheckPointStore
	Delete(ctx context.Context, checkpointID string) error
	SaveJSON(ctx context.Context, checkpointID string, payload []byte) error
	LoadJSON(ctx context.Context, checkpointID string) ([]byte, bool, error)
	DeleteJSON(ctx context.Context, checkpointID string) error
}

type ProgressPublisher interface {
	Publish(ctx context.Context, sessionID uuid.UUID, event ProgressEvent) error
	Subscribe(ctx context.Context, sessionID uuid.UUID) (<-chan ProgressEvent, func(), error)
}

type LLMClient interface {
	Complete(ctx context.Context, request LLMRequest) (*ProviderResult, error)
}

type RecoveryAgent interface {
	Recover(ctx context.Context, request RecoveryRequest) (*RecoveryDecision, error)
}

type Rewriter interface {
	ProcessChunk(ctx context.Context, requestID string, chunk Chunk) (*ProviderResult, error)
}

type Parser interface {
	Parse(ctx context.Context, path string, format DocumentFormat) (string, error)
}

type Exporter interface {
	Export(ctx context.Context, text string, format DocumentFormat, destination string) error
}

type SessionListFilter struct {
	Page   int
	Size   int
	Status SessionStatus
}

type LLMRequest struct {
	RequestID string
	Prompt    string
}

type RecoveryRequest struct {
	RequestID     string
	Chunk         Chunk
	OriginalInput string
	CurrentOutput string
	Prompt        string
	FailedChecks  []CheckType
	Reasons       []string
}

type ProgressEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}
