import { createContext } from "react";
import type { RTCConnectionState, VoicePeer } from "../types/rtc";

export interface RTCContextValue {
  connectionState: RTCConnectionState;
  isAudioMuted: boolean;
  isDeafened: boolean;
  isCameraOn: boolean;
  isScreenSharing: boolean;
  /** Everyone in the call, you included, from the server's roster. */
  peers: VoicePeer[];
  /** Subset of peers whose audio is currently carrying speech. */
  speakingIds: Set<string>;
  localScreenStream: MediaStream | null;
  /**
   * People this client invited to voice recently: until they answer and for a
   * while after, so the same person is not invited again straight away.
   */
  invitedToVoice: ReadonlySet<string>;
  /** Asks someone in the room to join the call. Only works from inside it. */
  inviteToVoice: (userId: string) => void;
  connectAudio: () => Promise<void>;
  disconnectAudio: () => void;
  toggleMute: () => void;
  toggleDeafen: () => void;
  toggleCamera: () => Promise<void>;
  toggleScreenShare: () => Promise<void>;
}

export const RTCContext = createContext<RTCContextValue | null>(null);
