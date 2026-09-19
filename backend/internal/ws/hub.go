package ws

import (
	"context"
	"encoding/json"
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

func (h *Hub) dispatchEvent(event *Event) {
	if h.sfuManager != nil && (event.Type == EventSFUOffer || event.Type == EventSFUICECandidate) {
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
	peer, err := sfuRoom.GetPeer(event.SenderID)
	if err != nil {
		peer, err = sfu.NewPeer(event.SenderID, event.SenderID, event.SenderName, event.RoomID, nil)
		if err != nil {
			slog.Error("failed to initialize sfu peer", "user_id", event.SenderID, "error", err)
			return
		}
		sfuRoom.AddPeer(peer)
	}

	if event.Type == EventSFUOffer {
		var sdpPayload SFUSDOPayload
		if err := json.Unmarshal(event.Payload, &sdpPayload); err != nil {
			return
		}
		offer := webrtc.SessionDescription{SDP: sdpPayload.SDP, Type: webrtc.SDPTypeOffer}
		if err := peer.SetRemoteDescription(offer); err != nil {
			return
		}
		answer, err := peer.CreateAnswer()
		if err != nil {
			return
		}
		respBytes, _ := json.Marshal(SFUSDOPayload{SDP: answer.SDP, Type: "answer"})
		answerEvt := &Event{
			Type:      EventSFUAnswer,
			RoomID:    event.RoomID,
			TargetID:  event.SenderID,
			Payload:   respBytes,
			Timestamp: event.Timestamp,
		}
		h.sendToTarget(answerEvt)
	} else if event.Type == EventSFUICECandidate {
		var candPayload SFUICECandidatePayload
		if err := json.Unmarshal(event.Payload, &candPayload); err != nil {
			return
		}
		_ = peer.AddICECandidate(webrtc.ICECandidateInit{
			Candidate:     candPayload.Candidate,
			SDPMid:        candPayload.SDPMid,
			SDPMLineIndex: candPayload.SDPMLineIndex,
		})
	}
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
