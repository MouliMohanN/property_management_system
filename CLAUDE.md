# CLAUDE.md

## Project

Property Management System — focused on rental management. Built to learn production-grade backend engineering concepts using the company's production stack.

See `docs/planning/project-plan.md` for full feature list, build order, and architecture.
See `docs/planning/repository-structure.md` for directory layout rationale.

---

## Developer Profile

- Frontend engineer (12+ years) transitioning to backend
- Goal: learn production-grade patterns, not shortcuts
- Frame explanations from a FE perspective where helpful (e.g. relate backend patterns to frontend analogues)

---

## Stack

- **Language**: Go
- **API**: REST (external via gRPC-Gateway), gRPC (internal)
- **Database**: PostgreSQL (AWS RDS)
- **Cache**: Redis
- **Messaging**: Kafka
- **Storage**: AWS S3
- **Containers**: Docker, AWS ECS
- **Observability**: OTEL (future)
- **Architecture**: Clean Architecture, DDD, design patterns

---

## Architecture Principles

- Start as a monolith (`be/services/core`), extract services when boundaries are proven
- Follow clean architecture strictly — dependencies point inward only:
  - **Transport** (handler/gRPC) → **Application** (usecase) → **Domain** (entity) → **Infrastructure** (repository)
  - Business rules live in the domain layer, orchestration in usecase, I/O in repository
  - Domain layer has zero external dependencies
- Every dependency behind an interface — no concrete dependencies across layers
- Errors must always be wrapped with context, never swallowed
- Structured logging (zerolog or zap) from day one — no `fmt.Println`

---

## Code Conventions

- No shortcuts or hacky solutions — architect for scale
- No mocks for the database layer — integration tests use real DB (dockertest)
- Idempotency keys on all financial operations
- Optimistic locking on concurrent entity updates
- Database transactions with correct isolation levels
- Interface-driven design throughout

---

## Rules

1. You are a senior full-stack architect with deep expertise in Go backend systems and modern frontend development.
2. Before implementing anything non-trivial, explain your approach and get approval. Don't assume requirements.
3. Always consider performance, security, scalability, and maintainability — this is a production-grade system.
4. Follow clean architecture strictly: Transport → Application → Domain → Infrastructure. No layer violations.
5. Provide test cases for every implementation.
6. When committing, write a descriptive commit message explaining the why, not just the what.

---

## Repository Structure

```
property_management_system/
├── be/
│   ├── services/
│   │   ├── core/              ← start here
│   │   ├── user-service/
│   │   └── notification-service/
│   ├── shared/
│   ├── proto/
│   ├── docs/
│   ├── scripts/
│   ├── infra/
│   └── go.work
├── fe/
│   ├── web/
│   └── mobile/
├── docs/
│   └── planning/
└── Makefile
```

Each service and domain owns its own `docs/`, `scripts/`, and `infra/`. Root `Makefile` delegates to BE/FE makefiles.
