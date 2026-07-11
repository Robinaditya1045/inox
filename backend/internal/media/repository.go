package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the storage contract for media library cataloging and transcoding renditions.
type Repository interface {
	CreateAsset(ctx context.Context, asset *domain.MediaAsset) error
	GetAssetByID(ctx context.Context, id string) (*domain.MediaAsset, error)
	ListAssets(ctx context.Context, limit, offset int) ([]*domain.MediaAsset, error)
	UpdateAssetStatus(ctx context.Context, id string, status domain.MediaStatus, hlsMasterURL string) error
	UpdateAssetProgress(ctx context.Context, id string, progress int, status domain.MediaStatus) error
	GetAssetsByStatus(ctx context.Context, statuses []domain.MediaStatus) ([]*domain.MediaAsset, error)
	AddRendition(ctx context.Context, r *domain.MediaRendition) error
	DeleteAsset(ctx context.Context, id string) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository initializes a PostgreSQL persistence adapter for the media library.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// CreateAsset inserts a new media asset record into the database.
func (r *postgresRepository) CreateAsset(ctx context.Context, a *domain.MediaAsset) error {
	query := `
		INSERT INTO media_assets (title, description, source_url, status, duration_seconds, progress, thumbnail_url, hls_master_url, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, a.Title, a.Description, a.SourceURL, a.Status, a.DurationSeconds, a.Progress, a.ThumbnailURL, a.HLSMasterURL, a.CreatedBy).Scan(
		&a.ID, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create media asset: %w", err)
	}
	return nil
}

// GetAssetByID retrieves a media asset along with all its adaptive bitrate HLS renditions.
func (r *postgresRepository) GetAssetByID(ctx context.Context, id string) (*domain.MediaAsset, error) {
	query := `
		SELECT id, title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
		FROM media_assets WHERE id = $1
	`
	a := &domain.MediaAsset{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("media asset not found")
		}
		return nil, fmt.Errorf("failed to query media asset: %w", err)
	}

	rendQuery := `SELECT id, media_asset_id, resolution, bitrate_kbps, playlist_url, created_at FROM media_renditions WHERE media_asset_id = $1 ORDER BY bitrate_kbps DESC`
	rows, err := r.db.Query(ctx, rendQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			mr := &domain.MediaRendition{}
			if err := rows.Scan(&mr.ID, &mr.MediaAssetID, &mr.Resolution, &mr.BitrateKbps, &mr.PlaylistURL, &mr.CreatedAt); err == nil {
				a.Renditions = append(a.Renditions, mr)
			}
		}
	}
	return a, nil
}

// ListAssets returns paginated media assets ordered chronologically.
func (r *postgresRepository) ListAssets(ctx context.Context, limit, offset int) ([]*domain.MediaAsset, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT id, title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
		FROM media_assets ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list media assets: %w", err)
	}
	defer rows.Close()

	var assets []*domain.MediaAsset
	for rows.Next() {
		a := &domain.MediaAsset{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	if assets == nil {
		assets = []*domain.MediaAsset{}
	}
	return assets, nil
}

// UpdateAssetStatus updates the transcoding status and HLS master playlist URL of an asset.
func (r *postgresRepository) UpdateAssetStatus(ctx context.Context, id string, status domain.MediaStatus, hlsMasterURL string) error {
	query := `UPDATE media_assets SET status = $2, hls_master_url = COALESCE(NULLIF($3, ''), hls_master_url), updated_at = NOW() WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id, status, hlsMasterURL)
	if err != nil {
		return fmt.Errorf("failed to update media status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("media asset not found")
	}
	return nil
}

// UpdateAssetProgress updates the percentage progression and status of a media asset during transcoding or uploading.
func (r *postgresRepository) UpdateAssetProgress(ctx context.Context, id string, progress int, status domain.MediaStatus) error {
	query := `UPDATE media_assets SET progress = $2, status = $3, updated_at = NOW() WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id, progress, status)
	if err != nil {
		return fmt.Errorf("failed to update media progress: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("media asset not found")
	}
	return nil
}

// AddRendition records a new resolution quality level in the database.
func (r *postgresRepository) AddRendition(ctx context.Context, mr *domain.MediaRendition) error {
	query := `
		INSERT INTO media_renditions (media_asset_id, resolution, bitrate_kbps, playlist_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	err := r.db.QueryRow(ctx, query, mr.MediaAssetID, mr.Resolution, mr.BitrateKbps, mr.PlaylistURL).Scan(&mr.ID, &mr.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert media rendition: %w", err)
	}
	return nil
}

// DeleteAsset removes a media asset and all its cascading renditions.
func (r *postgresRepository) DeleteAsset(ctx context.Context, id string) error {
	query := `DELETE FROM media_assets WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete media asset: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("media asset not found")
	}
	return nil
}

// GetAssetsByStatus returns all media assets matching any of the given status values.
func (r *postgresRepository) GetAssetsByStatus(ctx context.Context, statuses []domain.MediaStatus) ([]*domain.MediaAsset, error) {
	if len(statuses) == 0 {
		return nil, nil
	}
	query := `
		SELECT id, title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
		FROM media_assets WHERE status = ANY($1) ORDER BY created_at ASC
	`
	statusStrings := make([]string, len(statuses))
	for i, s := range statuses {
		statusStrings[i] = string(s)
	}
	rows, err := r.db.Query(ctx, query, statusStrings)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets by status: %w", err)
	}
	defer rows.Close()

	var assets []*domain.MediaAsset
	for rows.Next() {
		a := &domain.MediaAsset{}
		if err := rows.Scan(&a.ID, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}
