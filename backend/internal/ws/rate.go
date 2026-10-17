package ws

import (
	"encoding/json"
	"math"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

// The speeds a room may play at. The player offers a fixed set inside this range;
// the bounds are what the hub enforces.
const (
	MinPlaybackRate = 0.25
	MaxPlaybackRate = 3
)

// applyPlaybackRate folds a SET_RATE into the room's state and reports whether it
// should be relayed. Live rooms keep normal speed -- their followers already use
// playbackRate to close drift -- so a rate change there is dropped.
func (h *Hub) applyPlaybackRate(event *Event) bool {
	var payload VideoControlPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return false
	}
	rate := payload.PlaybackRate
	if math.IsNaN(rate) || rate < MinPlaybackRate || rate > MaxPlaybackRate {
		return false
	}
	if math.IsNaN(payload.MediaTimeSeconds) || payload.MediaTimeSeconds < 0 {
		return false
	}

	state := h.getOrCreatePlaybackState(event.RoomID)
	if state.Kind == domain.MediaKindLive {
		return false
	}
	// The position is rebased here so that late joiners, who extrapolate from
	// LastUpdated, count the time before the change at the old speed.
	state.MediaTimeSeconds = payload.MediaTimeSeconds
	state.LastUpdated = time.Now().UnixMilli()
	if rate == 1 {
		rate = 0
	}
	state.PlaybackRate = rate
	h.persistPlaybackState(event.RoomID)
	return true
}
