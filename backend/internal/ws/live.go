package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/inox/inox/backend/internal/domain"
)

const (
	// liveLeaderTTL bounds how long a leadership claim survives without a refresh.
	// The leader renews it on every position report, so this only ever expires when a
	// leader's process dies without running its unregister path. Followers keep
	// extrapolating at 1x meanwhile, so the gap costs accuracy rather than playback.
	liveLeaderTTL = 20 * time.Second

	// maxSegmentsBehindEdge is how far behind the live edge the room is allowed to
	// sit. A coarse guard, deliberately: its job is to stop a leader on a bad
	// connection parking fifty followers a minute in the past, not to enforce a
	// precise latency target.
	maxSegmentsBehindEdge = 12

	// liveLeadershipSweepInterval is how often orphaned rooms are checked. Slower
	// than the claim TTL so a sweep never races a leader that is simply between
	// reports.
	liveLeadershipSweepInterval = 30 * time.Second
)

// LiveEdgeSource lets the hub ask the live proxy about a channel without depending
// on the live package's types. The proxy already parses every playlist it serves in
// order to rewrite URIs, so the edge sequence is free information.
type LiveEdgeSource interface {
	// EdgeSequenceForURL reports the newest media sequence published for the channel
	// a room media URL points at.
	EdgeSequenceForURL(masterURL string) (int64, bool)
	// IsLiveURL reports whether a media URL addresses a live channel.
	IsLiveURL(masterURL string) bool
}

// SetLiveEdgeSource wires the live proxy so the hub can classify media URLs and
// clamp a leader's reported position against the true live edge.
func (h *Hub) SetLiveEdgeSource(src LiveEdgeSource) {
	h.liveEdges = src
}

// kindForMediaURL classifies a room's media URL. Live channels are served from our
// own proxy under a recognisable path, so this needs no database round-trip inside
// the hub's event loop.
func (h *Hub) kindForMediaURL(mediaURL string) domain.MediaKind {
	if h.liveEdges != nil && h.liveEdges.IsLiveURL(mediaURL) {
		return domain.MediaKindLive
	}
	return domain.MediaKindVOD
}

// leaderRank orders candidates for sync leadership. Guests rank last but remain
// eligible: a room of nothing but guests should still stay in sync with each other,
// and having no leader at all would leave it permanently uncorrected.
func leaderRank(role domain.Role) int {
	switch role {
	case domain.RoleOwner:
		return 0
	case domain.RoleModerator:
		return 1
	case domain.RoleMember:
		return 2
	default:
		return 3
	}
}

// bestLeaderCandidate picks this process's nominee: highest role, then whoever has
// been connected longest, then user ID so the choice is deterministic when a room is
// spread across processes.
func (h *Hub) bestLeaderCandidate(roomID string) *Client {
	var best *Client
	for client := range h.rooms[roomID] {
		// A client that has told us it cannot produce stream positions -- Safari and
		// iOS play HLS natively, with no hls.js instance and so no fragment list --
		// would hold the claim and publish nothing, leaving the room uncorrected.
		if !client.CanLead {
			continue
		}
		if best == nil {
			best = client
			continue
		}
		switch {
		case leaderRank(client.Role) != leaderRank(best.Role):
			if leaderRank(client.Role) < leaderRank(best.Role) {
				best = client
			}
		case !client.JoinedAt.Equal(best.JoinedAt):
			if client.JoinedAt.Before(best.JoinedAt) {
				best = client
			}
		case client.UserID < best.UserID:
			best = client
		}
	}
	return best
}

func (h *Hub) hasLocalClient(roomID, userID string) bool {
	for client := range h.rooms[roomID] {
		if client.UserID == userID {
			return true
		}
	}
	return false
}

