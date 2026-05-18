package domain

import (
	"context"
	"time"
)

type CacheRepository interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, dest interface{}) error

	DecrementStock(ctx context.Context, productID string, quantity int) (int, error)
	PublishToQueue(ctx context.Context, queueName string, payload interface{}) error
	ConsumeFromQueue(ctx context.Context, queueName string) (string, error)
	PublishToDLQ(ctx context.Context, queueName string, payload interface{}) error

	ConsumeReliably(ctx context.Context, sourceQueue, processingQueue string) (string, error)
	AckMessage(ctx context.Context, processingQueue string, msg string) error
}
