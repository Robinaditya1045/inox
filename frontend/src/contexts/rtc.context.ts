import { createContext } from 'react';
import type { RTCConnectionState } from '../types/rtc';

export interface RTCContextValue {
  connectionState: RTCConnectionState;
  isAudioMuted: boolean;
  isDeafened: boolean;
  isScreenSharing: boolean;
  remoteStreams: Map<string, MediaStream>;
  localScreenStream: MediaStream | null;
  connectAudio: () => Promise<void>;
  disconnectAudio: () => void;
  toggleMute: () => void;
  toggleDeafen: () => void;
  toggleScreenShare: () => Promise<void>;
}

export const RTCContext = createContext<RTCContextValue | null>(null);
