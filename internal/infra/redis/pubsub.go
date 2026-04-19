package redis

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func (p *ProgressPublisher) Publish(ctx context.Context, sessionID uuid.UUID, event domain.ProgressEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, p.channel(sessionID.String()), data).Err()
}

func (p *ProgressPublisher) Subscribe(ctx context.Context, sessionID uuid.UUID) (<-chan domain.ProgressEvent, func(), error) {
	pubsub := p.client.Subscribe(ctx, p.channel(sessionID.String()))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}

	out := make(chan domain.ProgressEvent, 16)
	go func() {
		defer close(out)
		defer pubsub.Close()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event domain.ProgressEvent
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					continue
				}
				out <- event
			}
		}
	}()

	cancel := func() {
		_ = pubsub.Close()
	}
	return out, cancel, nil
}
