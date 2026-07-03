package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/inox/inox/backend/internal/api/respond"
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
