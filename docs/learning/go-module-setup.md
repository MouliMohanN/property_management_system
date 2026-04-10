# Go Module Setup — Commands & Concepts

Commands used when setting up the Go module structure for this project, with detailed explanations.

---

## 1. `brew install go`

Installs Go using Homebrew (macOS package manager).

```bash
brew install go
```

Homebrew downloads the precompiled Go binary and places it at `/opt/homebrew/Cellar/go/1.26.2/`.
It also symlinks the `go` binary to `/opt/homebrew/bin/go` so you can run it from anywhere in the terminal.

---

## 2. `go version`

Verifies the installation and shows which version is active.

```bash
go version
# go version go1.26.2 darwin/arm64
```

`darwin/arm64` = macOS on Apple Silicon (M-chip).

---

## 3. `go mod init <module-name>`

Creates a `go.mod` file — the **module definition file** for a Go project. Every Go module must have one.

```bash
cd be/services/core
go mod init github.com/MouliMohanN/property_management_system/be/services/core
```

Creates this file:

```
# be/services/core/go.mod
module github.com/MouliMohanN/property_management_system/be/services/core

go 1.26
```

Run once per module. In this project we ran it twice:

```bash
# for the core service
go mod init github.com/MouliMohanN/property_management_system/be/services/core

# for the shared package
go mod init github.com/MouliMohanN/property_management_system/be/shared
```

Each is an independent module with its own dependency graph.

### Module naming convention

The module name must be a valid, fetchable path — even if the repo is private.

| Option | Example | When to use |
|---|---|---|
| GitHub path | `github.com/MouliMohanN/pms/be/services/core` | Standard, works everywhere |
| Custom domain | `go.company.com/pms/core` | Large orgs with vanity URLs |
| No host | `core` | Never — breaks `go get` |

---

## 4. `go work init`

Creates a `go.work` file — the **Go workspace definition file**. Workspaces let multiple modules in the same repo resolve each other locally without publishing to GitHub.

```bash
cd be
go work init
```

Creates an empty workspace:

```
# be/go.work
go 1.26
```

---

## 5. `go work use <path>`

Registers modules into the workspace.

```bash
go work use ./services/core ./shared
```

Updates `go.work` to:

```
# be/go.work
go 1.26

use (
    ./services/core
    ./shared
)
```

### Why this matters

When `core` imports from `shared`:

```go
import "github.com/MouliMohanN/property_management_system/be/shared/logger"
```

**Without** `go.work` → Go tries to fetch `shared` from `github.com` → fails (not published yet).

**With** `go.work` → Go resolves it from your local `./shared` directory → works instantly.

This means during development you never need to push `shared` to GitHub before `core` can use it.

---

## 6. `mkdir -p <path>`

Creates directories. The `-p` flag means:
- Create all intermediate parent directories in one shot
- Don't error if the directory already exists

```bash
mkdir -p be/services/core/internal/domain/property
```

Without `-p`, you'd have to create each level manually:

```bash
mkdir be/services/core
mkdir be/services/core/internal
mkdir be/services/core/internal/domain
mkdir be/services/core/internal/domain/property
```

With `-p`, one command handles any depth. We used this to scaffold the entire clean architecture structure at once.

---

## 7. `find <path> -type d | sort`

Lists all directories under a path, sorted alphabetically. Used to verify structure after scaffolding.

```bash
find be/services/core -type d | sort
```

| Part | Meaning |
|---|---|
| `find be/services/core` | Walk the directory tree from this path |
| `-type d` | Only show directories, not files |
| `\| sort` | Pipe output to sort alphabetically |

---

## What it all produced

```
be/
├── go.work                        ← go work init + go work use
├── Makefile
└── services/
    └── core/
        ├── go.mod                 ← go mod init
        ├── cmd/
        │   └── api/
        │       └── main.go        ← entry point
        └── internal/
            ├── domain/            ← entities, no external deps
            │   ├── property/
            │   ├── tenant/
            │   └── lease/
            ├── usecase/           ← business logic, orchestration
            │   ├── property/
            │   ├── tenant/
            │   └── lease/
            ├── infrastructure/    ← concrete I/O implementations
            │   ├── postgres/
            │   └── redis/
            └── transport/         ← HTTP and gRPC handlers
                ├── http/
                │   └── handler/
                └── grpc/
shared/
└── go.mod                         ← go mod init
```

### Why `internal/`?

The `internal/` directory name is special in Go. Any package inside `internal/` can **only** be imported by code within the parent directory tree. This is enforced at compile time.

In our case, `be/services/core/internal/...` means nothing outside `core/` can import these packages — not even `user-service` or `shared`. This enforces clean architecture boundaries at the language level, not just by convention.
