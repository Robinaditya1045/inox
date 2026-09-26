export type RTCConnectionState =
  "disconnected" | "connecting" | "connected" | "failed";

/** What a peer can publish. Mirrors the SFU's track naming. */
export type RemoteTrackKind = "mic" | "screen";

/**
 * One participant in the call, as the UI needs them: identity from the server's
 * roster, media from this client's peer connection.
 */
export interface VoicePeer {
  userId: string;
  username: string;
  /** True for the person at this keyboard, whose media never round-trips the SFU. */
  isLocal: boolean;
  isMuted: boolean;
  isScreenSharing: boolean;
  audioStream?: MediaStream;
  screenStream?: MediaStream;
}

export interface MediaControlsState {
  isMuted: boolean;
  isDeafened: boolean;
  isScreenSharing: boolean;
}
