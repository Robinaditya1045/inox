package media

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/storage"
)

// Service defines business logic for managing the media library, uploading assets, and streaming HLS renditions.
type Service interface {
	RegisterExternalMedia(ctx context.Context, title, description, sourceURL, thumbnailURL string, createdBy *string) (*domain.MediaAsset, error)
	UploadMedia(ctx context.Context, title, description string, reader io.Reader, size int64, contentType string, createdBy *string) (*domain.MediaAsset, error)
	CreatePresignedUpload(ctx context.Context, title, description, filename, contentType string, size int64, createdBy *string) (*domain.MediaAsset, string, string, error)
	CompleteDirectUpload(ctx context.Context, assetID string, key string) (*domain.MediaAsset, error)
	GetMediaByID(ctx context.Context, id string) (*domain.MediaAsset, error)
	ListMedia(ctx context.Context, limit, offset int) ([]*domain.MediaAsset, error)
	DeleteMedia(ctx context.Context, id string) error
	GetStreamFile(ctx context.Context, key string) (io.ReadCloser, string, error)
	SaveDirectLocalFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error)
	ReconcileOrphanedAssets(ctx context.Context)
}

type mediaService struct {
	repo      Repository
	storage   storage.Service
	processor *Processor
}

// NewService initializes the media library business service.
func NewService(repo Repository, storage storage.Service, processor *Processor) Service {
	return &mediaService{
		repo:      repo,
		storage:   storage,
		processor: processor,
	}
}

// RegisterExternalMedia catalogs an existing HTTP media link or HLS manifest without uploading.
func (s *mediaService) RegisterExternalMedia(ctx context.Context, title, description, sourceURL, thumbnailURL string, createdBy *string) (*domain.MediaAsset, error) {
	asset := &domain.MediaAsset{
		Title:        title,
		Description:  description,
		SourceURL:    sourceURL,
		Status:       domain.MediaStatusReady,
		ThumbnailURL: thumbnailURL,
		HLSMasterURL: sourceURL, // Direct stream
		CreatedBy:    createdBy,
	}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to register external media: %w", err)
	}
	return asset, nil
}

// UploadMedia saves an uploaded video file to object storage and triggers background HLS processing.
func (s *mediaService) UploadMedia(ctx context.Context, title, description string, reader io.Reader, size int64, contentType string, createdBy *string) (*domain.MediaAsset, error) {
	// Generate unique storage key
	ext := ".mp4"
	if contentType == "application/vnd.apple.mpegurl" || contentType == "audio/x-mpegurl" {
		ext = ".m3u8"
	}
	key := fmt.Sprintf("raw/%s%s", generateUUID(), ext)

	slog.Info("uploading file to object storage", "key", key, "size", size)
	url, err := s.storage.SaveFile(ctx, key, reader, size, contentType)
	if err != nil {
		return nil, fmt.Errorf("storage save failed: %w", err)
	}

	asset := &domain.MediaAsset{
		Title:       title,
		Description: description,
		SourceURL:   url,
		Status:      domain.MediaStatusPending,
		CreatedBy:   createdBy,
	}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to save asset in database: %w", err)
	}

	// Trigger asynchronous HLS transcoding ladder generation
	if s.processor != nil {
		go s.processor.ProcessAsset(asset.ID, key, url)
	} else {
		// If processor not wired, mark as ready for direct playback
		_ = s.repo.UpdateAssetStatus(context.Background(), asset.ID, domain.MediaStatusReady, url)
	}

	return asset, nil
}

// CreatePresignedUpload generates a direct storage PUT URL for files up to 3GB, bypassing server RAM buffering.
func (s *mediaService) CreatePresignedUpload(ctx context.Context, title, description, filename, contentType string, size int64, createdBy *string) (*domain.MediaAsset, string, string, error) {
	if size > 3<<30 { // 3GB limit (3,221,225,472 bytes)
		return nil, "", "", fmt.Errorf("file size %d exceeds 3GB direct upload limit", size)
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".mp4"
	}
	key := fmt.Sprintf("raw/%s%s", generateUUID(), ext)

	uploadURL, err := s.storage.GeneratePresignedURL(ctx, key, contentType, 3<<30)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to generate presigned upload url: %w", err)
	}

	streamURL, err := s.storage.GetStreamURL(ctx, key)
	if err != nil {
		streamURL = key
	}

	asset := &domain.MediaAsset{
		Title:       title,
		Description: description,
		SourceURL:   streamURL,
		Status:      domain.MediaStatusPending,
		CreatedBy:   createdBy,
	}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		return nil, "", "", fmt.Errorf("failed to create asset in database: %w", err)
	}

	return asset, uploadURL, key, nil
}

