export type MediaStatus = "pending" | "processing" | "ready" | "failed";

export interface MediaRendition {
  id: string;
  media_asset_id: string;
  resolution: string;
  bitrate_kbps: number;
  playlist_url: string;
  created_at: string;
}

export interface MediaAsset {
  id: string;
  /** "live" for a live channel's library entry; uploads are "vod" */
  kind?: "vod" | "live";
  title: string;
  description: string;
  source_url: string;
  status: MediaStatus;
  duration_seconds: number;
  progress: number;
  thumbnail_url: string;
  hls_master_url: string;
  created_by?: string | null;
  created_at: string;
  updated_at: string;
  renditions?: MediaRendition[];
}

export interface PlaylistItem {
  id: string;
  playlist_id: string;
  media_asset_id: string;
  position: number;
  added_at: string;
  media: MediaAsset;
}

/** One of the signed-in user's playlists. Reads always carry the items, in order. */
export interface Playlist {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  items: PlaylistItem[];
  created_at: string;
  updated_at: string;
}

export type LiveResolver = "direct" | "api" | "static" | "browser";

export type LiveChannelStatus =
  "idle" | "resolving" | "live" | "degraded" | "error" | "disabled";

/** Viewer-safe facts about a live channel (GET /live/channels). */
export interface LiveChannelSummary {
  media_asset_id: string;
  slug: string;
  title: string;
  /** How the stream is obtained; unknown values pass through as strings */
  resolver: LiveResolver | string;
  status: LiveChannelStatus;
  is_dvr: boolean;
}
