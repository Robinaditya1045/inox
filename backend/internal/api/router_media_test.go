package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/media"
)

// fakeMediaService answers the two lookups these tests make; anything else
// panics on the nil embedded interface.
type fakeMediaService struct {
	media.Service
	gotURL string
}

func (f *fakeMediaService) CaptionsForMediaURL(_ context.Context, mediaURL string) ([]*domain.MediaCaption, error) {
	f.gotURL = mediaURL
	return []*domain.MediaCaption{{ID: "c1", Language: "en", Label: "English", URL: "http://localhost:9000/inox-media/captions/a/en-1.vtt"}}, nil
}

func (f *fakeMediaService) GetStreamFile(_ context.Context, key string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("object not found: " + key)
}

func newMediaRouter(t *testing.T, svc media.Service) http.Handler {
	t.Helper()
	// Registering with every media and admin route present is the point: the
	// mux panics on overlapping patterns, such as a /media/{id}/captions route
	// against the /media/stream/ prefix.
	return NewRouter(nil, auth.NewAuthService(nil, nil), nil, nil, nil, nil, &handler.AdminHandler{},
		handler.NewMediaHandler(svc, "https://cdn.example/media/stream"), &handler.LiveHandler{}, &handler.FriendHandler{}, &handler.PlaylistHandler{}, nil, nil)
}

func TestRouter_CaptionsLookupByMediaURL(t *testing.T) {
	svc := &fakeMediaService{}
	router := newMediaRouter(t, svc)

	req := httptest.NewRequest("GET", "/api/v1/media/captions?url="+"https%3A%2F%2Fcdn.example%2Fmedia%2Fstream%2Fhls%2Fa%2Fmaster.m3u8", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if svc.gotURL != "https://cdn.example/media/stream/hls/a/master.m3u8" {
		t.Errorf("service got url %q", svc.gotURL)
	}
	var tracks []domain.MediaCaption
	if err := json.NewDecoder(rec.Body).Decode(&tracks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tracks) != 1 || tracks[0].URL != "https://cdn.example/media/stream/captions/a/en-1.vtt" {
		t.Errorf("tracks = %+v, want one track with its URL moved onto the stream base", tracks)
	}
}

func TestRouter_StreamPrefixStillOwnsItsPaths(t *testing.T) {
	router := newMediaRouter(t, &fakeMediaService{})

	// Would reach the captions lookup if /media/{id}/... patterns had been
	// allowed to shadow the stream proxy.
	req := httptest.NewRequest("GET", "/api/v1/media/stream/captions/a/en-1.vtt", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("status = %d, ACAO = %q; want the stream proxy's 404", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestRouter_AdminCaptionRoutesRequireAuth(t *testing.T) {
	router := newMediaRouter(t, &fakeMediaService{})

	for _, tc := range []struct{ method, path string }{
		{"PATCH", "/api/v1/admin/media/a"},
		{"DELETE", "/api/v1/admin/media/a/thumbnail"},
		{"POST", "/api/v1/admin/media/a/captions"},
		{"PUT", "/api/v1/admin/media/a/captions/c1"},
		{"DELETE", "/api/v1/admin/media/a/captions/c1"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}
