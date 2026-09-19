package friend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrFriendshipNotFound = errors.New("friendship not found")
	// ErrPairConflict surfaces the (LEAST, GREATEST) unique index firing, which only
	// happens when both users send a request in the same instant. The service turns
	// it back into a retry-able message rather than a 500.
	ErrPairConflict = errors.New("a friendship record already exists for this pair")
)

// Repository is the persistence contract for friendships. Reads are expressed from
// one viewer's side (ListFriends, ListPendingRequests) because that is the only
// shape the API ever needs; the symmetric pair lookup is GetByPair.
type Repository interface {
	Create(ctx context.Context, f *domain.Friendship) error
	GetByID(ctx context.Context, id string) (*domain.Friendship, error)
	GetByPair(ctx context.Context, userA, userB string) (*domain.Friendship, error)
	UpdateStatus(ctx context.Context, id string, status domain.FriendshipStatus) error
	Reopen(ctx context.Context, id, requesterID, addresseeID string) error
	Delete(ctx context.Context, id string) error
	ListFriends(ctx context.Context, userID string) ([]*domain.Friend, error)
	ListPendingRequests(ctx context.Context, userID string) ([]*domain.FriendRequest, error)
	SearchUsers(ctx context.Context, viewerID, query string, limit int) ([]*domain.UserSearchResult, error)
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL-backed friendship repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, f *domain.Friendship) error {
	query := `
		INSERT INTO friendships (requester_id, addressee_id, status)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRow(ctx, query, f.RequesterID, f.AddresseeID, f.Status).Scan(
		&f.ID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrPairConflict
		}
		return fmt.Errorf("failed to insert friendship: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*domain.Friendship, error) {
	query := `
		SELECT id, requester_id, addressee_id, status, created_at, updated_at
		FROM friendships
		WHERE id = $1
	`
	return r.scanOne(ctx, query, id)
}

// GetByPair looks a relationship up without knowing who asked whom.
func (r *postgresRepository) GetByPair(ctx context.Context, userA, userB string) (*domain.Friendship, error) {
	query := `
		SELECT id, requester_id, addressee_id, status, created_at, updated_at
		FROM friendships
		WHERE (requester_id = $1 AND addressee_id = $2)
		   OR (requester_id = $2 AND addressee_id = $1)
	`
	return r.scanOne(ctx, query, userA, userB)
}

func (r *postgresRepository) scanOne(ctx context.Context, query string, args ...any) (*domain.Friendship, error) {
	var f domain.Friendship
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&f.ID, &f.RequesterID, &f.AddresseeID, &f.Status, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFriendshipNotFound
		}
		return nil, fmt.Errorf("failed to query friendship: %w", err)
	}
	return &f, nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, id string, status domain.FriendshipStatus) error {
	query := `UPDATE friendships SET status = $2, updated_at = NOW() WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update friendship status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

// Reopen re-arms a settled row as a fresh pending request, rewriting the direction:
// after B declines A, B may later ask A, and that is the same pair row with the
// requester and addressee swapped.
func (r *postgresRepository) Reopen(ctx context.Context, id, requesterID, addresseeID string) error {
	query := `
		UPDATE friendships
		SET requester_id = $2, addressee_id = $3, status = 'pending', created_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	tag, err := r.db.Exec(ctx, query, id, requesterID, addresseeID)
	if err != nil {
		return fmt.Errorf("failed to reopen friendship: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM friendships WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete friendship: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

// ListFriends returns the accepted counterparties for a user. The CASE join picks
// the other side of each pair so the caller never has to look at direction.
func (r *postgresRepository) ListFriends(ctx context.Context, userID string) ([]*domain.Friend, error) {
	query := `
		SELECT f.id, u.id, u.username, u.avatar_url, f.updated_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
		WHERE (f.requester_id = $1 OR f.addressee_id = $1)
		  AND f.status = 'accepted'
		ORDER BY LOWER(u.username) ASC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list friends: %w", err)
	}
	defer rows.Close()

	friends := []*domain.Friend{}
	for rows.Next() {
		f := &domain.Friend{}
		if err := rows.Scan(&f.FriendshipID, &f.UserID, &f.Username, &f.AvatarURL, &f.FriendsSince); err != nil {
			return nil, fmt.Errorf("failed to scan friend: %w", err)
		}
		friends = append(friends, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	return friends, nil
}

// ListPendingRequests returns both directions in one pass; the caller splits them.
func (r *postgresRepository) ListPendingRequests(ctx context.Context, userID string) ([]*domain.FriendRequest, error) {
	query := `
		SELECT f.id,
		       CASE WHEN f.requester_id = $1 THEN 'outgoing' ELSE 'incoming' END,
		       u.id, u.username, u.avatar_url, f.created_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.requester_id = $1 THEN f.addressee_id ELSE f.requester_id END
		WHERE (f.requester_id = $1 OR f.addressee_id = $1)
		  AND f.status = 'pending'
		ORDER BY f.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list friend requests: %w", err)
	}
	defer rows.Close()

	requests := []*domain.FriendRequest{}
	for rows.Next() {
		req := &domain.FriendRequest{}
		if err := rows.Scan(&req.ID, &req.Direction, &req.UserID, &req.Username, &req.AvatarURL, &req.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan friend request: %w", err)
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	return requests, nil
}

// SearchUsers finds users by username prefix/substring, annotated with the viewer's
// existing relationship so the UI can pick the right action per row.
func (r *postgresRepository) SearchUsers(ctx context.Context, viewerID, query string, limit int) ([]*domain.UserSearchResult, error) {
	sql := `
		SELECT u.id, u.username, u.avatar_url, f.status, f.requester_id
		FROM users u
		LEFT JOIN friendships f
		       ON (f.requester_id = u.id AND f.addressee_id = $1)
		       OR (f.addressee_id = u.id AND f.requester_id = $1)
		WHERE u.id <> $1
		  AND u.username ILIKE $2
		ORDER BY LOWER(u.username) ASC
		LIMIT $3
	`
	rows, err := r.db.Query(ctx, sql, viewerID, "%"+escapeLike(query)+"%", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer rows.Close()

	results := []*domain.UserSearchResult{}
	for rows.Next() {
		var (
			res         domain.UserSearchResult
			status      *string
			requesterID *string
		)
		if err := rows.Scan(&res.UserID, &res.Username, &res.AvatarURL, &status, &requesterID); err != nil {
			return nil, fmt.Errorf("failed to scan user search result: %w", err)
		}
		res.Relationship = relationshipFor(viewerID, status, requesterID)
		results = append(results, &res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	return results, nil
}

// relationshipFor maps a joined friendship row onto the viewer's point of view. A
// declined row reads as "none" so a past refusal does not permanently grey out the
// add button.
func relationshipFor(viewerID string, status, requesterID *string) string {
	if status == nil || requesterID == nil {
		return domain.RelationshipNone
	}
	switch domain.FriendshipStatus(*status) {
	case domain.FriendshipStatusAccepted:
		return domain.RelationshipFriends
	case domain.FriendshipStatusPending:
		if *requesterID == viewerID {
			return domain.RelationshipRequestSent
		}
		return domain.RelationshipRequestReceived
	default:
		return domain.RelationshipNone
	}
}

// escapeLike neutralizes the wildcards a user can type into the search box, so
// searching for "_" does not match every username of that length.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
