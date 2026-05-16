package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/config"
)

// New creates and validates a Redis client.
//
// The client is thread-safe and manages its own internal connection pool.
// Default pool size is 10 connections — sufficient for rate limiting and
// caching workloads at this scale.
//
// The client is NOT closed here — the caller must call client.Close() on shutdown.
func New(ctx context.Context, cfg *config.Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("pinging redis: %w", err)
	}

	return client, nil
}
