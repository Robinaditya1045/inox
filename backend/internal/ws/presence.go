package ws

import (
	"encoding/json"
	"log/slog"
	"sort"
	"time"
)

// broadcastPresence tells everyone in a room who is connected to it. Must only
// run on the hub loop.
//
// It is delivered locally rather than through the Redis event bus. The list is
// this process's connections, and since each client replaces its copy with the
// latest one, lists published by several processes would overwrite each other.
// As with RoomActivity, a room spread over several processes shows each client
// the people on its own process.
func (h *Hub) broadcastPresence(roomID string) {
	clients := h.rooms[roomID]
	if len(clients) == 0 {
		return
	}
	payload, err := json.Marshal(PresencePayload{Users: presentUsers(clients)})
	if err != nil {
		slog.Error("failed to marshal room presence", "room_id", roomID, "error", err)
		return
	}
	h.dispatchLocal(&Event{
		Type:      EventPresence,
		RoomID:    roomID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	})
}

// presentUsers lists the people behind a room's connections, once each and in a
// stable order.
func presentUsers(clients map[*Client]bool) []PresenceUser {
	byID := make(map[string]PresenceUser, len(clients))
	for client := range clients {
		if _, seen := byID[client.UserID]; seen {
			continue
		}
		byID[client.UserID] = PresenceUser{
			UserID:   client.UserID,
			Username: client.Username,
			Role:     client.Role,
		}
	}
	users := make([]PresenceUser, 0, len(byID))
	for _, user := range byID {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].UserID < users[j].UserID })
	return users
}
