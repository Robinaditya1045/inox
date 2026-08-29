import { createContext } from "react";

export interface PlayerSyncContextValue {
  /** Canonical media URL as broadcast by the room. Normalize at playback time, not here. */
  mediaUrl: string;
  isPlaying: boolean;
  currentTime: number;
  lastSyncTimestamp: number;
  setMediaUrl: (url: string) => void;
  play: (time: number) => void;
  pause: (time: number) => void;
  seek: (time: number) => void;
  notifyLocalProgress: (time: number) => void;
  clearRemoteFlag: () => void;
}

export const PlayerSyncContext = createContext<PlayerSyncContextValue | null>(
  null,
);
