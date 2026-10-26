package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/api/respond"
	"github.com/inox/inox/backend/internal/community"
	"github.com/inox/inox/backend/internal/domain"
)

// CommunityHandler exposes platform update posts: the feed every viewer reads
// and the admin routes that write it.
type CommunityHandler struct {
	service community.Service
}

// NewCommunityHandler initializes the community post controller.
func NewCommunityHandler(service community.Service) *CommunityHandler {
	return &CommunityHandler{service: service}
}

type communityPostBody struct {
	Category  string `json:"category"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Pinned    bool   `json:"pinned"`
	Published bool   `json:"published"`
}

func (b communityPostBody) input() community.PostInput {
	return community.PostInput{
		Category:  b.Category,
		Title:     b.Title,
		Body:      b.Body,
		Pinned:    b.Pinned,
		Published: b.Published,
	}
}

type communityFeedResponse struct {
	Posts   []*domain.CommunityPost `json:"posts"`
	HasMore bool                    `json:"has_more"`
}

// Feed returns a page of published posts, pinned first and then newest first.
// Takes optional ?category=, ?limit= and ?offset=.
func (h *CommunityHandler) Feed(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	posts, hasMore, err := h.service.Feed(r.Context(), r.URL.Query().Get("category"), limit, offset)
	if err != nil {
		h.writeError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, communityFeedResponse{Posts: posts, HasMore: hasMore})
}

// AdminList returns every post, drafts first.
func (h *CommunityHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	posts, err := h.service.AdminList(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, posts)
}

// Create writes a new post, published or as a draft, authored by the caller.
func (h *CommunityHandler) Create(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		respond.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var body communityPostBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	p, err := h.service.Create(r.Context(), session.UserID, body.input())
	if err != nil {
		h.writeError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusCreated, p)
}

// Update replaces a post's content and state. Every field is sent each time.
func (h *CommunityHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body communityPostBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.WriteError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	p, err := h.service.Update(r.Context(), r.PathValue("id"), body.input())
	if err != nil {
		h.writeError(w, err)
		return
	}
	respond.WriteJSON(w, http.StatusOK, p)
}

// Delete removes a post for good.
func (h *CommunityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(r.Context(), r.PathValue("id")); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CommunityHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, community.ErrPostNotFound):
		respond.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, community.ErrInvalidTitle),
		errors.Is(err, community.ErrInvalidBody),
		errors.Is(err, community.ErrInvalidCategory):
		respond.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		respond.WriteError(w, http.StatusInternalServerError, "community request failed")
	}
}
