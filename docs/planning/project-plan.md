# Project Plan — Property Management System

## Goal

Build a production-grade rental management system using the same stack used in production at the company. The primary objective is learning backend engineering concepts that apply at scale — not building a toy project.

Start focused on **rental management**. Expand to lease management and buy/sell later.

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go |
| API (external) | REST (gRPC-Gateway) |
| API (internal) | gRPC |
| Database | PostgreSQL (AWS RDS) |
| Cache | Redis |
| Messaging | Kafka |
| File storage | AWS S3 |
| Containerization | Docker, AWS ECS |
| Compute | AWS EC2 |
| API Gateway | AWS API Gateway |
| Observability | OTEL (future) |
| Architecture | Clean Architecture, DDD, design patterns |

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  API Gateway (AWS)                   │
└──────────────────┬──────────────────────────────────┘
                   │ REST (external)
        ┌──────────▼──────────┐
        │   BFF / REST Layer  │  ← gRPC-Gateway or dedicated BFF
        └──────────┬──────────┘
                   │ gRPC (internal)
    ┌──────────────┼───────────────────┐
    │              │                   │
┌───▼───┐    ┌─────▼────┐    ┌────────▼───────┐
│  UMS  │    │  PMS Core│    │ Billing Service │
│(Users)│    │(Property)│    │  (Rent/Invoice) │
└───────┘    └─────┬────┘    └────────────────┘
                   │
          ┌────────▼────────┐
          │  Kafka (events) │
          └────────┬────────┘
                   │
          ┌────────▼────────┐
          │ Notification Svc│
          └─────────────────┘
```

**Start as a monolith** (`be/services/core`). Extract services once domain boundaries are proven and operational overhead is justified.

---

## Phases

### Phase 0 — Foundation ✅

Prerequisites completed before any feature work. Not a "feature phase" — just the ground the system stands on.

| Done | Work |
|---|---|
| ✅ | Go workspace + multi-module setup (`be/go.work`) |
| ✅ | PostgreSQL + Redis (docker-compose, pgxpool, go-redis) |
| ✅ | golang-migrate setup + first migration (uuid-ossp, pgcrypto) |
| ✅ | Shared structured logger (zerolog) |
| ✅ | Config loading from environment variables |
| ✅ | Application bootstrap with graceful shutdown |

---

### Phase 1 — Core Rental Management

Each feature is a self-contained sub-phase. Dependencies flow top-to-bottom — each sub-phase builds on the one above it.

---

#### Phase 1a — User & Auth ✅ Complete

**Endpoints**

| Method | Path | Access |
|---|---|---|
| `POST` | `/api/v1/auth/register` | admin only |
| `POST` | `/api/v1/auth/login` | public |
| `POST` | `/api/v1/auth/refresh` | public (valid refresh token) |
| `POST` | `/api/v1/auth/logout` | authenticated |
| `GET` | `/api/v1/auth/me` | authenticated |

**Concepts:** JWT (HMAC-SHA256), refresh token rotation, RBAC (role-only, Phase 1), bcrypt, middleware chains, dockertest integration tests

**Token strategy**
- Access token: JWT, 15 min TTL, stateless — payload: `{ user_id, role, jti }`
- Refresh token: opaque 32-byte random string, stored in Redis, 7-day TTL, rotated on every use

**RBAC roles:** `admin`, `landlord`, `tenant`, `maintenance_staff`
- Phase 1: coarse-grained — role checked per route, no per-resource permissions
- Phase 2: fine-grained — ownership checks when multi-tenancy is introduced

**Bootstrap:** on startup, if zero users exist, create admin from `ADMIN_EMAIL` + `ADMIN_PASSWORD` env vars

**DB schema**
```sql
CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(255) NOT NULL UNIQUE,
    phone_number  VARCHAR(20)  UNIQUE,
    password_hash TEXT         NOT NULL,
    first_name    VARCHAR(100) NOT NULL,
    last_name     VARCHAR(100) NOT NULL,
    role          VARCHAR(50)  NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'active',
    version       INTEGER      NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

**Milestones**

