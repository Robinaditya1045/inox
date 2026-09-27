export type LiveChannelStatus =
  "idle" | "resolving" | "live" | "degraded" | "error" | "disabled";

/**
 * Resolver IDs; the backend reports the authoritative list with each channel fetch.
 * "browser" is only listed where the backend has a Chromium to run it with.
 */
export type LiveResolver = "direct" | "api" | "static" | "browser" | string;

/**
 * The broad result of resolving a channel's source. DRM-protected sources are
 * reported, never worked around.
 */
export type ResolveOutcome =
  | "STREAM_FOUND"
  | "DRM_PROTECTED"
  | "NO_STREAM_FOUND"
  | "SERVER_REPLAY_FAILED"
  | "RESOLUTION_TIMEOUT"
  | "RESOLUTION_FAILED";

/**
 * What a resolve saw. Holds hosts rather than URLs, DRM system names rather than
 * DRM data, and never a header value, cookie, token or licence payload.
 */
export interface ResolveDiagnostics {
  outcome: ResolveOutcome;
  /** Stable refinement of the outcome, e.g. "manifest_protected". */
  reason?: string;
  resolver?: string;
  elapsed_ms?: number;
  page?: {
    host?: string;
    status?: number;
    loaded: boolean;
    frames: number;
    nudges: number;
    popups_closed?: number;
    budget_seconds?: number;
    /** Page-session cookies carried over to the server's stream requests. Counts only. */
    cookies?: number;
    /** Bot-protection clearance cookies deliberately not carried over. */
    cookies_withheld?: number;
    /** Names of the withheld bot-protection cookies, so the label's reason is visible. */
    cookies_withheld_names?: string[];
    /** Hosts the cookies belong to. Hostnames only, never values. */
    cookie_domains?: string[];
    /** Hosts of the frames the page loaded. */
    frame_hosts?: string[];
    /** Hosts the top-level page redirected through, in order. */
    redirects?: string[];
  };
  manifests?: {
    host: string;
    protocol?: string;
    kind?: string;
    live: boolean;
    /** chosen, usable, drm_protected, refused, not_allowed, not_a_manifest, not_loaded, unmatched, unchecked */
    verdict: string;
    detail?: string;
    drm?: string[];
    /** How many cookies the page's player sent with its own request for this manifest. */
    browser_cookies?: number;
  }[];
  drm?: {
    /** The stream itself needs DRM, as opposed to the page only asking what the browser supports. */
    confirmed: boolean;
    systems?: string[];
    key_systems?: string[];
    active_key_systems?: string[];
    init_data_types?: string[];
    schemes?: string[];
    license_hosts?: string[];
    signals?: string[];
    protected_renditions?: number;
    renditions?: number;
  };
}

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
  last_outcome?: ResolveOutcome;
  last_diagnostics?: ResolveDiagnostics;
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
  /** Headers the proxy sends upstream, by name only; the values are credentials. */
  upstream_headers?: string[];
  outcome?: ResolveOutcome;
  reason?: string;
  diagnostics?: ResolveDiagnostics;
}

export interface LiveChannelsResponse {
  channels: LiveChannel[] | null;
  resolvers: string[];
}
