.PHONY: run build migrate-up migrate-down migrate-new migrate-status migrate-force

MIGRATIONS_PATH := ./scripts/migrations

# DB_URL is built from environment variables.
# Before running any migrate-* target, export your env:
#   $ export $(grep -v '^#' infra/.env | xargs)
#   or: $ set -a && source infra/.env && set +a
DB_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

## Start the API server (requires env vars to be set)
run:
	go run ./cmd/api

## Build both binaries into bin/
build:
	@mkdir -p bin
	go build -o bin/api     ./cmd/api
	go build -o bin/migrate ./cmd/migrate

## Apply all pending migrations
migrate-up:
	@echo "→ Running migrations up..."
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

## Roll back the most recent migration
migrate-down:
	@echo "→ Rolling back last migration..."
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down 1

## Create a new migration pair: make migrate-new name=<migration_name>
migrate-new:
	@test -n "$(name)" || (echo "Error: provide a name. Usage: make migrate-new name=<migration_name>" && exit 1)
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(name)
	@echo "→ Created migration: $(MIGRATIONS_PATH)/*_$(name).{up,down}.sql"

## Show current migration version
migrate-status:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" version

## Force migration version (use after fixing a dirty state): make migrate-force version=<N>
migrate-force:
	@test -n "$(version)" || (echo "Error: provide a version. Usage: make migrate-force version=<N>" && exit 1)
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" force $(version)
