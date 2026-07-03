package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient initializes a production connection pool to Redis.
func NewRedisClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	// 1. Parse connection URL (supports redis://, rediss:// for SSL, and password authentication)
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse REDIS_URL: %w", err)
	}

	// 2. Configure production connection pool settings
	opts.PoolSize = 50        // Maximum number of concurrent socket connections
	opts.MinIdleConns = 10    // Keep 10 warm sockets ready for immediate traffic spikes
	opts.ConnMaxLifetime = 1 * time.Hour

	// 3. Initialize the Redis client instance
	client := redis.NewClient(opts)

	// 4. Fail-Fast Verification: PING the Redis server immediately.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis unreachable on startup ping: %w", err)
	}

	return client, nil
}
