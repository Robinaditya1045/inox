package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
	"github.com/inox/inox/backend/internal/ws"

	"github.com/google/uuid"
)

// ActivitySource reports which rooms are in use right now. The WebSocket hub
// satisfies it.
type ActivitySource interface {
	RoomActivity(ctx context.Context) (map[string]domain.RoomActivity, error)
}

// activityTimeout bounds how long the room list waits on the hub. Activity is a
// decoration: a list without it is better than a slow list.
const activityTimeout = 300 * time.Millisecond

// RoomCloser disconnects everyone still in a room that has been deleted. The
// WebSocket hub satisfies it.
type RoomCloser interface {
	CloseRoom(roomID string)
}

// MemberRemover disconnects someone whose membership of a room was just taken
// away. The WebSocket hub satisfies it.
type MemberRemover interface {
	RemoveMember(roomID, userID string, reason ws.RemovalReason)
}

// maxAdminRoomDeletes bounds one bulk delete request.
const maxAdminRoomDeletes = 500

type RoomHandler struct {
	roomService room.RoomService
	activity    ActivitySource
	closer      RoomCloser
	remover     MemberRemover
}

func NewRoomHandler(roomService room.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
}

// SetActivitySource lets ListRooms mark rooms that people are in and what they
// are watching. Optional: without it rooms are listed with no activity.
func (h *RoomHandler) SetActivitySource(src ActivitySource) {
	h.activity = src
}

// SetRoomCloser lets room deletion turn out the people still connected to the
// room. Optional: without it they stay connected until they leave.
func (h *RoomHandler) SetRoomCloser(c RoomCloser) {
	h.closer = c
}

// SetMemberRemover lets kicking and leaving close the member's connections to
// the room. Optional: without it they stay connected until they disconnect.
func (h *RoomHandler) SetMemberRemover(m MemberRemover) {
	h.remover = m
}

func (h *RoomHandler) removeMember(roomID, userID string, reason ws.RemovalReason) {
	if h.remover != nil {
		h.remover.RemoveMember(roomID, userID, reason)
	}
}

func (h *RoomHandler) closeRooms(roomIDs []string) {
	if h.closer == nil {
		return
	}
	for _, id := range roomIDs {
		h.closer.CloseRoom(id)
	}
}

type createRoomRequest struct {
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

type assignRoleRequest struct {
	Role        domain.Role         `json:"role"`
	Permissions *domain.Permissions `json:"permissions"`
}

// CreateRoom handles new room workspace provisioning.
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rm, member, err := h.roomService.CreateRoom(r.Context(), session.UserID, req.Name, req.IsPrivate)
	if err != nil {
		if errors.Is(err, room.ErrInvalidRoomName) {
			respond.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	respond.WriteJSON(w, http.StatusCreated, map[string]any{
		"room":   rm,
		"member": member,
	})
}

// ListRooms retrieves all public rooms and private rooms where the user is an owner or member.
func (h *RoomHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rooms, err := h.roomService.ListRooms(r.Context(), session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to list rooms")
		return
	}

	h.attachActivity(r.Context(), rooms)
	respond.WriteJSON(w, http.StatusOK, rooms)
}

// attachActivity marks each listed room that someone is connected to. Only rooms
// already in the caller's list are touched, so a private room's activity is never
// shown to anyone who could not see the room itself.
func (h *RoomHandler) attachActivity(ctx context.Context, rooms []*domain.Room) {
	if h.activity == nil || len(rooms) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, activityTimeout)
	defer cancel()

	snapshot, err := h.activity.RoomActivity(ctx)
	if err != nil {
		slog.Warn("room list served without activity", "error", err)
		return
	}
	for _, rm := range rooms {
		if a, ok := snapshot[rm.ID]; ok {
			activity := a
			rm.Activity = &activity
		}
	}
}

// JoinRoom allows an authenticated user to join a room via ID.
func (h *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	if roomID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing room id")
		return
	}

	member, err := h.roomService.JoinRoom(r.Context(), roomID, session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to join room")
		return
	}

	respond.WriteJSON(w, http.StatusOK, member)
}

// GetRoom retrieves details of a room the user is actively participating in.
func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	rm, ok1 := middleware.GetRoomFromContext(r.Context())
	member, ok2 := middleware.GetRoomMemberFromContext(r.Context())
	if !ok1 || !ok2 {
		respond.WriteError(w, http.StatusInternalServerError, "missing room context")
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]any{
		"room":   rm,
		"member": member,
	})
}

// AssignRole handles promoting/demoting or altering permissions of room members.
func (h *RoomHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	targetUserID := r.PathValue("user_id")
	if roomID == "" || targetUserID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing path parameters")
		return
	}

	var req assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	updated, err := h.roomService.AssignRole(r.Context(), roomID, session.UserID, targetUserID, req.Role, req.Permissions)
	if err != nil {
		if errors.Is(err, room.ErrPermissionDenied) || errors.Is(err, room.ErrCannotAlterOwner) {
			respond.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to update member role")
		return
	}

	respond.WriteJSON(w, http.StatusOK, updated)
}

// KickMember handles removing a participant from a room.
func (h *RoomHandler) KickMember(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	targetUserID := r.PathValue("user_id")

	err := h.roomService.KickMember(r.Context(), roomID, session.UserID, targetUserID)
	if err != nil {
		if errors.Is(err, room.ErrPermissionDenied) || errors.Is(err, room.ErrCannotAlterOwner) {
			respond.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to kick room member")
		return
	}

	// A moderator can remove themselves this way; to them that is leaving.
	reason := ws.RemovalKicked
	if targetUserID == session.UserID {
		reason = ws.RemovalLeft
	}
	h.removeMember(roomID, targetUserID, reason)

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "member kicked successfully"})
}

