package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/iwen-conf/Naturalize/pkg/config"
)

type Client struct {
	*goredis.Client
}

func Connect(cfg config.RedisConfig) *Client {
	return &Client{
		Client: goredis.NewClient(&goredis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
	}
}

type CheckpointStore struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewCheckpointStore(client *Client, ttl time.Duration) *CheckpointStore {
	return &CheckpointStore{client: client.Client, ttl: ttl}
}

func (s *CheckpointStore) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, "cp:"+checkpointID).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (s *CheckpointStore) Set(ctx context.Context, checkpointID string, checkpoint []byte) error {
	return s.client.Set(ctx, "cp:"+checkpointID, checkpoint, s.ttl).Err()
}

func (s *CheckpointStore) Delete(ctx context.Context, checkpointID string) error {
	return s.client.Del(ctx, "cp:"+checkpointID).Err()
}

func (s *CheckpointStore) SaveJSON(ctx context.Context, checkpointID string, payload []byte) error {
	return s.client.Set(ctx, "cp:data:"+checkpointID, payload, s.ttl).Err()
}

func (s *CheckpointStore) LoadJSON(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, "cp:data:"+checkpointID).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (s *CheckpointStore) DeleteJSON(ctx context.Context, checkpointID string) error {
	return s.client.Del(ctx, "cp:data:"+checkpointID).Err()
}

type ProgressPublisher struct {
	client *goredis.Client
	prefix string
}

func NewProgressPublisher(client *Client, prefix string) *ProgressPublisher {
	return &ProgressPublisher{client: client.Client, prefix: prefix}
}

func (p *ProgressPublisher) channel(sessionID string) string {
	return fmt.Sprintf("%s:session:%s:events", p.prefix, sessionID)
}
