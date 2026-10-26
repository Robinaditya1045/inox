import { createContext } from "react";
import type {
  MediaKind,
  WSLivePosition,
  WSLiveStatusPayload,
  WSQueue,
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

/**
 * This client opened the room with nobody else in it and nothing playing, so
 * the room is waiting for someone to decide what to watch.
 */
export interface RoomArrival {
  roomId: string;
  /**
   * What the room was left on, and where. Null when it has nothing of its own
   * to go back to: still on the stream every room starts with, never played.
   */
  leftOn: { mediaUrl: string; kind: MediaKind; time: number } | null;
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
  /** The room's VOD speed. Live streams always play at 1. */
  playbackRate: number;
  /** Changes the room's speed, realigning everyone at `time`. */
  setPlaybackRate: (rate: number, time: number) => void;
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

  // ── Play queue ────────────────────────────────────────────
  /** The list the room is playing through, as the server last announced it. */
  queue: WSQueue | null;
  /** Starts the room playing through a list, at queue.index. */
  playQueue: (queue: WSQueue) => void;
  /** Moves the room's queue on: to toIndex, or to the next item. */
  advanceQueue: (toIndex?: number) => void;
  /** Stops playing through the queue; the current video carries on. */
  clearQueue: () => void;

  // ── Arrival ───────────────────────────────────────────────
  /** Set once per visit, when this client opened the room. */
  arrival: RoomArrival | null;
  /** Starts the room playing at `time`, from outside the player. */
  resumePlayback: (time: number) => void;
}

export const PlayerSyncContext = createContext<PlayerSyncContextValue | null>(
  null,
);
