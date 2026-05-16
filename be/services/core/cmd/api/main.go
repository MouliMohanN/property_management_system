package main

import (
	"context"
	"fmt"
	"os"

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
//
// TODO: Implement this function. Here are the steps:
//
//  1. Create a buffered signal channel and subscribe to SIGTERM + SIGINT:
//       quit := make(chan os.Signal, 1)
//       signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
//
//  2. Create a ServeMux and register a health check route:
//       mux := http.NewServeMux()
//       mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
//           w.WriteHeader(http.StatusOK)
//       })
//
//  3. Create the HTTP server (do NOT start it yet):
//       srv := &http.Server{Addr: ":" + cfg.ServerPort, Handler: mux}
//
//  4. Start the server in a goroutine (ListenAndServe blocks):
//       go func() {
//           if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
//               log.Error().Err(err).Msg("server error")
//           }
//       }()
//       log.Info().Str("port", cfg.ServerPort).Msg("server started")
//
//  5. Block until a signal arrives:
//       <-quit
//       log.Info().Msg("shutdown signal received")
//
//  6. Create a 30-second timeout for in-flight requests to finish, then shut down:
//       shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
//       defer cancel()
//       return srv.Shutdown(shutdownCtx)
//
// Why this matters: without step 6, a SIGTERM (e.g., from `kubectl rollout`)
// kills the process instantly — any in-flight HTTP request gets a broken pipe.
// srv.Shutdown() stops accepting new connections and waits for active ones to
// finish before returning, giving clients a clean response.
func run(ctx context.Context, cfg *config.Config, log zerolog.Logger, db *pgxpool.Pool, rdb *goredis.Client) error {
	// Your implementation here.
	// Imports you will need: net/http, os, os/signal, syscall, time
	log.Info().Msg("TODO: implement graceful HTTP server in run()")
	return nil
}
