package domain

import "context"

type AIScorer interface {
	Score(ctx context.Context, text string) (AIScore, error)
	ScoreSentences(ctx context.Context, text string) ([]SentenceScore, error)
}
