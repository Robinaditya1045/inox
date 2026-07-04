package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/room"
)

type ChatHandler struct {
	chatService room.ChatService
}

// NewChatHandler initializes a controller for chat history retrieval.
func NewChatHandler(chatService room.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// GetRecentMessages handles GET /api/v1/rooms/{id}/messages to retrieve chronological room chat history.
func (h *ChatHandler) GetRecentMessages(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	roomID := r.PathValue("id")
	if roomID == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing room id parameter")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	messages, err := h.chatService.GetRecentMessages(r.Context(), roomID, session.UserID, limit)
	if err != nil {
		if errors.Is(err, room.ErrMemberNotFound) {
			respond.WriteError(w, http.StatusForbidden, "you must be a member of this room to view chat history")
			return
		}
		respond.WriteError(w, http.StatusInternalServerError, "failed to retrieve chat history")
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]any{
		"room_id":  roomID,
		"messages": messages,
		"count":    len(messages),
	})
}
