package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the persistence contract for admins added from the admin portal.
// Owners come from ADMIN_EMAILS and are never stored.
type Repository interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
	// List returns every stored admin with Role left unset; the service decides it.
	List(ctx context.Context) ([]*domain.AdminAccount, error)
	// Add returns ErrAlreadyAdmin when the user is already an admin.
	Add(ctx context.Context, userID, grantedBy string) (time.Time, error)
	// Remove returns ErrNotAdmin when the user was not an admin.
	Remove(ctx context.Context, userID string) error
}

type postgresRepository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a PostgreSQL-backed admin repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE user_id = $1)`, userID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("failed to check admin access: %w", err)
	}
	return ok, nil
}

func (r *postgresRepository) List(ctx context.Context) ([]*domain.AdminAccount, error) {
	query := `
		SELECT a.user_id, u.username, u.email, COALESCE(g.username, ''), a.created_at
		FROM admin_users a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN users g ON g.id = a.granted_by
		ORDER BY a.created_at
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}
	defer rows.Close()

	accounts := []*domain.AdminAccount{}
	for rows.Next() {
		var a domain.AdminAccount
		var grantedAt time.Time
		if err := rows.Scan(&a.UserID, &a.Username, &a.Email, &a.GrantedBy, &grantedAt); err != nil {
			return nil, fmt.Errorf("failed to scan admin: %w", err)
		}
		a.GrantedAt = &grantedAt
		accounts = append(accounts, &a)
	}
	return accounts, rows.Err()
}

func (r *postgresRepository) Add(ctx context.Context, userID, grantedBy string) (time.Time, error) {
	query := `
		INSERT INTO admin_users (user_id, granted_by)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING
		RETURNING created_at
	`
	var createdAt time.Time
	err := r.db.QueryRow(ctx, query, userID, grantedBy).Scan(&createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, ErrAlreadyAdmin
		}
		return time.Time{}, fmt.Errorf("failed to add admin: %w", err)
	}
	return createdAt, nil
}

func (r *postgresRepository) Remove(ctx context.Context, userID string) error {
	res, err := r.db.Exec(ctx, `DELETE FROM admin_users WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("failed to remove admin: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotAdmin
	}
	return nil
}
