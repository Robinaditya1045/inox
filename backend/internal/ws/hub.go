package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/inox/inox/backend/internal/observability"
	"github.com/inox/inox/backend/internal/room"
	"github.com/inox/inox/backend/internal/sfu"
	"github.com/pion/webrtc/v3"
)

// RoomPlaybackState maintains the authoritative real-time media state of an active watch party room.
type RoomPlaybackState struct {
	MediaURL         string  `json:"media_url"`
	IsPlaying        bool    `json:"is_playing"`
	MediaTimeSeconds float64 `json:"media_time_seconds"`
	LastUpdated      int64   `json:"last_updated"` // Epoch millis
}

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

	// Authoritative in-memory watch party playback state for active rooms.
	playbackStates map[string]*RoomPlaybackState

	// Optional SFU manager to route WebRTC voice and screen share media streams.
	sfuManager *sfu.Manager
	// Optional event aggregator for background analytics persistence.
	eventAggregator *observability.EventAggregator
}

// NewHub initializes a new Hub instance with buffered channels.
func NewHub() *Hub {
	return &Hub{
		rooms:          make(map[string]map[*Client]bool),
		playbackStates: make(map[string]*RoomPlaybackState),
		Register:       make(chan *Client),
		Unregister:     make(chan *Client),
		Broadcast:      make(chan *Event, 256),
		Stop:           make(chan struct{}),
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

// SetSFUManager wires the Selective Forwarding Unit for voice chat and video routing.
func (h *Hub) SetSFUManager(mgr *sfu.Manager) {
	h.sfuManager = mgr
}

// SetEventAggregator wires the background telemetry event worker for historical analytics persistence.
func (h *Hub) SetEventAggregator(ea *observability.EventAggregator) {
	h.eventAggregator = ea
}

func (h *Hub) getOrCreatePlaybackState(roomID string) *RoomPlaybackState {
	state, ok := h.playbackStates[roomID]
	if !ok {
		mediaURL := "https://media.w3.org/2010/05/bunny/movie.mp4"
		if h.roomService != nil {
			if r, err := h.roomService.GetRoomByID(context.Background(), roomID); err == nil && r != nil && r.CurrentMediaURL != "" {
				mediaURL = r.CurrentMediaURL
			}
		}
		state = &RoomPlaybackState{
			MediaURL:         mediaURL,
			IsPlaying:        false,
			MediaTimeSeconds: 0,
			LastUpdated:      time.Now().UnixMilli(),
		}
		h.playbackStates[roomID] = state
	}
	return state
}

// Run executes the central event loop of the Hub.
//
// By processing all map modifications and event routing inside a single goroutine via select,
// the Hub achieves 100% thread safety without requiring any sync.Mutex locks!
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case event := <-h.Broadcast:
			h.dispatchEvent(event)

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
		delete(h.rooms, roomID)
	}
	slog.Info("websocket hub shutdown complete")
}

func (h *Hub) registerClient(client *Client) {
	if h.rooms[client.RoomID] == nil {
		h.rooms[client.RoomID] = make(map[*Client]bool)
		observability.Global().IncActiveRooms()
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

	// Send authoritative playback synchronization state directly to the newly joined peer
	state := h.getOrCreatePlaybackState(client.RoomID)
	syncPayload, _ := json.Marshal(VideoControlPayload{
		MediaURL:         state.MediaURL,
		IsPlaying:        state.IsPlaying,
		MediaTimeSeconds: state.MediaTimeSeconds,
		LastUpdated:      state.LastUpdated,
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
			delete(h.rooms, client.RoomID)
			delete(h.playbackStates, client.RoomID)
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

	roomClients, ok := h.rooms[event.RoomID]
	if !ok || len(roomClients) == 0 {
		return
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
	if event.Type == EventPlay {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.IsPlaying = true
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
		}
	} else if event.Type == EventPause {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.IsPlaying = false
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
		}
	} else if event.Type == EventSeek {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.MediaTimeSeconds = payload.MediaTimeSeconds
			state.LastUpdated = time.Now().UnixMilli()
		}
	} else if event.Type == EventChangeMedia {
		var payload VideoControlPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state := h.getOrCreatePlaybackState(event.RoomID)
			state.MediaURL = payload.MediaURL
			state.IsPlaying = false
			state.MediaTimeSeconds = 0
			state.LastUpdated = time.Now().UnixMilli()
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
			if h.eventAggregator != nil {
				h.eventAggregator.RecordEvent("eviction_occurred", &event.RoomID, &client.UserID, map[string]any{
					"reason": "send buffer overflow",
				})
			}
			if len(roomClients) == 0 {
				delete(h.rooms, event.RoomID)
				delete(h.playbackStates, event.RoomID)
				observability.Global().DecActiveRooms()
				if h.sfuManager != nil {
					h.sfuManager.RemoveRoom(event.RoomID)
				}
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

