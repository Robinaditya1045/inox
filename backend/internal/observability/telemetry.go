package observability

import (
	"encoding/json"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RoomTelemetry represents the live inspection state of an individual watch party room.
type RoomTelemetry struct {
	RoomID           string  `json:"room_id"`
	ParticipantCount int     `json:"participant_count"`
	IsPlaying        bool    `json:"is_playing"`
	MediaURL         string  `json:"media_url"`
	MediaTimeSeconds float64 `json:"media_time_seconds"`
	SFUPeersCount    int     `json:"sfu_peers_count"`
	QoEScore         int     `json:"qoe_score"` // Quality of Experience score (0-100%)
}

// SystemTelemetry represents an authoritative real-time snapshot of the entire backend stack.
type SystemTelemetry struct {
	Timestamp      int64           `json:"timestamp"` // Epoch millis
	Metrics        Snapshot        `json:"metrics"`
	Goroutines     int             `json:"goroutines"`
	HeapAllocBytes uint64          `json:"heap_alloc_bytes"`
	Rooms          []RoomTelemetry `json:"rooms"`
}

// RoomInspector defines the interface required by the telemetry engine to inspect active room state from the Hub.
type RoomInspector interface {
	InspectRooms() []RoomTelemetry
}

// TelemetryHub coordinates real-time streaming of system telemetry snapshots to connected admin dashboard clients.
type TelemetryHub struct {
	clients   map[*websocket.Conn]bool
	register  chan *websocket.Conn
	unregister chan *websocket.Conn
	inspector RoomInspector
	stop      chan struct{}
	mu        sync.RWMutex
}

// NewTelemetryHub initializes a new telemetry broadcaster.
func NewTelemetryHub(inspector RoomInspector) *TelemetryHub {
	return &TelemetryHub{
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		inspector:  inspector,
		stop:       make(chan struct{}),
	}
}

// Register adds an admin WebSocket client to the live telemetry stream.
func (h *TelemetryHub) Register(conn *websocket.Conn) {
	h.register <- conn
}

// Unregister removes an admin WebSocket client from the stream.
func (h *TelemetryHub) Unregister(conn *websocket.Conn) {
	h.unregister <- conn
}

// GetSnapshot builds an immediate system telemetry snapshot.
func (h *TelemetryHub) GetSnapshot() SystemTelemetry {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var rooms []RoomTelemetry
	if h.inspector != nil {
		rooms = h.inspector.InspectRooms()
	}
	if rooms == nil {
		rooms = []RoomTelemetry{}
	}

	return SystemTelemetry{
		Timestamp:      time.Now().UnixMilli(),
		Metrics:        Global().GetSnapshot(),
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: mem.Alloc,
		Rooms:          rooms,
	}
}

// Run starts the 2-second telemetry broadcasting loop and handles admin client registrations.
func (h *TelemetryHub) Run() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
			slog.Info("admin client connected to live telemetry stream")
			// Send an immediate snapshot upon connection
			snapshot := h.GetSnapshot()
			if data, err := json.Marshal(snapshot); err == nil {
				_ = conn.WriteMessage(websocket.TextMessage, data)
			}

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				_ = conn.Close()
			}
			h.mu.Unlock()
			slog.Info("admin client disconnected from live telemetry stream")

		case <-ticker.C:
			h.mu.RLock()
			clientCount := len(h.clients)
			h.mu.RUnlock()

			if clientCount == 0 {
				continue
			}

			snapshot := h.GetSnapshot()
			data, err := json.Marshal(snapshot)
			if err != nil {
				continue
			}

			h.mu.Lock()
			for conn := range h.clients {
				_ = conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					_ = conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.Unlock()

		case <-h.stop:
			h.mu.Lock()
			for conn := range h.clients {
				_ = conn.Close()
				delete(h.clients, conn)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Stop terminates the telemetry hub and disconnects all admin clients.
func (h *TelemetryHub) Stop() {
	close(h.stop)
}

// CalculateQoE computes the Quality of Experience (QoE) score (0-100%) for an active room.
// It penalizes sync drift, high eviction rates, or uninitialized media playback.
func CalculateQoE(participantCount int, isPlaying bool, mediaURL string, evictions uint64) int {
	score := 100
	if mediaURL == "" {
		score -= 20
	}
	if evictions > 0 {
		score -= int(evictions * 10)
	}
	if score < 0 {
		score = 0
	}
	return score
}
