export type LiveChannelStatus =
  "idle" | "resolving" | "live" | "degraded" | "error" | "disabled";

/** Resolver IDs; the backend reports the authoritative list with each channel fetch. */
export type LiveResolver = "direct" | "api" | "static" | string;

export interface LiveChannel {
  id: string;
  media_asset_id: string;
  slug: string;
  resolver: LiveResolver;
  source_url: string;
  resolver_config: Record<string, unknown>;
  fallback_sources: string[];
  protocol: string;
  is_dvr: boolean;
  dvr_window_seconds: number;
  status: LiveChannelStatus;
  upstream_expires_at: string | null;
  last_resolved_at: string | null;
  last_error: string;
  created_by: string | null;
  created_at: string;
  updated_at: string;
  title?: string;
}

export interface CreateLiveChannelRequest {
  title: string;
  description?: string;
  thumbnail_url?: string;
  slug?: string;
  resolver: LiveResolver;
  source_url: string;
  resolver_config?: Record<string, unknown>;
  protocol?: string;
  is_dvr?: boolean;
  dvr_window_seconds?: number;
}

/**
 * What a resolver actually produced, shown before a channel is saved.
 *
 * Configuring a scraped source without this is edit, save, join a room, squint,
 * repeat — the failure modes all look identical from the room.
 */
export interface LiveTestResult {
  ok: boolean;
  error?: string;
  manifest_host?: string;
  protocol?: string;
  is_master: boolean;
  is_live: boolean;
  variants?: string[];
  target_duration?: number;
  media_sequence?: number;
  segment_count?: number;
  window_seconds?: number;
  expires_at?: string;
}

export interface LiveChannelsResponse {
  channels: LiveChannel[] | null;
  resolvers: string[];
}
