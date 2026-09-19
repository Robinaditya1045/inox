package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/live"
)

// LiveHandler exposes live channel administration and the streaming proxy.
type LiveHandler struct {
	service live.Service
	proxy   *live.Proxy
}

// NewLiveHandler constructs the live channel controller.
func NewLiveHandler(service live.Service, proxy *live.Proxy) *LiveHandler {
	return &LiveHandler{service: service, proxy: proxy}
}

// List returns every configured live channel. Upstream URLs and headers are omitted
// by the domain type's JSON tags, so an operator listing channels never sees the
// credential behind them.
func (h *LiveHandler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.List(r.Context())
	if err != nil {
		slog.Error("failed to list live channels", "error", err)
		respond.WriteError(w, http.StatusInternalServerError, "failed to list live channels")
		return
	}
	respond.WriteJSON(w, http.StatusOK, map[string]any{
		"channels":  channels,
		"resolvers": h.service.ResolverIDs(),
	})
}

// Create registers a new live channel and its companion media library entry.
func (h *LiveHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req live.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var createdBy *string
	if session, ok := middleware.GetSessionFromContext(r.Context()); ok {
		createdBy = &session.UserID
	}

	channel, err := h.service.Create(r.Context(), req, createdBy)
	if err != nil {
		// Every failure here is something the operator typed, so return the reason
		// rather than a generic 500 they cannot act on.
		respond.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	respond.WriteJSON(w, http.StatusCreated, channel)
}

// TestResolve runs a resolver against an unsaved configuration and reports the
// stream's actual shape, so a scraped source can be verified before it is committed.
func (h *LiveHandler) TestResolve(w http.ResponseWriter, r *http.Request) {
	var req live.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respond.WriteJSON(w, http.StatusOK, h.service.TestResolve(r.Context(), req))
}

// Delete removes a live channel. The companion media asset cascades with it.
func (h *LiveHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing channel id")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		if errors.Is(err, live.ErrChannelNotFound) {
			respond.WriteError(w, http.StatusNotFound, "live channel not found")
			return
		}
		slog.Error("failed to delete live channel", "id", id, "error", err)
		respond.WriteError(w, http.StatusInternalServerError, "failed to delete live channel")
		return
	}
	respond.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ServeMaster serves a channel's master playlist. This is the only live route that
// requires a session: it mints a playback token and writes it into every URI of the
// manifest, so the segment requests that follow carry a credential scoped to this
// one channel instead of the caller's session ID.
func (h *LiveHandler) ServeMaster(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		respond.WriteError(w, http.StatusBadRequest, "missing channel slug")
		return
	}
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	h.proxy.ServeMaster(w, r, slug, session.UserID)
}

// ServePlaylist serves a variant or media playlist, authorized by playback token.
func (h *LiveHandler) ServePlaylist(w http.ResponseWriter, r *http.Request) {
	slug, ref, token, ok := liveRefParams(w, r)
	if !ok {
		return
	}
	h.proxy.ServePlaylist(w, r, slug, ref, token)
}

// ServeResource serves a segment, key, or init file, authorized by playback token.
func (h *LiveHandler) ServeResource(w http.ResponseWriter, r *http.Request) {
	slug, ref, token, ok := liveRefParams(w, r)
	if !ok {
		return
	}
	h.proxy.ServeResource(w, r, slug, ref, token)
}

// liveRefParams pulls the channel slug, sealed upstream reference, and playback
// token out of a proxy URL.
func liveRefParams(w http.ResponseWriter, r *http.Request) (slug, ref, token string, ok bool) {
	slug = r.PathValue("slug")
	ref = r.PathValue("ref")
	token = r.URL.Query().Get("t")
	if slug == "" || ref == "" || token == "" {
		respond.WriteError(w, http.StatusBadRequest, "malformed live stream request")
		return "", "", "", false
	}
	// Nested playlists keep an .m3u8 suffix because hls.js and Safari both branch on
	// the extension in places; it is not part of the sealed reference.
	ref = strings.TrimSuffix(ref, ".m3u8")
	return slug, ref, token, true
}
