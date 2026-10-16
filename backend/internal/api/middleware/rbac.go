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

const adminRoleContextKey contextKey = "admin_role"

// AdminAccess resolves a session's admin portal role. admin.Service implements it.
type AdminAccess interface {
	RoleFor(ctx context.Context, session *domain.Session) (domain.AdminRole, error)
}

// RequireAdminRole admits owners (ADMIN_EMAILS) and the admins they add, and records
// which one the caller is for GetAdminRoleFromContext. Must be chained after RequireAuth.
func RequireAdminRole(access AdminAccess) func(http.Handler) http.Handler {
	return requireAdmin(access, false)
}

// RequireAdminOwner admits owners only: deciding who has admin access is theirs alone.
// Must be chained after RequireAuth.
func RequireAdminOwner(access AdminAccess) func(http.Handler) http.Handler {
	return requireAdmin(access, true)
}

func requireAdmin(access AdminAccess, ownerOnly bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := GetSessionFromContext(r.Context())
			// With no access resolver nobody is an admin: fail closed.
			if !ok || access == nil {
				respond.WriteError(w, http.StatusForbidden, "access denied: system administrator privileges required")
				return
			}
			role, err := access.RoleFor(r.Context(), session)
			if err != nil {
				respond.WriteError(w, http.StatusServiceUnavailable, "admin access check unavailable, try again shortly")
				return
			}
			if role == domain.AdminRoleNone {
				respond.WriteError(w, http.StatusForbidden, "access denied: system administrator privileges required")
				return
			}
			if ownerOnly && role != domain.AdminRoleOwner {
				respond.WriteError(w, http.StatusForbidden, "access denied: only owners can manage admins")
				return
			}
			ctx := context.WithValue(r.Context(), adminRoleContextKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetAdminRoleFromContext retrieves the role recorded by RequireAdminRole or RequireAdminOwner.
func GetAdminRoleFromContext(ctx context.Context) (domain.AdminRole, bool) {
	role, ok := ctx.Value(adminRoleContextKey).(domain.AdminRole)
	return role, ok
}

// RequireMetricsAccess guards telemetry exposition endpoints like /metrics.
// Allows access if (1) request is loopback IP, (2) Authorization Bearer token matches METRICS_AUTH_TOKEN,
// or (3) request is made by an authenticated owner or admin.
func RequireMetricsAccess(authService auth.AuthService, access AdminAccess) func(http.Handler) http.Handler {
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
			if authService != nil && access != nil {
				session, err := authenticate(r, authService)
				if errors.Is(err, errSessionStoreUnavailable) {
					respond.WriteError(w, http.StatusServiceUnavailable, "session store unavailable, try again shortly")
					return
				}
				if err == nil {
					role, err := access.RoleFor(r.Context(), session)
					if err != nil {
						respond.WriteError(w, http.StatusServiceUnavailable, "admin access check unavailable, try again shortly")
						return
					}
					if role != domain.AdminRoleNone {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			respond.WriteError(w, http.StatusForbidden, "access denied: metrics access protected")
		})
	}
}
