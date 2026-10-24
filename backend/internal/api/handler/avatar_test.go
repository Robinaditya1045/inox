package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inox/inox/backend/internal/api/handler"
	"github.com/inox/inox/backend/internal/api/middleware"
	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// avatarUsers stores one user's avatar; the embedded interface panics if the
// handler reaches for anything else.
type avatarUsers struct {
	auth.UserRepository
	user *domain.User
}

func (u *avatarUsers) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if id != u.user.ID {
		return nil, auth.ErrUserNotFound
	}
	return u.user, nil
}

func (u *avatarUsers) UpdateAvatar(ctx context.Context, userID string, avatarURL *string) error {
	u.user.AvatarURL = avatarURL
	return nil
}

type avatarSessions struct{ auth.SessionStore }

func (avatarSessions) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	return nil, auth.ErrSessionNotFound
}

type avatarFiles struct{ saved map[string]string }

func (f *avatarFiles) SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	f.saved[key] = contentType
	return key, nil
}

func (f *avatarFiles) DeleteFile(ctx context.Context, key string) error { return nil }

// pngHeader is enough of a PNG for http.DetectContentType.
var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func avatarUploadRequest(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(data)
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/users/profile/avatar", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req.WithContext(middleware.WithSessionContext(req.Context(), &domain.Session{ID: "sess-1", UserID: "user-1"}))
}

func newAvatarHandler() (*handler.AuthHandler, *avatarUsers, *avatarFiles) {
	users := &avatarUsers{user: &domain.User{ID: "user-1"}}
	files := &avatarFiles{saved: make(map[string]string)}
	h := handler.NewAuthHandler(newMockAuthService(), false)
	h.SetAvatarService(auth.NewAvatarService(users, avatarSessions{}, files, "https://api.inox.test/media/stream"))
	return h, users, files
}

func TestUploadAvatarHTTPHandler(t *testing.T) {
	h, users, files := newAvatarHandler()

	// The filename claims HTML; the bytes are what decide.
	rec := httptest.NewRecorder()
	h.UploadAvatar(rec, avatarUploadRequest(t, "me.html", pngHeader))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !strings.HasPrefix(body.AvatarURL, "https://api.inox.test/media/stream/avatars/user-1-") || !strings.HasSuffix(body.AvatarURL, ".png") {
		t.Errorf("unexpected avatar URL %q", body.AvatarURL)
	}
	if users.user.AvatarURL == nil || *users.user.AvatarURL != body.AvatarURL {
		t.Errorf("expected the stored avatar to match the response, got %v", users.user.AvatarURL)
	}
	for key, contentType := range files.saved {
		if contentType != "image/png" {
			t.Errorf("%s stored as %q, want the sniffed image/png", key, contentType)
		}
	}
}

func TestUploadAvatarRejectsNonImages(t *testing.T) {
	h, users, files := newAvatarHandler()

	rec := httptest.NewRecorder()
	h.UploadAvatar(rec, avatarUploadRequest(t, "me.png", []byte("<html><script>alert(1)</script></html>")))

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(files.saved) != 0 || users.user.AvatarURL != nil {
		t.Errorf("a rejected upload must change nothing (saved %v, avatar %v)", files.saved, users.user.AvatarURL)
	}
}

func TestRemoveAvatarHTTPHandler(t *testing.T) {
	h, users, _ := newAvatarHandler()
	link := "https://cdn.example/me.png"
	users.user.AvatarURL = &link

	req := httptest.NewRequest("DELETE", "/api/v1/users/profile/avatar", nil)
	req = req.WithContext(middleware.WithSessionContext(req.Context(), &domain.Session{ID: "sess-1", UserID: "user-1"}))
	rec := httptest.NewRecorder()
	h.RemoveAvatar(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if users.user.AvatarURL != nil {
		t.Errorf("expected the avatar to be cleared, got %q", *users.user.AvatarURL)
	}
}
