package ws

import (
	"encoding/json"
	"log/slog"
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
}

// NewHub initializes a new Hub instance with buffered channels.
func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *Event, 256),
		Stop:       make(chan struct{}),
	}
}

// Shutdown initiates a graceful teardown of the WebSocket Hub.
func (h *Hub) Shutdown() {
	close(h.Stop)
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
	}
	h.rooms[client.RoomID][client] = true

	slog.Info("client joined room hub", "room_id", client.RoomID, "user_id", client.UserID, "username", client.Username)

	// Notify existing participants that a new peer joined
	joinEvt := &Event{
		Type:       EventJoinRoom,
		RoomID:     client.RoomID,
		SenderID:   client.UserID,
		SenderName: client.Username,
	}
	h.dispatchEvent(joinEvt)
}

func (h *Hub) unregisterClient(client *Client) {
	roomClients, ok := h.rooms[client.RoomID]
	if !ok {
		return
	}

	if _, exists := roomClients[client]; exists {
		delete(roomClients, client)
		close(client.Send)

		slog.Info("client left room hub", "room_id", client.RoomID, "user_id", client.UserID)

		// Clean up empty rooms from memory
		if len(roomClients) == 0 {
			delete(h.rooms, client.RoomID)
		} else {
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
	roomClients, ok := h.rooms[event.RoomID]
	if !ok || len(roomClients) == 0 {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to serialize event payload for broadcast", "error", err)
		return
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
		}
	}
}