// ensureLiveLeader makes sure a live room is following someone, electing a new
// leader if the previous one is gone.
//
// Redis arbitrates the claim so that two processes serving the same room cannot each
// decide they are in charge and fight over the room's position. Without Redis this
// degrades to a purely local election, which is correct for a single-process
// deployment and the best available answer otherwise.
func (h *Hub) ensureLiveLeader(roomID string) {
	state := h.getOrCreatePlaybackState(roomID)
	if state.Kind != domain.MediaKindLive {
		return
	}
	// The sitting leader is still here; nothing to do.
	if state.LeaderID != "" && h.hasLocalClient(roomID, state.LeaderID) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if h.stateRepo != nil {
		holder, holderName, err := h.stateRepo.GetLiveLeader(ctx, roomID)
		if err == nil && holder != "" && !h.hasLocalClient(roomID, holder) {
			// Another process is leading this room. Adopt its choice silently: that
			// process already announced it, and re-announcing would flap the UI.
			state.LeaderID = holder
			state.LeaderName = holderName
			return
		}
	}

	candidate := h.bestLeaderCandidate(roomID)
	if candidate == nil {
		return
	}
	if h.stateRepo != nil {
		claimed, err := h.stateRepo.TryClaimLiveLeader(ctx, roomID, candidate.UserID, candidate.Username, liveLeaderTTL)
		if err != nil {
			slog.Warn("failed to claim live sync leadership", "room_id", roomID, "error", err)
		} else if !claimed {
			// Another process won the race. Adopt whoever it picked rather than
			// returning with LeaderID empty, which would leave this node unable to
			// tell a leader's position report from a follower's.
			if holder, holderName, err := h.stateRepo.GetLiveLeader(ctx, roomID); err == nil && holder != "" {
				state.LeaderID = holder
				state.LeaderName = holderName
			}
			return
		}
	}

	state.LeaderID = candidate.UserID
	state.LeaderName = candidate.Username
	slog.Info("elected live sync leader", "room_id", roomID, "user_id", candidate.UserID, "username", candidate.Username, "role", candidate.Role)

	payload, _ := json.Marshal(LiveLeaderPayload{LeaderID: candidate.UserID, LeaderName: candidate.Username})
	h.dispatchEvent(&Event{
		Type:      EventLiveLeader,
		RoomID:    roomID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	})
}

// releaseLiveLeadership hands leadership on when the current leader disconnects.
// The replacement is seeded with the room's last known position before it starts
// reporting, so a handoff does not yank everyone to wherever the new leader happens
// to be sitting.
func (h *Hub) releaseLiveLeadership(client *Client) {
	state, ok := h.playbackStates[client.RoomID]
	if !ok || state.Kind != domain.MediaKindLive || state.LeaderID != client.UserID {
		return
	}
	state.LeaderID = ""
	state.LeaderName = ""
	h.dropLeaderClaim(client.RoomID, client.UserID)
	h.ensureLiveLeader(client.RoomID)
}

// dropLeaderClaim gives up the Redis claim for a user who is no longer leading.
//
// Separate from releaseLiveLeadership because the last client leaving a room deletes
// its playback state first, and an unreleased claim would then outlive the room: the
// next person to join adopts a leader who is not there, and no election ever runs.
func (h *Hub) dropLeaderClaim(roomID, userID string) {
	if h.stateRepo == nil || userID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.stateRepo.ReleaseLiveLeader(ctx, roomID, userID); err != nil {
		slog.Warn("failed to release live sync leadership", "room_id", roomID, "error", err)
	}
}

// MarkCannotLead records that a client is unable to act as sync leader, and hands the
// role on if it currently holds it.
func (h *Hub) markCannotLead(roomID, userID string) {
	for client := range h.rooms[roomID] {
		if client.UserID == userID {
			client.CanLead = false
		}
	}
	state, ok := h.playbackStates[roomID]
	if !ok || state.LeaderID != userID {
		return
	}
	slog.Info("live sync leader cannot publish positions; re-electing", "room_id", roomID, "user_id", userID)
	state.LeaderID = ""
	state.LeaderName = ""
	h.dropLeaderClaim(roomID, userID)
	h.ensureLiveLeader(roomID)
}

// sweepLiveLeadership re-elects for any live room whose leader has gone without a
// clean handoff: a node that crashed, a connection evicted for being too slow, or a
// leader whose claim lapsed because it stopped publishing.
func (h *Hub) sweepLiveLeadership() {
	for roomID, state := range h.playbackStates {
		if state.Kind != domain.MediaKindLive || len(h.rooms[roomID]) == 0 {
			continue
		}
		if state.LeaderID != "" && h.hasLocalClient(roomID, state.LeaderID) {
			continue
		}
		if h.stateRepo != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			holder, _, err := h.stateRepo.GetLiveLeader(ctx, roomID)
			cancel()
			// A claim still held by a client on another node is fine; leave it be.
			if err == nil && holder != "" {
				continue
			}
		} else if state.LeaderID != "" {
			// Without Redis there is nothing to expire, so a leader that is no longer
			// a local client is simply gone.
			state.LeaderID = ""
			state.LeaderName = ""
		}
		h.ensureLiveLeader(roomID)
	}
}

