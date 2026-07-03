package ws

import "encoding/json"

type EventType string

const (
	EventJoinRoom        EventType = "JOIN_ROOM"
	EventLeaveRoom       EventType = "LEAVE_ROOM"
	EventChatMessage     EventType = "CHAT_MESSAGE"
	EventPlay            EventType = "PLAY"
	EventPause           EventType = "PAUSE"
	EventSeek            EventType = "SEEK"
	EventWebRTCOffer     EventType = "WEBRTC_OFFER"
	EventWebRTCAnswer    EventType = "WEBRTC_ANSWER"
	EventWebRTCICECand   EventType = "WEBRTC_ICE_CANDIDATE"
	EventError           EventType = "ERROR"
)

// Event represents a standardized real-time message exchanged over WebSocket connections.
type Event struct {
	Type       EventType       `json:"type"`
	RoomID     string          `json:"room_id,omitempty"`
	SenderID   string          `json:"sender_id,omitempty"`
	SenderName string          `json:"sender_name,omitempty"`
	TargetID   string          `json:"target_id,omitempty"` // For peer-to-peer WebRTC signaling
	Payload    json.RawMessage `json:"payload,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}

// VideoControlPayload represents sync timestamp and media state for PLAY, PAUSE, and SEEK events.
type VideoControlPayload struct {
	MediaTimeSeconds float64 `json:"media_time_seconds"`
}

// ChatPayload represents a room text chat message.
type ChatPayload struct {
	Message string `json:"message"`
}
