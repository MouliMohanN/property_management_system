package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the core service.
// Values are read from environment variables at startup.
//
// Clean architecture note: Config lives in infrastructure/ because it is an
// I/O concern (reading env vars). Domain and usecase layers must never import
// this package — they receive their dependencies via constructor injection.
type Config struct {
	// Application runtime
	Env        string // "development" | "staging" | "production"
	ServerPort string
	LogLevel   string // "debug" | "info" | "warn" | "error"

	// PostgreSQL
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string // "disable" | "require" | "verify-full"

	// Redis
	RedisAddr     string // "host:port"
	RedisPassword string
	RedisDB       int // logical DB index (0–15)

	// JWT
	JWTSecret       string
	AccessTokenTTL  time.Duration // default 15m
	RefreshTokenTTL time.Duration // default 168h (7 days)

	// Admin bootstrap — optional; used only on first startup when users table is empty
	AdminEmail    string
	AdminPassword string
}

// Load reads all configuration from environment variables.
// Returns an error if any required variable is absent.
func Load() (*Config, error) {
	dbPassword, err := requireEnv("DB_PASSWORD")
	if err != nil {
		return nil, err
	}

	jwtSecret, err := requireEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	redisDB, err := getEnvInt("REDIS_DB", 0)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}

	accessTokenTTL, err := getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}

	refreshTokenTTL, err := getEnvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}

	return &Config{
		Env:        getEnv("ENV", "development"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		LogLevel:   getEnv("LOG_LEVEL", "info"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "pms_user"),
		DBPassword: dbPassword,
		DBName:     getEnv("DB_NAME", "property_mgmt"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,

		JWTSecret:       jwtSecret,
		AccessTokenTTL:  accessTokenTTL,
		RefreshTokenTTL: refreshTokenTTL,

		AdminEmail:    getEnv("ADMIN_EMAIL", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
	}, nil
}

// DBDSN returns a PostgreSQL DSN in key=value format, understood by pgx.
// Example: "host=localhost port=5432 user=pms_user password=... dbname=property_mgmt sslmode=disable"
func (c *Config) DBDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

// DBMigrateURL returns a postgres:// URL for golang-migrate.
// Example: "postgres://pms_user:pms_pass@localhost:5432/property_mgmt?sslmode=disable"
func (c *Config) DBMigrateURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func requireEnv(key string) (string, error) {
	if val := os.Getenv(key); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("required environment variable %q is not set", key)
}

func getEnvInt(key string, defaultVal int) (int, error) {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parsing %q=%q as int: %w", key, val, err)
	}
	return n, nil
}

func getEnvDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("parsing %q=%q as duration: %w", key, val, err)
	}
	return d, nil
}
