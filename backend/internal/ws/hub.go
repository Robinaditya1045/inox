package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/inox/inox/backend/internal/domain"
	"github.com/inox/inox/backend/internal/observability"
	"github.com/inox/inox/backend/internal/room"
	"github.com/inox/inox/backend/internal/sfu"
	"github.com/pion/webrtc/v3"
)

// Hub maintains the pool of active WebSocket clients partitioned by room IDs,
// and coordinates real-time event broadcasting across connections.
type Hub struct {
	// Registered clients partitioned by room ID: rooms[roomID][client] = true
	rooms map[string]map[*Client]bool

	// Inbound registration requests from newly connected WebSocket clients.
	Register chan *Client

	// Inbound unregistration requests from disconnecting clients.
	Unregister chan *Client

	// Inbound events from clients waiting to be dispatched to room members.
	Broadcast chan *Event

	// Stop signals the Hub event loop to gracefully shut down and disconnect all clients.
	Stop chan struct{}

	// Optional chat service to persist real-time chat messages to PostgreSQL.
	chatService room.ChatService

	// Optional room service to query and update persisted room media URLs.
	roomService room.RoomService

	// Optional state repository to persist playback state to Redis.
	stateRepo room.StateRepository

	// Authoritative watch party playback state for active rooms.
	playbackStates map[string]*domain.RoomPlaybackState

	// Optional SFU manager to route WebRTC voice and screen share media streams.
	sfuManager *sfu.Manager
	// Optional event aggregator for background analytics persistence.
	eventAggregator *observability.EventAggregator

	// Redis Pub/Sub adapter for scaling horizontally across multiple nodes.
	redisEventBus *RedisEventBus

	// Cancels the Redis subscription for a room when it becomes empty.
	roomCancelFuncs map[string]context.CancelFunc

	// Optional live proxy, used to classify media URLs and to clamp a sync leader's
	// reported position against the real live edge.
	liveEdges LiveEdgeSource

	// liveStatusNotices carries upstream health changes from the proxy's HTTP
	// goroutines into this hub's single-threaded loop, which is the only place room
	// state may be touched.
	liveStatusNotices chan liveStatusNotice

	// sfuSignals carries locally gathered ICE candidates from Pion's goroutines
	// into the same single-threaded loop, for the same reason: delivering one
	// means walking h.rooms.
	sfuSignals chan *Event
}

// NewHub initializes a new Hub instance with buffered channels.
func NewHub() *Hub {
	return &Hub{
		rooms:             make(map[string]map[*Client]bool),
		playbackStates:    make(map[string]*domain.RoomPlaybackState),
		roomCancelFuncs:   make(map[string]context.CancelFunc),
		Register:          make(chan *Client),
		Unregister:        make(chan *Client),
		Broadcast:         make(chan *Event, 256),
		Stop:              make(chan struct{}),
		liveStatusNotices: make(chan liveStatusNotice, 64),
		sfuSignals:        make(chan *Event, 256),
	}
}

// Shutdown initiates a graceful teardown of the WebSocket Hub.
func (h *Hub) Shutdown() {
	close(h.Stop)
}

// SetChatService wires the persistence layer for archiving room chat messages.
func (h *Hub) SetChatService(cs room.ChatService) {
	h.chatService = cs
}

// SetRoomService wires the room management service for persisting media URL changes.
func (h *Hub) SetRoomService(rs room.RoomService) {
	h.roomService = rs
}

// SetStateRepository wires the Redis state repository for room playback state.
func (h *Hub) SetStateRepository(sr room.StateRepository) {
	h.stateRepo = sr
}

// SetSFUManager wires the Selective Forwarding Unit for voice chat and video routing.
func (h *Hub) SetSFUManager(mgr *sfu.Manager) {
	h.sfuManager = mgr
	mgr.SetStateHandler(h.broadcastVoiceRoster)
}

