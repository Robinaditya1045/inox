package community

import (
	"context"
	"errors"
	"fmt"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPostNotFound = errors.New("post not found")

// Repository is the persistence contract for community posts. Every read joins
// the author's current username and avatar.
type Repository interface {
	Create(ctx context.Context, p *domain.CommunityPost) error
	GetByID(ctx context.Context, id string) (*domain.CommunityPost, error)
	// List returns published posts, pinned first and then newest first. With
	// IncludeDrafts it returns drafts too, ahead of everything else.
	List(ctx context.Context, opts ListOptions) ([]*domain.CommunityPost, error)
	// Update writes the post's category, title, body, pinned and published_at.
	Update(ctx context.Context, p *domain.CommunityPost) error
	Delete(ctx context.Context, id string) error
}

// ListOptions narrows and pages Repository.List. An empty Category means all.
type ListOptions struct {
	IncludeDrafts bool
	Category      string
	Limit         int
	Offset        int
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL-backed community post repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

const selectPost = `
	SELECT p.id, p.category, p.title, p.body, p.pinned, p.published_at,
	       COALESCE(p.author_id::text, ''), COALESCE(u.username, ''), COALESCE(u.avatar_url, ''),
	       p.created_at, p.updated_at
	FROM community_posts p
	LEFT JOIN users u ON u.id = p.author_id
`

func scanPost(row pgx.Row) (*domain.CommunityPost, error) {
	p := &domain.CommunityPost{}
	err := row.Scan(&p.ID, &p.Category, &p.Title, &p.Body, &p.Pinned, &p.PublishedAt,
		&p.AuthorID, &p.AuthorUsername, &p.AuthorAvatarURL, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// isMissing reports whether err means the row is not there. A malformed UUID is
// as absent as an unknown one.
func isMissing(err error) bool {
	var pgErr *pgconn.PgError
	return errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "22P02")
}

func (r *postgresRepository) Create(ctx context.Context, p *domain.CommunityPost) error {
	var authorID any
	if p.AuthorID != "" {
		authorID = p.AuthorID
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO community_posts (author_id, category, title, body, pinned, published_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, authorID, p.Category, p.Title, p.Body, p.Pinned, p.PublishedAt).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create community post: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*domain.CommunityPost, error) {
	p, err := scanPost(r.db.QueryRow(ctx, selectPost+` WHERE p.id = $1`, id))
	if err != nil {
		if isMissing(err) {
			return nil, ErrPostNotFound
		}
		return nil, fmt.Errorf("failed to query community post: %w", err)
	}
	return p, nil
}

func (r *postgresRepository) List(ctx context.Context, opts ListOptions) ([]*domain.CommunityPost, error) {
	query := selectPost + `
		WHERE ($1 OR p.published_at IS NOT NULL)
		  AND ($2::text = '' OR p.category = $2::text)
		ORDER BY (p.published_at IS NULL) DESC, p.pinned DESC,
		         COALESCE(p.published_at, p.updated_at) DESC, p.id
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, opts.IncludeDrafts, opts.Category, opts.Limit, opts.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list community posts: %w", err)
	}
	defer rows.Close()

	posts := []*domain.CommunityPost{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan community post: %w", err)
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return posts, nil
}

func (r *postgresRepository) Update(ctx context.Context, p *domain.CommunityPost) error {
	err := r.db.QueryRow(ctx, `
		UPDATE community_posts
		SET category = $2, title = $3, body = $4, pinned = $5, published_at = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`, p.ID, p.Category, p.Title, p.Body, p.Pinned, p.PublishedAt).Scan(&p.UpdatedAt)
	if err != nil {
		if isMissing(err) {
			return ErrPostNotFound
		}
		return fmt.Errorf("failed to update community post: %w", err)
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM community_posts WHERE id = $1`, id)
	if err != nil {
		if isMissing(err) {
			return ErrPostNotFound
		}
		return fmt.Errorf("failed to delete community post: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrPostNotFound
	}
	return nil
}