// applyLiveLeader records a leadership announcement. Every process running the room
// needs it, so that each can tell a leader's report from a follower's.
func (h *Hub) applyLiveLeader(event *Event) {
	var payload LiveLeaderPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return
	}
	state := h.getOrCreatePlaybackState(event.RoomID)
	state.LeaderID = payload.LeaderID
	state.LeaderName = payload.LeaderName
}

// applyLivePosition validates a position report and folds it into room state,
// returning false when the event should not be fanned out.
func (h *Hub) applyLivePosition(event *Event) bool {
	var payload LivePositionPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return false
	}

	state := h.getOrCreatePlaybackState(event.RoomID)
	if state.Kind != domain.MediaKindLive {
		return false
	}
	// Only the leader drives the room, and an unknown leader means nobody does.
	// Accepting reports while LeaderID is empty -- which happens on a node that lost
	// the claim race -- would let any follower push the room to a position of its
	// choosing, and the Redis bus would carry that to every other node.
	if state.LeaderID == "" || event.SenderID != state.LeaderID {
		return false
	}

	// The leader is telling us it cannot produce positions at all.
	if payload.Unable {
		h.markCannotLead(event.RoomID, event.SenderID)
		return false
	}

	if h.stateRepo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, _ = h.stateRepo.RefreshLiveLeader(ctx, event.RoomID, event.SenderID, liveLeaderTTL)
		cancel()
	}

	// A stalled leader is buffering, so its playhead has stopped advancing while the
	// broadcast has not. Accepting the frozen reading would drag every follower
	// backwards and then forwards again when it recovers. Followers keep
	// extrapolating from the last good position instead.
	if payload.Stalled {
		return false
	}

	position := payload.Position
	if edge, ok := h.liveEdgeSequence(state.MediaURL); ok {
		if position.MediaSequence > edge {
			position.MediaSequence = edge
			position.OffsetSeconds = 0
		} else if floor := edge - maxSegmentsBehindEdge; position.MediaSequence < floor {
			slog.Debug("clamping live sync leader position to the DVR window",
				"room_id", event.RoomID, "reported", position.MediaSequence, "floor", floor)
			position.MediaSequence = floor
			position.OffsetSeconds = 0
		}
	}

	now := time.Now().UnixMilli()
	state.LivePosition = &position
	state.LiveReceivedAt = now
	state.IsPlaying = true
	state.LastUpdated = now

	// Rewrite the outgoing payload: age zero because this is being fanned out now,
	// and the leader stamped so followers can tell whose playhead they are tracking.
	outgoing, err := json.Marshal(LivePositionPayload{
		Position:  position,
		LeaderID:  state.LeaderID,
		AgeMillis: 0,
	})
	if err != nil {
		return false
	}
	event.Payload = outgoing
	return true
}

func (h *Hub) liveEdgeSequence(mediaURL string) (int64, bool) {
	if h.liveEdges == nil {
		return 0, false
	}
	return h.liveEdges.EdgeSequenceForURL(mediaURL)
}

// liveStatusNotice reports an upstream health change for one live channel.
type liveStatusNotice struct {
	Slug    string
	Status  string
	Message string
}

// NotifyLiveChannelStatus reports that a live channel's upstream became healthy or
// unhealthy. Safe to call from any goroutine: the notice is handed to the hub loop,
// which is the only place room state may be read or written.
//
// Dropping the notice when the buffer is full is deliberate. This is advisory
// information for the UI, and blocking an HTTP handler on the hub loop to deliver it
// would turn a degraded upstream into a stalled request.
func (h *Hub) NotifyLiveChannelStatus(slug, status, message string) {
	select {
	case h.liveStatusNotices <- liveStatusNotice{Slug: slug, Status: status, Message: message}:
	default:
		slog.Warn("dropped live channel status notice; hub queue full", "slug", slug)
	}
}

// broadcastLiveChannelStatus tells every room watching a channel that its upstream
// changed state, so a dead broadcast shows an explanation instead of a frozen frame.
func (h *Hub) broadcastLiveChannelStatus(notice liveStatusNotice) {
	marker := "/" + notice.Slug + "/master.m3u8"
	for roomID, state := range h.playbackStates {
		if state.Kind != domain.MediaKindLive || !strings.Contains(state.MediaURL, marker) {
			continue
		}
		if len(h.rooms[roomID]) == 0 {
			continue
		}
		payload, err := json.Marshal(LiveStatusPayload{Status: notice.Status, Message: notice.Message})
		if err != nil {
			return
		}
		h.dispatchEvent(&Event{
			Type:      EventLiveStatus,
			RoomID:    roomID,
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		})
	}
}
