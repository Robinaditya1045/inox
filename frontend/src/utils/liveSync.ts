import type Hls from "hls.js";
import type { WSLivePosition } from "../types/ws";

/**
 * Converting between a viewer's own timeline and the stream's.
 *
 * A live HLS playlist is a sliding window with no fixed origin. hls.js starts each
 * client's media timeline near the live edge at the moment that client attached, so
 * the same frame is at `currentTime` 612 for someone who joined ten minutes ago and
 * 18 for someone who joined twenty seconds ago. Broadcasting one viewer's
 * `currentTime` to the room therefore sends everyone else somewhere wrong — usually
 * outside `video.seekable`, where hls.js silently clamps to the edge, which is why
 * the failure looks like "sync is flaky" rather than like an error.
 *
 * Media sequence numbers come from the playlist itself, so they mean the same thing
 * to every viewer regardless of when they joined.
 */

/** Below this, drift is indistinguishable from measurement noise; acting on it oscillates. */
const DRIFT_DEADBAND_S = 0.15;
/** Inaudible rate nudge, used to close small gaps without a visible artifact. */
const DRIFT_GENTLE_S = 1;
const RATE_GENTLE = 0.02;
/** Faintly audible on speech, but still far cheaper than a rebuffer. */
const DRIFT_FIRM_S = 4;
const RATE_FIRM = 0.08;

/** How often the leader publishes its playhead. Followers run at 1x in between. */
export const LEADER_REPORT_INTERVAL_MS = 5000;
/** How often followers measure and correct their own drift. */
export const DRIFT_CHECK_INTERVAL_MS = 1000;

export interface DriftAction {
  /** playbackRate to apply. */
  rate: number;
  /** Whether the gap is too wide to close by rate and needs a seek. */
  seek: boolean;
}

/**
 * The drift ladder.
 *
 * Seeking a live stream flushes the buffer and rebuffers, so it is the last resort
 * rather than the first: correcting by playback rate is invisible where a seek would
 * show a spinner every time it fires.
 *
 * @param drift seconds the follower is behind the room (negative means ahead).
 */
export function driftAction(drift: number): DriftAction {
  const magnitude = Math.abs(drift);
  if (magnitude < DRIFT_DEADBAND_S) return { rate: 1, seek: false };
  if (magnitude > DRIFT_FIRM_S) return { rate: 1, seek: true };

  const step = magnitude < DRIFT_GENTLE_S ? RATE_GENTLE : RATE_FIRM;
  // Behind the room (positive drift) means play slightly faster to catch up.
  return { rate: drift > 0 ? 1 + step : 1 - step, seek: false };
}

interface FragmentLike {
  sn: number | "initSegment";
  cc: number;
  start: number;
  duration: number;
}

/**
 * The fragment list for the level currently being played.
 *
 * `currentLevel` is -1 until the first ABR switch resolves, so fall back through
 * `loadLevel` and then to any level that has parsed details.
 */
function currentFragments(hls: Hls): FragmentLike[] | null {
  const levels = hls.levels ?? [];
  for (const index of [hls.currentLevel, hls.loadLevel]) {
    const details = index >= 0 ? levels[index]?.details : undefined;
    if (details?.fragments?.length) return details.fragments as FragmentLike[];
  }
  for (const level of levels) {
    if (level?.details?.fragments?.length) {
      return level.details.fragments as FragmentLike[];
    }
  }
  return null;
}

/** Converts this client's playhead into the stream's own coordinates. */
export function toStreamPosition(
  hls: Hls,
  currentTime: number,
): WSLivePosition | null {
  const fragments = currentFragments(hls);
  if (!fragments) return null;

  for (const fragment of fragments) {
    if (typeof fragment.sn !== "number") continue;
    if (
      currentTime >= fragment.start &&
      currentTime < fragment.start + fragment.duration
    ) {
      return {
        sn: fragment.sn,
        cc: fragment.cc ?? 0,
        offset: currentTime - fragment.start,
      };
    }
  }
  return null;
}

/**
 * Converts a room position back into this client's timeline.
 *
 * Returns null when the segment is no longer in this client's window — the viewer was
 * disconnected, or is on a variant whose segments are not aligned with the leader's.
 * Callers should snap to the live edge rather than retrying.
 */
export function toLocalTime(hls: Hls, position: WSLivePosition): number | null {
  const fragments = currentFragments(hls);
  if (!fragments) return null;

  for (const fragment of fragments) {
    if (fragment.sn === position.sn && (fragment.cc ?? 0) === position.cc) {
      return fragment.start + position.offset;
    }
  }
  return null;
}

/**
 * Where the room is now, in this client's timeline.
 *
 * The anchor is a position plus how long ago it was true. Since a live room never
 * pauses, its position advances at exactly 1x, so the elapsed time is simply added.
 * Every clock reading involved is local: the server measured the age, the client
 * measures the time since receipt, and the two are never compared.
 */
export function roomLocalTime(
  hls: Hls,
  position: WSLivePosition,
  receivedAt: number,
  ageMs: number,
): number | null {
  const anchored = toLocalTime(hls, position);
  if (anchored === null) return null;
  return anchored + (Date.now() - receivedAt + ageMs) / 1000;
}

/**
 * Whether a media URL addresses a live channel on our own proxy.
 *
 * Derived from the URL rather than read from provider state so the player's hls.js
 * setup does not depend on a value that arrives a beat later: keying the setup effect
 * on the server-sent kind tore down and rebuilt every live stream just after join,
 * costing an extra master fetch, another minted token, and a visible flash.
 */
export function isLiveMediaUrl(url: string): boolean {
  return url.includes("/api/v1/live/") && url.endsWith("master.m3u8");
}

/** Seconds of content still ahead of the playhead, i.e. distance to the live edge. */
export function secondsBehindEdge(video: HTMLVideoElement): number {
  if (video.seekable.length === 0) return 0;
  return Math.max(
    0,
    video.seekable.end(video.seekable.length - 1) - video.currentTime,
  );
}

/**
 * Keeps a seek target inside the playable window, with a small margin so the player
 * does not land exactly on a boundary that is about to move.
 */
export function clampToSeekable(
  video: HTMLVideoElement,
  target: number,
): number {
  if (video.seekable.length === 0) return target;
  const start = video.seekable.start(0);
  const end = video.seekable.end(video.seekable.length - 1);
  if (end - start < 1) return target;
  return Math.min(Math.max(target, start + 0.5), end - 0.5);
}

/** The furthest playable point, used when a follower has fallen out of the window. */
export function liveEdgeTime(video: HTMLVideoElement): number | null {
  if (video.seekable.length === 0) return null;
  return Math.max(0, video.seekable.end(video.seekable.length - 1) - 0.5);
}
