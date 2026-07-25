package middleware

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
)

const (
	roomContextKey       contextKey = "current_room"
	roomMemberContextKey contextKey = "current_room_member"
)

// RequireRoomMembership checks if the authenticated session user is an active participant
// in the requested {id} room, and injects the room and membership profile into context.
func RequireRoomMembership(roomService room.RoomService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := GetSessionFromContext(r.Context())
			if !ok {
				respond.WriteError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			roomID := r.PathValue("id")
			if roomID == "" {
				respond.WriteError(w, http.StatusBadRequest, "missing room id path parameter")
				return
			}

			rm, member, err := roomService.GetRoomAndMember(r.Context(), roomID, session.UserID)
			if err != nil {
				if errors.Is(err, room.ErrMemberNotFound) || errors.Is(err, room.ErrRoomNotFound) {
					respond.WriteError(w, http.StatusForbidden, "access denied: not a member of this room")
					return
				}
				respond.WriteError(w, http.StatusInternalServerError, "failed to verify room membership")
				return
			}

			ctx := context.WithValue(r.Context(), roomContextKey, rm)
			ctx = context.WithValue(ctx, roomMemberContextKey, member)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRoomFromContext retrieves the Room struct injected by RequireRoomMembership.
func GetRoomFromContext(ctx context.Context) (*domain.Room, bool) {
	rm, ok := ctx.Value(roomContextKey).(*domain.Room)
	return rm, ok
}

// GetRoomMemberFromContext retrieves the RoomMember struct injected by RequireRoomMembership.
func GetRoomMemberFromContext(ctx context.Context) (*domain.RoomMember, bool) {
	m, ok := ctx.Value(roomMemberContextKey).(*domain.RoomMember)
	return m, ok
}

// IsAdminSession helper verifies if the current session has system administrator privileges.
func IsAdminSession(session *domain.Session) bool {
	if session == nil {
		return false
	}
	if session.Role == domain.SystemRoleAdmin {
		return true
	}
	// Also allow configured admin emails via ADMIN_EMAILS environment variable
	adminEmails := strings.Split(os.Getenv("ADMIN_EMAILS"), ",")
	for _, e := range adminEmails {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" && e == strings.ToLower(session.Email) {
			return true
		}
	}
	return false
}

// RequireAdminRole verifies that the authenticated user has system administrator privileges.
// Must be chained after RequireAuth.
func RequireAdminRole() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := GetSessionFromContext(r.Context())
			if !ok || !IsAdminSession(session) {
				respond.WriteError(w, http.StatusForbidden, "access denied: system administrator privileges required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireMetricsAccess guards telemetry exposition endpoints like /metrics.
// Allows access if (1) request is loopback IP, (2) Authorization Bearer token matches METRICS_AUTH_TOKEN,
// or (3) request is made by an authenticated system administrator.
func RequireMetricsAccess(authService auth.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Check if loopback address (local Prometheus scraping)
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil && (host == "127.0.0.1" || host == "::1" || host == "localhost") {
				next.ServeHTTP(w, r)
				return
			}

			// 2. Check METRICS_AUTH_TOKEN if configured
			token := os.Getenv("METRICS_AUTH_TOKEN")
			if token != "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") && strings.TrimPrefix(authHeader, "Bearer ") == token {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 3. Check if caller has an active admin session
			cookie, err := r.Cookie("session")
			if err == nil && cookie.Value != "" && authService != nil {
				session, err := authService.ValidateSession(r.Context(), cookie.Value)
				if err == nil && IsAdminSession(session) {
					next.ServeHTTP(w, r)
					return
				}
			}

			respond.WriteError(w, http.StatusForbidden, "access denied: metrics access protected")
		})
	}
}
