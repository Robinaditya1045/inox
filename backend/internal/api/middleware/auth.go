package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
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

var (
	errNoCredentials = errors.New("no session credentials")
	// errSessionStoreUnavailable means the store could not be asked, which says
	// nothing about whether the caller's session is valid.
	errSessionStoreUnavailable = errors.New("session store unavailable")
)

// RequireAuth intercepts HTTP requests, verifies the session cookie or token against Redis,
// and injects the active user session into the request context.
func RequireAuth(authService auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := authenticate(r, authService)
			switch {
			case errors.Is(err, errNoCredentials):
				respond.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			case errors.Is(err, errSessionStoreUnavailable):
				// Clients treat a 401 as "your session is gone" and discard it, so a
				// store outage must not be reported as one.
				respond.WriteError(w, http.StatusServiceUnavailable, "session store unavailable, try again shortly")
				return
			case err != nil:
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

// authenticate resolves the request's session from the first credential that
// validates. It fails with errNoCredentials when the request carries none,
// errSessionStoreUnavailable when a lookup errored and none succeeded, and
// auth.ErrSessionNotFound when every credential is expired or unknown.
func authenticate(r *http.Request, authService auth.AuthService) (*domain.Session, error) {
	sessionIDs := SessionIDsFromRequest(r)
	if len(sessionIDs) == 0 {
		return nil, errNoCredentials
	}

	var lookupErr error
	for _, sessionID := range sessionIDs {
		session, err := authService.ValidateSession(r.Context(), sessionID)
		if err == nil {
			return session, nil
		}
		if !errors.Is(err, auth.ErrSessionNotFound) {
			lookupErr = err
		}
	}

	if lookupErr != nil {
		slog.Error("session lookup failed", "error", lookupErr, "path", r.URL.Path)
		return nil, errSessionStoreUnavailable
	}
	return nil, auth.ErrSessionNotFound
}

// SessionIDsFromRequest returns every distinct session ID the request carries,
// explicit credentials first. The browser attaches the cookie on its own, so it
// can outlive the session the client actually holds; a stale one must not
// shadow a valid token sent alongside it.
func SessionIDsFromRequest(r *http.Request) []string {
	var ids []string
	add := func(id string) {
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}

	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		add(strings.TrimPrefix(authHeader, "Bearer "))
	}
	add(r.Header.Get("X-Session-ID"))
	add(r.URL.Query().Get("session_id"))
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		add(cookie.Value)
	}

	return ids
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
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}
