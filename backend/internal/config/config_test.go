package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_ProductionSecretValidation(t *testing.T) {
	// Save and restore os env
	origEnv := os.Getenv("APP_ENV")
	origSecret := os.Getenv("SESSION_SECRET")
	origDB := os.Getenv("DATABASE_URL")
	origRedis := os.Getenv("REDIS_URL")
	defer func() {
		os.Setenv("APP_ENV", origEnv)
		os.Setenv("SESSION_SECRET", origSecret)
		os.Setenv("DATABASE_URL", origDB)
		os.Setenv("REDIS_URL", origRedis)
	}()

	os.Setenv("DATABASE_URL", "postgres://test")
	os.Setenv("REDIS_URL", "redis://test")
	os.Setenv("APP_ENV", "production")

	// 1. Default secret in production should fail
	os.Setenv("SESSION_SECRET", "supersecretkey1234567890abcdefghijklmnopqrstuvwxyz")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must be explicitly configured") {
		t.Fatalf("expected error about explicit SESSION_SECRET configuration, got %v", err)
	}

	// 2. Short secret (< 32 chars) in production should fail
	os.Setenv("SESSION_SECRET", "shortsecret123")
	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "must be at least 32 characters") {
		t.Fatalf("expected error about minimum 32 character length, got %v", err)
	}

	// 3. Valid secret (>= 32 chars and not default) in production should succeed
	os.Setenv("SESSION_SECRET", "mycustomsecretkeythatislongerthan32bytes123!")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected Load to succeed with valid production secret, got %v", err)
	}
	if !cfg.IsProd() {
		t.Errorf("expected IsProd to be true when APP_ENV=production")
	}
}