// SetEventAggregator wires the background telemetry event worker for historical analytics persistence.
func (h *Hub) SetEventAggregator(ea *observability.EventAggregator) {
	h.eventAggregator = ea
}

// SetRedisEventBus wires the Redis Pub/Sub event bus.
func (h *Hub) SetRedisEventBus(bus *RedisEventBus) {
	h.redisEventBus = bus
}

func (h *Hub) getOrCreatePlaybackState(roomID string) *domain.RoomPlaybackState {
	state, ok := h.playbackStates[roomID]
	if !ok {
		// 1. Try to load from Redis state repository
		if h.stateRepo != nil {
			if s, err := h.stateRepo.GetPlaybackState(context.Background(), roomID); err == nil && s != nil {
				// State persisted before this room last changed media may carry no
				// kind; the media URL is authoritative, so re-derive it.
				s.Kind = h.kindForMediaURL(s.MediaURL)
				h.playbackStates[roomID] = s
				return s
			}
		}

		// 2. Fallback to database media URL if not in Redis
		mediaURL := "https://media.w3.org/2010/05/bunny/movie.mp4"
		if h.roomService != nil {
			if r, err := h.roomService.GetRoomByID(context.Background(), roomID); err == nil && r != nil && r.CurrentMediaURL != "" {
				mediaURL = r.CurrentMediaURL
			}
		}
		state = &domain.RoomPlaybackState{
			MediaURL:         mediaURL,
			Kind:             h.kindForMediaURL(mediaURL),
			IsPlaying:        false,
			MediaTimeSeconds: 0,
			LastUpdated:      time.Now().UnixMilli(),
		}
		h.playbackStates[roomID] = state

		if h.stateRepo != nil {
			go func(rID string, st *domain.RoomPlaybackState) {
				_ = h.stateRepo.SetPlaybackState(context.Background(), rID, st)
			}(roomID, state)
		}
	}
	return state
}

// Run executes the central event loop of the Hub.
//
// By processing all map modifications and event routing inside a single goroutine via select,
// the Hub achieves 100% thread safety without requiring any sync.Mutex locks!
func (h *Hub) Run() {
	// Recovers leadership for rooms whose leader vanished without a clean handoff:
	// a crashed node, an evicted connection, or a claim that lapsed because the
	// leader stopped publishing.
	leadershipSweep := time.NewTicker(liveLeadershipSweepInterval)
	defer leadershipSweep.Stop()

	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case event := <-h.Broadcast:
			h.dispatchEvent(event)

		case notice := <-h.liveStatusNotices:
			h.broadcastLiveChannelStatus(notice)

		case signal := <-h.sfuSignals:
			if signal.TargetID != "" {
				h.sendToTarget(signal)
			} else {
				h.dispatchEvent(signal)
			}

		case <-leadershipSweep.C:
			h.sweepLiveLeadership()

		case <-h.Stop:
			h.shutdownAll()
			return
		}
	}
}

func (h *Hub) shutdownAll() {
	slog.Info("draining all active websocket connections across rooms...")
	if h.sfuManager != nil {
		h.sfuManager.Shutdown()
	}
	for roomID, clients := range h.rooms {
		for client := range clients {
			close(client.Send)
			if client.Conn != nil {
				_ = client.Conn.Close()
			}
		}
		if cancel, ok := h.roomCancelFuncs[roomID]; ok {
			cancel()
			delete(h.roomCancelFuncs, roomID)
		}
		delete(h.rooms, roomID)
	}
	slog.Info("websocket hub shutdown complete")
}