| Status | Milestone |
|---|---|
| ✅ | Domain layer — User entity, Role type, UserRepository interface, domain errors |
| ✅ | DB migration — `000002_users` |
| ✅ | Infrastructure: postgres — `user_repository.go` |
| ✅ | Infrastructure: redis — `token_store.go` (refresh token CRUD) |
| ✅ | Infrastructure: token — `jwt.go` (generate + validate access tokens) |
| ✅ | Use cases — Register, Login, Refresh, Logout, GetMe |
| ✅ | Transport — chi router, auth handler, JWT middleware, RBAC middleware |
| ✅ | Wire — update `main.go` + `config.go` (JWT secret, token TTLs, admin bootstrap) |
| ✅ | Integration tests — dockertest setup, repository tests, use case tests |

**New dependencies**
- `github.com/go-chi/chi/v5` — router with middleware chain support
- `github.com/golang-jwt/jwt/v5` — JWT generation + validation
- `golang.org/x/crypto` — bcrypt
- `github.com/ory/dockertest/v3` — integration tests against real Postgres + Redis
- `github.com/go-playground/validator/v10` — request validation

---

#### Phase 1b — Property CRUD ⬜ Not Started

**Concepts:** REST API design, repository pattern, full clean architecture vertical slice, pagination

---

#### Phase 1c — Unit / Room Management ⬜ Not Started

**Concepts:** parent-child entity relationships, DB normalization, cascading state

---

#### Phase 1d — Tenant Management ⬜ Not Started

**Concepts:** domain modeling, value objects (DDD), KYC document workflow

**Extended profile (separate from auth User — linked by user_id)**
- Employment: employer name, phone, monthly income
- Identity documents: govt ID, company ID, income proof, address proof (type + number + S3 URL)
- Emergency contacts (one-to-many)
- Social/reference links
- Previous rental: prior address, prior landlord name + phone
- Background check status (`pending` / `cleared` / `flagged`)
- PAN number (TDS compliance)

---

#### Phase 1e — Lease / Rental Agreement ⬜ Not Started

**Concepts:** state machines, document lifecycle, optimistic locking

---

#### Phase 1f — Rent Collection & Invoicing ⬜ Not Started

**Concepts:** financial data modeling, idempotency keys, double-entry accounting

---

#### Phase 1g — Maintenance Requests ⬜ Not Started

**Concepts:** event-driven design, status transitions, Kafka pub/sub

---

#### Phase 1h — Notifications ⬜ Not Started

**Concepts:** Kafka consumers, async processing, retry with exponential backoff

---

#### Phase 1i — File Uploads ⬜ Not Started

**Concepts:** S3 presigned URLs, multipart upload, CDN patterns

---

### Phase 2 — Operational Excellence

Planned after Phase 1 is complete. Exact sequencing within this phase will be determined by real usage patterns and pain points.

| Feature | Concepts |
|---|---|
| Multi-tenancy | Tenant isolation strategies (schema-per-tenant vs row-level security) |
| Search & Filters | PostgreSQL full-text search, indexes, query optimization |
| Reporting & Analytics | Read models, CQRS, materialized views |
| Audit Logging | Event sourcing basics, append-only logs, Kafka as audit trail |
| Rate Limiting | Redis token bucket / sliding window, API gateway integration |
| Background Jobs | Worker pools, job queues, at-least-once vs exactly-once delivery |

---

### Phase 3 — Scale & Production Hardening

Planned after Phase 2. Sequencing driven by which scaling bottlenecks surface first.

| Feature | Concepts |
|---|---|
| Service decomposition | Microservices vs monolith trade-offs, bounded contexts |
| gRPC internal APIs | Protobuf schema design, service mesh basics, interceptors |
| Caching layer | Redis cache-aside pattern, cache invalidation strategies |
| Distributed tracing | OTEL instrumentation, trace propagation across services |
| Health checks & metrics | Prometheus metrics, readiness/liveness probes, SLO/SLI |
| Circuit breaker | Resilience patterns, fallback strategies |
| Config management | AWS Secrets Manager, environment-specific configs, 12-factor app |

---

## Production-Grade Principles (non-negotiable from day one)

- **Idempotency keys** on all financial operations
- **Optimistic locking** on concurrent entity updates
- **Database transactions** with correct isolation levels
- **Structured logging** (zerolog or zap) from the first line of code
- **Error wrapping** — never swallow errors, always add context
- **Interface-driven design** — every dependency behind an interface (testability)
- **Integration tests against real DB** (dockertest) — no mocks for DB layer
- **No shortcuts** — architecture decisions made for scale, not convenience

---

## Future Scope

- Lease management
- Buy/sell transactions
- Owner portal
- Vendor/contractor management
- Document e-signing integration
