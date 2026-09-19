import { createContext } from "react";
import type {
  MediaKind,
  WSLivePosition,
  WSLiveStatusPayload,
} from "../types/ws";

/**
 * The room's live playhead as this client last heard it.
 *
 * Only an anchor, not a current position: advancing it needs segment durations, which
 * only the player knows. The player converts `position` to its own timeline and then
 * adds the elapsed wall-clock time.
 */
export interface LiveAnchor {
  position: WSLivePosition;
  /**
   * Local `Date.now()` at the moment this arrived. Deliberately a local reading:
   * comparing a server timestamp against a local clock bakes in the skew between the
   * two machines permanently.
   */
  receivedAt: number;
  /** How stale the position already was when the server sent it. */
  ageMs: number;
}

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

  // ── Live channels ─────────────────────────────────────────
  /** Selects the sync coordinate: "vod" uses currentTime, "live" uses liveAnchor. */
  kind: MediaKind;
  leaderId: string | null;
  leaderName: string | null;
  /** Whether this client is the one the room follows. */
  isLeader: boolean;
  liveAnchor: LiveAnchor | null;
  liveStatus: WSLiveStatusPayload | null;
  /** Leader-only: publish this client's playhead to the room. */
  reportLivePosition: (position: WSLivePosition, stalled: boolean) => void;
  /**
   * Leader-only: report that this player cannot express positions at all, so the
   * server hands the role to someone who can. Distinct from a stall, which is
   * temporary and must not trigger a handover.
   */
  declineLiveLeadership: () => void;
}

export const PlayerSyncContext = createContext<PlayerSyncContextValue | null>(
  null,
);