func (h *Hub) registerClient(client *Client) {
	if h.rooms[client.RoomID] == nil {
		h.rooms[client.RoomID] = make(map[*Client]bool)
		observability.Global().IncActiveRooms()

		if h.redisEventBus != nil {
			ctx, cancel := context.WithCancel(context.Background())
			h.roomCancelFuncs[client.RoomID] = cancel
			h.redisEventBus.Subscribe(ctx, client.RoomID)
		}
	}
	h.rooms[client.RoomID][client] = true
	observability.Global().IncActiveWSConnections()

	slog.Info("client joined room hub", "room_id", client.RoomID, "user_id", client.UserID, "username", client.Username)

	if h.eventAggregator != nil {
		h.eventAggregator.RecordEvent("user_joined", &client.RoomID, &client.UserID, map[string]any{
			"username": client.Username,
		})
	}

	// Notify existing participants that a new peer joined
	joinEvt := &Event{
		Type:       EventJoinRoom,
		RoomID:     client.RoomID,
		SenderID:   client.UserID,
		SenderName: client.Username,
	}
	h.dispatchEvent(joinEvt)

	// A live room needs someone to follow before the newcomer is told where the room
	// is, so that the sync payload below can name the leader.
	h.ensureLiveLeader(client.RoomID)

	// Send authoritative playback synchronization state directly to the newly joined peer
	state := h.getOrCreatePlaybackState(client.RoomID)
	syncPayload, _ := json.Marshal(VideoControlPayload{
		MediaURL:         state.MediaURL,
		Kind:             string(state.Kind),
		IsPlaying:        state.IsPlaying,
		MediaTimeSeconds: state.MediaTimeSeconds,
		LastUpdated:      state.LastUpdated,
		LeaderID:         state.LeaderID,
		LeaderName:       state.LeaderName,
		LivePosition:     state.LivePosition,
		AgeMillis:        state.LiveAgeMillis(time.Now().UnixMilli()),
	})
	syncEvt := &Event{
		Type:      EventSyncPlayback,
		RoomID:    client.RoomID,
		TargetID:  client.UserID,
		Payload:   syncPayload,
		Timestamp: time.Now().UnixMilli(),
	}
	h.dispatchEvent(syncEvt)

	// Who is already in voice. The roster is only broadcast when it changes, so
	// without this a late arrival sees an empty call until someone joins or leaves.
	h.sendVoiceRoster(client)
}

// sendVoiceRoster gives one client the room's current voice roster.
func (h *Hub) sendVoiceRoster(client *Client) {
	if h.sfuManager == nil {
		return
	}
	sfuRoom, err := h.sfuManager.GetRoom(client.RoomID)
	if err != nil {
		return
	}
	roster := sfuRoom.Roster()
	if len(roster) == 0 {
		return
	}
	payload, err := json.Marshal(SFUPeersPayload{Peers: roster})
	if err != nil {
		return
	}
	h.sendToTarget(&Event{
		Type:      EventSFUPeers,
		RoomID:    client.RoomID,
		TargetID:  client.UserID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	})
}

func (h *Hub) unregisterClient(client *Client) {
	roomClients, ok := h.rooms[client.RoomID]
	if !ok {
		return
	}

	if _, exists := roomClients[client]; exists {
		delete(roomClients, client)
		close(client.Send)
		observability.Global().DecActiveWSConnections()

		slog.Info("client left room hub", "room_id", client.RoomID, "user_id", client.UserID)

		if h.eventAggregator != nil {
			h.eventAggregator.RecordEvent("user_left", &client.RoomID, &client.UserID, map[string]any{
				"username": client.Username,
			})
		}

		// Clean up empty rooms from memory
		if len(roomClients) == 0 {
			// Drop the leadership claim before the room state goes: otherwise it
			// outlives the room, and the next person to join adopts a leader who
			// is not there instead of triggering an election.
			if state, ok := h.playbackStates[client.RoomID]; ok && state.LeaderID != "" {
				h.dropLeaderClaim(client.RoomID, state.LeaderID)
			}
			delete(h.rooms, client.RoomID)
			delete(h.playbackStates, client.RoomID)

			if cancel, ok := h.roomCancelFuncs[client.RoomID]; ok {
				cancel()
				delete(h.roomCancelFuncs, client.RoomID)
			}

			observability.Global().DecActiveRooms()
			if h.sfuManager != nil {
				h.sfuManager.RemoveRoom(client.RoomID)
			}
		} else {
			if h.sfuManager != nil {
				if sfuRoom, err := h.sfuManager.GetRoom(client.RoomID); err == nil {
					sfuRoom.RemovePeer(client.UserID)
				}
			}
			// If the departing client was driving a live room, elect a replacement
			// now rather than leaving the room uncorrected until the claim expires.
			h.releaseLiveLeadership(client)
			// Notify remaining room participants
			leaveEvt := &Event{
				Type:       EventLeaveRoom,
				RoomID:     client.RoomID,
				SenderID:   client.UserID,
				SenderName: client.Username,
			}
			h.dispatchEvent(leaveEvt)
		}
	}
}

