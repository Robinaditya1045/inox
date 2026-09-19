package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/friend"
)

type FriendHandler struct {
	friendService friend.Service
}

func NewFriendHandler(friendService friend.Service) *FriendHandler {
	return &FriendHandler{friendService: friendService}
}

type sendFriendRequestBody struct {
	Username string `json:"username"`
}

// ListFriends returns the caller's accepted friends.
func (h *FriendHandler) ListFriends(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	friends, err := h.friendService.ListFriends(r.Context(), session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to list friends")
		return
	}

	respond.WriteJSON(w, http.StatusOK, friends)
}

// ListRequests returns the caller's pending requests, split by direction.
func (h *FriendHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	incoming, outgoing, err := h.friendService.ListRequests(r.Context(), session.UserID)
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to list friend requests")
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]any{
		"incoming": incoming,
		"outgoing": outgoing,
	})
}

// SendRequest sends a friend request to a user identified by username.
func (h *FriendHandler) SendRequest(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body sendFriendRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	outcome, err := h.friendService.SendRequest(r.Context(), session.UserID, body.Username)
	if err != nil {
		writeFriendError(w, err)
		return
	}

	respond.WriteJSON(w, http.StatusCreated, outcome)
}

// AcceptRequest accepts an incoming friend request.
func (h *FriendHandler) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	newFriend, err := h.friendService.RespondToRequest(r.Context(), r.PathValue("id"), session.UserID, true)
	if err != nil {
		writeFriendError(w, err)
		return
	}

	respond.WriteJSON(w, http.StatusOK, newFriend)
}

// DeclineRequest refuses an incoming friend request.
func (h *FriendHandler) DeclineRequest(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if _, err := h.friendService.RespondToRequest(r.Context(), r.PathValue("id"), session.UserID, false); err != nil {
		writeFriendError(w, err)
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

// CancelRequest withdraws a request the caller sent.
func (h *FriendHandler) CancelRequest(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.friendService.CancelRequest(r.Context(), r.PathValue("id"), session.UserID); err != nil {
		writeFriendError(w, err)
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// RemoveFriend unfriends the user named in the path.
func (h *FriendHandler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.friendService.RemoveFriend(r.Context(), session.UserID, r.PathValue("user_id")); err != nil {
		writeFriendError(w, err)
		return
	}

	respond.WriteJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// SearchUsers backs the add-friend picker's type-ahead.
func (h *FriendHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	results, err := h.friendService.SearchUsers(r.Context(), session.UserID, r.URL.Query().Get("q"))
	if err != nil {
		respond.WriteError(w, http.StatusInternalServerError, "failed to search users")
		return
	}

	respond.WriteJSON(w, http.StatusOK, results)
}

// writeFriendError maps service errors onto status codes in one place, so every
// friend route reports the same failure the same way.
func writeFriendError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, friend.ErrUserNotFound), errors.Is(err, friend.ErrRequestNotFound):
		respond.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, friend.ErrNotYourRequest):
		respond.WriteError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, friend.ErrSelfFriendship), errors.Is(err, friend.ErrInvalidUsername):
		respond.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, friend.ErrAlreadyFriends),
		errors.Is(err, friend.ErrRequestAlreadySent),
		errors.Is(err, friend.ErrNotFriends):
		respond.WriteError(w, http.StatusConflict, err.Error())
	default:
		respond.WriteError(w, http.StatusInternalServerError, "friend request failed")
	}
}
