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

// Room represents a Watch Party workspace where users gather to watch synchronized video.
type Room struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	IsPrivate bool      `json:"is_private"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RoomMember represents a user's membership and permission set inside a specific room.
type RoomMember struct {
	RoomID      string      `json:"room_id"`
	UserID      string      `json:"user_id"`
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
