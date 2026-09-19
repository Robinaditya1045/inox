package ws

import (
	"encoding/json"

	"github.com/inox/inox/backend/internal/domain"
)

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

	// Live streaming. A live room follows one member's playhead rather than an
	// offset from the start of a file, so it needs its own small vocabulary.
	EventLivePosition EventType = "LIVE_POSITION" // leader→server, then server→room
	EventLiveLeader   EventType = "LIVE_LEADER"   // server→room on election and handoff
	EventLiveStatus   EventType = "LIVE_STATUS"   // server→room when a channel degrades
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
//
// Kind selects how the rest of the payload is read. For "vod" the room's position is
// MediaTimeSeconds. For "live" that field is meaningless -- see domain.LivePosition --
// and LivePosition carries the room's position instead.
type VideoControlPayload struct {
	MediaURL         string  `json:"media_url,omitempty"`
	IsPlaying        bool    `json:"is_playing"`
	MediaTimeSeconds float64 `json:"media_time_seconds"`
	LastUpdated      int64   `json:"last_updated,omitempty"` // Epoch millis

	// ── Live only ───────────────────────────────────────────
	Kind         string               `json:"kind,omitempty"`
	LeaderID     string               `json:"leader_id,omitempty"`
	LeaderName   string               `json:"leader_name,omitempty"`
	LivePosition *domain.LivePosition `json:"live_position,omitempty"`
	AgeMillis    int64                `json:"age_ms,omitempty"`
	IsDVR        bool                 `json:"is_dvr,omitempty"`
}

// LivePositionPayload carries the sync leader's playhead, expressed in the stream's
// own coordinates so that every follower resolves it to the same frame.
//
// AgeMillis, not a timestamp, is deliberate. A client that compares a server-stamped
// time against its own Date.now() inherits the full clock skew between the two
// machines and stays wrong by that amount forever, however often the server
// broadcasts. An age is measured entirely on the server and applied entirely on the
// client, so the only error left is one-way network latency.
type LivePositionPayload struct {
	Position  domain.LivePosition `json:"position"`
	LeaderID  string              `json:"leader_id,omitempty"`
	AgeMillis int64               `json:"age_ms"`

	// Stalled is set by the leader when its own player is buffering. The server keeps
	// extrapolating from the last good position instead of accepting a frozen one,
	// so one viewer's rebuffer does not drag the whole room backwards and then
	// forwards again.
	Stalled bool `json:"stalled,omitempty"`

	// Unable is set by a leader that cannot produce positions at all, rather than
	// one that is momentarily stuck. The server hands leadership to someone else.
	Unable bool `json:"unable,omitempty"`
}

// LiveLeaderPayload announces which member the room is currently following.
type LiveLeaderPayload struct {
	LeaderID   string `json:"leader_id"`
	LeaderName string `json:"leader_name"`
}

// LiveStatusPayload reports upstream health so a dead channel shows an explanation
// rather than a frozen frame.
type LiveStatusPayload struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
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
