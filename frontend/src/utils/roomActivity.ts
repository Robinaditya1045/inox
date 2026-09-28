import type { Room } from "../types/room";
import type { MediaAsset } from "../types/media";
import { isLiveMediaUrl } from "./liveSync";

/** Someone is in the room and its media is running. */
export function isStreaming(room: Room): boolean {
  return (
    !!room.activity && room.activity.viewers > 0 && room.activity.is_playing
  );
}

/** People connected right now, as opposed to stored memberships. */
export function viewersOf(room: Room): number {
  return room.activity?.viewers ?? 0;
}

/**
 * Streaming rooms first (most viewers first), then rooms people are sitting in,
 * then the rest in their original order. Stable, so the list does not shuffle
 * between polls when nothing changed.
 */
export function sortRoomsByActivity(rooms: Room[]): Room[] {
  const rank = (room: Room) =>
    isStreaming(room) ? 0 : viewersOf(room) > 0 ? 1 : 2;
  return rooms
    .map((room, index) => ({ room, index }))
    .sort(
      (a, b) =>
        rank(a.room) - rank(b.room) ||
        viewersOf(b.room) - viewersOf(a.room) ||
        a.index - b.index,
    )
    .map(({ room }) => room);
}

export interface MediaPreview {
  title: string;
  thumbnailUrl?: string;
  isLive: boolean;
  /** Where it comes from, when there is no library entry to name it */
  source?: string;
}

// The stream every new room starts on (PlayerSyncProvider's default). It is not a
// library entry, so without this it would be named after its file: "movie".
const KNOWN_TITLES: Record<string, string> = {
  "https://media.w3.org/2010/05/bunny/movie.mp4": "Big Buck Bunny",
};

// File names that say what a file is for rather than what is in it.
const GENERIC_NAME =
  /^(master|index|playlist|stream|movie|video|output|media)\.[a-z0-9]{2,5}$/i;

function stripQuery(url: string): string {
  return url.split(/[?#]/)[0];
}

/** "…/bunny/movie.mp4" → "movie"; "…/big-buck-bunny/master.m3u8" → "big buck bunny". */
function titleFromUrl(url: string): string {
  try {
    const segments = new URL(url).pathname.split("/").filter(Boolean);
    let name = decodeURIComponent(segments.pop() ?? "");
    // Named after its role, not its content — the folder says more.
    if (GENERIC_NAME.test(name) && segments.length) {
      name = decodeURIComponent(segments.pop()!);
    }
    return (
      name
        .replace(/\.[a-z0-9]{2,5}$/i, "")
        .replace(/[-_]+/g, " ")
        .trim() || url
    );
  } catch {
    return url;
  }
}

function hostOf(url: string): string | undefined {
  try {
    return new URL(url).host;
  } catch {
    return undefined;
  }
}

/**
 * Names what a room is playing. A room's media URL is exactly what the library
 * picker sent (hls_master_url || source_url), so a library entry is found by URL;
 * anything pasted in by hand falls back to the file name.
 */
export function describeMedia(
  url: string,
  library: MediaAsset[],
  /** The server's classification, when known; otherwise inferred from the URL */
  kind?: "vod" | "live",
): MediaPreview {
  const target = stripQuery(url);
  const known = KNOWN_TITLES[target];
  if (known) return { title: known, isLive: false };

  const asset = library.find(
    (a) =>
      (a.hls_master_url && stripQuery(a.hls_master_url) === target) ||
      (a.source_url && stripQuery(a.source_url) === target),
  );
  const isLive = kind ? kind === "live" : isLiveMediaUrl(target);

  if (asset) {
    return {
      title: asset.title,
      thumbnailUrl: asset.thumbnail_url || undefined,
      isLive,
    };
  }
  return { title: titleFromUrl(url), isLive, source: hostOf(url) };
}
