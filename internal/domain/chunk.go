package domain

import "github.com/google/uuid"

type ChunkMetric string

const (
	ChunkMetricChar ChunkMetric = "char"
	ChunkMetricWord ChunkMetric = "word"
)

// ChunkReviewState tracks whether a rewrite was accepted for export.
type ChunkReviewState string

const (
	ChunkReviewPending  ChunkReviewState = "pending"
	ChunkReviewAccepted ChunkReviewState = "accepted"
	ChunkReviewRejected ChunkReviewState = "rejected"
)

type ParagraphMapping struct {
	ParagraphIndex int      `json:"paragraph_index"`
	ChunkIDs       []string `json:"chunk_ids"`
	Text           string   `json:"text"`
}

type Chunk struct {
	ID             string           `json:"id"`
	ParagraphIndex int              `json:"paragraph_index"`
	ChunkIndex     int              `json:"chunk_index"`
	Text           string           `json:"text"`
	MetricValue    int              `json:"metric_value"`
	Output         string           `json:"output,omitempty"`
	Status         ChunkStatus      `json:"status,omitempty"`
	Checks         []CheckResult    `json:"checks,omitempty"`
	AIRate         float64          `json:"ai_rate,omitempty"`
	State          ChunkReviewState `json:"state,omitempty"`
}

type Manifest struct {
	ID             uuid.UUID          `json:"id"`
	RoundID        uuid.UUID          `json:"round_id"`
	ChunkLimit     int                `json:"chunk_limit"`
	ChunkMetric    ChunkMetric        `json:"chunk_metric"`
	ParagraphCount int                `json:"paragraph_count"`
	ChunkCount     int                `json:"chunk_count"`
	Paragraphs     []ParagraphMapping `json:"paragraphs"`
	Chunks         []Chunk            `json:"chunks"`
}

type ChunkResult struct {
	Chunk          Chunk
	Input          string
	Output         string
	Status         ChunkStatus
	Checks         []CheckResult
	Recovered      bool
	RecoveryMethod string
	RecoverySteps  int
	TokenCost      int
}

type ChunkStatus string

const (
	ChunkPassed    ChunkStatus = "passed"
	ChunkRecovered ChunkStatus = "recovered"
	ChunkFailed    ChunkStatus = "failed"
)

type ChunkDiff struct {
	ID             string           `json:"id"`
	ParagraphIndex int              `json:"paragraphIndex"`
	ChunkIndex     int              `json:"chunkIndex"`
	Input          string           `json:"input"`
	Output         string           `json:"output"`
	Status         ChunkStatus      `json:"status"`
	CharDelta      int              `json:"charDelta"`
	AIRate         float64          `json:"aiRate"`
	State          ChunkReviewState `json:"state"`
	Checks         []CheckResult    `json:"checks"`
}

type RoundDiff struct {
	Round  int         `json:"round"`
	Chunks []ChunkDiff `json:"chunks"`
}

func ChunkStatusFromReport(report QualityReport) ChunkStatus {
	switch {
	case !report.AllPassed:
		return ChunkFailed
	case report.Recovered:
		return ChunkRecovered
	default:
		return ChunkPassed
	}
}
