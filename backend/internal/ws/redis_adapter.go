package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// RedisEventBus provides Pub/Sub capabilities for WebSocket events across multiple instances.
type RedisEventBus struct {
	client *redis.Client
	hub    *Hub
}

// NewRedisEventBus initializes a new RedisEventBus.
func NewRedisEventBus(client *redis.Client) *RedisEventBus {
	return &RedisEventBus{
		client: client,
	}
}

// SetHub binds the local Hub to the event bus.
func (r *RedisEventBus) SetHub(hub *Hub) {
	r.hub = hub
}

// Publish serializes and publishes an event to the room's Redis channel.
func (r *RedisEventBus) Publish(ctx context.Context, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	channel := fmt.Sprintf("inox:room:events:%s", event.RoomID)
	return r.client.Publish(ctx, channel, data).Err()
}

// Subscribe listens to the room's Redis channel and routes events to the local Hub.
func (r *RedisEventBus) Subscribe(ctx context.Context, roomID string) {
	channel := fmt.Sprintf("inox:room:events:%s", roomID)
	pubsub := r.client.Subscribe(ctx, channel)

	go func() {
		defer pubsub.Close()
		ch := pubsub.Channel()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event Event
				if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
					slog.Error("failed to unmarshal event from redis", "error", err)
					continue
				}

				if r.hub != nil {
					r.hub.dispatchLocal(&event)
				}
			}
		}
	}()
}
