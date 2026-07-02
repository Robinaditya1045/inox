package config

import (
	"fmt"
	"os"
)

type Config struct {
	Environment string
	HTTPPort    string
	LogLevel    string
	DatabaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		Environment: getEnv("APP_ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "debug"),
		// Default local dev DSN (Data Source Name)
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/inox?sslmode=disable"),
	}

	// Fail fast if DATABASE_URL is empty
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}
