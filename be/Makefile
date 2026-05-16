.PHONY: build test lint tidy infra-up infra-down infra-logs

# ── Workspace ────────────────────────────────────────────────────────────────

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go work sync
	find . -name "go.mod" -execdir go mod tidy \;

# ── Infrastructure (docker-compose) ──────────────────────────────────────────

## Start PostgreSQL and Redis in the background
infra-up:
	docker compose -f infra/docker-compose.yml up -d

## Stop and remove containers (volumes are preserved)
infra-down:
	docker compose -f infra/docker-compose.yml down

## Tail logs from all infra containers
infra-logs:
	docker compose -f infra/docker-compose.yml logs -f

## Tear down containers AND delete all data volumes
infra-clean:
	docker compose -f infra/docker-compose.yml down -v

# ── Core service delegation ───────────────────────────────────────────────────

## Delegate any core-* target to be/services/core/Makefile
## Examples: make core-run  make core-migrate-up  make core-migrate-new name=create_users
core-%:
	$(MAKE) -C services/core $* name=$(name) version=$(version)
