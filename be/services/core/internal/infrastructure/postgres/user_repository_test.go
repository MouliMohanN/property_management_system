package postgres_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/postgres"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not construct dockertest pool: %v", err)
	}
	if err := pool.Client.Ping(); err != nil {
		log.Fatalf("could not connect to docker: %v", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15-alpine",
		Env: []string{
			"POSTGRES_USER=test",
			"POSTGRES_PASSWORD=test",
			"POSTGRES_DB=testdb",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	// Set a hard expiry so the container is cleaned up even if the test panics.
	_ = resource.Expire(120)

	dsn := fmt.Sprintf(
		"host=localhost port=%s user=test password=test dbname=testdb sslmode=disable",
		resource.GetPort("5432/tcp"),
	)

	pool.MaxWait = 30 * time.Second
	if err := pool.Retry(func() error {
		var err error
		testPool, err = pgxpool.New(context.Background(), dsn)
		if err != nil {
			return err
		}
		return testPool.Ping(context.Background())
	}); err != nil {
		log.Fatalf("could not connect to postgres: %v", err)
	}

	// Run migrations from the project root so paths resolve correctly.
	migrateURL := fmt.Sprintf(
		"postgres://test:test@localhost:%s/testdb?sslmode=disable",
		resource.GetPort("5432/tcp"),
	)
	mig, err := migrate.New("file://../../../scripts/migrations", migrateURL)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("could not run migrations: %v", err)
	}

	code := m.Run()

	testPool.Close()
	if err := pool.Purge(resource); err != nil {
		log.Printf("could not purge resource: %v", err)
	}

	os.Exit(code)
}

func TestUserRepository_Create(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	ctx := context.Background()

	u := &user.User{
		Email:        "create@example.com",
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Alice",
		LastName:     "Smith",
		Role:         user.RoleTenant,
		Status:       user.StatusActive,
	}

	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if u.ID == uuid.Nil {
		t.Error("expected ID to be set after Create()")
	}
	if u.Version != 1 {
		t.Errorf("expected Version = 1, got %d", u.Version)
	}
	if u.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestUserRepository_FindByEmail(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	ctx := context.Background()

	u := &user.User{
		Email:        "findbyemail@example.com",
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Bob",
		LastName:     "Jones",
		Role:         user.RoleLandlord,
		Status:       user.StatusActive,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := repo.FindByEmail(ctx, u.Email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("FindByEmail() ID mismatch: got %v, want %v", found.ID, u.ID)
	}
	if found.Email != u.Email {
		t.Errorf("FindByEmail() Email mismatch: got %v, want %v", found.Email, u.Email)
	}

	_, err = repo.FindByEmail(ctx, "notexist@example.com")
	if err == nil {
		t.Error("FindByEmail() expected error for non-existent email")
	}
}

func TestUserRepository_FindByID(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	ctx := context.Background()

	u := &user.User{
		Email:        "findbyid@example.com",
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Carol",
		LastName:     "White",
		Role:         user.RoleTenant,
		Status:       user.StatusActive,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("FindByID() ID mismatch")
	}

	_, err = repo.FindByID(ctx, uuid.New())
	if err == nil {
		t.Error("FindByID() expected error for non-existent id")
	}
}

func TestUserRepository_Update_OptimisticLocking(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	ctx := context.Background()

	u := &user.User{
		Email:        "optlock@example.com",
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Dan",
		LastName:     "Brown",
		Role:         user.RoleTenant,
		Status:       user.StatusActive,
	}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// First update should succeed and increment version.
	u.FirstName = "Daniel"
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("first Update() error = %v", err)
	}
	if u.Version != 2 {
		t.Errorf("expected Version = 2 after first update, got %d", u.Version)
	}

	// Simulate a concurrent write by rolling back the version.
	u.Version = 1
	err := repo.Update(ctx, u)
	if err == nil {
		t.Fatal("expected ErrConflict on stale version update, got nil")
	}
}

func TestUserRepository_Count(t *testing.T) {
	repo := postgres.NewUserRepository(testPool)
	ctx := context.Background()

	before, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	_ = repo.Create(ctx, &user.User{
		Email:        fmt.Sprintf("count-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "$2a$10$placeholder",
		FirstName:    "Eve",
		LastName:     "Black",
		Role:         user.RoleTenant,
		Status:       user.StatusActive,
	})

	after, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if after != before+1 {
		t.Errorf("Count() expected %d after insert, got %d", before+1, after)
	}
}
