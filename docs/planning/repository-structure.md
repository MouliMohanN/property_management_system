# Repository Structure

## Overview

The repository is organized into two top-level domains — `be/` (backend) and `fe/` (frontend) — with a root `Makefile` as the single orchestration entrypoint for CI and local development.

Each layer (service, domain, root) owns its own `docs/`, `scripts/`, and `infra/`. Nothing leaks across boundaries.

---

## Directory Layout

```
property_management_system/
├── be/
│   ├── services/
│   │   ├── core/                      ← monolith to start, split into services later
│   │   │   ├── docs/                  ← service-level: API specs, DB schema, decisions
│   │   │   ├── scripts/               ← service-level: run, migrate, seed
│   │   │   └── infra/                 ← service-level: Dockerfile, env configs
│   │   ├── user-service/              ← future
│   │   │   ├── docs/
│   │   │   ├── scripts/
│   │   │   └── infra/
│   │   └── notification-service/      ← future
│   │       ├── docs/
│   │       ├── scripts/
│   │       └── infra/
│   ├── shared/                        ← shared Go packages (errors, logger, middleware)
│   ├── proto/                         ← .proto definitions for gRPC (shared across services)
│   ├── docs/                          ← BE-wide: architecture diagrams, ADRs
│   ├── scripts/                       ← BE-wide: build all services, lint, workspace tooling
│   ├── infra/                         ← BE-wide: docker-compose wiring all services together
│   └── go.work                        ← Go workspace linking all service modules
├── fe/
│   ├── web/
│   │   ├── docs/                      ← component guidelines, page specs
│   │   ├── scripts/                   ← web-specific build/deploy scripts
│   │   └── infra/                     ← web infra config (e.g. nginx, CDN)
│   ├── mobile/
│   │   ├── docs/
│   │   ├── scripts/
│   │   └── infra/
│   ├── docs/                          ← FE-wide: design system, shared conventions
│   └── scripts/                       ← FE-wide: shared tooling across web and mobile
└── Makefile                           ← root: delegates to be/ and fe/ makefiles
```

---

## Key Principles

### docs/scripts/infra at every meaningful level
- **Per service** — owns its own Dockerfile, migration scripts, API docs
- **Per domain (be/fe)** — cross-service wiring, architecture decisions, shared tooling
- **Root** — thin orchestration only; delegates, never duplicates

### Root Makefile as single entrypoint
The root `Makefile` is the CI and onboarding entrypoint. It delegates to BE/FE makefiles:

```makefile
be-build:
    $(MAKE) -C be build

fe-build:
    $(MAKE) -C fe build

build: be-build fe-build
```

This means:
- `make build` builds everything
- `make -C be build` builds only backend
- Each layer can be developed and deployed independently

### Go Workspaces for multi-service backend
`be/go.work` links all Go service modules so they can share local packages (`be/shared/`) without publishing to a registry during development.

### Proto definitions at be/proto/
gRPC `.proto` files live at the BE level (not per-service) since service contracts are shared between multiple services. Generated code is output into each service's directory.

### Start as monolith, split later
`be/services/core/` begins as the full application. Services are extracted when domain boundaries are clear and the operational overhead is justified.

---

## What goes where — quick reference

| Artifact | Location |
|---|---|
| Docker Compose (all services) | `be/infra/docker-compose.yml` |
| Dockerfile (per service) | `be/services/<name>/infra/Dockerfile` |
| DB migrations | `be/services/<name>/scripts/migrations/` |
| gRPC proto files | `be/proto/` |
| Shared Go utilities | `be/shared/` |
| Architecture diagrams | `be/docs/` |
| API specs (per service) | `be/services/<name>/docs/` |
| CI/CD pipelines | `.github/workflows/` (root) |
