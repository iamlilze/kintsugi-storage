package storage

import (
	"context"
	"encoding/json"
	"time"
)

type Store interface {
	Put(ctx context.Context, key string, value json.RawMessage, ttl *time.Duration) error
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Len(ctx context.Context) (int, error)
	Ping(ctx context.Context) error
	Close() error
}
