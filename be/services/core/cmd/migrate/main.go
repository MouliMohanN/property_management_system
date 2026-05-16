package main

import (
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver
	_ "github.com/golang-migrate/migrate/v4/source/file"       // file-based source

	"github.com/MouliMohanN/property_management_system/be/services/core/internal/infrastructure/config"
	"github.com/MouliMohanN/property_management_system/be/shared/logger"
)

// migrationsPath is relative to the directory from which the binary is run.
// When using `go run ./cmd/migrate` from be/services/core/, this resolves correctly.
const migrationsPath = "scripts/migrations"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New("core-migrate", cfg.LogLevel, cfg.Env)

	if len(os.Args) < 2 {
		log.Fatal().Msg("usage: migrate <up|down|version>")
	}

	m, err := migrate.New("file://"+migrationsPath, cfg.DBMigrateURL())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create migrator")
	}
	defer m.Close()

	command := os.Args[1]

	switch command {
	case "up":
		if err := m.Up(); err != nil {
			if err == migrate.ErrNoChange {
				log.Info().Msg("no new migrations to apply")
				return
			}
			log.Fatal().Err(err).Msg("migration up failed")
		}
		log.Info().Msg("all migrations applied")

	case "down":
		if err := m.Steps(-1); err != nil {
			if err == migrate.ErrNoChange {
				log.Info().Msg("no migrations to roll back")
				return
			}
			log.Fatal().Err(err).Msg("migration down failed")
		}
		log.Info().Msg("last migration rolled back")

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get migration version")
		}
		log.Info().Uint("version", version).Bool("dirty", dirty).Msg("current migration state")

	default:
		log.Fatal().Str("command", command).Msg("unknown command — use: up | down | version")
	}
}
