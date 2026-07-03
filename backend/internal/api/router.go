package api

import (
	"encoding/json"
	"net/http"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/auth"
)

// NewRouter initializes Go 1.22 standard library ServeMux and registers application routes.
func NewRouter(authHandler *handler.AuthHandler, authService auth.AuthService) http.Handler {
	mux := http.NewServeMux()

	// Register liveness check endpoint for Kubernetes / Docker health probes.
	mux.HandleFunc("GET /healthz", healthCheckHandler)

	if authHandler != nil {
		// Public Auth Endpoints
		mux.HandleFunc("POST /api/v1/auth/signup", authHandler.Signup)
		mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
		mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	}

	if authService != nil {
		// Protected Endpoints requiring explicit Redis session authentication
		requireAuth := middleware.RequireAuth(authService)
		mux.Handle("GET /api/v1/users/me", requireAuth(http.HandlerFunc(meHandler)))
	}

	return mux
}

// meHandler returns the authenticated user's profile retrieved from Redis session context.
func meHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		handler.WriteError(w, http.StatusUnauthorized, "session context missing")
		return
	}
	handler.WriteJSON(w, http.StatusOK, session)
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
