package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/config"
)

// New creates a validated pgxpool connection pool.
//
// pgxpool is the right choice over database/sql here because:
//   - It is the native PostgreSQL driver — no CGO, no C library
//   - It supports PostgreSQL-specific types (arrays, JSON, UUIDs) natively
//   - The pool is built-in and goroutine-safe
//   - It is 3–5× faster than lib/pq for high-throughput workloads
//
// The pool is NOT closed here — the caller must call pool.Close() on shutdown.
func New(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DBDSN())
	if err != nil {
		return nil, fmt.Errorf("parsing postgres config: %w", err)
	}

	configurePool(poolCfg)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	return pool, nil
}

// configurePool sets connection pool parameters.
//
// These values directly impact database performance and resource usage:
//
//   - MaxConns: maximum open connections in the pool.
//     Too high → DB overwhelmed (Postgres default max_connections = 100).
//     Too low  → requests queue under load.
//     Rule of thumb: leave headroom for migrations, admin tools, other services.
//
//   - MinConns: connections kept alive while idle.
//     Eliminates cold-start latency, at the cost of holding DB connection slots.
//
//   - MaxConnLifetime: a connection is replaced after this age.
//     Prevents stale connections behind load balancers or network proxies.
//
//   - MaxConnIdleTime: idle connections are closed after this duration.
//     Frees DB slots during low-traffic periods.
//
//   - HealthCheckPeriod: how often the pool pings idle connections in background.
func configurePool(cfg *pgxpool.Config) {
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute
}
