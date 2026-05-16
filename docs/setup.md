# Project Setup

End-to-end guide for getting the system running locally from a clean checkout.

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.22+ | Backend runtime |
| Node.js | 20+ | Frontend tooling |
| Docker + Docker Compose | any recent | Postgres + Redis |
| `golang-migrate` CLI | latest | Running migrations from `make` targets |

Install `golang-migrate`:
```bash
brew install golang-migrate
```

---

## Repository layout (quick reference)

```
property_management_system/
├── be/
│   ├── infra/docker-compose.yml   ← Postgres + Redis
│   ├── services/core/             ← Go monolith (Phase 1)
│   │   ├── cmd/api/               ← HTTP server entrypoint
│   │   ├── cmd/migrate/           ← Migration runner entrypoint
│   │   ├── infra/.env.example     ← Environment variable reference
│   │   └── scripts/migrations/    ← SQL migration files
│   └── shared/                    ← Shared Go packages (logger, etc.)
├── fe/
│   └── web/                       ← Vite + React frontend
└── docs/
    ├── setup.md                   ← this file
    └── planning/project-plan.md   ← phase plan + feature status
```

---

## 1. Infrastructure (Postgres + Redis)

```bash
# Start both containers in the background
make -C be infra-up

# Verify they are healthy
docker ps
```

Containers expose:
- Postgres on `localhost:5432`
- Redis on `localhost:6379`

To stop: `make -C be infra-down`  
To wipe all data: `make -C be infra-clean`

---

## 2. Backend environment

```bash
cd be/services/core

# Copy the example and fill in values
cp infra/.env.example infra/.env
```

Edit `infra/.env`. The only values that must be changed from the example are:

| Variable | What to set |
|---|---|
| `JWT_SECRET` | Run `openssl rand -hex 32` and paste the output |
| `ADMIN_EMAIL` | Email for the first admin account |
| `ADMIN_PASSWORD` | Password for the first admin (min 8 chars) |

All other values match the Docker Compose defaults and work as-is in development.

Load the env vars into your shell before running any `make` targets:
```bash
export $(grep -v '^#' infra/.env | xargs)
```

---

## 3. Database migrations

Run from `be/services/core/` with env vars loaded:

```bash
# Apply all pending migrations
make migrate-up

# Check current version
make migrate-status

# Roll back the last migration
make migrate-down
```

Migrations live in `scripts/migrations/`. Current state:

| Version | Name | Description |
|---|---|---|
| 000001 | init | Enables `uuid-ossp` and `pgcrypto` extensions |
| 000002 | users | Creates the `users` table with optimistic locking |

To create a new migration:
```bash
make migrate-new name=create_properties
# → creates scripts/migrations/000003_create_properties.{up,down}.sql
```

> **Note:** You can also run migrations via the Go binary directly (useful in CI or without the `migrate` CLI):
> ```bash
> go run ./cmd/migrate up
> ```

---

## 4. Backend server

From `be/services/core/` with env vars loaded:

```bash
make run
# or: go run ./cmd/api
```

On first startup with an empty `users` table, the server automatically creates the admin account from `ADMIN_EMAIL` + `ADMIN_PASSWORD`. You will see:

```
INF bootstrap admin created email=admin@pms.dev
INF server started port=8080
```

The server starts on port `8080` (override with `SERVER_PORT`).

**Health check:**
```bash
curl http://localhost:8080/health
# → 200 OK
```

---

## 5. Frontend

```bash
cd fe/web
npm install
npm run dev
```

Opens at **http://localhost:5173**.

The frontend expects the backend running at `http://localhost:8080`. Override with a `.env.local` file:
```
VITE_API_URL=http://localhost:8080
```

---

## 6. Full local stack (all at once)

```bash
# Terminal 1 — infrastructure
make -C be infra-up

# Terminal 2 — backend (from be/services/core/, env vars loaded)
make migrate-up && make run

# Terminal 3 — frontend
cd fe/web && npm run dev
```

---

## 7. Auth endpoints (Phase 1a)

Base URL: `http://localhost:8080`

| Method | Path | Access | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/login` | Public | Returns access + refresh tokens |
| `POST` | `/api/v1/auth/refresh` | Public | Rotates refresh token, issues new access token |
| `POST` | `/api/v1/auth/logout` | Authenticated | Revokes refresh token |
| `GET` | `/api/v1/auth/me` | Authenticated | Returns current user |
| `POST` | `/api/v1/auth/register` | Admin only | Creates a new user account |

**Login example:**
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@pms.dev","password":"Admin1234!"}' | jq
```

**Authenticated request:**
```bash
curl -s http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>" | jq
```

**Register a new user (admin token required):**
```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{
    "email": "tenant@pms.dev",
    "password": "Tenant1234!",
    "first_name": "Jane",
    "last_name": "Doe",
    "role": "tenant"
  }' | jq
```

Valid roles: `admin`, `landlord`, `tenant`, `maintenance_staff`

---

## 8. Make command reference

### Root
| Command | Description |
|---|---|
| `make build` | Build backend + frontend |
| `make test` | Run all tests |

### Backend (`make -C be <target>`)
| Command | Description |
|---|---|
| `make -C be infra-up` | Start Postgres + Redis |
| `make -C be infra-down` | Stop containers |
| `make -C be infra-clean` | Stop containers + delete volumes |
| `make -C be infra-logs` | Tail container logs |
| `make -C be build` | Build all Go binaries |
| `make -C be test` | Run all Go tests |
| `make -C be core-run` | Start the core service |
| `make -C be core-migrate-up` | Apply migrations |
| `make -C be core-migrate-down` | Roll back last migration |
| `make -C be core-migrate-status` | Show current migration version |
| `make -C be core-migrate-new name=<name>` | Create a new migration pair |

### Core service (from `be/services/core/`)
| Command | Description |
|---|---|
| `make run` | Start the API server |
| `make build` | Build `bin/api` + `bin/migrate` |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back last migration |
| `make migrate-status` | Show current migration version |
| `make migrate-new name=<name>` | Create a new migration pair |
| `make migrate-force version=<N>` | Force migration version (fixes dirty state) |

---

## 9. Environment variables reference

All variables for `be/services/core`. See `infra/.env.example` for the full file.

| Variable | Default | Required | Description |
|---|---|---|---|
| `ENV` | `development` | No | Runtime environment |
| `SERVER_PORT` | `8080` | No | HTTP listen port |
| `LOG_LEVEL` | `info` | No | `debug` \| `info` \| `warn` \| `error` |
| `DB_HOST` | `localhost` | No | Postgres host |
| `DB_PORT` | `5432` | No | Postgres port |
| `DB_USER` | `pms_user` | No | Postgres user |
| `DB_PASSWORD` | — | **Yes** | Postgres password |
| `DB_NAME` | `property_mgmt` | No | Postgres database name |
| `DB_SSL_MODE` | `disable` | No | `disable` \| `require` \| `verify-full` |
| `REDIS_ADDR` | `localhost:6379` | No | Redis address |
| `REDIS_PASSWORD` | — | No | Redis password |
| `REDIS_DB` | `0` | No | Redis logical DB index |
| `JWT_SECRET` | — | **Yes** | HMAC-SHA256 signing key (32 random bytes) |
| `ACCESS_TOKEN_TTL` | `15m` | No | Access token lifetime |
| `REFRESH_TOKEN_TTL` | `168h` | No | Refresh token lifetime (7 days) |
| `ADMIN_EMAIL` | — | No | Bootstrap admin email (first-run only) |
| `ADMIN_PASSWORD` | — | No | Bootstrap admin password (first-run only) |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | No | Comma-separated allowed frontend origins |
