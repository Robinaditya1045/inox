package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrChannelNotFound is returned when no live channel matches the given key.
var ErrChannelNotFound = errors.New("live channel not found")

// Repository is the storage contract for live channel ingest configuration.
type Repository interface {
	Create(ctx context.Context, ch *domain.LiveChannel) error
	GetBySlug(ctx context.Context, slug string) (*domain.LiveChannel, error)
	GetByID(ctx context.Context, id string) (*domain.LiveChannel, error)
	List(ctx context.Context) ([]*domain.LiveChannel, error)
	UpdateUpstream(ctx context.Context, id string, upstreamURL string, headers map[string]string, expiresAt *time.Time, isDVR bool, windowSecs int) error
	UpdateStatus(ctx context.Context, id string, status domain.LiveChannelStatus, lastErr string) error
	Delete(ctx context.Context, id string) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository initializes the PostgreSQL adapter for live channels.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

// selectColumns joins in the companion asset title so the admin list is one query.
const selectColumns = `
	SELECT c.id, c.media_asset_id, c.slug, c.resolver, c.source_url,
	       c.resolver_config, c.fallback_sources, c.protocol, c.is_dvr, c.dvr_window_seconds,
	       c.status, COALESCE(c.upstream_url, ''), c.upstream_headers, c.upstream_expires_at,
	       c.last_resolved_at, COALESCE(c.last_error, ''), c.created_by, c.created_at, c.updated_at,
	       COALESCE(a.title, '')
	FROM live_channels c
	LEFT JOIN media_assets a ON a.id = c.media_asset_id
`

func (r *postgresRepository) Create(ctx context.Context, ch *domain.LiveChannel) error {
	cfg, err := json.Marshal(orEmptyMap(ch.ResolverConfig))
	if err != nil {
		return fmt.Errorf("failed to encode resolver config: %w", err)
	}
	fallbacks, err := json.Marshal(orEmptySlice(ch.Fallbacks))
	if err != nil {
		return fmt.Errorf("failed to encode fallback sources: %w", err)
	}

	query := `
		INSERT INTO live_channels
			(media_asset_id, slug, resolver, source_url, resolver_config, fallback_sources,
			 protocol, is_dvr, dvr_window_seconds, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	err = r.db.QueryRow(ctx, query,
		ch.MediaAssetID, ch.Slug, ch.Resolver, ch.SourceURL, cfg, fallbacks,
		ch.Protocol, ch.IsDVR, ch.DVRWindowSecs, ch.Status, ch.CreatedBy,
	).Scan(&ch.ID, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create live channel: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetBySlug(ctx context.Context, slug string) (*domain.LiveChannel, error) {
	return r.queryOne(ctx, selectColumns+` WHERE c.slug = $1`, slug)
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*domain.LiveChannel, error) {
	return r.queryOne(ctx, selectColumns+` WHERE c.id = $1`, id)
}

func (r *postgresRepository) List(ctx context.Context) ([]*domain.LiveChannel, error) {
	rows, err := r.db.Query(ctx, selectColumns+` ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list live channels: %w", err)
	}
	defer rows.Close()

	channels := []*domain.LiveChannel{}
	for rows.Next() {
		ch, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// UpdateUpstream stores a freshly resolved manifest and marks the channel live.
func (r *postgresRepository) UpdateUpstream(ctx context.Context, id string, upstreamURL string, headers map[string]string, expiresAt *time.Time, isDVR bool, windowSecs int) error {
	hdr, err := json.Marshal(orEmptyStringMap(headers))
	if err != nil {
		return fmt.Errorf("failed to encode upstream headers: %w", err)
	}
	query := `
		UPDATE live_channels
		SET upstream_url = $2, upstream_headers = $3, upstream_expires_at = $4,
		    is_dvr = $5, dvr_window_seconds = $6,
		    status = 'live', last_error = '', last_resolved_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	res, err := r.db.Exec(ctx, query, id, upstreamURL, hdr, expiresAt, isDVR, windowSecs)
	if err != nil {
		return fmt.Errorf("failed to update live channel upstream: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id string, status domain.LiveChannelStatus, lastErr string) error {
	query := `UPDATE live_channels SET status = $2, last_error = $3, updated_at = NOW() WHERE id = $1`
	res, err := r.db.Exec(ctx, query, id, status, lastErr)
	if err != nil {
		return fmt.Errorf("failed to update live channel status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM live_channels WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete live channel: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrChannelNotFound
	}
	return nil
}

func (r *postgresRepository) queryOne(ctx context.Context, query string, args ...any) (*domain.LiveChannel, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query live channel: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("failed to query live channel: %w", err)
		}
		return nil, ErrChannelNotFound
	}
	return scanChannel(rows)
}

func scanChannel(rows pgx.Rows) (*domain.LiveChannel, error) {
	ch := &domain.LiveChannel{}
	var cfg, fallbacks, headers []byte

	err := rows.Scan(
		&ch.ID, &ch.MediaAssetID, &ch.Slug, &ch.Resolver, &ch.SourceURL,
		&cfg, &fallbacks, &ch.Protocol, &ch.IsDVR, &ch.DVRWindowSecs,
		&ch.Status, &ch.UpstreamURL, &headers, &ch.UpstreamExpiresAt,
		&ch.LastResolvedAt, &ch.LastError, &ch.CreatedBy, &ch.CreatedAt, &ch.UpdatedAt,
		&ch.Title,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan live channel: %w", err)
	}

	// A malformed JSONB blob should degrade the channel, not fail the whole listing.
	_ = json.Unmarshal(cfg, &ch.ResolverConfig)
	_ = json.Unmarshal(fallbacks, &ch.Fallbacks)
	_ = json.Unmarshal(headers, &ch.UpstreamHeaders)

	if ch.ResolverConfig == nil {
		ch.ResolverConfig = map[string]any{}
	}
	return ch, nil
}

func orEmptyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func orEmptyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func orEmptySlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
