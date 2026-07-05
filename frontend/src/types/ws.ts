export type WSEventType =
  | 'JOIN_ROOM'
  | 'LEAVE_ROOM'
  | 'CHAT_MESSAGE'
  | 'PLAY'
  | 'PAUSE'
  | 'SEEK'
  | 'CHANGE_MEDIA'
  | 'SYNC_PLAYBACK'
  | 'WEBRTC_OFFER'
  | 'WEBRTC_ANSWER'
  | 'WEBRTC_ICE_CANDIDATE'
  | 'SFU_JOIN'
  | 'SFU_OFFER'
  | 'SFU_ANSWER'
  | 'SFU_ICE_CANDIDATE'
  | 'ERROR';

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

export interface WSPlaybackPayload {
  media_url?: string;
  is_playing?: boolean;
  media_time_seconds: number;
  last_updated?: number;
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
