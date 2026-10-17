package playlist

import (
	"context"
	"errors"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrPlaylistNotFound = errors.New("playlist not found")
	ErrItemNotFound     = errors.New("that video is not in this playlist")
	// ErrDuplicateItem surfaces the (playlist_id, media_asset_id) unique constraint.
	ErrDuplicateItem = errors.New("that video is already in this playlist")
)

// Repository is the persistence contract for playlists. Every read returns the
// playlist with its items in order, each joined to its media asset.
type Repository interface {
	Create(ctx context.Context, p *domain.Playlist) error
	GetByID(ctx context.Context, id string) (*domain.Playlist, error)
	ListByOwner(ctx context.Context, ownerID string) ([]*domain.Playlist, error)
	Update(ctx context.Context, id, name, description string) error
	Delete(ctx context.Context, id string) error
	AddItem(ctx context.Context, playlistID, mediaAssetID string) error
	RemoveItem(ctx context.Context, playlistID, itemID string) error
	// Reorder sets each item's position to its index in itemIDs, which must name
	// every item in the playlist exactly once.
	Reorder(ctx context.Context, playlistID string, itemIDs []string) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL-backed playlist repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, p *domain.Playlist) error {
	query := `
		INSERT INTO playlists (owner_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	if err := r.db.QueryRow(ctx, query, p.OwnerID, p.Name, p.Description).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return fmt.Errorf("failed to create playlist: %w", err)
	}
	p.Items = []*domain.PlaylistItem{}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*domain.Playlist, error) {
	p := &domain.Playlist{}
	err := r.db.QueryRow(ctx,
		`SELECT id, owner_id, name, description, created_at, updated_at FROM playlists WHERE id = $1`, id,
	).Scan(&p.ID, &p.OwnerID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaylistNotFound
		}
		// A malformed UUID is as absent as an unknown one.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
			return nil, ErrPlaylistNotFound
		}
		return nil, fmt.Errorf("failed to query playlist: %w", err)
	}
	if err := r.attachItems(ctx, []*domain.Playlist{p}); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *postgresRepository) ListByOwner(ctx context.Context, ownerID string) ([]*domain.Playlist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, owner_id, name, description, created_at, updated_at
		FROM playlists WHERE owner_id = $1
		ORDER BY updated_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list playlists: %w", err)
	}
	defer rows.Close()

	playlists := []*domain.Playlist{}
	for rows.Next() {
		p := &domain.Playlist{}
		if err := rows.Scan(&p.ID, &p.OwnerID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan playlist: %w", err)
		}
		playlists = append(playlists, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	if err := r.attachItems(ctx, playlists); err != nil {
		return nil, err
	}
	return playlists, nil
}

// attachItems loads the items of every playlist given in one query.
func (r *postgresRepository) attachItems(ctx context.Context, playlists []*domain.Playlist) error {
	if len(playlists) == 0 {
		return nil
	}
	byID := make(map[string]*domain.Playlist, len(playlists))
	ids := make([]string, 0, len(playlists))
	for _, p := range playlists {
		p.Items = []*domain.PlaylistItem{}
		byID[p.ID] = p
		ids = append(ids, p.ID)
	}

	rows, err := r.db.Query(ctx, `
		SELECT pi.id, pi.playlist_id, pi.media_asset_id, pi.position, pi.added_at,
		       m.id, COALESCE(m.kind, 'vod'), m.title, COALESCE(m.description, ''), m.source_url, m.status,
		       COALESCE(m.duration_seconds, 0), COALESCE(m.progress, 0), COALESCE(m.thumbnail_url, ''),
		       COALESCE(m.hls_master_url, ''), m.created_by, m.created_at, m.updated_at
		FROM playlist_items pi
		JOIN media_assets m ON m.id = pi.media_asset_id
		WHERE pi.playlist_id = ANY($1::uuid[])
		ORDER BY pi.playlist_id, pi.position, pi.added_at
	`, ids)
	if err != nil {
		return fmt.Errorf("failed to query playlist items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := &domain.PlaylistItem{Media: &domain.MediaAsset{}}
		m := item.Media
		if err := rows.Scan(
			&item.ID, &item.PlaylistID, &item.MediaAssetID, &item.Position, &item.AddedAt,
			&m.ID, &m.Kind, &m.Title, &m.Description, &m.SourceURL, &m.Status,
			&m.DurationSeconds, &m.Progress, &m.ThumbnailURL,
			&m.HLSMasterURL, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan playlist item: %w", err)
		}
		if p, ok := byID[item.PlaylistID]; ok {
			p.Items = append(p.Items, item)
		}
	}
	return rows.Err()
}

func (r *postgresRepository) Update(ctx context.Context, id, name, description string) error {
	res, err := r.db.Exec(ctx,
		`UPDATE playlists SET name = $2, description = $3, updated_at = NOW() WHERE id = $1`,
		id, name, description)
	if err != nil {
		return fmt.Errorf("failed to update playlist: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete playlist: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

// AddItem appends a video to the end of a playlist.
func (r *postgresRepository) AddItem(ctx context.Context, playlistID, mediaAssetID string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		// Locking the playlist row serialises concurrent appends, which would
		// otherwise both read the same MAX(position) and land on one slot.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM playlists WHERE id = $1 FOR UPDATE`, playlistID); err != nil {
			return fmt.Errorf("failed to lock playlist: %w", err)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO playlist_items (playlist_id, media_asset_id, position)
			SELECT $1, $2, COALESCE(MAX(position) + 1, 0) FROM playlist_items WHERE playlist_id = $1
		`, playlistID, mediaAssetID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrDuplicateItem
			}
			return fmt.Errorf("failed to add playlist item: %w", err)
		}
		return touch(ctx, tx, playlistID)
	})
}

// RemoveItem drops one item and closes the gap it leaves, so positions stay dense.
func (r *postgresRepository) RemoveItem(ctx context.Context, playlistID, itemID string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var position int
		err := tx.QueryRow(ctx,
			`DELETE FROM playlist_items WHERE id = $1 AND playlist_id = $2 RETURNING position`,
			itemID, playlistID).Scan(&position)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "22P02") {
				return ErrItemNotFound
			}
			return fmt.Errorf("failed to remove playlist item: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE playlist_items SET position = position - 1 WHERE playlist_id = $1 AND position > $2`,
			playlistID, position); err != nil {
			return fmt.Errorf("failed to compact playlist positions: %w", err)
		}
		return touch(ctx, tx, playlistID)
	})
}

func (r *postgresRepository) Reorder(ctx context.Context, playlistID string, itemIDs []string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		// One statement, so the new order is applied whole: the array's ordinality
		// is each item's new position.
		res, err := tx.Exec(ctx, `
			UPDATE playlist_items pi
			SET position = o.ord - 1
			FROM unnest($2::uuid[]) WITH ORDINALITY AS o(id, ord)
			WHERE pi.id = o.id AND pi.playlist_id = $1
		`, playlistID, itemIDs)
		if err != nil {
			return fmt.Errorf("failed to reorder playlist: %w", err)
		}
		if int(res.RowsAffected()) != len(itemIDs) {
			return ErrItemNotFound
		}
		return touch(ctx, tx, playlistID)
	})
}

// touch bumps a playlist's updated_at so the most recently edited one lists first.
func touch(ctx context.Context, tx pgx.Tx, playlistID string) error {
	if _, err := tx.Exec(ctx, `UPDATE playlists SET updated_at = NOW() WHERE id = $1`, playlistID); err != nil {
		return fmt.Errorf("failed to touch playlist: %w", err)
	}
	return nil
}
