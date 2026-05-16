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

| Feature | Concepts |
|---|---|
| User & Auth | JWT, refresh tokens, RBAC, bcrypt, middleware chains |
| Property CRUD | REST API design, repository pattern, clean architecture layers |
| Unit/Room Management | Parent-child entity relationships, DB normalization |
| Tenant Management | Domain modeling, value objects (DDD) |
| Lease/Rental Agreement | State machines, document lifecycle, optimistic locking |
| Rent Collection & Invoicing | Financial data modeling, idempotency, double-entry accounting |
| Maintenance Requests | Event-driven design, status transitions, Kafka pub/sub |
| Notifications (email/SMS) | Kafka consumers, async processing, retry with backoff |
| File Uploads (docs, photos) | S3 presigned URLs, multipart upload, CDN patterns |

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
