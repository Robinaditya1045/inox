package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/room"
)

type RoomHandler struct {
	roomService room.RoomService
}

func NewRoomHandler(roomService room.RoomService) *RoomHandler {
	return &RoomHandler{roomService: roomService}
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

	respond.WriteJSON(w, http.StatusOK, rooms)
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
