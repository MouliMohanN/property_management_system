# Phase 1 Setup: PostgreSQL + Redis + Application Bootstrap

This document walks through every decision made in the initial infrastructure setup — what was built, why each choice was made, and how to use it day-to-day.

---

## Table of Contents

1. [What was built](#what-was-built)
2. [Docker Compose — local infrastructure](#docker-compose--local-infrastructure)
3. [Environment variables and config](#environment-variables-and-config)
4. [Shared logger (zerolog)](#shared-logger-zerolog)
5. [PostgreSQL connection pool (pgxpool)](#postgresql-connection-pool-pgxpool)
6. [Redis client (go-redis)](#redis-client-go-redis)
7. [Database migrations (golang-migrate)](#database-migrations-golang-migrate)
8. [Application bootstrap and graceful shutdown](#application-bootstrap-and-graceful-shutdown)
9. [Go workspace and module setup](#go-workspace-and-module-setup)
10. [Makefile targets reference](#makefile-targets-reference)
11. [Day-to-day workflow](#day-to-day-workflow)

---

## What was built

```
be/
├── infra/
│   └── docker-compose.yml              ← PostgreSQL 16 + Redis 7 for local dev
├── shared/
│   └── logger/logger.go                ← zerolog structured logger (used by all services)
└── services/core/
    ├── cmd/
    │   ├── api/main.go                 ← HTTP server bootstrap (your TODO: graceful shutdown)
    │   └── migrate/main.go             ← Migration runner binary
    ├── internal/
    │   ├── domain/                     ← [FUTURE] entities, value objects
    │   ├── usecase/                    ← [FUTURE] application logic
    │   ├── transport/                  ← [FUTURE] HTTP/gRPC handlers
    │   └── infrastructure/
    │       ├── config/config.go        ← Config struct + env loading
    │       ├── postgres/postgres.go    ← pgxpool connection pool
    │       └── redis/redis.go          ← go-redis client
    ├── scripts/
    │   └── migrations/
    │       ├── 000001_init.up.sql      ← Enable UUID + pgcrypto extensions
    │       └── 000001_init.down.sql    ← Rollback
    ├── infra/
    │   └── .env.example                ← Environment variable template
    └── Makefile                        ← migrate-up, migrate-down, migrate-new, run
```

---

## Docker Compose — local infrastructure

**File:** `be/infra/docker-compose.yml`

### Starting and stopping

```bash
# From be/ directory:

# Start PostgreSQL + Redis in the background
make infra-up

# View logs
make infra-logs

# Stop containers (data volumes are preserved)
make infra-down

# Stop containers AND delete all data (fresh start)
make infra-clean
```

Or directly with docker compose:

```bash
cd be/infra
docker compose up -d                        # start in background
docker compose down                         # stop (keep volumes)
docker compose down -v                      # stop + delete volumes
docker compose logs -f postgres             # tail postgres logs only
```

### Verify the containers are healthy

```bash
docker compose -f be/infra/docker-compose.yml ps
```

Expected output:
```
NAME           IMAGE              STATUS
pms_postgres   postgres:16-alpine Up (healthy)
pms_redis      redis:7-alpine     Up (healthy)
```

> **Why "healthy"?** The compose file uses health checks (`pg_isready` for postgres,
> `redis-cli ping` for redis). The service won't show "Up (healthy)" until the health
> check succeeds — so "healthy" is a real signal, not just "container started".

### Connect manually

```bash
# PostgreSQL
psql -h localhost -U pms_user -d property_mgmt
# Password: pms_pass

# Redis CLI
redis-cli -h localhost -a pms_redis_pass
127.0.0.1:6379> PING
PONG
```

### Why Alpine images?

`postgres:16-alpine` instead of `postgres:16`:
- Alpine base image is ~8 MB vs ~250 MB for Debian-based
- Shorter pull times, smaller attack surface
- Identical postgres functionality

---

## Environment variables and config

**File:** `be/services/core/internal/infrastructure/config/config.go`

### Setup

```bash
# Copy the example file
cp be/services/core/infra/.env.example be/services/core/infra/.env

# Edit with your values (defaults match docker-compose defaults)
```

Your `.env` file:
```bash
ENV=development
SERVER_PORT=8080
LOG_LEVEL=debug

DB_HOST=localhost
DB_PORT=5432
DB_USER=pms_user
DB_PASSWORD=pms_pass
DB_NAME=property_mgmt
DB_SSL_MODE=disable

REDIS_ADDR=localhost:6379
REDIS_PASSWORD=pms_redis_pass
REDIS_DB=0
```

### Loading environment variables

Before running any command that needs env vars, export them into your shell:

```bash
# Method 1: export individually
export DB_PASSWORD=pms_pass

# Method 2: source the whole file (recommended for development)
set -a && source be/services/core/infra/.env && set +a

# Verify
echo $DB_PASSWORD   # → pms_pass
```

> **`set -a`** makes every variable defined after it automatically exported.
> **`set +a`** turns that off so you don't export unintended variables later.

### Why plain env vars instead of Viper or YAML config?

This follows the [12-factor app](https://12factor.net/config) principle:
- Config is in the environment, not in files committed to version control
- The same binary runs in dev, staging, and production — only env vars change
- AWS ECS, Kubernetes, and Lambda all inject config via env vars natively

When secrets management is needed later, AWS Secrets Manager injects values as env vars at container startup — so the code never changes.

### Why `requireEnv` for DB_PASSWORD but defaults for others?

The service cannot start safely without a database password. Failing fast with a clear error message is better than starting with an empty password and getting a cryptic auth failure later.

```go
// config.go — fails at startup if DB_PASSWORD is missing
dbPassword, err := requireEnv("DB_PASSWORD")
if err != nil {
    return nil, err  // causes os.Exit(1) in main()
}
```

---

## Shared logger (zerolog)

**File:** `be/shared/logger/logger.go`

### Usage

```go
import "github.com/MouliMohanN/property_management_system/be/shared/logger"

log := logger.New("core", cfg.LogLevel, cfg.Env)

// Structured log fields
log.Info().
    Str("user_id", "u-123").
    Int("status", 200).
    Msg("request completed")

// Error with stack context
log.Error().Err(err).Str("path", "/api/properties").Msg("handler failed")
```

### Development vs production output

**Development** (`ENV=development`) — human-readable:
```
10:24:31 INF server started port=8080 service=core
10:24:31 INF connected to postgres host=localhost db=property_mgmt service=core
```

**Production** (`ENV=production`) — structured JSON:
```json
{"level":"info","service":"core","time":"2026-04-15T10:24:31Z","port":"8080","message":"server started"}
{"level":"info","service":"core","time":"2026-04-15T10:24:31Z","host":"localhost","db":"property_mgmt","message":"connected to postgres"}
```

> **Why JSON in production?** Log aggregators (CloudWatch Logs Insights, Datadog,
> Grafana Loki) parse structured JSON to let you query and filter by any field.
> With plain text logs, you'd need regex parsing. With JSON, you just filter on
> `level = "error"` or `service = "core"`.

### Why zerolog over log/slog or zap?

| | zerolog | zap | slog (stdlib) |
|---|---|---|---|
| Allocations | Zero | Near-zero | Some |
| API ergonomics | Builder chain | Verbose | Simple |
| Production use | Widespread | Widespread | Growing |

zerolog's zero-allocation design means logging at INFO level has no impact on GC
pressure — important for a service handling hundreds of requests per second.

### Why is this in `be/shared/` and not in `be/services/core/`?

The `be/shared/` module is the home for packages that every future service will use.
When we extract the notification service or billing service later, they each import
the same logger rather than reimplementing it. The `be/go.work` workspace links
`shared/` so all services resolve it to the local path during development.

---

## PostgreSQL connection pool (pgxpool)

**File:** `be/services/core/internal/infrastructure/postgres/postgres.go`

### Why pgx/v5 instead of `database/sql` + `lib/pq`?

| | pgx/v5 | database/sql + lib/pq |
|---|---|---|
| Driver type | Native Go | CGO (historical) / pure Go fork |
| Performance | 3–5× faster | Baseline |
| PostgreSQL types | Full support (arrays, JSONB, UUIDs, ranges) | Limited |
| Connection pool | Built-in (`pgxpool`) | Separate library needed |
| Prepared statements | Automatic | Manual |

`pgx` is the standard choice for new Go + PostgreSQL projects. `lib/pq` is maintained
for legacy compatibility. We use `lib/pq` only as a side-effect import in the
`golang-migrate` postgres driver (which runs DDL migrations where performance doesn't matter).

### Connection pool: how it works

```
Your code                pgxpool                   PostgreSQL
─────────               ─────────────────────────  ─────────────
pool.Query()      →     [picks an idle conn]   →   executes query
pool.Query()      →     [picks another conn]   →   executes query
...                      MaxConns = 25             max_connections = 100
```

The pool manages `MaxConns = 25` connections. Requests wait if all are busy.
On startup, `MinConns = 5` connections are opened to avoid cold-start latency.

### Pool configuration explained

```go
cfg.MaxConns = 25              // max concurrent connections
cfg.MinConns = 5               // connections kept open while idle
cfg.MaxConnLifetime = 1 * time.Hour     // replace connections after 1 hour
cfg.MaxConnIdleTime = 30 * time.Minute  // close idle connections after 30 min
cfg.HealthCheckPeriod = 1 * time.Minute // background health ping interval
```

**MaxConns sizing rule**: PostgreSQL defaults to `max_connections = 100`.
With a single app server: `25` leaves 75 for migrations, `psql` sessions, pgAdmin,
and future services. As you scale to multiple pods, reduce per-pod MaxConns.

**MaxConnLifetime**: Load balancers (AWS RDS Proxy, pgBouncer) sometimes silently
drop long-lived connections. Replacing connections after 1 hour prevents the app
from holding stale connections that would fail on first use.

### The Ping on startup

```go
if err := pool.Ping(ctx); err != nil {
    pool.Close()
    return nil, fmt.Errorf("pinging postgres: %w", err)
}
```

This is a fail-fast check: if postgres is unreachable at startup (wrong password,
host not up), the service exits immediately with a clear error message instead of
starting and failing on the first real request.

---

## Redis client (go-redis)

**File:** `be/services/core/internal/infrastructure/redis/redis.go`

### Usage

The `redis.New()` function returns a `*redis.Client` from the `go-redis/v9` library.
In `main.go`, we alias the go-redis import to avoid collision with our internal package:

```go
import (
    goredis "github.com/redis/go-redis/v9"           // aliased
    "github.com/.../internal/infrastructure/redis"  // our constructor package
)

rdb, err := redis.New(ctx, cfg)   // returns *goredis.Client
```

### What Redis will be used for (Phase 2+)

| Use case | Pattern |
|---|---|
| Rate limiting | Sliding window counter per IP/user |
| Session cache | JWT denylist for revoked tokens |
| Caching | Cache-aside for frequently-read property data |
| Leaderboards | Sorted sets for tenant payment ranking (reporting) |

For now, the client is wired up and verified on startup. Actual Redis usage comes in Phase 2.

### Redis database index (DB 0–15)

Redis supports 16 logical databases (0–15) within a single server. The default is `DB=0`.
In development, you can use different DB numbers to separate concerns:
- `DB=0` → application cache / rate limiting
- `DB=1` → test data (so tests don't pollute app cache)

---

## Database migrations (golang-migrate)

### The migration file format

```
scripts/migrations/
├── 000001_init.up.sql      ← apply: runs forward
├── 000001_init.down.sql    ← rollback: undoes the up migration
├── 000002_create_users.up.sql
├── 000002_create_users.down.sql
...
```

**Naming convention:** `<sequence>_<description>.{up,down}.sql`
- Sequence is zero-padded to 6 digits by `golang-migrate`
- One pair of files per logical change (a table, an index, a constraint)
- Always write the `down` migration — you WILL need it

### Install the migrate CLI

```bash
# macOS
brew install golang-migrate

# Verify
migrate --version
# → v4.x.x
```

### Day-to-day migration commands

First, export your env vars:
```bash
set -a && source be/services/core/infra/.env && set +a
```

Then from `be/services/core/`:

```bash
# Apply all pending migrations
make migrate-up

# Roll back the most recent migration (1 step)
make migrate-down

# Create a new migration pair
make migrate-new name=create_users_table

# Check which version is currently applied
make migrate-status
```

Or from `be/` (delegated via core-* targets):
```bash
make core-migrate-up
make core-migrate-new name=create_users_table
```

### Example: creating the users table migration

```bash
# 1. Create the migration files
make migrate-new name=create_users_table
# → Created migration: scripts/migrations/*_create_users_table.{up,down}.sql

# 2. Edit the up file
cat scripts/migrations/000002_create_users_table.up.sql
```

```sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

```bash
# 3. Edit the down file
cat scripts/migrations/000002_create_users_table.down.sql
```

```sql
DROP TABLE IF EXISTS users;
```

```bash
# 4. Apply
make migrate-up
# → Running migrations up...
# → All migrations applied

# 5. Verify
psql -h localhost -U pms_user -d property_mgmt -c "\d users"
```

### Handling a "dirty" state

If a migration fails partway through, `golang-migrate` marks the DB as "dirty":
```
error: Dirty database version 2. Fix and force version.
```

Fix it:
```bash
# 1. Manually undo what the failed migration did (in psql)
# 2. Force the version back to the last good state
make migrate-force version=1
# 3. Re-run
make migrate-up
```

> **Why not auto-run migrations on server startup?**
> In a production system with multiple replicas, if every pod ran migrations on
> startup, you'd get race conditions — two pods both trying to apply the same
> migration simultaneously. The production pattern is:
> 1. Deploy the migration as a one-off task (ECS task, K8s Job)
> 2. Wait for it to succeed
> 3. Deploy the new app version
>
> For local development, `make migrate-up` is the explicit, safe equivalent.

### What `000001_init.up.sql` does

The first migration enables two PostgreSQL extensions:

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "pgcrypto";    -- crypt(), gen_salt()
```

- **uuid-ossp**: Lets you write `DEFAULT gen_random_uuid()` in column definitions.
  Every domain entity (users, properties, leases) will use UUID primary keys.
- **pgcrypto**: Provides `crypt()` and `gen_salt()` for password hashing in SQL.
  Even though we'll hash passwords in Go (bcrypt), having pgcrypto available is
  standard practice for PostgreSQL production setups.

---

## Application bootstrap and graceful shutdown

**File:** `be/services/core/cmd/api/main.go`

### The bootstrap sequence in `main()`

```go
func main() {
    // 1. Load config from env vars → fail fast if DB_PASSWORD missing
    cfg, err := config.Load()

    // 2. Create structured logger
    log := logger.New("core", cfg.LogLevel, cfg.Env)

    ctx := context.Background()

    // 3. Connect to postgres → fail fast if unreachable
    db, err := postgres.New(ctx, cfg)
    defer db.Close()  // cleanup on exit

    // 4. Connect to redis → fail fast if unreachable
    rdb, err := redis.New(ctx, cfg)
    defer rdb.Close()  // cleanup on exit

    // 5. Hand off to run() which starts the HTTP server
    if err := run(ctx, cfg, log, db, rdb); err != nil {
        log.Fatal().Err(err).Msg("server error")
    }
}
```

Why this order? Each step depends on the previous one succeeding. By wiring
dependencies in `main()` and passing them down, `run()` receives fully-initialized,
tested connections. No nil checks needed inside `run()`.

### Your task: implement `run()`

The `run()` function in `cmd/api/main.go` is where you implement graceful shutdown.
The step-by-step instructions are in the file as comments. Here's what you're building:

```
main()
  └── run()
        ├── creates HTTP server + health handler
        ├── starts server in goroutine (non-blocking)
        ├── blocks on OS signal (SIGTERM/SIGINT)
        ├── on signal: calls srv.Shutdown(ctx) with 30s timeout
        └── returns → main() runs defers → db.Close() + rdb.Close()
```

**Why does this matter?** In Kubernetes or ECS, when you deploy a new version:
1. The orchestrator sends `SIGTERM` to the old container
2. Without graceful shutdown: the process dies instantly, active requests fail
3. With graceful shutdown: the process stops accepting new connections, waits for
   in-flight requests to finish (up to 30s), then exits cleanly

Imports you will need in `run()`:
```go
"net/http"
"os"
"os/signal"
"syscall"
"time"
```

---

## Go workspace and module setup

### Structure

```
be/
├── go.work                    ← workspace file
├── shared/
│   └── go.mod                 ← module: be/shared
└── services/
    └── core/
        └── go.mod             ← module: be/services/core
```

### How the workspace resolves the shared module

```
be/go.work:
    use ./shared           ← "for this module path, use the local directory"
    use ./services/core
```

When `be/services/core` imports `be/shared/logger`:
1. Go reads `be/go.work`
2. Finds `use ./shared` → resolves to the local path
3. Reads `be/shared/logger/logger.go` directly

**No version number is needed in `go.mod` for workspace-local modules.** The workspace
acts as the authoritative source. This is why `be/services/core/go.mod` does NOT have
a `require` for `be/shared` — the workspace handles it.

### `go mod tidy` vs `go build` in workspaces

| Command | Behavior |
|---|---|
| `go build ./...` | Uses workspace; resolves local modules correctly |
| `go mod tidy -e` | `-e` flag needed; workspace-local modules cause harmless warnings |

The warning from `go mod tidy`:
```
module github.com/.../be/shared@latest found, but does not contain package ...
```
is expected. The module isn't published to a registry. `go build` resolves it via
the workspace. Use `go mod tidy -e` to suppress the error exit code.

### Adding a new dependency to a module

```bash
# Add to shared module
cd be/shared && go get github.com/some/package@latest

# Add to core service
cd be/services/core && go get github.com/some/package@latest

# Tidy after adding
cd be/services/core && go mod tidy -e
```

---

## Makefile targets reference

### From `be/` (workspace level)

| Target | Description |
|---|---|
| `make infra-up` | Start PostgreSQL + Redis |
| `make infra-down` | Stop containers (keep data) |
| `make infra-clean` | Stop containers + delete volumes |
| `make infra-logs` | Tail container logs |
| `make build` | Build all modules in workspace |
| `make test` | Run all tests |
| `make tidy` | Sync workspace + tidy all go.mod files |
| `make core-run` | Start the API server |
| `make core-migrate-up` | Apply pending migrations |
| `make core-migrate-down` | Roll back last migration |
| `make core-migrate-new name=foo` | Create new migration pair |

### From `be/services/core/` (service level)

```bash
# Set env vars first
set -a && source infra/.env && set +a

make run              # go run ./cmd/api
make migrate-up       # apply migrations
make migrate-down     # roll back 1 step
make migrate-new name=create_users_table
make migrate-status   # show current version
make migrate-force version=1  # fix dirty state
make build            # compile both binaries into bin/
```

---

## Day-to-day workflow

### First-time setup

```bash
# 1. Install tools
brew install golang-migrate
brew install docker  # if not installed

# 2. Clone and navigate
cd be/services/core

# 3. Set up env
cp infra/.env.example infra/.env
# Edit infra/.env if needed (defaults work with docker-compose)

# 4. Start infrastructure
cd ../..  # → be/
make infra-up
# Wait for "Up (healthy)" status: docker compose ps

# 5. Export env and run migrations
cd services/core
set -a && source infra/.env && set +a
make migrate-up

# 6. Build and run
make run
# → TODO: implement graceful shutdown in run()
```

### Typical development loop

```bash
# Start infra (once per machine reboot)
make -C be infra-up

# Create a new migration for your feature
cd be/services/core
set -a && source infra/.env && set +a
make migrate-new name=create_properties_table
# Edit scripts/migrations/000002_create_properties_table.up.sql
make migrate-up

# Run the server
make run
```

### Resetting local state

```bash
# Nuke everything and start fresh
make -C be infra-clean  # deletes volumes
make -C be infra-up
# Run migrations again
cd be/services/core && make migrate-up
```
