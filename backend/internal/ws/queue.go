package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/inox/inox/backend/internal/domain"
)

// Bounds on a queue a member may start. The queue is stored with the room's
// playback state and sent whole to every member on each change, so it is capped
// at the size of the largest playlist.
const (
	maxQueueItems    = 500
	maxQueueTitle    = 100
	maxQueueItemText = 255
	maxQueueURL      = 2048
)

// isQueueCommand reports whether an event is a member asking the hub to change
// the room's queue, which the hub answers with events of its own rather than
// fanning the command out.
func isQueueCommand(t EventType) bool {
	switch t {
	case EventQueueSet, EventQueueAdvance, EventQueueClear:
		return true
	}
	return false
}

// applyQueueCommand folds a queue command into the room's state and tells the
// room what changed. It runs inside dispatchLocal, so with the Redis event bus
// every process applies the same command to its own copy of the state; that
// only stays consistent because nothing here depends on anything but the
// command and the state it is applied to.
func (h *Hub) applyQueueCommand(event *Event) {
	state := h.getOrCreatePlaybackState(event.RoomID)

	var target string
	switch event.Type {
	case EventQueueSet:
		var payload QueueSetPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		queue, ok := sanitizeQueue(payload.Queue)
		if !ok {
			slog.Warn("dropping invalid play queue", "room_id", event.RoomID, "user_id", event.SenderID)
			return
		}
		state.Queue = queue
		target = queue.Items[queue.Index].MediaURL

	case EventQueueAdvance:
		var payload QueueAdvancePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return
		}
		// A stale advance -- another member's video ended first -- or one for a
		// queue that has since been replaced or cleared.
		if state.Queue == nil || state.Queue.Index != payload.FromIndex {
			return
		}
		to := payload.FromIndex + 1
		if payload.ToIndex != nil {
			to = *payload.ToIndex
		}
		// Past the end means the queue has finished. It stays, so the room can
		// still see and replay it; the clients pause on the last item.
		if to < 0 || to >= len(state.Queue.Items) || to == payload.FromIndex {
			return
		}
		next := *state.Queue
		next.Index = to
		state.Queue = &next
		target = next.Items[to].MediaURL

	case EventQueueClear:
		if state.Queue == nil {
			return
		}
		state.Queue = nil
		h.persistPlaybackState(event.RoomID)
		h.dispatchLocal(queueStateEvent(event.RoomID, nil))
		return

	default:
		return
	}

	// The video change goes out as an ordinary CHANGE_MEDIA, so it is applied to
	// the room state and seen by clients exactly as a pick from the library is.
	// It starts playing: moving on to the next item should not need someone to
	// press play, and starting a playlist is a request to play it.
	payload, _ := json.Marshal(VideoControlPayload{MediaURL: target, IsPlaying: true})
	h.dispatchLocal(&Event{
		Type:       EventChangeMedia,
		RoomID:     event.RoomID,
		SenderID:   event.SenderID,
		SenderName: event.SenderName,
		Payload:    payload,
		Timestamp:  time.Now().UnixMilli(),
	})
	h.dispatchLocal(queueStateEvent(event.RoomID, state.Queue))
}

// reconcileQueue keeps a room's queue in step with media changed by other means:
// picking a video from the library that is in the queue moves the queue to it,
// and picking anything else means the room has stopped playing through it. The
// queue is returned unchanged (the same pointer) when the media is already the
// current item.
func reconcileQueue(queue *domain.PlaybackQueue, mediaURL string) *domain.PlaybackQueue {
	if queue == nil {
		return nil
	}
	if queue.Items[queue.Index].MediaURL == mediaURL {
		return queue
	}
	for i, item := range queue.Items {
		if item.MediaURL == mediaURL {
			next := *queue
			next.Index = i
			return &next
		}
	}
	return nil
}

// sanitizeQueue validates a queue a member asked to start and trims its display
// text, returning a copy the hub can own.
func sanitizeQueue(in domain.PlaybackQueue) (*domain.PlaybackQueue, bool) {
	if len(in.Items) == 0 || len(in.Items) > maxQueueItems {
		return nil, false
	}
	if in.Index < 0 || in.Index >= len(in.Items) {
		return nil, false
	}
	out := &domain.PlaybackQueue{
		PlaylistID: truncateRunes(in.PlaylistID, maxQueueItemText),
		Title:      truncateRunes(strings.TrimSpace(in.Title), maxQueueTitle),
		Index:      in.Index,
		Items:      make([]domain.QueueItem, len(in.Items)),
	}
	for i, item := range in.Items {
		url := strings.TrimSpace(item.MediaURL)
		if url == "" || len(url) > maxQueueURL || len(item.ThumbnailURL) > maxQueueURL {
			return nil, false
		}
		out.Items[i] = domain.QueueItem{
			MediaURL:        url,
			Title:           truncateRunes(strings.TrimSpace(item.Title), maxQueueItemText),
			ThumbnailURL:    item.ThumbnailURL,
			MediaAssetID:    truncateRunes(item.MediaAssetID, maxQueueItemText),
			DurationSeconds: max(item.DurationSeconds, 0),
		}
	}
	return out, true
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}

// queueStateEvent announces a room's queue. It is a server event, carrying no
// sender, and is delivered with dispatchLocal: each process derives it from the
// command it has just applied, so it must not be published to the bus again.
func queueStateEvent(roomID string, queue *domain.PlaybackQueue) *Event {
	payload, _ := json.Marshal(QueueStatePayload{Queue: queue})
	return &Event{
		Type:      EventQueueState,
		RoomID:    roomID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}
}

// persistPlaybackState saves a room's playback state to Redis in the background.
// The copy is taken here, on the loop, because the loop keeps changing the state
// after this returns. The queue in it may be shared, as queues are never
// modified in place.
func (h *Hub) persistPlaybackState(roomID string) {
	if h.stateRepo == nil {
		return
	}
	state, ok := h.playbackStates[roomID]
	if !ok {
		return
	}
	snapshot := *state
	go func() {
		_ = h.stateRepo.SetPlaybackState(context.Background(), roomID, &snapshot)
	}()
}