// isSFUSignal reports whether an event is browser-to-SFU signaling, which is
// handled by the media layer instead of being fanned out to the room.
func isSFUSignal(t EventType) bool {
	switch t {
	case EventSFUOffer, EventSFUAnswer, EventSFUICECandidate, EventSFULeave, EventSFUState:
		return true
	}
	return false
}

func (h *Hub) dispatchEvent(event *Event) {
	if h.sfuManager != nil && event.SenderID != "" && isSFUSignal(event.Type) {
		h.handleSFUSignaling(event)
		return
	}

	// Server-originated events carry no sender. They are not user input -- the hub
	// itself produced them -- so there is no membership to verify, and running them
	// through the check below drops every one of them: GetRoomAndMember("") always
	// errors. This silently disabled SYNC_PLAYBACK on join and, later, LIVE_LEADER
	// and LIVE_STATUS, in any deployment where roomService is wired.
	if event.SenderID == "" {
		if h.redisEventBus != nil {
			if err := h.redisEventBus.Publish(context.Background(), event); err != nil {
				slog.Error("failed to publish server event to redis", "error", err)
			}
			return
		}
		h.dispatchLocal(event)
		return
	}

	// 1. Verify sender is still an active member using Redis cache to eliminate DB round-trips
	verified := false
	if h.stateRepo != nil {
		v, err := h.stateRepo.IsMemberVerified(context.Background(), event.RoomID, event.SenderID)
		if err == nil && v {
			verified = true
		}
	}

	// 2. Fallback to DB if not in cache (and populate cache)
	if !verified && h.roomService != nil {
		_, _, err := h.roomService.GetRoomAndMember(context.Background(), event.RoomID, event.SenderID)
		if err != nil {
			slog.Warn("dropping websocket event from unauthorized or kicked user", "user_id", event.SenderID, "room_id", event.RoomID)
			return
		}
		if h.stateRepo != nil {
			_ = h.stateRepo.CacheMemberVerification(context.Background(), event.RoomID, event.SenderID)
		}
	}

	if h.redisEventBus != nil {
		err := h.redisEventBus.Publish(context.Background(), event)
		if err != nil {
			slog.Error("failed to publish event to redis", "error", err)
		}
		// Event will loop back via Subscribe() and hit dispatchLocal
		return
	}

	h.dispatchLocal(event)
}

