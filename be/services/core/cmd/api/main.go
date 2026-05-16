package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/config"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/postgres"
	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/redis"
	"github.com/MouliMohanN/property_management_system/be/shared/logger"
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

// run wires the HTTP server and handles graceful shutdown on OS signals.
// It blocks until SIGTERM or SIGINT is received, then drains in-flight requests
// before returning — allowing main() defers (db.Close, rdb.Close) to run cleanly.
func run(ctx context.Context, cfg *config.Config, log zerolog.Logger, db *pgxpool.Pool, rdb *goredis.Client) error {
	// Buffered channel of size 1: if the signal arrives before we call <-quit,
	// the OS can still deliver it without blocking. An unbuffered channel would
	// silently drop the signal if no goroutine is ready to receive yet.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	mux := http.NewServeMux()

	// Health check endpoint — used by load balancers (ALB, ECS) to decide whether
	// to route traffic to this instance. Must respond 200 quickly with no side effects.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: mux,
		// Timeouts prevent slow or malicious clients from holding connections open
		// indefinitely and exhausting goroutines/file descriptors.
		ReadTimeout:  10 * time.Second, // max time to read the full request (headers + body)
		WriteTimeout: 30 * time.Second, // max time to write the full response — matches shutdown grace period
		IdleTimeout:  60 * time.Second, // max time a keep-alive connection can sit idle between requests
	}

	// ListenAndServe blocks forever, so it must run in its own goroutine.
	// Think of it like starting an event loop in JS — it occupies the thread.
	// http.ErrServerClosed is not a real error; it's how Shutdown() signals
	// that the server stopped intentionally. Any other error is unexpected.
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("server stopped unexpectedly")
		}
	}()
	log.Info().Str("port", cfg.ServerPort).Msg("server started")

	// Block here until the OS sends SIGTERM (kubectl rollout, ECS task stop)
	// or SIGINT (Ctrl+C during local dev). Nothing runs past this line until then.
	<-quit
	log.Info().Msg("shutdown signal received")

	// Give in-flight requests up to 30 seconds to finish before we force-close.
	// Without this, active HTTP requests would get a broken pipe mid-response.
	// The context timeout ensures we don't wait forever if a handler is stuck.
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
