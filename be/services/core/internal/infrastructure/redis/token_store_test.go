package redis_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/redis"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
)

var testClient *goredis.Client

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not construct dockertest pool: %v", err)
	}
	if err := pool.Client.Ping(); err != nil {
		log.Fatalf("could not connect to docker: %v", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7-alpine",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("could not start redis container: %v", err)
	}

	_ = resource.Expire(120)

	pool.MaxWait = 30 * time.Second
	if err := pool.Retry(func() error {
		testClient = goredis.NewClient(&goredis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})
		_, err := testClient.Ping(context.Background()).Result()
		return err
	}); err != nil {
		log.Fatalf("could not connect to redis: %v", err)
	}

	code := m.Run()

	testClient.Close()
	if err := pool.Purge(resource); err != nil {
		log.Printf("could not purge resource: %v", err)
	}

	os.Exit(code)
}

func TestTokenStore_SetAndGet(t *testing.T) {
	store := redis.NewTokenStore(testClient)
	ctx := context.Background()

	token := "test-token-set-get"
	userID := uuid.New()

	if err := store.Set(ctx, token, userID, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != userID {
		t.Errorf("Get() = %v, want %v", got, userID)
	}
}

func TestTokenStore_GetExpired(t *testing.T) {
	store := redis.NewTokenStore(testClient)
	ctx := context.Background()

	token := "test-token-expired"
	userID := uuid.New()

	// Use a very short TTL so the key expires quickly.
	if err := store.Set(ctx, token, userID, 50*time.Millisecond); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err := store.Get(ctx, token)
	if err == nil {
		t.Fatal("Get() expected error for expired token, got nil")
	}
	if err != auth.ErrTokenNotFound {
		t.Errorf("Get() error = %v, want ErrTokenNotFound", err)
	}
}

func TestTokenStore_Delete(t *testing.T) {
	store := redis.NewTokenStore(testClient)
	ctx := context.Background()

	token := "test-token-delete"
	userID := uuid.New()

	if err := store.Set(ctx, token, userID, time.Minute); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := store.Delete(ctx, token); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get(ctx, token)
	if err == nil {
		t.Fatal("Get() expected ErrTokenNotFound after delete, got nil")
	}
	if err != auth.ErrTokenNotFound {
		t.Errorf("Get() error = %v, want ErrTokenNotFound", err)
	}
}

func TestTokenStore_DeleteNotFound(t *testing.T) {
	store := redis.NewTokenStore(testClient)
	ctx := context.Background()

	// Deleting a non-existent token should return ErrTokenNotFound.
	err := store.Delete(ctx, "non-existent-token")
	if err != auth.ErrTokenNotFound {
		t.Errorf("Delete() error = %v, want ErrTokenNotFound", err)
	}
}
