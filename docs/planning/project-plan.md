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

## Features & Concepts

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

### Phase 2 — Operational Excellence

| Feature | Concepts |
|---|---|
| Multi-tenancy | Tenant isolation strategies (schema-per-tenant vs row-level security) |
| Search & Filters | PostgreSQL full-text search, indexes, query optimization |
| Reporting & Analytics | Read models, CQRS, materialized views |
| Audit Logging | Event sourcing basics, append-only logs, Kafka as audit trail |
| Rate Limiting | Redis token bucket / sliding window, API gateway integration |
| Background Jobs | Worker pools, job queues, at-least-once vs exactly-once delivery |

### Phase 3 — Scale & Production Hardening

| Feature | Concepts |
|---|---|
| Service decomposition | Microservices vs monolith trade-offs, bounded contexts |
| gRPC internal APIs | Protobuf schema design, service mesh basics, interceptors |
| Caching layer | Redis cache-aside pattern, cache invalidation strategies |
| Distributed tracing | OTEL instrumentation, trace propagation across services |
| Health checks & metrics | Prometheus metrics, readiness/liveness probes, SLO/SLI |
| DB migrations | golang-migrate, zero-downtime migrations, blue/green schema changes |
| Circuit breaker | Resilience patterns, fallback strategies |
| Config management | AWS Secrets Manager, environment-specific configs, 12-factor app |

---

## Build Order

1. Project structure + Go module setup + Go workspace
2. PostgreSQL setup + golang-migrate
3. User & Auth service (JWT, RBAC, middleware)
4. Property + Unit + Tenant domain (CRUD, clean architecture)
5. Lease lifecycle (state machine, optimistic locking)
6. Rent collection (idempotency, transactions, financial modeling)
7. Kafka integration — maintenance requests + notifications
8. Redis — caching + rate limiting
9. File uploads — S3 presigned URLs
10. Extract services from monolith (UMS, Billing, Notifications)
11. gRPC internal communication between services
12. OTEL tracing + metrics + health checks

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
