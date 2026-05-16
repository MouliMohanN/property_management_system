package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/domain/user"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/config"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/password"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/postgres"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/redis"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/token"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http/handler"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/usecase/auth"
	"github.com/MouliMohanN/property_management_system/be/shared/logger"

	transporthttp "github.com/MouliMohanN/property_management_system/be/services/core/internal/transport/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New("core", cfg.LogLevel, cfg.Env)

	ctx := context.Background()

	db, err := postgres.New(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer db.Close()
	log.Info().Str("host", cfg.DBHost).Str("db", cfg.DBName).Msg("connected to postgres")

	rdb, err := redis.New(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer rdb.Close()
	log.Info().Str("addr", cfg.RedisAddr).Msg("connected to redis")

	if err := run(ctx, cfg, log, db, rdb); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}

func run(ctx context.Context, cfg *config.Config, log zerolog.Logger, db *pgxpool.Pool, rdb *goredis.Client) error {
	// ── Infrastructure adapters ──────────────────────────────────────────────
	userRepo := postgres.NewUserRepository(db)
	tokenStore := redis.NewTokenStore(rdb)
	jwtSvc := token.NewJWTService(cfg.JWTSecret, cfg.AccessTokenTTL)
	hasher := password.NewBcryptHasher()

	// ── Admin bootstrap ──────────────────────────────────────────────────────
	if err := bootstrapAdmin(ctx, cfg, log, userRepo, hasher); err != nil {
		return fmt.Errorf("admin bootstrap: %w", err)
	}

	// ── Use cases ────────────────────────────────────────────────────────────
	registerUC := auth.NewRegisterUseCase(userRepo, hasher)
	loginUC := auth.NewLoginUseCase(userRepo, hasher, jwtSvc, tokenStore, cfg.RefreshTokenTTL)
	refreshUC := auth.NewRefreshUseCase(tokenStore, jwtSvc, cfg.RefreshTokenTTL)
	logoutUC := auth.NewLogoutUseCase(tokenStore)
	getMeUC := auth.NewGetMeUseCase(userRepo)

	// ── Transport ────────────────────────────────────────────────────────────
	authHandler := handler.NewAuthHandler(registerUC, loginUC, refreshUC, logoutUC, getMeUC, log)
	srv := transporthttp.NewServer(cfg.ServerPort, authHandler, jwtSvc, cfg.CORSAllowedOrigins)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("server stopped unexpectedly")
		}
	}()
	log.Info().Str("port", cfg.ServerPort).Msg("server started")

	<-quit
	log.Info().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

// bootstrapAdmin creates the initial admin account when the users table is empty.
// This is a one-time operation on first deployment. If ADMIN_EMAIL or
// ADMIN_PASSWORD are not set, a warning is logged and bootstrap is skipped —
// the system assumes it is already initialised.
func bootstrapAdmin(ctx context.Context, cfg *config.Config, log zerolog.Logger, userRepo *postgres.UserRepository, hasher *password.BcryptHasher) error {
	count, err := userRepo.Count(ctx)
	if err != nil {
		return fmt.Errorf("counting users: %w", err)
	}
	if count > 0 {
		return nil
	}

	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		log.Warn().Msg("users table is empty but ADMIN_EMAIL/ADMIN_PASSWORD are not set; skipping admin bootstrap")
		return nil
	}

	hash, err := hasher.Hash(cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}

	admin := &user.User{
		Email:        cfg.AdminEmail,
		PasswordHash: hash,
		FirstName:    "Admin",
		LastName:     "User",
		Role:         user.RoleAdmin,
		Status:       user.StatusActive,
	}

	if err := userRepo.Create(ctx, admin); err != nil {
		return fmt.Errorf("creating bootstrap admin: %w", err)
	}

	log.Info().Str("email", cfg.AdminEmail).Msg("bootstrap admin created")
	return nil
}
