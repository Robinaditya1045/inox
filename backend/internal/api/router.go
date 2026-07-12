package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/observability"
	"github.com/inox/inox/backend/internal/room"
)

// NewRouter initializes Go 1.22 standard library ServeMux and registers application routes.
func NewRouter(
	authHandler *handler.AuthHandler, authService auth.AuthService,
	roomHandler *handler.RoomHandler, roomService room.RoomService,
	wsHandler *handler.WSHandler,
	chatHandler *handler.ChatHandler,
	adminHandler *handler.AdminHandler,
	mediaHandler *handler.MediaHandler,
) http.Handler {
	mux := http.NewServeMux()

	// Register liveness check endpoint for Kubernetes / Docker health probes.
	mux.HandleFunc("GET /healthz", healthCheckHandler)

	if authHandler != nil {
		// Public Auth Endpoints
		mux.HandleFunc("POST /api/v1/auth/signup", authHandler.Signup)
		mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
		mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	}

	if adminHandler != nil {
		// Observability & Admin Endpoints
		mux.HandleFunc("GET /metrics", adminHandler.ServePrometheus)
		mux.HandleFunc("GET /api/v1/admin/telemetry", adminHandler.GetSnapshot)
		mux.HandleFunc("GET /api/v1/admin/telemetry/ws", adminHandler.ServeTelemetryWS)
	}

	if mediaHandler != nil {
		// Public / Authenticated Media Endpoints
		mux.HandleFunc("GET /api/v1/media", mediaHandler.List)
		mux.HandleFunc("GET /api/v1/media/{id}", mediaHandler.GetByID)

		// Admin Media Endpoints
		mux.HandleFunc("POST /api/v1/admin/media/upload", mediaHandler.Upload)
		mux.HandleFunc("POST /api/v1/admin/media/presigned-url", mediaHandler.CreatePresignedUpload)
		mux.HandleFunc("POST /api/v1/admin/media/complete-upload", mediaHandler.CompleteDirectUpload)
		mux.HandleFunc("POST /api/v1/admin/media/register", mediaHandler.Register)
		mux.HandleFunc("DELETE /api/v1/admin/media/{id}", mediaHandler.Delete)

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
		mux.Handle("GET /api/v1/users/me", requireAuth(http.HandlerFunc(meHandler)))

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

			if wsHandler != nil {
				// Real-time WebSocket upgrade endpoint (Requires both login AND room membership)
				mux.Handle("GET /api/v1/rooms/{id}/ws", requireAuth(requireMember(http.HandlerFunc(wsHandler.ServeWS))))
			}

			if chatHandler != nil {
				// Room chat message history retrieval endpoint
				mux.Handle("GET /api/v1/rooms/{id}/messages", requireAuth(requireMember(http.HandlerFunc(chatHandler.GetRecentMessages))))
			}
		}
	}

	return middleware.CORS(withMetrics(mux))
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

// meHandler returns the authenticated user's profile retrieved from Redis session context.
func meHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "session context missing")
		return
	}
	respond.WriteJSON(w, http.StatusOK, session)
}

// healthCheckHandler responds with HTTP 200 and JSON status indicating the server is alive.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status": "ok",
	}

	_ = json.NewEncoder(w).Encode(response)
}
