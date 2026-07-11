package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Service defines the abstraction for media file storage, enabling seamless migration
// between local development storage (MinIO/Filesystem) and production cloud buckets (AWS S3 / GCS).
type Service interface {
	SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error)
	GetFile(ctx context.Context, key string) (io.ReadCloser, string, error)
	GetStreamURL(ctx context.Context, key string) (string, error)
	DeleteFile(ctx context.Context, key string) error
	GeneratePresignedURL(ctx context.Context, key string, contentType string, maxSizeBytes int64) (string, error)
}

// LocalStorage implements Service using the local filesystem, serving as a zero-dependency
// drop-in replacement for MinIO/S3 during development and offline environments.
type LocalStorage struct {
	baseDir   string
	publicURL string
}

// NewLocalStorage initializes a local storage engine.
func NewLocalStorage(baseDir, publicURL string) (*LocalStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage base directory: %w", err)
	}
	return &LocalStorage{
		baseDir:   baseDir,
		publicURL: publicURL,
	}, nil
}

// SaveFile writes an object to the storage bucket directory.
func (s *LocalStorage) SaveFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	fullPath := filepath.Join(s.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directory for object %s: %w", key, err)
	}

	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file for writing %s: %w", key, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return "", fmt.Errorf("failed to write object data %s: %w", key, err)
	}

	url := fmt.Sprintf("%s/%s", s.publicURL, key)
	return url, nil
}

// GetFile opens a stored object for reading.
func (s *LocalStorage) GetFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	fullPath := filepath.Join(s.baseDir, key)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("object not found: %s", key)
		}
		return nil, "", fmt.Errorf("failed to open object %s: %w", key, err)
	}

	// Simple MIME type detection by extension
	contentType := "application/octet-stream"
	switch filepath.Ext(key) {
	case ".mp4":
		contentType = "video/mp4"
	case ".m3u8":
		contentType = "application/vnd.apple.mpegurl"
	case ".ts":
		contentType = "video/mp2t"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	}

	return file, contentType, nil
}

// GetStreamURL returns the public HTTP URL for streaming the object.
func (s *LocalStorage) GetStreamURL(ctx context.Context, key string) (string, error) {
	return fmt.Sprintf("%s/%s", s.publicURL, key), nil
}

// DeleteFile removes an object from storage.
func (s *LocalStorage) DeleteFile(ctx context.Context, key string) error {
	fullPath := filepath.Join(s.baseDir, key)
	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove object %s: %w", key, err)
	}
	return nil
}

// GeneratePresignedURL generates a direct HTTP PUT URL for uploading large files (up to 3GB) directly to storage.
func (s *LocalStorage) GeneratePresignedURL(ctx context.Context, key string, contentType string, maxSizeBytes int64) (string, error) {
	// In local dev/offline mode, return a direct PUT endpoint URL on the storage server
	url := fmt.Sprintf("%s/upload-direct?key=%s&max_size=%d", s.publicURL, key, maxSizeBytes)
	return url, nil
}
