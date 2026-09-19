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

	// Live sync leadership.
	//
	// A live room follows one member's playhead, and exactly one process must decide
	// whose. Holding that decision in each hub's memory would let two nodes elect
	// different leaders for the same room and fight over the room's position, so the
	// claim is arbitrated by Redis. The TTL is what recovers leadership when a node
	// dies without ever running its unregister path.
	TryClaimLiveLeader(ctx context.Context, roomID, userID, username string, ttl time.Duration) (bool, error)
	RefreshLiveLeader(ctx context.Context, roomID, userID string, ttl time.Duration) (bool, error)
	// GetLiveLeader returns the holder's user ID and display name, or empty strings
	// when the room has no leader. The name is stored alongside the ID so a node that
	// adopts another node's leader can still say who the room is following.
	GetLiveLeader(ctx context.Context, roomID string) (string, string, error)
	ReleaseLiveLeader(ctx context.Context, roomID, userID string) error
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

func liveLeaderKey(roomID string) string {
	return fmt.Sprintf("inox:room:live_leader:%s", roomID)
}

// liveLeaderRecord is the stored claim. JSON rather than a delimiter because a
// username may contain anything a user can type.
type liveLeaderRecord struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// TryClaimLiveLeader takes leadership only if the room currently has none.
func (r *redisStateRepository) TryClaimLiveLeader(ctx context.Context, roomID, userID, username string, ttl time.Duration) (bool, error) {
	value, err := json.Marshal(liveLeaderRecord{UserID: userID, Username: username})
	if err != nil {
		return false, fmt.Errorf("failed to encode live leader claim: %w", err)
	}
	ok, err := r.client.SetNX(ctx, liveLeaderKey(roomID), value, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to claim live leader: %w", err)
	}
	return ok, nil
}

// RefreshLiveLeader extends the current holder's claim. It reports false if someone
// else now holds it, which tells the caller to stop acting as leader.
func (r *redisStateRepository) RefreshLiveLeader(ctx context.Context, roomID, userID string, ttl time.Duration) (bool, error) {
	holder, _, err := r.GetLiveLeader(ctx, roomID)
	if err != nil {
		return false, err
	}
	if holder != userID {
		return false, nil
	}
	return r.client.Expire(ctx, liveLeaderKey(roomID), ttl).Result()
}

// GetLiveLeader returns the current leader's user ID and name, or empty strings if
// the room has none.
func (r *redisStateRepository) GetLiveLeader(ctx context.Context, roomID string) (string, string, error) {
	raw, err := r.client.Get(ctx, liveLeaderKey(roomID)).Bytes()
	if err == redis.Nil {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to read live leader: %w", err)
	}
	var record liveLeaderRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		// A claim written by an older build stored the bare user ID.
		return string(raw), "", nil
	}
	return record.UserID, record.Username, nil
}

// ReleaseLiveLeader gives up leadership, but only if this user still holds it.
// Releasing unconditionally would let a departing straggler evict whoever replaced it.
func (r *redisStateRepository) ReleaseLiveLeader(ctx context.Context, roomID, userID string) error {
	holder, _, err := r.GetLiveLeader(ctx, roomID)
	if err != nil || holder != userID {
		return err
	}
	return r.client.Del(ctx, liveLeaderKey(roomID)).Err()
}