type inviteUserRequest struct {
	Username string `json:"username"`
}

// InviteUser handles inviting a user by username to a room.
func (h *RoomHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	var req inviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		respond.WriteError(w, http.StatusBadRequest, "valid username is required")
		return
	}

	inv, err := h.roomService.InviteUserByUsername(r.Context(), roomID, session.UserID, req.Username)
	if err != nil {
		if errors.Is(err, room.ErrPermissionDenied) {
			respond.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusCreated, inv)
}

// ListInvitations handles listing pending invitations for the logged-in user.
func (h *RoomHandler) ListInvitations(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	invs, err := h.roomService.ListUserPendingInvitations(r.Context(), session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to list invitations")
		return
	}

	respond.WriteJSON(w, http.StatusOK, invs)
}

// AcceptInvitation handles accepting a room invitation.
func (h *RoomHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	invID := r.PathValue("id")
	rm, member, err := h.roomService.RespondToInvitation(r.Context(), invID, session.UserID, true)
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]any{
		"room":   rm,
		"member": member,
	})
}

// DeclineInvitation handles declining a room invitation.
func (h *RoomHandler) DeclineInvitation(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	invID := r.PathValue("id")
	_, _, err := h.roomService.RespondToInvitation(r.Context(), invID, session.UserID, false)
	if err != nil {
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

// LeaveRoom allows a non-owner member to permanently leave a room.
func (h *RoomHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	if roomID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing room id")
		return
	}

	if err := h.roomService.LeaveRoom(r.Context(), roomID, session.UserID); err != nil {
		if errors.Is(err, room.ErrCannotAlterOwner) {
			respond.WriteError(w, http.StatusBadRequest, "owners cannot leave the room, they must delete it")
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to leave room")
		return
	}
	// Their other tabs are still connected.
	h.removeMember(roomID, session.UserID, ws.RemovalLeft)

	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "left room successfully"})
}

// DeleteRoom handles room deletion by the owner.
func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	if err := h.roomService.DeleteRoom(r.Context(), roomID, session.UserID); err != nil {
		if errors.Is(err, room.ErrPermissionDenied) {
			respond.WriteError(w, http.StatusForbidden, err.Error())
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to delete room")
		return
	}

	h.closeRooms([]string{roomID})
	respond.WriteJSON(w, http.StatusOK, map[string]string{"message": "room deleted successfully"})
}

// AdminListRooms lists every room, public and private, with who is in it now.
func (h *RoomHandler) AdminListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.roomService.ListAllRooms(r.Context())
	if err != nil {
		slog.Error("admin room list failed", "error", err)
		respond.WriteError(w, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	h.attachActivity(r.Context(), rooms)
	respond.WriteJSON(w, http.StatusOK, rooms)
}

// AdminDeleteRoom deletes one room whoever owns it.
func (h *RoomHandler) AdminDeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := uuid.Parse(roomID); err != nil {
		respond.WriteError(w, http.StatusNotFound, "room not found")
		return
	}
	deleted, err := h.adminDelete(r.Context(), []string{roomID})
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to delete room")
		return
	}
	if len(deleted) == 0 {
		respond.WriteError(w, http.StatusNotFound, "room not found")
		return
	}
	respond.WriteJSON(w, http.StatusOK, map[string][]string{"deleted": deleted})
}

type adminDeleteRoomsRequest struct {
	RoomIDs []string `json:"room_ids"`
}

// AdminDeleteRooms deletes several rooms at once. IDs that match no room are
// skipped; the response lists the rooms that were deleted.
func (h *RoomHandler) AdminDeleteRooms(w http.ResponseWriter, r *http.Request) {
	var req adminDeleteRoomsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.RoomIDs) == 0 {
		respond.WriteError(w, http.StatusBadRequest, "room_ids is required")
		return
	}
	if len(req.RoomIDs) > maxAdminRoomDeletes {
		respond.WriteError(w, http.StatusBadRequest, fmt.Sprintf("at most %d rooms can be deleted at once", maxAdminRoomDeletes))
		return
	}
	ids := make([]string, 0, len(req.RoomIDs))
	seen := make(map[string]bool, len(req.RoomIDs))
	for _, id := range req.RoomIDs {
		if _, err := uuid.Parse(id); err != nil {
			respond.WriteError(w, http.StatusBadRequest, fmt.Sprintf("%q is not a room ID", id))
			return
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	deleted, err := h.adminDelete(r.Context(), ids)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to delete rooms")
		return
	}
	respond.WriteJSON(w, http.StatusOK, map[string][]string{"deleted": deleted})
}

func (h *RoomHandler) adminDelete(ctx context.Context, roomIDs []string) ([]string, error) {
	deleted, err := h.roomService.DeleteRoomsAsAdmin(ctx, roomIDs)
	if err != nil {
		slog.Error("admin room delete failed", "error", err, "rooms", len(roomIDs))
		return nil, err
	}
	var adminID string
	if session, ok := middleware.GetSessionFromContext(ctx); ok {
		adminID = session.UserID
	}
	slog.Info("rooms deleted by admin", "admin_id", adminID, "rooms", deleted)
	h.closeRooms(deleted)
	return deleted, nil
}
