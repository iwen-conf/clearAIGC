package domain

import "math"

const DefaultRoundStopThreshold = 0.30

type ScoreSignal struct {
	Name        string  `json:"name"`
	Label       string  `json:"label"`
	Value       float64 `json:"value"`
	Normalized  float64 `json:"normalized"`
	Description string  `json:"description,omitempty"`
}

type CalibratedScore struct {
	Detector    string  `json:"detector"`
	Score       float64 `json:"score"`
	Correlation float64 `json:"correlation"`
	Mode        string  `json:"mode,omitempty"`
	SampleCount int     `json:"sampleCount,omitempty"`
}

type AIScore struct {
	Total          float64          `json:"total"`
	Target         float64          `json:"target,omitempty"`
	Detector       string           `json:"detector"`
	ExternalStatus ExternalDetectorStatus `json:"externalStatus,omitempty"`
	Signals        []ScoreSignal    `json:"signals,omitempty"`
	Features       []string         `json:"features,omitempty"`
	Forbidden      []string         `json:"forbidden,omitempty"`
	Calibrated     *CalibratedScore `json:"calibrated,omitempty"`
	ThresholdGreen float64          `json:"thresholdGreen,omitempty"`
	ThresholdRed   float64          `json:"thresholdRed,omitempty"`
}

func DecisionScore(score *AIScore) (float64, bool) {
	if score == nil {
		return 0, false
	}
	if score.Calibrated != nil {
		return clampScore(score.Calibrated.Score), true
	}
	return clampScore(score.Total), true
}

func MeetsRoundStopTarget(score *AIScore) bool {
	value, ok := DecisionScore(score)
	return ok && value <= DefaultRoundStopThreshold
}

func clampScore(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

type SentenceScore struct {
	Index      int     `json:"index"`
	Text       string  `json:"text"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Total      float64 `json:"total"`
	Importance float64 `json:"importance"`
}

type RewriteCandidate struct {
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	Worker         string  `json:"worker"`
	Output         string  `json:"output"`
	Loss           float64 `json:"loss"`
	Accepted       bool    `json:"accepted"`
	Reason         string  `json:"reason,omitempty"`
	TokenCost      int     `json:"tokenCost"`
	Score          AIScore `json:"score"`
	Similarity     float64 `json:"similarity,omitempty"`
	SimilarityMode string  `json:"similarityMode,omitempty"`
}

type SentenceDecision struct {
	Index      int                `json:"index"`
	Input      string             `json:"input"`
	Output     string             `json:"output"`
	Start      int                `json:"start,omitempty"`
	End        int                `json:"end,omitempty"`
	Original   AIScore            `json:"original"`
	Final      AIScore            `json:"final"`
	Candidates []RewriteCandidate `json:"candidates,omitempty"`
	Accepted   bool               `json:"accepted"`
	Reason     string             `json:"reason,omitempty"`
}
