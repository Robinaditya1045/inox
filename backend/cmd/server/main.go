package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/inox/inox/backend/internal/app"
	"github.com/inox/inox/backend/internal/config"
	"github.com/inox/inox/backend/internal/database"
	"github.com/inox/inox/backend/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)

	// Create startup context with 10s timeout for connecting to external infrastructure
	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("connecting to PostgreSQL...")
	dbPool, err := database.NewPostgresPool(bootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Error("fatal database connection error", "error", err)
		os.Exit(1)
	}
	log.Info("connected to PostgreSQL successfully")

	log.Info("connecting to Redis...")
	redisClient, err := database.NewRedisClient(bootCtx, cfg.RedisURL)
	if err != nil {
		log.Error("fatal redis connection error", "error", err)
		os.Exit(1)
	}
	log.Info("connected to Redis successfully")

	application := app.New(cfg, log, dbPool, redisClient)

	if err := application.Run(); err != nil {
		log.Error("fatal server error", "error", err)
		os.Exit(1)
	}
}