// dispatchLocal handles processing and sending an event to local connections.
func (h *Hub) dispatchLocal(event *Event) {
	roomClients, ok := h.rooms[event.RoomID]
	if !ok || len(roomClients) == 0 {
		return
	}

	// Live sync events are folded into room state before fan-out. A position report
	// is rewritten with a fresh age, and dropped outright when it did not come from
	// the room's leader.
	switch event.Type {
	case EventLivePosition:
		if !h.applyLivePosition(event) {
			return
		}
	case EventLiveLeader:
		h.applyLiveLeader(event)
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to serialize event payload for broadcast", "error", err)
		return
	}

	if event.Type == EventChatMessage {
		observability.Global().IncChatMessages()
		if h.chatService != nil && len(event.Payload) > 0 {
			var payload ChatPayload
			if err := json.Unmarshal(event.Payload, &payload); err == nil {
				_, _ = h.chatService.SaveMessage(context.Background(), event.RoomID, event.SenderID, event.SenderName, payload.Message)
			}
		}
	}

	// Update authoritative in-memory playback state when media control events occur
	var stateUpdated bool
	if event.Type == EventPlay {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.IsPlaying = true
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
			stateUpdated = true
		}
	} else if event.Type == EventPause {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.IsPlaying = false
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
			stateUpdated = true
		}
	} else if event.Type == EventSeek {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
			stateUpdated = true
		}
	} else if event.Type == EventChangeMedia {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.MediaURL = payload.MediaURL
			state.Kind = h.kindForMediaURL(payload.MediaURL)
			state.IsPlaying = false
			state.MediaTimeSeconds = 0
			state.LastUpdated = time.Now().UnixMilli()
			stateUpdated = true

			// Switching media invalidates any live position: the old one names a
			// segment in a stream nobody is watching any more.
			state.LivePosition = nil
			state.LiveReceivedAt = 0
			state.LeaderID = ""
			state.LeaderName = ""
			defer h.ensureLiveLeader(event.RoomID)
			if h.roomService != nil && payload.MediaURL != "" {
				go func(roomID, url string) {
					_ = h.roomService.UpdateRoomMediaURL(context.Background(), roomID, url)
				}(event.RoomID, payload.MediaURL)
			}
		}
		if h.eventAggregator != nil {
			h.eventAggregator.RecordEvent("media_control", &event.RoomID, &event.SenderID, map[string]any{
				"action": string(event.Type),
			})
		}
	}

	if stateUpdated && h.stateRepo != nil {
		go func(rID string, st *domain.RoomPlaybackState) {
			// make a copy for thread-safety before passing to goroutine
			stCopy := *st
			_ = h.stateRepo.SetPlaybackState(context.Background(), rID, &stCopy)
		}(event.RoomID, h.playbackStates[event.RoomID])
	}

	for client := range roomClients {
		// If event specifies a TargetID (e.g. peer-to-peer WebRTC SDP signaling), route only to that peer
		if event.TargetID != "" && client.UserID != event.TargetID {
			continue
		}

		select {
		case client.Send <- data:
		default:
			// If client's send buffer is full, evict the slow/unresponsive connection
			close(client.Send)
			delete(roomClients, client)
			observability.Global().IncWSEvictions()
			observability.Global().DecActiveWSConnections()
			// A leader with a full send buffer is exactly the client that gets
			// evicted, so leadership has to move on here as well as on a clean leave.
			evicted := client
			if h.eventAggregator != nil {
				h.eventAggregator.RecordEvent("eviction_occurred", &event.RoomID, &client.UserID, map[string]any{
					"reason": "send buffer overflow",
				})
			}
			if len(roomClients) == 0 {
				if state, ok := h.playbackStates[event.RoomID]; ok && state.LeaderID != "" {
					h.dropLeaderClaim(event.RoomID, state.LeaderID)
				}
				delete(h.rooms, event.RoomID)
				delete(h.playbackStates, event.RoomID)
				observability.Global().DecActiveRooms()
				if h.sfuManager != nil {
					h.sfuManager.RemoveRoom(event.RoomID)
				}
			} else {
				h.releaseLiveLeadership(evicted)
			}
		}
	}
}

