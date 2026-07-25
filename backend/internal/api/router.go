package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/observability"
	"github.com/inox/inox/backend/internal/room"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// NewRouter initializes Go 1.22 standard library ServeMux and registers application routes.
func NewRouter(
	authHandler *handler.AuthHandler, authService auth.AuthService,
	roomHandler *handler.RoomHandler, roomService room.RoomService,
	wsHandler *handler.WSHandler,
	chatHandler *handler.ChatHandler,
	adminHandler *handler.AdminHandler,
	mediaHandler *handler.MediaHandler,
	dbPool *pgxpool.Pool,
	redisClient *redis.Client,
) http.Handler {
	mux := http.NewServeMux()

	// Register liveness check endpoint for Kubernetes / Docker health probes.
	mux.HandleFunc("GET /healthz", healthCheckHandler(dbPool, redisClient))

	if authHandler != nil {
		// Public Auth Endpoints (rate-limited to 5 req/min per IP to mitigate brute force attacks)
		authRateLimit := middleware.RateLimit(5, time.Minute)
		mux.Handle("POST /api/v1/auth/signup", authRateLimit(http.HandlerFunc(authHandler.Signup)))
		mux.Handle("POST /api/v1/auth/login", authRateLimit(http.HandlerFunc(authHandler.Login)))
		mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
		mux.Handle("POST /api/v1/auth/forgot-password", authRateLimit(http.HandlerFunc(authHandler.ForgotPassword)))
		mux.Handle("POST /api/v1/auth/reset-password", authRateLimit(http.HandlerFunc(authHandler.ResetPassword)))
	}

	if mediaHandler != nil {
		// Public Media Endpoints (read-only, no auth required for streaming)
		mux.HandleFunc("GET /api/v1/media", mediaHandler.List)
		mux.HandleFunc("GET /api/v1/media/{id}", mediaHandler.GetByID)

		// Direct HTTP byte-range stream endpoint for locally stored media assets and HLS segments
		mux.HandleFunc("PUT /media/stream/upload-direct", mediaHandler.UploadDirectLocal)
		mux.HandleFunc("OPTIONS /media/stream/upload-direct", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		mux.HandleFunc("GET /media/stream/", mediaHandler.StreamProxy)
		mux.HandleFunc("HEAD /media/stream/", mediaHandler.StreamProxy)
		mux.HandleFunc("OPTIONS /media/stream/", mediaHandler.StreamProxy)
		mux.HandleFunc("GET /inox-media/", mediaHandler.StreamProxy)
		mux.HandleFunc("HEAD /inox-media/", mediaHandler.StreamProxy)
		mux.HandleFunc("OPTIONS /inox-media/", mediaHandler.StreamProxy)
		mux.HandleFunc("GET /api/v1/media/stream/", mediaHandler.StreamProxy)
		mux.HandleFunc("HEAD /api/v1/media/stream/", mediaHandler.StreamProxy)
		mux.HandleFunc("OPTIONS /api/v1/media/stream/", mediaHandler.StreamProxy)
	}

	if authService != nil {
		// Protected Endpoints requiring explicit Redis session authentication
		requireAuth := middleware.RequireAuth(authService)
		if authHandler != nil {
			mux.Handle("GET /api/v1/users/me", requireAuth(http.HandlerFunc(authHandler.Me)))
			mux.Handle("PUT /api/v1/users/profile/avatar", requireAuth(http.HandlerFunc(authHandler.UpdateAvatar)))
		}

		if roomHandler != nil && roomService != nil {
			requireMember := middleware.RequireRoomMembership(roomService)

			// Room Creation, Listing & Joining
			mux.Handle("GET /api/v1/rooms", requireAuth(http.HandlerFunc(roomHandler.ListRooms)))
			mux.Handle("POST /api/v1/rooms", requireAuth(http.HandlerFunc(roomHandler.CreateRoom)))
			mux.Handle("POST /api/v1/rooms/{id}/join", requireAuth(http.HandlerFunc(roomHandler.JoinRoom)))

			// Room Invitations
			mux.Handle("GET /api/v1/invitations", requireAuth(http.HandlerFunc(roomHandler.ListInvitations)))
			mux.Handle("POST /api/v1/invitations/{id}/accept", requireAuth(http.HandlerFunc(roomHandler.AcceptInvitation)))
			mux.Handle("POST /api/v1/invitations/{id}/decline", requireAuth(http.HandlerFunc(roomHandler.DeclineInvitation)))

			// Protected Room Workspace Endpoints (Requires both login AND room membership)
			mux.Handle("POST /api/v1/rooms/{id}/invite", requireAuth(requireMember(http.HandlerFunc(roomHandler.InviteUser))))
			mux.Handle("GET /api/v1/rooms/{id}", requireAuth(requireMember(http.HandlerFunc(roomHandler.GetRoom))))
			mux.Handle("PUT /api/v1/rooms/{id}/members/{user_id}/role", requireAuth(http.HandlerFunc(roomHandler.AssignRole)))
			mux.Handle("DELETE /api/v1/rooms/{id}/members/{user_id}", requireAuth(http.HandlerFunc(roomHandler.KickMember)))
			
			// Room deletion and leaving
			mux.Handle("DELETE /api/v1/rooms/{id}", requireAuth(requireMember(http.HandlerFunc(roomHandler.DeleteRoom))))
			mux.Handle("DELETE /api/v1/rooms/{id}/members/me", requireAuth(requireMember(http.HandlerFunc(roomHandler.LeaveRoom))))

			if wsHandler != nil {
				// Real-time WebSocket upgrade endpoint (Requires both login AND room membership)
				mux.Handle("GET /api/v1/rooms/{id}/ws", requireAuth(requireMember(http.HandlerFunc(wsHandler.ServeWS))))
			}

			if chatHandler != nil {
				// Room chat message history retrieval endpoint
				mux.Handle("GET /api/v1/rooms/{id}/messages", requireAuth(requireMember(http.HandlerFunc(chatHandler.GetRecentMessages))))
			}
		}

		// Protected Admin Endpoints (requires authenticated session and system admin privileges)
		if adminHandler != nil {
			mux.Handle("GET /metrics", middleware.RequireMetricsAccess(authService)(http.HandlerFunc(adminHandler.ServePrometheus)))
			mux.Handle("GET /api/v1/admin/telemetry", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(adminHandler.GetSnapshot))))
			mux.Handle("GET /api/v1/admin/telemetry/ws", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(adminHandler.ServeTelemetryWS))))
		}

		// Protected Admin Media Endpoints (requires authenticated session and system admin privileges)
		if mediaHandler != nil {
			mux.Handle("POST /api/v1/admin/media/upload", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(mediaHandler.Upload))))
			mux.Handle("POST /api/v1/admin/media/presigned-url", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(mediaHandler.CreatePresignedUpload))))
			mux.Handle("POST /api/v1/admin/media/complete-upload", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(mediaHandler.CompleteDirectUpload))))
			mux.Handle("POST /api/v1/admin/media/register", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(mediaHandler.Register))))
			mux.Handle("DELETE /api/v1/admin/media/{id}", requireAuth(middleware.RequireAdminRole()(http.HandlerFunc(mediaHandler.Delete))))
		}
	}

	return middleware.CORS(middleware.RequestLogger(withMetrics(mux)))
}

func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		observability.Global().IncHTTPRequest(rw.status)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Hijack implements http.Hijacker to allow WebSocket upgrades through the statusRecorder middleware.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}

// Flush implements http.Flusher to allow streaming responses through the statusRecorder middleware.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter for Go standard library response controllers and hijackers.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// healthCheckHandler responds with HTTP 200/503 and JSON status indicating the server and infrastructure are alive.
func healthCheckHandler(dbPool *pgxpool.Pool, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		response := map[string]string{
			"status": "ok",
		}

		if dbPool == nil || dbPool.Ping(ctx) == nil {
			response["database"] = "ok"
		} else {
			status = http.StatusServiceUnavailable
			response["status"] = "error"
			response["database"] = "down"
		}

		if redisClient == nil || redisClient.Ping(ctx).Err() == nil {
			response["redis"] = "ok"
		} else {
			status = http.StatusServiceUnavailable
			response["status"] = "error"
			response["redis"] = "down"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}
}
