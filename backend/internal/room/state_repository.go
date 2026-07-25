package room

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/redis/go-redis/v9"
)

// StateRepository manages ephemeral watch party playback state and membership caches.
type StateRepository interface {
	GetPlaybackState(ctx context.Context, roomID string) (*domain.RoomPlaybackState, error)
	SetPlaybackState(ctx context.Context, roomID string, state *domain.RoomPlaybackState) error
	ClearPlaybackState(ctx context.Context, roomID string) error

	// Memberships
	CacheMemberVerification(ctx context.Context, roomID, userID string) error
	IsMemberVerified(ctx context.Context, roomID, userID string) (bool, error)
	InvalidateMemberVerification(ctx context.Context, roomID, userID string) error
}

type redisStateRepository struct {
	client *redis.Client
}

// NewRedisStateRepository creates a StateRepository backed by Redis.
func NewRedisStateRepository(client *redis.Client) StateRepository {
	return &redisStateRepository{
		client: client,
	}
}

func (r *redisStateRepository) GetPlaybackState(ctx context.Context, roomID string) (*domain.RoomPlaybackState, error) {
	key := fmt.Sprintf("inox:room:state:%s", roomID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get playback state: %w", err)
	}

	var state domain.RoomPlaybackState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal playback state: %w", err)
	}

	return &state, nil
}

func (r *redisStateRepository) SetPlaybackState(ctx context.Context, roomID string, state *domain.RoomPlaybackState) error {
	key := fmt.Sprintf("inox:room:state:%s", roomID)
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal playback state: %w", err)
	}

	// 24-hour TTL as specified
	return r.client.Set(ctx, key, data, 24*time.Hour).Err()
}

func (r *redisStateRepository) ClearPlaybackState(ctx context.Context, roomID string) error {
	key := fmt.Sprintf("inox:room:state:%s", roomID)
	return r.client.Del(ctx, key).Err()
}

func (r *redisStateRepository) CacheMemberVerification(ctx context.Context, roomID, userID string) error {
	key := fmt.Sprintf("inox:room:members:%s:%s", roomID, userID)
	// 30-minute TTL as specified
	return r.client.Set(ctx, key, "1", 30*time.Minute).Err()
}

func (r *redisStateRepository) IsMemberVerified(ctx context.Context, roomID, userID string) (bool, error) {
	key := fmt.Sprintf("inox:room:members:%s:%s", roomID, userID)
	_, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check member verification: %w", err)
	}
	return true, nil
}

func (r *redisStateRepository) InvalidateMemberVerification(ctx context.Context, roomID, userID string) error {
	key := fmt.Sprintf("inox:room:members:%s:%s", roomID, userID)
	return r.client.Del(ctx, key).Err()
}
