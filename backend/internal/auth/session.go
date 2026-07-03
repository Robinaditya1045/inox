package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/redis/go-redis/v9"
)

var (
	ErrSessionNotFound = errors.New("session expired or not found")
)

const (
	sessionKeyPrefix = "session:"
	DefaultSessionDuration = 7 * 24 * time.Hour // 7 days
)

// SessionStore defines the contract for server-side session persistence.
type SessionStore interface {
	Create(ctx context.Context, session *domain.Session) error
	Get(ctx context.Context, sessionID string) (*domain.Session, error)
	Delete(ctx context.Context, sessionID string) error
	GenerateSessionID() (string, error)
}

type redisSessionStore struct {
	client *redis.Client
}

// NewSessionStore initializes a Redis-backed session store.
func NewSessionStore(client *redis.Client) SessionStore {
	return &redisSessionStore{client: client}
}

// GenerateSessionID generates a cryptographically secure 256-bit random session identifier.
func (s *redisSessionStore) GenerateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to read random entropy for session id: %w", err)
	}
	return "sess_" + hex.EncodeToString(bytes), nil
}

// Create stores the serialized user session in Redis with an automatic TTL expiration.
func (s *redisSessionStore) Create(ctx context.Context, session *domain.Session) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	key := sessionKeyPrefix + session.ID
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		ttl = DefaultSessionDuration
	}

	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to save session to redis: %w", err)
	}

	return nil
}

// Get fetches and deserializes a session by its unique ID.
func (s *redisSessionStore) Get(ctx context.Context, sessionID string) (*domain.Session, error) {
	key := sessionKeyPrefix + sessionID

	data, err := s.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to fetch session from redis: %w", err)
	}

	var session domain.Session
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to deserialize session data: %w", err)
	}

	return &session, nil
}

// Delete instantly revokes a user's session from Redis memory.
func (s *redisSessionStore) Delete(ctx context.Context, sessionID string) error {
	key := sessionKeyPrefix + sessionID

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from redis: %w", err)
	}

	return nil
}
