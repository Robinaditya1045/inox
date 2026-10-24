package auth_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/inox/inox/backend/internal/auth"
	"github.com/inox/inox/backend/internal/domain"
)

// memoryFileStore implements auth.FileStore in memory.
type memoryFileStore struct {
	files   map[string][]byte
	deleted []string
}

func newMemoryFileStore() *memoryFileStore {
	return &memoryFileStore{files: make(map[string][]byte)}
}

func (m *memoryFileStore) SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	m.files[key] = data
	// Like MinIO, hand back an internal address the service must not store.
	return "http://localhost:9000/inox-media/" + key, nil
}

func (m *memoryFileStore) DeleteFile(ctx context.Context, key string) error {
	delete(m.files, key)
	m.deleted = append(m.deleted, key)
	return nil
}

const testStreamBase = "https://api.inox.test/media/stream"

type avatarFixture struct {
	svc      *auth.AvatarService
	users    *mockUserRepository
	sessions *mockSessionStore
	files    *memoryFileStore
	user     *domain.User
}

func newAvatarFixture(t *testing.T) *avatarFixture {
	t.Helper()
	users := newMockUserRepository()
	user := &domain.User{ID: "user-1", Username: "robin", Email: "robin@inox.com"}
	users.users[user.Email] = user
	sessions := newMockSessionStore()
	sessions.sessions["sess-1"] = &domain.Session{ID: "sess-1", UserID: user.ID, ExpiresAt: time.Now().Add(time.Hour)}
	files := newMemoryFileStore()
	return &avatarFixture{
		svc:      auth.NewAvatarService(users, sessions, files, testStreamBase+"/"),
		users:    users,
		sessions: sessions,
		files:    files,
		user:     user,
	}
}

func (f *avatarFixture) upload(t *testing.T) string {
	t.Helper()
	url, err := f.svc.Upload(context.Background(), f.user.ID, "sess-1", strings.NewReader("jpeg bytes"), 10, "image/jpeg")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	return url
}

func TestAvatarUploadStoresBrowserReachableURL(t *testing.T) {
	f := newAvatarFixture(t)

	url := f.upload(t)

	key, ok := strings.CutPrefix(url, testStreamBase+"/avatars/")
	if !ok || !strings.HasPrefix(key, "user-1-") || !strings.HasSuffix(key, ".jpg") {
		t.Fatalf("expected a stream-proxy URL under avatars/, got %q", url)
	}
	if _, ok := f.files.files["avatars/"+key]; !ok {
		t.Fatalf("expected the image to be stored under avatars/%s, have %v", key, f.files.files)
	}
	if f.user.AvatarURL == nil || *f.user.AvatarURL != url {
		t.Errorf("expected the user's avatar to be %q, got %v", url, f.user.AvatarURL)
	}
	if got := f.sessions.sessions["sess-1"].AvatarURL; got == nil || *got != url {
		t.Errorf("expected the caller's session to carry the new avatar, got %v", got)
	}
}

func TestAvatarUploadRejectsNonImages(t *testing.T) {
	f := newAvatarFixture(t)

	for _, contentType := range []string{"text/html; charset=utf-8", "image/svg+xml", "application/octet-stream"} {
		_, err := f.svc.Upload(context.Background(), f.user.ID, "sess-1", strings.NewReader("x"), 1, contentType)
		if !errors.Is(err, auth.ErrUnsupportedAvatar) {
			t.Errorf("%s: expected ErrUnsupportedAvatar, got %v", contentType, err)
		}
	}
	if len(f.files.files) != 0 {
		t.Errorf("rejected uploads must not be stored, have %v", f.files.files)
	}
}

// Every upload gets a fresh key so the proxy can cache avatars forever; the
// one it replaces would otherwise be left in storage for good.
func TestAvatarReplacementPurgesThePreviousUpload(t *testing.T) {
	f := newAvatarFixture(t)
	first := f.upload(t)
	firstKey := strings.TrimPrefix(first, testStreamBase+"/")

	second := f.upload(t)
	if second == first {
		t.Fatalf("expected a fresh key per upload, got %q twice", first)
	}
	if _, ok := f.files.files[firstKey]; ok {
		t.Errorf("expected %s to be deleted after replacement", firstKey)
	}

	secondKey := strings.TrimPrefix(second, testStreamBase+"/")
	if err := f.svc.SetLink(context.Background(), f.user.ID, "sess-1", "https://cdn.example/me.png"); err != nil {
		t.Fatalf("set link failed: %v", err)
	}
	if _, ok := f.files.files[secondKey]; ok {
		t.Errorf("expected %s to be deleted after switching to a link", secondKey)
	}

	if err := f.svc.Remove(context.Background(), f.user.ID, "sess-1"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if f.user.AvatarURL != nil {
		t.Errorf("expected the avatar to be cleared, got %q", *f.user.AvatarURL)
	}
	if got := f.sessions.sessions["sess-1"].AvatarURL; got != nil {
		t.Errorf("expected the session's avatar to be cleared, got %q", *got)
	}
	// The linked image lives elsewhere and must never be deleted from our bucket.
	if len(f.files.deleted) != 2 {
		t.Errorf("expected exactly the two uploads to be deleted, got %v", f.files.deleted)
	}
}

func TestAvatarLinkMustBeHTTP(t *testing.T) {
	f := newAvatarFixture(t)

	for _, link := range []string{"", "javascript:alert(1)", "/avatars/x.png", "data:image/png;base64,AAAA", "https://" + strings.Repeat("a", 1100)} {
		if err := f.svc.SetLink(context.Background(), f.user.ID, "sess-1", link); !errors.Is(err, auth.ErrInvalidAvatarURL) {
			t.Errorf("%.40q: expected ErrInvalidAvatarURL, got %v", link, err)
		}
	}
	if f.user.AvatarURL != nil {
		t.Errorf("rejected links must not be saved, got %q", *f.user.AvatarURL)
	}
}

func TestAvatarUploadForUnknownUserLeavesNoFile(t *testing.T) {
	f := newAvatarFixture(t)

	_, err := f.svc.Upload(context.Background(), "ghost", "", strings.NewReader("x"), 1, "image/png")
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if len(f.files.files) != 0 {
		t.Errorf("expected the orphaned upload to be deleted, have %v", f.files.files)
	}
}
