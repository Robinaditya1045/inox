package media

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskTypeTranscodeMedia = "transcode:media"
)

// TranscodeMediaPayload represents the data required to process an uploaded media asset.
type TranscodeMediaPayload struct {
	AssetID string `json:"asset_id"`
	Key     string `json:"key"`
}

// QueueClient defines an interface for enqueueing asynchronous background jobs.
type QueueClient interface {
	EnqueueTranscodeMedia(ctx context.Context, assetID, key string) error
	Close() error
}

type asynqQueueClient struct {
	client *asynq.Client
}

// NewAsynqQueueClient creates a new queue client backed by Redis using asynq.
func NewAsynqQueueClient(redisOpt asynq.RedisConnOpt) QueueClient {
	return &asynqQueueClient{
		client: asynq.NewClient(redisOpt),
	}
}

func (q *asynqQueueClient) EnqueueTranscodeMedia(ctx context.Context, assetID, key string) error {
	payload, err := json.Marshal(TranscodeMediaPayload{
		AssetID: assetID,
		Key:     key,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	task := asynq.NewTask(TaskTypeTranscodeMedia, payload, asynq.MaxRetry(3), asynq.Timeout(2*time.Hour))
	
	info, err := q.client.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("failed to enqueue transcode task: %w", err)
	}

	slog.Info("enqueued media transcode task successfully", "task_id", info.ID, "asset_id", assetID, "queue", info.Queue)
	return nil
}

func (q *asynqQueueClient) Close() error {
	return q.client.Close()
}
