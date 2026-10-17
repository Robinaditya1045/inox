import type { MediaAsset, Playlist } from "../types/media";
import type { WSQueue, WSQueueItem } from "../types/ws";
import { isLiveMediaUrl } from "./liveSync";

/**
 * The URL a room plays an asset from: the HLS master for adaptive bitrate when
 * there is one, the source otherwise. Deliberately NOT normalized: this URL is
 * broadcast to every peer and persisted on the room, so it must stay canonical.
 * normalizeMediaUrl runs per-client at playback time instead.
 */
export function getPlayUrl(asset: MediaAsset): string {
  return asset.hls_master_url || asset.source_url;
}

export function isLiveAsset(asset: MediaAsset): boolean {
  return asset.kind === "live" || isLiveMediaUrl(getPlayUrl(asset));
}

/** Running time as a player shows it: 4:05, 1:02:09. Empty when unknown. */
export function formatDuration(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0)
    return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

/** A total running time in words: "1 hr 12 min", "8 min". Empty when unknown. */
export function formatTotalDuration(seconds: number): string {
  if (seconds <= 0) return "";
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  if (h > 0) return m > 0 ? `${h} hr ${m} min` : `${h} hr`;
  return `${Math.max(m, 1)} min`;
}

/** Highest rendition height, e.g. "1080p", or null for a single-quality asset. */
export function getResolutionLabel(asset: MediaAsset): string | null {
  if (!asset.renditions || asset.renditions.length === 0) return null;
  const maxRes = Math.max(
    ...asset.renditions.map((r) => parseInt(r.resolution) || 0),
  );
  return maxRes > 0 ? `${maxRes}p` : null;
}

export function toQueueItem(asset: MediaAsset): WSQueueItem {
  return {
    media_url: getPlayUrl(asset),
    title: asset.title,
    thumbnail_url: asset.thumbnail_url || undefined,
    media_asset_id: asset.id,
    duration_seconds: asset.duration_seconds || undefined,
  };
}

/**
 * The queue a room plays a playlist as, starting at `startIndex`. Videos that
 * can no longer be played (deleted, or re-transcoding) are left out, and the
 * start index is moved to account for them.
 */
export function queueFromPlaylist(
  playlist: Playlist,
  startIndex = 0,
): WSQueue | null {
  const items: WSQueueItem[] = [];
  let index = 0;
  playlist.items.forEach((item, i) => {
    if (item.media.status !== "ready") return;
    if (i === startIndex) index = items.length;
    items.push(toQueueItem(item.media));
  });
  if (items.length === 0) return null;
  return { playlist_id: playlist.id, title: playlist.name, items, index };
}

export function playlistDuration(playlist: Playlist): number {
  return playlist.items.reduce(
    (sum, item) => sum + (item.media.duration_seconds || 0),
    0,
  );
}