func (h *Hub) handleSFUSignaling(event *Event) {
	sfuRoom := h.sfuManager.GetOrCreateRoom(event.RoomID)

	if event.Type == EventSFULeave {
		sfuRoom.RemovePeer(event.SenderID)
		return
	}

	peer, err := sfuRoom.GetPeer(event.SenderID)
	// A peer left over from a browser that vanished without hanging up cannot
	// answer a new call; an offer means the user is starting over, so replace it.
	if err == nil && event.Type == EventSFUOffer && !peer.Usable() {
		slog.Info("replacing dead sfu peer on rejoin", "user_id", event.SenderID, "room_id", event.RoomID)
		sfuRoom.RemovePeer(event.SenderID)
		peer, err = nil, sfu.ErrPeerNotFound
	}
	if err != nil {
		// Only an offer starts a call. Stray candidates or state updates from a peer
		// that has already hung up must not resurrect it.
		if event.Type != EventSFUOffer {
			return
		}
		peer, err = sfuRoom.NewPeer(event.SenderID, event.SenderID, event.SenderName)
		if err != nil {
			slog.Error("failed to initialize sfu peer", "user_id", event.SenderID, "error", err)
			return
		}
		// The browser can only reach the SFU via candidates the SFU tells it
		// about. CreateAnswer returns before gathering finishes, so the answer
		// SDP carries none of them and this callback is the only delivery path;
		// without it every peer connection stalls in "checking" and fails.
		// Wired before the offer is processed so no candidate is missed.
		h.trickleLocalCandidates(peer, event.RoomID, event.SenderID)
		// Subscriptions made after this peer is connected -- someone else joining,
		// or starting a screen share -- need a fresh offer from the SFU. This is how
		// it reaches the browser.
		h.signalSFUOffers(peer, event.RoomID, event.SenderID)
		sfuRoom.AddPeer(peer)
	}

	switch event.Type {
	case EventSFUOffer:
		var sdpPayload SFUSDOPayload
		if err := json.Unmarshal(event.Payload, &sdpPayload); err != nil {
			return
		}
		offer := webrtc.SessionDescription{SDP: sdpPayload.SDP, Type: webrtc.SDPTypeOffer}
		answer, err := peer.HandleOffer(offer)
		if err != nil {
			// A collision is expected and self-healing: the browser rolls its own
			// offer back, answers the SFU's, and offers again afterwards.
			if !errors.Is(err, sfu.ErrOfferCollision) {
				slog.Error("failed to answer sfu offer", "user_id", event.SenderID, "error", err)
			}
			return
		}
		respBytes, _ := json.Marshal(SFUSDOPayload{SDP: answer.SDP, Type: "answer"})
		h.sendToTarget(&Event{
			Type:      EventSFUAnswer,
			RoomID:    event.RoomID,
			TargetID:  event.SenderID,
			Payload:   respBytes,
			Timestamp: event.Timestamp,
		})
		// Tracks that were attached before this exchange but did not fit in the
		// answer -- a third person's microphone, a share already in progress -- are
		// flushed here, now that the connection is established.
		peer.Negotiate()

	case EventSFUAnswer:
		var sdpPayload SFUSDOPayload
		if err := json.Unmarshal(event.Payload, &sdpPayload); err != nil {
			return
		}
		if err := peer.HandleAnswer(webrtc.SessionDescription{SDP: sdpPayload.SDP, Type: webrtc.SDPTypeAnswer}); err != nil {
			slog.Error("failed to apply sfu answer", "user_id", event.SenderID, "error", err)
		}

	case EventSFUICECandidate:
		var candPayload SFUICECandidatePayload
		if err := json.Unmarshal(event.Payload, &candPayload); err != nil {
			return
		}
		_ = peer.AddICECandidate(webrtc.ICECandidateInit{
			Candidate:     candPayload.Candidate,
			SDPMid:        candPayload.SDPMid,
			SDPMLineIndex: candPayload.SDPMLineIndex,
		})

	case EventSFUState:
		var statePayload SFUStatePayload
		if err := json.Unmarshal(event.Payload, &statePayload); err != nil {
			return
		}
		sfuRoom.SetMuted(event.SenderID, statePayload.IsMuted)
	}
}