// CompleteDirectUpload confirms a direct storage upload has finished and initiates HLS transcoding.
func (s *mediaService) CompleteDirectUpload(ctx context.Context, assetID string, key string) (*domain.MediaAsset, error) {
	asset, err := s.repo.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}

	streamURL, err := s.storage.GetStreamURL(ctx, key)
	if err != nil {
		streamURL = key
	}

	if err := s.repo.UpdateAssetStatus(ctx, assetID, domain.MediaStatusProcessing, streamURL); err != nil {
		return nil, fmt.Errorf("failed to update asset status: %w", err)
	}

	if s.processor != nil {
		go s.processor.ProcessAsset(asset.ID, key, streamURL)
	} else {
		_ = s.repo.UpdateAssetStatus(context.Background(), asset.ID, domain.MediaStatusReady, streamURL)
	}

	return s.repo.GetAssetByID(ctx, assetID)
}

// GetMediaByID retrieves an asset and its multi-bitrate HLS renditions.
func (s *mediaService) GetMediaByID(ctx context.Context, id string) (*domain.MediaAsset, error) {
	return s.repo.GetAssetByID(ctx, id)
}

// ListMedia returns paginated media library items.
func (s *mediaService) ListMedia(ctx context.Context, limit, offset int) ([]*domain.MediaAsset, error) {
	return s.repo.ListAssets(ctx, limit, offset)
}

// DeleteMedia removes an asset from PostgreSQL and purges its stored objects.
func (s *mediaService) DeleteMedia(ctx context.Context, id string) error {
	asset, err := s.repo.GetAssetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteAsset(ctx, id); err != nil {
		return err
	}
	// Attempt to delete stored object from MinIO / local storage
	_ = s.storage.DeleteFile(ctx, filepath.Base(asset.SourceURL))
	return nil
}

// GetStreamFile retrieves an object reader for streaming video segments or manifests.
func (s *mediaService) GetStreamFile(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return s.storage.GetFile(ctx, key)
}

// SaveDirectLocalFile saves a file streamed directly via PUT in local/offline storage mode.
func (s *mediaService) SaveDirectLocalFile(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	return s.storage.SaveFile(ctx, key, reader, size, contentType)
}

// ReconcileOrphanedAssets detects assets stuck in 'pending' or 'processing' states after a server
// restart or crash. If their source file is available in storage, it automatically resumes HLS
// transcoding. Otherwise, it cleanly marks them 'failed' so the UI does not show false transcode states.
func (s *mediaService) ReconcileOrphanedAssets(ctx context.Context) {
	slog.Info("reconciling orphaned or interrupted media assets on startup...")
	assets, err := s.repo.GetAssetsByStatus(ctx, []domain.MediaStatus{
		domain.MediaStatusPending,
		domain.MediaStatusProcessing,
	})
	if err != nil {
		slog.Error("failed to query orphaned media assets during startup reconciliation", "error", err)
		return
	}
	if len(assets) == 0 {
		slog.Info("no orphaned or stuck media assets found")
		return
	}
	slog.Info("found orphaned media assets from previous session; reconciling status and resuming processing...", "count", len(assets))

	for _, asset := range assets {
		key := extractStorageKey(asset.SourceURL)
		
		// Check if the underlying file exists in object storage
		reader, _, err := s.storage.GetFile(ctx, key)
		if err != nil {
			// If file cannot be found in storage, we cannot resume transcoding.
			// Mark asset as failed so UI doesn't show false 'processing' state.
			slog.Warn("orphaned asset source file not found in storage; marking asset as failed", "asset_id", asset.ID, "key", key, "error", err)
			_ = s.repo.UpdateAssetProgress(ctx, asset.ID, 0, domain.MediaStatusFailed)
			continue
		}
		reader.Close()

		// Source file exists! We can cleanly resume background HLS processing.
		slog.Info("resuming background HLS processing for orphaned asset", "asset_id", asset.ID, "key", key)
		if s.processor != nil {
			go s.processor.ProcessAsset(asset.ID, key, asset.SourceURL)
		} else {
			_ = s.repo.UpdateAssetStatus(ctx, asset.ID, domain.MediaStatusReady, asset.SourceURL)
		}
	}
}

func extractStorageKey(sourceURL string) string {
	if idx := strings.Index(sourceURL, "raw/"); idx != -1 {
		return sourceURL[idx:]
	}
	if idx := strings.Index(sourceURL, "hls/"); idx != -1 {
		return sourceURL[idx:]
	}
	return filepath.Base(sourceURL)
}
