package sfu

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/inox/inox/backend/internal/observability"
	"github.com/pion/webrtc/v3"
)

var (
	ErrRoomNotFound = errors.New("sfu room not found")
)

// Manager orchestrates lifecycle management across all active SFU media rooms.
type Manager struct {
	rooms      map[string]*Room
	api        *webrtc.API
	iceServers []webrtc.ICEServer
	mu         sync.RWMutex
}

// NewManager initializes the central SFU voice and video routing coordinator
// using Pion's default networking, which is correct for a directly addressable
// host. Deployments behind NAT should use NewNetworkedManager instead.
func NewManager() *Manager {
	return &Manager{
		rooms:      make(map[string]*Room),
		iceServers: DefaultICEServers(),
	}
}

// NewNetworkedManager initializes the coordinator with an explicit NAT and UDP
// port configuration, so every peer it creates advertises candidates the
// outside world can actually reach.
func NewNetworkedManager(cfg NetworkConfig) (*Manager, error) {
	api, err := buildAPI(cfg)
	if err != nil {
		return nil, err
	}

	m := NewManager()
	m.api = api
	m.iceServers = ParseICEServers(cfg.ICEServers)
	return m, nil
}

// GetOrCreateRoom retrieves an existing media room or initializes a new one.
func (m *Manager) GetOrCreateRoom(roomID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		room = NewRoom(roomID, m.api, m.iceServers)
		m.rooms[roomID] = room
		slog.Info("created new sfu media room", "room_id", roomID)
		observability.Global().IncActiveSFURooms()
	}
	return room
}

// GetRoom retrieves an existing media room by ID.
func (m *Manager) GetRoom(roomID string) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// RemoveRoom terminates and removes an SFU media room.
func (m *Manager) RemoveRoom(roomID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if ok {
		delete(m.rooms, roomID)
	}
	m.mu.Unlock()

	if ok {
		observability.Global().DecActiveSFURooms()
		room.mu.Lock()
		for uid, peer := range room.Peers {
			observability.Global().DecActiveSFUPeers()
			_ = peer.Close()
			delete(room.Peers, uid)
		}
		room.mu.Unlock()
		slog.Info("removed sfu media room and disconnected peers", "room_id", roomID)
	}
}

// Shutdown gracefully terminates all media rooms and WebRTC peer connections.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("shutting down sfu media manager...")
	for roomID, room := range m.rooms {
		observability.Global().DecActiveSFURooms()
		room.mu.Lock()
		for uid, peer := range room.Peers {
			observability.Global().DecActiveSFUPeers()
			_ = peer.Close()
			delete(room.Peers, uid)
		}
		room.mu.Unlock()
		delete(m.rooms, roomID)
	}
	slog.Info("sfu media manager shutdown complete")
}