// signalSFUOffers installs the transport for server-initiated renegotiation.
// Pion produces these offers on its own goroutines, so they go through the same
// channel as gathered candidates rather than touching h.rooms directly.
func (h *Hub) signalSFUOffers(peer *sfu.Peer, roomID, targetID string) {
	peer.SetSignaler(func(sdp webrtc.SessionDescription) {
		payload, err := json.Marshal(SFUSDOPayload{SDP: sdp.SDP, Type: "offer"})
		if err != nil {
			slog.Error("failed to marshal sfu renegotiation offer", "user_id", targetID, "error", err)
			return
		}
		h.queueSFUSignal(&Event{
			Type:      EventSFUOffer,
			RoomID:    roomID,
			TargetID:  targetID,
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		}, "renegotiation offer")
	})
}

// broadcastVoiceRoster publishes a room's voice roster to everyone in it. The SFU
// calls this from its own goroutines whenever the roster changes.
func (h *Hub) broadcastVoiceRoster(roomID string, peers []sfu.PeerState) {
	payload, err := json.Marshal(SFUPeersPayload{Peers: peers})
	if err != nil {
		slog.Error("failed to marshal sfu voice roster", "room_id", roomID, "error", err)
		return
	}
	h.queueSFUSignal(&Event{
		Type:      EventSFUPeers,
		RoomID:    roomID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}, "voice roster")
}

// queueSFUSignal hands an event produced outside the hub loop to that loop.
func (h *Hub) queueSFUSignal(event *Event, kind string) {
	select {
	case h.sfuSignals <- event:
	default:
		slog.Error("dropped sfu signal; buffer full", "kind", kind, "room_id", event.RoomID, "target_id", event.TargetID)
	}
}

// trickleLocalCandidates forwards each ICE candidate the SFU gathers to the peer
// that offered, as an SFU_ICE_CANDIDATE event the client already knows how to apply.
func (h *Hub) trickleLocalCandidates(peer *sfu.Peer, roomID, targetID string) {
	peer.PC.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		// A nil candidate marks the end of gathering, not a candidate to send.
		if candidate == nil {
			return
		}

		init := candidate.ToJSON()
		payload, err := json.Marshal(SFUICECandidatePayload{
			Candidate:     init.Candidate,
			SDPMid:        init.SDPMid,
			SDPMLineIndex: init.SDPMLineIndex,
		})
		if err != nil {
			slog.Error("failed to marshal sfu ice candidate", "user_id", targetID, "error", err)
			return
		}

		h.queueSFUSignal(&Event{
			Type:      EventSFUICECandidate,
			RoomID:    roomID,
			TargetID:  targetID,
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		}, "ice candidate")
	})
}

func (h *Hub) sendToTarget(event *Event) {
	roomClients, ok := h.rooms[event.RoomID]
	if !ok {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	for client := range roomClients {
		if client.UserID == event.TargetID {
			select {
			case client.Send <- data:
			default:
			}
			break
		}
	}
}

// InspectRooms satisfies the observability.RoomInspector interface to report active watch party metrics.
func (h *Hub) InspectRooms() []observability.RoomTelemetry {
	var rooms []observability.RoomTelemetry
	for roomID, clients := range h.rooms {
		state := h.getOrCreatePlaybackState(roomID)
		sfuPeersCount := 0
		if h.sfuManager != nil {
			if sfuRoom, err := h.sfuManager.GetRoom(roomID); err == nil {
				sfuPeersCount = sfuRoom.GetPeerCount()
			}
		}
		qoe := observability.CalculateQoE(len(clients), state.IsPlaying, state.MediaURL, observability.Global().GetSnapshot().WSEvictionsTotal)
		rooms = append(rooms, observability.RoomTelemetry{
			RoomID:           roomID,
			ParticipantCount: len(clients),
			IsPlaying:        state.IsPlaying,
			MediaURL:         state.MediaURL,
			MediaTimeSeconds: state.MediaTimeSeconds,
			SFUPeersCount:    sfuPeersCount,
			QoEScore:         qoe,
		})
	}
	return rooms
}
