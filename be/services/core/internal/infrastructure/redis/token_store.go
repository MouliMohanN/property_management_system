package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/google/uuid"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

const refreshKeyPrefix = "refresh:"

// TokenStore implements auth.TokenStore using Redis.
// Each refresh token is stored as a string key mapping to the user's UUID.
type TokenStore struct {
	client *goredis.Client
}

// NewTokenStore constructs a TokenStore backed by the provided Redis client.
func NewTokenStore(client *goredis.Client) *TokenStore {
	return &TokenStore{client: client}
}

// Set stores the mapping token → userID with the given TTL.
func (s *TokenStore) Set(ctx context.Context, token string, userID uuid.UUID, ttl time.Duration) error {
	key := refreshKeyPrefix + token
	if err := s.client.Set(ctx, key, userID.String(), ttl).Err(); err != nil {
		return fmt.Errorf("setting refresh token in redis: %w", err)
	}
	return nil
}

// Get retrieves the user ID associated with the given token.
// Returns auth.ErrTokenNotFound when the key is absent or has expired.
func (s *TokenStore) Get(ctx context.Context, token string) (uuid.UUID, error) {
	key := refreshKeyPrefix + token
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return uuid.Nil, auth.ErrTokenNotFound
		}
		return uuid.Nil, fmt.Errorf("getting refresh token from redis: %w", err)
	}

	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parsing stored user id %q: %w", val, err)
	}
	return id, nil
}

// Delete removes the token from Redis. If the key does not exist the operation
// is a no-op from Redis's perspective; we map 0 deleted keys to ErrTokenNotFound
// so callers that care can distinguish the case.
func (s *TokenStore) Delete(ctx context.Context, token string) error {
	key := refreshKeyPrefix + token
	n, err := s.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("deleting refresh token from redis: %w", err)
	}
	if n == 0 {
		return auth.ErrTokenNotFound
	}
	return nil
}
