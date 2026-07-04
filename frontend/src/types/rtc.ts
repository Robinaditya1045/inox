export type RTCConnectionState = 'disconnected' | 'connecting' | 'connected' | 'failed';

export interface VoiceParticipant {
  user_id: string;
  username: string;
  is_speaking: boolean;
  is_muted: boolean;
  is_deafened: boolean;
  connection_state: RTCConnectionState;
  audio_stream?: MediaStream;
}

export interface MediaControlsState {
  isMuted: boolean;
  isDeafened: boolean;
  isScreenSharing: boolean;
}
