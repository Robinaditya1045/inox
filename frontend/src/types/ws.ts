export type WSEventType =
  | "JOIN_ROOM"
  | "LEAVE_ROOM"
  | "CHAT_MESSAGE"
  | "PLAY"
  | "PAUSE"
  | "SEEK"
  | "CHANGE_MEDIA"
  | "SYNC_PLAYBACK"
  | "WEBRTC_OFFER"
  | "WEBRTC_ANSWER"
  | "WEBRTC_ICE_CANDIDATE"
  | "SFU_JOIN"
  | "SFU_OFFER"
  | "SFU_ANSWER"
  | "SFU_ICE_CANDIDATE"
  | "SFU_LEAVE"
  | "SFU_STATE"
  | "SFU_PEERS"
  | "LIVE_POSITION"
  | "LIVE_LEADER"
  | "LIVE_STATUS"
  | "ERROR";

export interface WSMessage<T = unknown> {
  type: WSEventType;
  room_id?: string;
  sender_id?: string;
  sender_name?: string;
  target_id?: string;
  payload?: T;
  timestamp: number;
}

export interface WSJoinPayload {
  sender_id: string;
  sender_name: string;
}

export type MediaKind = "vod" | "live";

/**
 * A position in the stream itself, rather than in any one viewer's timeline.
 *
 * hls.js starts each client's media timeline near the live edge at the moment that
 * client attached, so `video.currentTime` for the same frame differs per viewer and
 * means nothing to anyone else. Media sequence numbers come from the playlist, so
 * every viewer resolves the same tuple to the same frame.
 */
export interface WSLivePosition {
  /** #EXT-X-MEDIA-SEQUENCE of the segment containing the frame. */
  sn: number;
  /** Discontinuity counter, so ad breaks and source switches stay unambiguous. */
  cc: number;
  /** Seconds into that segment. */
  offset: number;
}

export interface WSPlaybackPayload {
  media_url?: string;
  is_playing?: boolean;
  media_time_seconds: number;
  last_updated?: number;

  /** Selects how the rest of this payload is read. Absent means "vod". */
  kind?: MediaKind;
  leader_id?: string;
  leader_name?: string;
  live_position?: WSLivePosition;
  /** How stale live_position was at transmission. Never a timestamp — see below. */
  age_ms?: number;
  is_dvr?: boolean;
}

/**
 * The sync leader's playhead.
 *
 * `age_ms` rather than a timestamp is deliberate: a client comparing a server-stamped
 * time against its own `Date.now()` inherits the full clock skew between the two
 * machines and stays wrong by that amount forever. An age is measured entirely on the
 * server and applied entirely on the client, leaving only one-way network latency.
 */
export interface WSLivePositionPayload {
  position: WSLivePosition;
  leader_id?: string;
  age_ms: number;
  /** Set by the leader while its own player is buffering. */
  stalled?: boolean;
  /**
   * Set by a leader that cannot produce positions at all — a native-HLS player with
   * no fragment list — rather than one that is momentarily stuck. The server hands
   * the role to someone else.
   */
  unable?: boolean;
}

export interface WSLiveLeaderPayload {
  leader_id: string;
  leader_name: string;
}

export interface WSLiveStatusPayload {
  status: string;
  message?: string;
}

export interface WSChatPayload {
  message: string;
  message_id?: string;
}

export interface WSSfuOfferPayload {
  sdp: string;
  type?: string;
}

export interface WSSfuAnswerPayload {
  sdp: string;
  type?: string;
}

export interface WSSfuIcePayload {
  candidate: string;
  sdpMid?: string;
  sdpMLineIndex?: number;
}

/** What this client reports about itself; the SFU cannot see a muted microphone. */
export interface WSSfuStatePayload {
  is_muted: boolean;
}

/** One row of the server's voice roster. */
export interface WSSfuPeer {
  user_id: string;
  username: string;
  is_muted: boolean;
  is_screen_sharing: boolean;
}

/**
 * The authoritative list of who is in the call.
 *
 * A client cannot derive this from its own peer connection: it has no track for
 * itself, none for anyone who is muted-from-the-start or has not published yet,
 * and none at all when it is not in voice. The server sends the whole roster on
 * join and on every change.
 */
export interface WSSfuPeersPayload {
  peers: WSSfuPeer[];
}
