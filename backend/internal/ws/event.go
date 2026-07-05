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
	EventChangeMedia     EventType = "CHANGE_MEDIA"
	EventSyncPlayback    EventType = "SYNC_PLAYBACK"
	EventWebRTCOffer     EventType = "WEBRTC_OFFER"
	EventWebRTCAnswer    EventType = "WEBRTC_ANSWER"
	EventWebRTCICECand   EventType = "WEBRTC_ICE_CANDIDATE"
	EventSFUJoin         EventType = "SFU_JOIN"
	EventSFUOffer        EventType = "SFU_OFFER"
	EventSFUAnswer       EventType = "SFU_ANSWER"
	EventSFUICECandidate EventType = "SFU_ICE_CANDIDATE"
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

// VideoControlPayload represents sync timestamp and media state for PLAY, PAUSE, SEEK, CHANGE_MEDIA, and SYNC_PLAYBACK events.
type VideoControlPayload struct {
	MediaURL         string  `json:"media_url,omitempty"`
	IsPlaying        bool    `json:"is_playing"`
	MediaTimeSeconds float64 `json:"media_time_seconds"`
	LastUpdated      int64   `json:"last_updated,omitempty"` // Epoch millis
}

// ChatPayload represents a room text chat message.
type ChatPayload struct {
	Message string `json:"message"`
}

// SFUSDOPayload represents WebRTC Session Description Protocol (SDP) offer/answer framing for SFU media routing.
type SFUSDOPayload struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"` // "offer" or "answer"
}

// SFUICECandidatePayload represents WebRTC ICE candidate framing for NAT traversal.
type SFUICECandidatePayload struct {
	Candidate     string  `json:"candidate"`
	SDPMid        *string `json:"sdpMid,omitempty"`
	SDPMLineIndex *uint16 `json:"sdpMLineIndex,omitempty"`
}
