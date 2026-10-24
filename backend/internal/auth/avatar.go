package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
)

var (
	// ErrUnsupportedAvatar rejects an upload that is not a web image format.
	ErrUnsupportedAvatar = errors.New("avatar must be a JPEG, PNG or WebP image")
	// ErrInvalidAvatarURL rejects a linked avatar that is not an http(s) URL.
	ErrInvalidAvatarURL = errors.New("avatar link must be an http or https URL")
)

// avatarPrefix is the storage folder uploaded avatars live under. Each upload
// gets a fresh key, so the stream proxy can cache them indefinitely.
const avatarPrefix = "avatars/"

// maxAvatarURLLength matches the users.avatar_url column.
const maxAvatarURLLength = 1024

var avatarExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// FileStore is the part of storage.Service avatars need.
type FileStore interface {
	SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error)
	DeleteFile(ctx context.Context, key string) error
}

// AvatarService sets, replaces and removes profile pictures, whether uploaded
// or linked. Whichever way an avatar is replaced, an uploaded one it replaces
// is purged from storage.
type AvatarService struct {
	users      UserRepository
	sessions   SessionStore
	files      FileStore
	publicBase string
}

// NewAvatarService builds an AvatarService. publicBase is the stream proxy's
// public URL (MEDIA_STREAM_BASE_URL): avatar URLs are handed straight to
// browsers by many endpoints, so they are stored already browser-reachable
// rather than as the storage backend's internal address.
func NewAvatarService(users UserRepository, sessions SessionStore, files FileStore, publicBase string) *AvatarService {
	return &AvatarService{
		users:      users,
		sessions:   sessions,
		files:      files,
		publicBase: strings.TrimSuffix(publicBase, "/"),
	}
}

// Upload stores an image as the user's avatar and returns its URL. The
// content type must be sniffed from the bytes, not taken from the client.
func (s *AvatarService) Upload(ctx context.Context, userID, sessionID string, reader io.Reader, size int64, contentType string) (string, error) {
	ext, ok := avatarExtensions[contentType]
	if !ok {
		return "", ErrUnsupportedAvatar
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate avatar key: %w", err)
	}
	key := fmt.Sprintf("%s%s-%s%s", avatarPrefix, userID, hex.EncodeToString(suffix), ext)
	if _, err := s.files.SaveFile(ctx, key, reader, size, contentType); err != nil {
		return "", fmt.Errorf("storage save failed: %w", err)
	}

	avatarURL := s.publicBase + "/" + key
	if err := s.replace(ctx, userID, sessionID, &avatarURL); err != nil {
		s.deleteKey(ctx, key)
		return "", err
	}
	return avatarURL, nil
}

// SetLink points the user's avatar at an image hosted elsewhere. The link is
// stored as-is and never fetched.
func (s *AvatarService) SetLink(ctx context.Context, userID, sessionID, link string) error {
	link = strings.TrimSpace(link)
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(link) > maxAvatarURLLength {
		return ErrInvalidAvatarURL
	}
	return s.replace(ctx, userID, sessionID, &link)
}

// Remove clears the user's avatar, so clients fall back to initials.
func (s *AvatarService) Remove(ctx context.Context, userID, sessionID string) error {
	return s.replace(ctx, userID, sessionID, nil)
}

func (s *AvatarService) replace(ctx context.Context, userID, sessionID string, avatarURL *string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	old := user.AvatarURL
	if err := s.users.UpdateAvatar(ctx, userID, avatarURL); err != nil {
		return err
	}

	// The session carries the avatar too; refresh the caller's so the next
	// request sees it without logging in again.
	if sessionID != "" {
		if session, err := s.sessions.Get(ctx, sessionID); err == nil {
			session.AvatarURL = avatarURL
			_ = s.sessions.Create(ctx, session)
		}
	}

	if old != nil && (avatarURL == nil || *old != *avatarURL) {
		if key, ok := s.uploadedKey(*old); ok {
			s.deleteKey(ctx, key)
		}
	}
	return nil
}

// uploadedKey returns the storage key of an avatar this service uploaded.
// Linked avatars, which live elsewhere, report false.
func (s *AvatarService) uploadedKey(avatarURL string) (string, bool) {
	key, ok := strings.CutPrefix(avatarURL, s.publicBase+"/")
	if !ok || !strings.HasPrefix(key, avatarPrefix) {
		return "", false
	}
	return key, true
}

func (s *AvatarService) deleteKey(ctx context.Context, key string) {
	if err := s.files.DeleteFile(ctx, key); err != nil {
		slog.Warn("failed to delete replaced avatar", "key", key, "error", err)
	}
}
