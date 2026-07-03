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
