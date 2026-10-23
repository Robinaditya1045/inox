package media

import (
	"context"
	"errors"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	SetThumbnail(ctx context.Context, id, thumbnailURL string, onlyIfMissing bool) error
	SetDuration(ctx context.Context, id string, seconds int) error
	ListAssetsMissingThumbnail(ctx context.Context) ([]*domain.MediaAsset, error)
	UpdateAssetDetails(ctx context.Context, id, title, description string) error
	FindAssetIDByMediaURL(ctx context.Context, mediaURL, storageKey string) (string, error)

	ListCaptions(ctx context.Context, assetID string) ([]*domain.MediaCaption, error)
	ListCaptionsForAssets(ctx context.Context, assetIDs []string) (map[string][]*domain.MediaCaption, error)
	GetCaption(ctx context.Context, assetID, captionID string) (*domain.MediaCaption, error)
	UpsertCaption(ctx context.Context, c *domain.MediaCaption) (replacedKey string, err error)
	UpdateCaption(ctx context.Context, c *domain.MediaCaption) error
	SetDefaultCaption(ctx context.Context, assetID, captionID string, isDefault bool) error
	DeleteCaption(ctx context.Context, assetID, captionID string) (*domain.MediaCaption, error)
}

var (
	ErrCaptionNotFound      = errors.New("caption track not found")
	ErrCaptionLanguageTaken = errors.New("this video already has captions in that language")
)

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository initializes a PostgreSQL persistence adapter for the media library.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// CreateAsset inserts a new media asset record into the database.
func (r *postgresRepository) CreateAsset(ctx context.Context, a *domain.MediaAsset) error {
	if a.Kind == "" {
		a.Kind = domain.MediaKindVOD
	}
	query := `
		INSERT INTO media_assets (kind, title, description, source_url, status, duration_seconds, progress, thumbnail_url, hls_master_url, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, a.Kind, a.Title, a.Description, a.SourceURL, a.Status, a.DurationSeconds, a.Progress, a.ThumbnailURL, a.HLSMasterURL, a.CreatedBy).Scan(
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
		SELECT id, COALESCE(kind, 'vod'), title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
		FROM media_assets WHERE id = $1
	`
	a := &domain.MediaAsset{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.Kind, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt,
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
		SELECT id, COALESCE(kind, 'vod'), title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
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
		if err := rows.Scan(&a.ID, &a.Kind, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
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
		SELECT id, COALESCE(kind, 'vod'), title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
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
		if err := rows.Scan(&a.ID, &a.Kind, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// SetThumbnail records an asset's poster image. With onlyIfMissing the write is
// skipped when the asset already has one, so a frame captured by the transcoder
// never replaces a poster someone uploaded -- the check is in the UPDATE itself
// because an upload can land while the transcode is still running.
func (r *postgresRepository) SetThumbnail(ctx context.Context, id, thumbnailURL string, onlyIfMissing bool) error {
	query := `UPDATE media_assets SET thumbnail_url = $2, updated_at = NOW() WHERE id = $1`
	if onlyIfMissing {
		query += ` AND COALESCE(thumbnail_url, '') = ''`
	}
	if _, err := r.db.Exec(ctx, query, id, thumbnailURL); err != nil {
		return fmt.Errorf("failed to update media thumbnail: %w", err)
	}
	return nil
}

// SetDuration records an asset's running time as measured by ffprobe.
func (r *postgresRepository) SetDuration(ctx context.Context, id string, seconds int) error {
	if _, err := r.db.Exec(ctx, `UPDATE media_assets SET duration_seconds = $2, updated_at = NOW() WHERE id = $1`, id, seconds); err != nil {
		return fmt.Errorf("failed to update media duration: %w", err)
	}
	return nil
}

// ListAssetsMissingThumbnail returns playable on-demand assets with no poster yet:
// the ones a frame can still be captured for. Live channels have no frame to take.
func (r *postgresRepository) ListAssetsMissingThumbnail(ctx context.Context) ([]*domain.MediaAsset, error) {
	query := `
		SELECT id, COALESCE(kind, 'vod'), title, description, source_url, status, duration_seconds, COALESCE(progress, 0), COALESCE(thumbnail_url, ''), COALESCE(hls_master_url, ''), created_by, created_at, updated_at
		FROM media_assets
		WHERE COALESCE(kind, 'vod') = 'vod' AND status = 'ready' AND COALESCE(thumbnail_url, '') = ''
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets missing thumbnails: %w", err)
	}
	defer rows.Close()

	var assets []*domain.MediaAsset
	for rows.Next() {
		a := &domain.MediaAsset{}
		if err := rows.Scan(&a.ID, &a.Kind, &a.Title, &a.Description, &a.SourceURL, &a.Status, &a.DurationSeconds, &a.Progress, &a.ThumbnailURL, &a.HLSMasterURL, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, a)
	}
	return assets, rows.Err()
}

// UpdateAssetDetails rewrites the descriptive fields an operator edits.
func (r *postgresRepository) UpdateAssetDetails(ctx context.Context, id, title, description string) error {
	res, err := r.db.Exec(ctx, `UPDATE media_assets SET title = $2, description = $3, updated_at = NOW() WHERE id = $1`, id, title, description)
	if err != nil {
		return fmt.Errorf("failed to update media details: %w", err)
	}
	if res.RowsAffected() == 0 {
		return errors.New("media asset not found")
	}
	return nil
}

// FindAssetIDByMediaURL finds the library entry a room is playing from the URL
// the room has. That URL went through the handler's NormalizeAsset, so it may
// name a different host than the stored one; anything in our own storage is
// therefore also matched by its object key, which is unique per asset.
func (r *postgresRepository) FindAssetIDByMediaURL(ctx context.Context, mediaURL, storageKey string) (string, error) {
	query := `SELECT id FROM media_assets WHERE hls_master_url = $1 OR source_url = $1`
	args := []any{mediaURL}
	if storageKey != "" {
		query += ` OR RIGHT(hls_master_url, LENGTH($2)) = $2 OR RIGHT(source_url, LENGTH($2)) = $2`
		args = append(args, "/"+storageKey)
	}
	query += ` ORDER BY created_at DESC LIMIT 1`

	var id string
	if err := r.db.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("failed to look up media by url: %w", err)
	}
	return id, nil
}

const captionColumns = `id, media_asset_id, language, label, url, storage_key, cue_count, is_default, created_at, updated_at`

func scanCaption(row pgx.Row) (*domain.MediaCaption, error) {
	c := &domain.MediaCaption{}
	err := row.Scan(&c.ID, &c.MediaAssetID, &c.Language, &c.Label, &c.URL, &c.StorageKey, &c.CueCount, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// ListCaptions returns a video's caption tracks, default first, then by label.
func (r *postgresRepository) ListCaptions(ctx context.Context, assetID string) ([]*domain.MediaCaption, error) {
	byAsset, err := r.ListCaptionsForAssets(ctx, []string{assetID})
	if err != nil {
		return nil, err
	}
	return byAsset[assetID], nil
}

// ListCaptionsForAssets loads the caption tracks of a page of assets in one query.
func (r *postgresRepository) ListCaptionsForAssets(ctx context.Context, assetIDs []string) (map[string][]*domain.MediaCaption, error) {
	out := make(map[string][]*domain.MediaCaption, len(assetIDs))
	if len(assetIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.Query(ctx, `SELECT `+captionColumns+` FROM media_captions
		WHERE media_asset_id = ANY($1::uuid[]) ORDER BY is_default DESC, label ASC`, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list captions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanCaption(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan caption: %w", err)
		}
		out[c.MediaAssetID] = append(out[c.MediaAssetID], c)
	}
	return out, rows.Err()
}

// GetCaption returns one of a video's caption tracks.
func (r *postgresRepository) GetCaption(ctx context.Context, assetID, captionID string) (*domain.MediaCaption, error) {
	c, err := scanCaption(r.db.QueryRow(ctx, `SELECT `+captionColumns+` FROM media_captions WHERE media_asset_id = $1 AND id = $2`, assetID, captionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCaptionNotFound
		}
		return nil, fmt.Errorf("failed to query caption: %w", err)
	}
	return c, nil
}

// UpsertCaption stores a language's track, replacing the one the video already
// has in that language. replacedKey is the storage object the replaced track
// pointed at, for the caller to purge; it is empty for a new language.
func (r *postgresRepository) UpsertCaption(ctx context.Context, c *domain.MediaCaption) (string, error) {
	// The CTE reads the row as it was before this statement, so it still sees
	// the key the update is about to overwrite.
	query := `
		WITH previous AS (
			SELECT storage_key FROM media_captions WHERE media_asset_id = $1 AND language = $2
		)
		INSERT INTO media_captions (media_asset_id, language, label, url, storage_key, cue_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (media_asset_id, language) DO UPDATE
			SET label = EXCLUDED.label, url = EXCLUDED.url, storage_key = EXCLUDED.storage_key,
			    cue_count = EXCLUDED.cue_count, updated_at = NOW()
		RETURNING id, is_default, created_at, updated_at, COALESCE((SELECT storage_key FROM previous), '')
	`
	var replacedKey string
	err := r.db.QueryRow(ctx, query, c.MediaAssetID, c.Language, c.Label, c.URL, c.StorageKey, c.CueCount).Scan(
		&c.ID, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt, &replacedKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to save caption: %w", err)
	}
	return replacedKey, nil
}

// UpdateCaption rewrites a track's language, label and content.
func (r *postgresRepository) UpdateCaption(ctx context.Context, c *domain.MediaCaption) error {
	err := r.db.QueryRow(ctx, `
		UPDATE media_captions
		SET language = $3, label = $4, url = $5, storage_key = $6, cue_count = $7, updated_at = NOW()
		WHERE media_asset_id = $1 AND id = $2
		RETURNING updated_at`,
		c.MediaAssetID, c.ID, c.Language, c.Label, c.URL, c.StorageKey, c.CueCount,
	).Scan(&c.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrCaptionNotFound
		case errors.As(err, &pgErr) && pgErr.Code == "23505":
			return ErrCaptionLanguageTaken
		}
		return fmt.Errorf("failed to update caption: %w", err)
	}
	return nil
}

// SetDefaultCaption makes a track the video's default, or stops it being one.
// Two statements in a transaction: the one-default index is checked row by row,
// so moving the flag in a single UPDATE could trip over its own halfway state.
func (r *postgresRepository) SetDefaultCaption(ctx context.Context, assetID, captionID string, isDefault bool) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if isDefault {
		if _, err := tx.Exec(ctx, `UPDATE media_captions SET is_default = FALSE WHERE media_asset_id = $1 AND is_default AND id <> $2`, assetID, captionID); err != nil {
			return fmt.Errorf("failed to clear default caption: %w", err)
		}
	}
	res, err := tx.Exec(ctx, `UPDATE media_captions SET is_default = $3 WHERE media_asset_id = $1 AND id = $2`, assetID, captionID, isDefault)
	if err != nil {
		return fmt.Errorf("failed to set default caption: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrCaptionNotFound
	}
	return tx.Commit(ctx)
}

// DeleteCaption removes a track and returns it, so its stored file can be purged.
func (r *postgresRepository) DeleteCaption(ctx context.Context, assetID, captionID string) (*domain.MediaCaption, error) {
	c, err := scanCaption(r.db.QueryRow(ctx, `DELETE FROM media_captions WHERE media_asset_id = $1 AND id = $2 RETURNING `+captionColumns, assetID, captionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrCaptionNotFound
		}
		return nil, fmt.Errorf("failed to delete caption: %w", err)
	}
	return c, nil
}
