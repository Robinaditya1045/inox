package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/inox/inox/backend/internal/api"
	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/config"
	"github.com/inox/inox/backend/internal/friend"
	"github.com/inox/inox/backend/internal/live"
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
	var stateRepo room.StateRepository
	if a.Redis != nil {
		stateRepo = room.NewRedisStateRepository(a.Redis)
	}
	roomService := room.NewRoomService(roomRepo, userRepo, stateRepo)
	chatService := room.NewChatService(chatRepo, roomRepo)
	friendService := friend.NewService(friend.NewRepository(a.DB), userRepo)

	obsRepo := observability.NewRepository(a.DB)
	eventAggregator := observability.NewEventAggregator(obsRepo)
	eventAggregator.Start()

	// 3. Initialize real-time WebSocket engine & SFU media manager
	hub := ws.NewHub()
	hub.SetChatService(chatService)
	hub.SetRoomService(roomService)
	hub.SetEventAggregator(eventAggregator)
	if a.Redis != nil {
		redisEventBus := ws.NewRedisEventBus(a.Redis)
		redisEventBus.SetHub(hub)
		hub.SetRedisEventBus(redisEventBus)
		hub.SetStateRepository(stateRepo)
	}
	sfuMgr, err := sfu.NewNetworkedManager(sfu.NetworkConfig{
		ICEServers: a.Config.WebRTCICEServers,
		PortMin:    parsePort(a.Config.WebRTCPortMin),
		PortMax:    parsePort(a.Config.WebRTCPortMax),
		PublicIP:   a.Config.WebRTCPublicIP,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize sfu media manager: %w", err)
	}
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
	friendHandler := handler.NewFriendHandler(friendService)

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
	var queueClient media.QueueClient
	if a.Redis != nil {
		redisOpt, err := asynq.ParseRedisURI(a.Config.RedisURL)
		if err == nil {
			queueClient = media.NewAsynqQueueClient(redisOpt)
		}
	}
	mediaService := media.NewService(mediaRepo, storageSvc, mediaProcessor, queueClient)
	mediaHandler := handler.NewMediaHandler(mediaService, a.Config.MediaStreamBaseURL)

	// Live channels. Both ingest paths -- a manifest URL or provider API we already
	// hold, and a page we resolve a manifest out of -- converge on one Resolver
	// interface, so the proxy, the refresh path and the room sync behind it are
	// written once.
	liveFetcher := live.NewFetcher(a.Config.LiveSourceAllowedHosts, a.Config.IsProd())
	liveRegistry := live.NewRegistry(
		live.NewDirectResolver(),
		live.NewAPIResolver(liveFetcher),
		live.NewStaticResolver(liveFetcher),
	)
	liveService := live.NewService(live.NewRepository(a.DB), mediaRepo, liveRegistry, liveFetcher, a.Config.LiveProxyBaseURL)

	var liveHandler *handler.LiveHandler
	liveSealer, err := live.NewSealer(a.Config.SessionSecret)
	if err != nil {
		// Without a sealer the proxy would have to expose upstream URLs to clients,
		// so live channels stay switched off rather than degrading to that.
		a.Logger.Error("failed to initialize live stream sealer; live channels disabled", "error", err)
	} else {
		liveProxy := live.NewProxy(liveService, liveFetcher, liveSealer, live.NewTokenSigner(a.Config.SessionSecret), a.Config.LiveProxyBaseURL)
		liveHandler = handler.NewLiveHandler(liveService, liveProxy)
		hub.SetLiveEdgeSource(liveProxy)
		// Upstream health reaches the rooms watching that channel, so a dead
		// broadcast shows an explanation rather than a frozen frame.
		liveProxy.SetStatusListener(hub.NotifyLiveChannelStatus)
	}

	// Reconcile stuck or orphaned transcode jobs from previous server sessions in the background
	go mediaService.ReconcileOrphanedAssets(context.Background())

	// 5. Initialize CORS origin whitelist from configuration before registering routes.
	// This ensures the CORS middleware validates origins against the explicit whitelist
	// instead of reflecting any arbitrary origin (which would allow credential theft).
	middleware.SetAllowedOrigins(a.Config.CORSAllowedOrigins)

	// 6. Register router with wired handlers & middleware
	router := api.NewRouter(authHandler, authService, roomHandler, roomService, wsHandler, chatHandler, adminHandler, mediaHandler, liveHandler, friendHandler, a.DB, a.Redis)

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

// parsePort converts a configured port string to the uint16 Pion expects,
// yielding 0 (meaning "leave the range unpinned") for anything unparseable or
// outside the valid port space.
func parsePort(raw string) uint16 {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return uint16(n)
}
