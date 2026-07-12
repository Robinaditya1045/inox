package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inox/inox/backend/internal/api"
	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/config"
	"github.com/inox/inox/backend/internal/media"
	"github.com/inox/inox/backend/internal/observability"
	"github.com/inox/inox/backend/internal/room"
	"github.com/inox/inox/backend/internal/sfu"
	"github.com/inox/inox/backend/internal/storage"
	"github.com/inox/inox/backend/internal/ws"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type App struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *pgxpool.Pool
	Redis  *redis.Client
}

func New(cfg *config.Config, log *slog.Logger, db *pgxpool.Pool, rdb *redis.Client) *App {
	return &App{
		Config: cfg,
		Logger: log,
		DB:     db,
		Redis:  rdb,
	}
}

func (a *App) Run() error {
	// 1. Initialize domain persistence layers
	userRepo := auth.NewUserRepository(a.DB)
	sessionStore := auth.NewSessionStore(a.Redis)
	roomRepo := room.NewRoomRepository(a.DB)
	chatRepo := room.NewChatRepository(a.DB)

	// 2. Initialize business logic services
	authService := auth.NewAuthService(userRepo, sessionStore)
	roomService := room.NewRoomService(roomRepo, userRepo)
	chatService := room.NewChatService(chatRepo, roomRepo)

	obsRepo := observability.NewRepository(a.DB)
	eventAggregator := observability.NewEventAggregator(obsRepo)
	eventAggregator.Start()

	// 3. Initialize real-time WebSocket engine & SFU media manager
	hub := ws.NewHub()
	hub.SetChatService(chatService)
	hub.SetRoomService(roomService)
	hub.SetEventAggregator(eventAggregator)
	sfuMgr := sfu.NewManager()
	hub.SetSFUManager(sfuMgr)
	go hub.Run()

	telemetryHub := observability.NewTelemetryHub(hub)
	go telemetryHub.Run()

	// 4. Initialize HTTP transport handlers
	isProd := strings.ToLower(a.Config.Environment) == "production"
	authHandler := handler.NewAuthHandler(authService, isProd)
	roomHandler := handler.NewRoomHandler(roomService)
	wsHandler := handler.NewWSHandler(hub)
	chatHandler := handler.NewChatHandler(chatService)
	adminHandler := handler.NewAdminHandler(telemetryHub)

	mediaRepo := media.NewRepository(a.DB)
	var storageSvc storage.Service
	minioSvc, err := storage.NewMinioStorage(
		a.Config.MinioEndpoint,
		a.Config.MinioRootUser,
		a.Config.MinioRootPassword,
		a.Config.MinioBucketName,
		a.Config.MediaStreamBaseURL,
		strings.ToLower(a.Config.MinioUseSSL) == "true",
	)
	if err != nil {
		a.Logger.Error("failed to connect to MinIO object storage; falling back to local filesystem", "error", err)
		storageSvc, _ = storage.NewLocalStorage(a.Config.StorageDir, a.Config.MediaStreamBaseURL)
	} else {
		a.Logger.Info("connected to MinIO object storage successfully", "endpoint", a.Config.MinioEndpoint, "bucket", a.Config.MinioBucketName)
		storageSvc = minioSvc
	}
	mediaProcessor := media.NewProcessor(mediaRepo, storageSvc)
	mediaService := media.NewService(mediaRepo, storageSvc, mediaProcessor)
	mediaHandler := handler.NewMediaHandler(mediaService, a.Config.MediaStreamBaseURL)

	// Reconcile stuck or orphaned transcode jobs from previous server sessions in the background
	go mediaService.ReconcileOrphanedAssets(context.Background())

	// 5. Register router with wired handlers & middleware
	router := api.NewRouter(authHandler, authService, roomHandler, roomService, wsHandler, chatHandler, adminHandler, mediaHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", a.Config.HTTPPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		a.Logger.Info("starting HTTP server", "addr", srv.Addr, "env", a.Config.Environment)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()
	a.Logger.Info("shutting down HTTP server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		a.Logger.Error("server shutdown error", "error", err)
	}

	a.Logger.Info("shutting down real-time WebSocket hub, media processor, and analytics worker...")
	mediaProcessor.Shutdown(shutdownCtx)
	eventAggregator.Stop()
	telemetryHub.Stop()
	hub.Shutdown()

	if a.Redis != nil {
		a.Logger.Info("closing Redis connection pool...")
		a.Redis.Close()
	}

	// Cleanly close database connections so PostgreSQL terminates sockets gracefully.
	if a.DB != nil {
		a.Logger.Info("closing PostgreSQL connection pool...")
		a.DB.Close()
	}

	a.Logger.Info("server exited cleanly")
	return nil
}
