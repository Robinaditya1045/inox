package domain

import (
	"time"
)

type Role string

const (
	RoleOwner     Role = "owner"
	RoleModerator Role = "moderator"
	RoleMember    Role = "member"
	RoleGuest     Role = "guest"
)

// Permissions defines granular boolean flags controlling user capabilities inside a Watch Party room.
type Permissions struct {
	CanControlPlayback bool `json:"can_control_playback"` // Play, pause, seek synchronized video
	CanStreamAudio     bool `json:"can_stream_audio"`     // Speak in WebRTC voice chat
	CanStreamVideo     bool `json:"can_stream_video"`     // Turn on webcam
	CanShareScreen     bool `json:"can_share_screen"`     // Broadcast screen/browser window
	CanSendMessages    bool `json:"can_send_messages"`    // Send text messages in room chat
	CanInviteUsers     bool `json:"can_invite_users"`     // Generate room invitation links
	CanKickUsers       bool `json:"can_kick_users"`       // Kick or ban participants
	CanManageRoles     bool `json:"can_manage_roles"`     // Assign roles and update permissions
}

// RoomPlaybackState maintains the authoritative real-time media state of an active watch party room.
//
// VOD and live rooms use different coordinates and the Kind field selects between
// them. For VOD the room's position is MediaTimeSeconds, an offset from the start of
// the file. For live there is no start of file, so the room follows one member's
// playhead expressed as a LivePosition; see domain.LivePosition for why.
type RoomPlaybackState struct {
	MediaURL         string    `json:"media_url"`
	Kind             MediaKind `json:"kind,omitempty"`
	IsPlaying        bool      `json:"is_playing"`
	MediaTimeSeconds float64   `json:"media_time_seconds"`
	LastUpdated      int64     `json:"last_updated"` // Epoch millis

	// ── Live only ───────────────────────────────────────────
	LeaderID   string `json:"leader_id,omitempty"`
	LeaderName string `json:"leader_name,omitempty"`

	LivePosition *LivePosition `json:"live_position,omitempty"`

	// LiveReceivedAt is the server clock reading when LivePosition arrived. It is
	// used only to compute an age at broadcast time and is never sent to clients:
	// a client comparing a server timestamp against its own Date.now() inherits the
	// full clock skew between the two machines. Clients receive an age instead and
	// extrapolate from their own local receipt time.
	LiveReceivedAt int64 `json:"live_received_at,omitempty"`
}

// LiveAgeMillis reports how stale the stored live position is, for the age field
// broadcast to followers.
func (s *RoomPlaybackState) LiveAgeMillis(nowMillis int64) int64 {
	if s.LiveReceivedAt <= 0 {
		return 0
	}
	if age := nowMillis - s.LiveReceivedAt; age > 0 {
		return age
	}
	return 0
}

// Room represents a Watch Party workspace where users gather to watch synchronized video.
type Room struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	OwnerID         string        `json:"owner_id"`
	IsPrivate       bool          `json:"is_private"`
	CurrentMediaURL string        `json:"current_media_url"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	Members         []*RoomMember `json:"members,omitempty"`
}

// RoomMember represents a user's membership and permission set inside a specific room.
type RoomMember struct {
	RoomID      string      `json:"room_id"`
	UserID      string      `json:"user_id"`
	Username    string      `json:"username,omitempty"`
	Role        Role        `json:"role"`
	Permissions Permissions `json:"permissions"`
	JoinedAt    time.Time   `json:"joined_at"`
}

// DefaultPermissionsForRole returns preset permissions for standard roles upon joining.
func DefaultPermissionsForRole(role Role) Permissions {
	switch role {
	case RoleOwner:
		return Permissions{
			CanControlPlayback: true,
			CanStreamAudio:     true,
			CanStreamVideo:     true,
			CanShareScreen:     true,
			CanSendMessages:    true,
			CanInviteUsers:     true,
			CanKickUsers:       true,
			CanManageRoles:     true,
		}
	case RoleModerator:
		return Permissions{
			CanControlPlayback: true,
			CanStreamAudio:     true,
			CanStreamVideo:     true,
			CanShareScreen:     true,
			CanSendMessages:    true,
			CanInviteUsers:     true,
			CanKickUsers:       true,
			CanManageRoles:     false,
		}
	case RoleMember:
		return Permissions{
			CanControlPlayback: false, // By default, standard members cannot pause the movie!
			CanStreamAudio:     true,
			CanStreamVideo:     true,
			CanShareScreen:     false,
			CanSendMessages:    true,
			CanInviteUsers:     false,
			CanKickUsers:       false,
			CanManageRoles:     false,
		}
	case RoleGuest:
		return Permissions{
			CanControlPlayback: false,
			CanStreamAudio:     false, // Guests are muted by default until permitted
			CanStreamVideo:     false,
			CanShareScreen:     false,
			CanSendMessages:    true,
			CanInviteUsers:     false,
			CanKickUsers:       false,
			CanManageRoles:     false,
		}
	default:
		return DefaultPermissionsForRole(RoleGuest)
	}
}

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusDeclined InvitationStatus = "declined"
)

// RoomInvitation represents an invitation sent to a user to join a room.
type RoomInvitation struct {
	ID          string           `json:"id"`
	RoomID      string           `json:"room_id"`
	RoomName    string           `json:"room_name,omitempty"`
	InviterID   string           `json:"inviter_id"`
	InviterName string           `json:"inviter_name,omitempty"`
	InviteeID   string           `json:"invitee_id"`
	InviteeName string           `json:"invitee_name,omitempty"`
	Status      InvitationStatus `json:"status"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}
