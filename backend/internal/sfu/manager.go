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
	rooms        map[string]*Room
	api          *webrtc.API
	iceServers   []webrtc.ICEServer
	stateHandler StateHandler
	mu           sync.RWMutex
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

// SetStateHandler installs the roster callback every room created from here on --
// and every room that already exists -- reports voice state changes to.
func (m *Manager) SetStateHandler(h StateHandler) {
	m.mu.Lock()
	m.stateHandler = h
	rooms := make([]*Room, 0, len(m.rooms))
	for _, room := range m.rooms {
		rooms = append(rooms, room)
	}
	m.mu.Unlock()

	for _, room := range rooms {
		room.SetStateHandler(h)
	}
}

// GetOrCreateRoom retrieves an existing media room or initializes a new one.
func (m *Manager) GetOrCreateRoom(roomID string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		room = NewRoom(roomID, m.api, m.iceServers)
		room.SetStateHandler(m.stateHandler)
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
		room.Close()
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
		room.Close()
		delete(m.rooms, roomID)
	}
	slog.Info("sfu media manager shutdown complete")
}
