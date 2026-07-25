package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/inox/inox/backend/internal/config"
	"github.com/inox/inox/backend/internal/database"
	"github.com/inox/inox/backend/internal/media"
	"github.com/inox/inox/backend/internal/storage"
	"github.com/inox/inox/backend/pkg/logger"
)

type TranscodeProcessor struct {
	mediaProcessor *media.Processor
	repo           media.Repository
}

func (p *TranscodeProcessor) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload media.TranscodeMediaPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal task payload: %w", err)
	}

	slog.Info("worker processing transcode task", "asset_id", payload.AssetID)

	asset, err := p.repo.GetAssetByID(ctx, payload.AssetID)
	if err != nil {
		return fmt.Errorf("failed to fetch asset from db: %w", err)
	}

	err = p.mediaProcessor.ProcessAsset(ctx, asset)
	if err != nil {
		slog.Error("transcoding failed", "asset_id", asset.ID, "error", err)
		return err
	}

	slog.Info("worker completed transcode task successfully", "asset_id", payload.AssetID)
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)

	bootCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Info("worker connecting to PostgreSQL...")
	dbPool, err := database.NewPostgresPool(bootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Error("fatal database connection error", "error", err)
		os.Exit(1)
	}
	log.Info("connected to PostgreSQL successfully")

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		log.Error("invalid redis uri for asynq", "error", err)
		os.Exit(1)
	}

	mediaRepo := media.NewRepository(dbPool)
	var storageSvc storage.Service
	minioSvc, err := storage.NewMinioStorage(
		cfg.MinioEndpoint,
		cfg.MinioRootUser,
		cfg.MinioRootPassword,
		cfg.MinioBucketName,
		cfg.MediaStreamBaseURL,
		strings.ToLower(cfg.MinioUseSSL) == "true",
	)
	if err != nil {
		log.Error("failed to connect to MinIO object storage; falling back to local filesystem", "error", err)
		storageSvc, _ = storage.NewLocalStorage(cfg.StorageDir, cfg.MediaStreamBaseURL)
	} else {
		log.Info("connected to MinIO object storage successfully")
		storageSvc = minioSvc
	}
	
	processor := media.NewProcessor(mediaRepo, storageSvc)

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"default": 10,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				slog.Error("task processing failed", "type", task.Type(), "error", err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	mux.Handle(media.TaskTypeTranscodeMedia, &TranscodeProcessor{
		mediaProcessor: processor,
		repo:           mediaRepo,
	})

	log.Info("starting asynq worker server...")
	if err := srv.Run(mux); err != nil {
		log.Error("could not start asynq server", "error", err)
		os.Exit(1)
	}
}
