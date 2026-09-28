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
