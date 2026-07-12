package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

type contextKey string

const (
	SessionCookieName            = "inox_session"
	sessionContextKey contextKey = "authenticated_session"
)

// RequireAuth intercepts HTTP requests, verifies the session cookie or token against Redis,
// and injects the active user session into the request context.
func RequireAuth(authService auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := ""
			cookie, err := r.Cookie(SessionCookieName)
			if err == nil && cookie.Value != "" {
				sessionID = cookie.Value
			} else if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				sessionID = strings.TrimPrefix(authHeader, "Bearer ")
			} else if xSession := r.Header.Get("X-Session-ID"); xSession != "" {
				sessionID = xSession
			} else if qSession := r.URL.Query().Get("session_id"); qSession != "" {
				sessionID = qSession
			}

			if sessionID == "" {
				respond.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			session, err := authService.ValidateSession(r.Context(), sessionID)
			if err != nil {
				// Clear expired or invalid session cookie from client
				ClearSessionCookie(w)
				respond.WriteError(w, http.StatusUnauthorized, "session expired or invalid")
				return
			}

			// Store session in context for downstream route handlers
			ctx := context.WithValue(r.Context(), sessionContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSessionFromContext retrieves the authenticated session payload stored by RequireAuth.
func GetSessionFromContext(ctx context.Context) (*domain.Session, bool) {
	session, ok := ctx.Value(sessionContextKey).(*domain.Session)
	return session, ok
}

// WithSessionContext returns a new context with the session injected (useful for unit testing and internal RPCs).
func WithSessionContext(ctx context.Context, session *domain.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// ClearSessionCookie sets an expired cookie to purge the session ID from browser memory.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
