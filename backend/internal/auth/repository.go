package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrEmailAlreadyTaken = errors.New("email is already registered")
)

// UserRepository defines the persistence contract for user entities.
// Using an interface decouples our business service from PostgreSQL, allowing clean unit testing.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
	UpdateAvatar(ctx context.Context, userID string, avatarURL string) error
	CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	GetPasswordResetToken(ctx context.Context, tokenHash string) (string, error)
	DeletePasswordResetToken(ctx context.Context, tokenHash string) error
	UpdatePassword(ctx context.Context, userID, passwordHash string) error
}

type postgresUserRepository struct {
	db *pgxpool.Pool
}

// NewUserRepository constructs a new PostgreSQL-backed user repository.
func NewUserRepository(db *pgxpool.Pool) UserRepository {
	return &postgresUserRepository{db: db}
}

// Create inserts a new user entity into PostgreSQL using parameterized placeholders ($1, $2, ...).
func (r *postgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query, user.Username, user.Email, user.PasswordHash).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		// Check for PostgreSQL unique constraint violation error code (23505)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "username") || strings.Contains(pgErr.Message, "username") || strings.Contains(pgErr.Detail, "username") {
				return errors.New("username is already taken")
			}
			return ErrEmailAlreadyTaken
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// GetByEmail fetches a user by their unique email address.
func (r *postgresUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, avatar_url, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	return &user, nil
}

// GetByID fetches a user by their UUID primary key.
func (r *postgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by id: %w", err)
	}

	return &user, nil
}

// GetByUsername fetches a user by their unique username.
func (r *postgresUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, avatar_url, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	var user domain.User
	err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user by username: %w", err)
	}

	return &user, nil
}

// UpdateAvatar updates the user's avatar URL.
func (r *postgresUserRepository) UpdateAvatar(ctx context.Context, userID string, avatarURL string) error {
	query := `
		UPDATE users 
		SET avatar_url = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	tag, err := r.db.Exec(ctx, query, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("failed to update avatar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *postgresUserRepository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	query := `INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	_, err := r.db.Exec(ctx, query, userID, tokenHash, expiresAt)
	return err
}

func (r *postgresUserRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	query := `SELECT user_id FROM password_reset_tokens WHERE token_hash = $1 AND expires_at > NOW()`
	var userID string
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("invalid or expired token")
		}
		return "", err
	}
	return userID, nil
}

func (r *postgresUserRepository) DeletePasswordResetToken(ctx context.Context, tokenHash string) error {
	query := `DELETE FROM password_reset_tokens WHERE token_hash = $1`
	_, err := r.db.Exec(ctx, query, tokenHash)
	return err
}

func (r *postgresUserRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.Exec(ctx, query, passwordHash, userID)
	return err
}
