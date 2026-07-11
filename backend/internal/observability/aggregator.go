package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"sync"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// EventAggregator buffers high-frequency telemetry events and flushes them to PostgreSQL
// asynchronously in batches to ensure zero latency impact on real-time WebSocket signaling.
type EventAggregator struct {
	repo      Repository
	eventChan chan *domain.AnalyticsEvent
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewEventAggregator initializes a new asynchronous telemetry event worker.
func NewEventAggregator(repo Repository) *EventAggregator {
	return &EventAggregator{
		repo:      repo,
		eventChan: make(chan *domain.AnalyticsEvent, 1000),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the background batching worker goroutine.
func (ea *EventAggregator) Start() {
	ea.wg.Add(1)
	go ea.worker()
}

// Stop gracefully shuts down the aggregator, flushing any remaining buffered events.
func (ea *EventAggregator) Stop() {
	close(ea.stopChan)
	ea.wg.Wait()
}

// RecordEvent queues an analytics milestone non-blockingly. If the 1000-buffer is full,
// the event is dropped to prevent backpressure on real-time room operations.
func (ea *EventAggregator) RecordEvent(eventType string, roomID, userID *string, metadata map[string]any) {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	event := &domain.AnalyticsEvent{
		ID:        generateID(),
		EventType: eventType,
		RoomID:    roomID,
		UserID:    userID,
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	}

	select {
	case ea.eventChan <- event:
		// Queued successfully
	default:
		slog.Warn("analytics event buffer full; dropping event to preserve real-time latency", "type", eventType)
	}
}

func (ea *EventAggregator) worker() {
	defer ea.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var buffer []*domain.AnalyticsEvent
	ctx := context.Background()

	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if ea.repo != nil {
			if err := ea.repo.SaveEventsBatch(ctx, buffer); err != nil {
				slog.Error("failed to flush analytics events batch to postgres", "error", err, "count", len(buffer))
			}
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case event := <-ea.eventChan:
			buffer = append(buffer, event)
			if len(buffer) >= 100 {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-ea.stopChan:
			// Drain remaining events in channel before exiting
			for {
				select {
				case event := <-ea.eventChan:
					buffer = append(buffer, event)
				default:
					flush()
					return
				}
			}
		}
	}
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
