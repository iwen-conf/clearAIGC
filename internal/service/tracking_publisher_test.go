package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type fakeProgressPublisher struct{}

func (fakeProgressPublisher) Publish(context.Context, uuid.UUID, domain.ProgressEvent) error {
	return nil
}

func (fakeProgressPublisher) Subscribe(context.Context, uuid.UUID) (<-chan domain.ProgressEvent, func(), error) {
	return nil, func() {}, nil
}

func TestRecordEventSetsRound(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	stateRepo := &fakeStateRepository{}
	publisher := NewTrackingPublisher(fakeProgressPublisher{}, stateRepo)

	if err := publisher.Publish(context.Background(), sessionID, domain.ProgressEvent{
		Type: "progress",
		Data: map[string]any{
			"sessionId":       sessionID,
			"round":           2,
			"phase":           "chunk-complete",
			"completedChunks": 1,
			"totalChunks":     3,
			"percent":         33.3,
			"providerUsed":    "gpt-4.1-mini",
		},
	}); err != nil {
		t.Fatalf("progress publish returned error: %v", err)
	}

	if len(stateRepo.timeline) != 1 || stateRepo.timeline[0].Round != 2 {
		t.Fatalf("progress round mismatch: %+v", stateRepo.timeline)
	}

	if err := publisher.Publish(context.Background(), sessionID, domain.ProgressEvent{
		Type: "quality_alert",
		Data: map[string]any{
			"reason": "check failed",
		},
	}); err != nil {
		t.Fatalf("quality alert publish returned error: %v", err)
	}

	if len(stateRepo.timeline) != 2 || stateRepo.timeline[0].Round != 2 {
		t.Fatalf("quality alert round mismatch: %+v", stateRepo.timeline)
	}
}
