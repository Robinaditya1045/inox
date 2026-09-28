export type RoomRole = "owner" | "moderator" | "member" | "guest";

export interface RoomPermissions {
  can_control_playback: boolean;
  can_stream_audio: boolean;
  can_stream_video: boolean;
  can_share_screen: boolean;
  can_send_messages: boolean;
  can_invite_users: boolean;
  can_kick_users: boolean;
  can_manage_roles: boolean;
}

export interface RoomMember {
  user_id: string;
  username: string;
  role: RoomRole;
  permissions: RoomPermissions;
  joined_at: string;
}

export interface Room {
  id: string;
  name: string;
  owner_id: string;
  is_private: boolean;
  current_media_url?: string;
  created_at: string;
  members?: RoomMember[];
  /** Present only while someone is connected to the room (from the live hub). */
  activity?: RoomActivity;
}

/** A room's live state: who is in it right now and what it is playing. */
export interface RoomActivity {
  /** Distinct people connected now — not the membership count. */
  viewers: number;
  /** A VOD someone pressed play on, or a live stream with a sync leader. */
  is_playing: boolean;
  media_url?: string;
  kind?: "vod" | "live";
}

export interface CreateRoomRequest {
  name: string;
  is_private: boolean;
}

export type InvitationStatus = "pending" | "accepted" | "declined";

export interface RoomInvitation {
  id: string;
  room_id: string;
  room_name?: string;
  inviter_id: string;
  inviter_name?: string;
  invitee_id: string;
  invitee_name?: string;
  status: InvitationStatus;
  created_at: string;
  updated_at: string;
}
