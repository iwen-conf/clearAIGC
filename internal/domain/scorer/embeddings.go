package scorer

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type SimilarityResult struct {
	Score float64
	Mode  string
}

type SemanticSimilarity interface {
	Name() string
	Compare(ctx context.Context, input, output string) (SimilarityResult, error)
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type OpenAICompatibleEmbeddings struct {
	requester *requester
}

func NewOpenAICompatibleEmbeddings(cfg domain.ProviderConfig) *OpenAICompatibleEmbeddings {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	return &OpenAICompatibleEmbeddings{requester: newRequester(cfg)}
}

func (e *OpenAICompatibleEmbeddings) Name() string {
	if e == nil || e.requester == nil {
		return "embedding_unavailable"
	}
	name := strings.TrimSpace(e.requester.cfg.Name)
	if name == "" {
		name = "openai-embeddings"
	}
	model := strings.TrimSpace(e.requester.cfg.Model)
	if model == "" {
		return name
	}
	return fmt.Sprintf("%s:%s", name, model)
}

func (e *OpenAICompatibleEmbeddings) Compare(ctx context.Context, input, output string) (SimilarityResult, error) {
	if e == nil || e.requester == nil {
		return SimilarityResult{}, fmt.Errorf("embeddings not configured")
	}
	body := embeddingRequest{
		Model: e.requester.cfg.Model,
		Input: []string{input, output},
	}
	var out embeddingResponse
	if err := e.requester.post(ctx, "/v1/embeddings", body, &out); err != nil {
		return SimilarityResult{}, err
	}
	if len(out.Data) < 2 {
		return SimilarityResult{}, fmt.Errorf("embeddings response missing vectors")
	}
	score := cosineSimilarity(out.Data[0].Embedding, out.Data[1].Embedding)
	return SimilarityResult{
		Score: math.Max(0, math.Min(1, score)),
		Mode:  inferDetectorMode(e.Name()),
	}, nil
}

func cosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(right) == 0 || len(left) != len(right) {
		return 0
	}
	dot := 0.0
	leftNorm := 0.0
	rightNorm := 0.0
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
