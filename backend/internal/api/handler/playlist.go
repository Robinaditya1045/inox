package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/playlist"
)

// PlaylistHandler exposes the caller's own playlists.
type PlaylistHandler struct {
	service playlist.Service
	// normalize rewrites stored media URLs for the browser, as the media library
	// endpoints do; playlist items embed the same assets.
	normalize func(*domain.MediaAsset) *domain.MediaAsset
}

// NewPlaylistHandler initializes the playlist controller. normalize may be nil.
func NewPlaylistHandler(service playlist.Service, normalize func(*domain.MediaAsset) *domain.MediaAsset) *PlaylistHandler {
	return &PlaylistHandler{service: service, normalize: normalize}
}

type playlistDetailsBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type addPlaylistItemBody struct {
	MediaAssetID string `json:"media_asset_id"`
}

type reorderPlaylistBody struct {
	ItemIDs []string `json:"item_ids"`
}

// List returns the caller's playlists, most recently edited first.
func (h *PlaylistHandler) List(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	playlists, err := h.service.List(r.Context(), session.UserID)
	if err != nil {
		h.writeError(w, err)
		return
	}
	for _, p := range playlists {
		h.normalizePlaylist(p)
	}
	respond.WriteJSON(w, http.StatusOK, playlists)
}

// Get returns one of the caller's playlists.
func (h *PlaylistHandler) Get(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	p, err := h.service.Get(r.Context(), r.PathValue("id"), session.UserID)
	h.writePlaylist(w, http.StatusOK, p, err)
}

// Create makes a new, empty playlist.
func (h *PlaylistHandler) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body playlistDetailsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	p, err := h.service.Create(r.Context(), session.UserID, body.Name, body.Description)
	h.writePlaylist(w, http.StatusCreated, p, err)
}

// Update renames a playlist or changes its description.
func (h *PlaylistHandler) Update(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body playlistDetailsBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	p, err := h.service.Update(r.Context(), r.PathValue("id"), session.UserID, body.Name, body.Description)
	h.writePlaylist(w, http.StatusOK, p, err)
}

// Delete removes a playlist. The videos in it stay in the library.
func (h *PlaylistHandler) Delete(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := h.service.Delete(r.Context(), r.PathValue("id"), session.UserID); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddItem appends a library video to a playlist.
func (h *PlaylistHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body addPlaylistItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MediaAssetID == "" {
		respond.WriteError(w, http.StatusBadRequest, "media_asset_id is required")
		return
	}
	p, err := h.service.AddItem(r.Context(), r.PathValue("id"), session.UserID, body.MediaAssetID)
	h.writePlaylist(w, http.StatusOK, p, err)
}

// RemoveItem takes one video out of a playlist.
func (h *PlaylistHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	p, err := h.service.RemoveItem(r.Context(), r.PathValue("id"), session.UserID, r.PathValue("item_id"))
	h.writePlaylist(w, http.StatusOK, p, err)
}

// Reorder sets the order of a playlist's videos.
func (h *PlaylistHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body reorderPlaylistBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	p, err := h.service.Reorder(r.Context(), r.PathValue("id"), session.UserID, body.ItemIDs)
	h.writePlaylist(w, http.StatusOK, p, err)
}

func (h *PlaylistHandler) writePlaylist(w http.ResponseWriter, status int, p *domain.Playlist, err error) {
	if err != nil {
		h.writeError(w, err)
		return
	}
	respond.WriteJSON(w, status, h.normalizePlaylist(p))
}

func (h *PlaylistHandler) normalizePlaylist(p *domain.Playlist) *domain.Playlist {
	if h.normalize == nil || p == nil {
		return p
	}
	for _, item := range p.Items {
		item.Media = h.normalize(item.Media)
	}
	return p
}

func (h *PlaylistHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, playlist.ErrPlaylistNotFound),
		errors.Is(err, playlist.ErrItemNotFound),
		errors.Is(err, playlist.ErrMediaNotFound):
		respond.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, playlist.ErrInvalidName),
		errors.Is(err, playlist.ErrDescriptionTooLong),
		errors.Is(err, playlist.ErrMediaNotPlayable),
		errors.Is(err, playlist.ErrInvalidOrder):
		respond.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, playlist.ErrDuplicateItem),
		errors.Is(err, playlist.ErrPlaylistFull),
		errors.Is(err, playlist.ErrTooManyPlaylists):
		respond.WriteError(w, http.StatusConflict, err.Error())
	default:
		respond.WriteError(w, http.StatusInternalServerError, "playlist request failed")
	}
}
