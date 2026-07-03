package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

var (
	ErrInvalidCredentials  = errors.New("invalid email or password")
	ErrInvalidInput        = errors.New("username, email, and password (min 8 chars) are required")
)

// AuthService defines the business logic contract for authentication workflows.
type AuthService interface {
	Signup(ctx context.Context, username, email, password string) (*domain.Session, error)
	Login(ctx context.Context, email, password string) (*domain.Session, error)
	Logout(ctx context.Context, sessionID string) error
	ValidateSession(ctx context.Context, sessionID string) (*domain.Session, error)
}

type authService struct {
	userRepo     UserRepository
	sessionStore SessionStore
}

// NewAuthService constructs a new business service with injected repository dependencies.
func NewAuthService(userRepo UserRepository, sessionStore SessionStore) AuthService {
	return &authService{
		userRepo:     userRepo,
		sessionStore: sessionStore,
	}
}

// Signup validates input, hashes the password, creates the PostgreSQL user record, and mints a Redis session.
func (s *authService) Signup(ctx context.Context, username, email, password string) (*domain.Session, error) {
	username = strings.TrimSpace(username)
	email = strings.ToLower(strings.TrimSpace(email))

	if username == "" || email == "" || len(password) < 8 {
		return nil, ErrInvalidInput
	}

	// 1. Compute Argon2id password hash
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 2. Persist user entity in PostgreSQL
	user := &domain.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err // Propagates ErrEmailAlreadyTaken cleanly
	}

	// 3. Create active server-side session in Redis
	return s.createSessionForUser(ctx, user)
}

// Login verifies credentials and mints a fresh Redis session.
func (s *authService) Login(ctx context.Context, email, password string) (*domain.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1. Fetch user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Return generic error to prevent email enumeration attacks
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to lookup user during login: %w", err)
	}

	// 2. Verify password against Argon2id hash using constant-time comparison
	valid, err := VerifyPassword(password, user.PasswordHash)
	if err != nil || !valid {
		return nil, ErrInvalidCredentials
	}

	// 3. Create active server-side session in Redis
	return s.createSessionForUser(ctx, user)
}

// Logout revokes the session from Redis memory.
func (s *authService) Logout(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	return s.sessionStore.Delete(ctx, sessionID)
}

// ValidateSession verifies if a session ID exists in Redis and returns the session payload.
func (s *authService) ValidateSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	if sessionID == "" {
		return nil, ErrSessionNotFound
	}
	return s.sessionStore.Get(ctx, sessionID)
}

// createSessionForUser helper generates session ID and populates the Redis store.
func (s *authService) createSessionForUser(ctx context.Context, user *domain.User) (*domain.Session, error) {
	sessionID, err := s.sessionStore.GenerateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session id: %w", err)
	}

	now := time.Now()
	session := &domain.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultSessionDuration),
	}

	if err := s.sessionStore.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to persist session: %w", err)
	}

	return session, nil
}
