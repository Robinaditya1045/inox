import * as React from "react";
import { apiFetch, apiJson } from "@/lib/api";
import type {
  CreateLiveChannelRequest,
  LiveChannel,
  LiveChannelsResponse,
  LiveTestResult,
} from "@/types/live";

export interface UseLiveChannelsResult {
  channels: LiveChannel[];
  resolvers: string[];
  isLoading: boolean;
  isDemoMode: boolean;
  error: string | null;
  refresh: () => Promise<void>;
  createChannel: (req: CreateLiveChannelRequest) => Promise<LiveChannel>;
  deleteChannel: (id: string) => Promise<void>;
  testResolve: (req: CreateLiveChannelRequest) => Promise<LiveTestResult>;
}

/**
 * Demo fixtures for when the backend is unreachable.
 *
 * Kept obviously fictional and never counted as connected: a fake channel that
 * reads as live is worse than an empty panel, because an operator will try to put
 * it in a room.
 */
const MOCK_CHANNELS: LiveChannel[] = [
  {
    id: "demo-channel-1",
    media_asset_id: "demo-asset-1",
    slug: "demo-news-24",
    resolver: "direct",
    source_url: "https://demo.invalid/live/news24/master.m3u8",
    resolver_config: {},
    fallback_sources: [],
    protocol: "hls",
    is_dvr: true,
    dvr_window_seconds: 120,
    status: "live",
    upstream_expires_at: new Date(Date.now() + 3600_000).toISOString(),
    last_resolved_at: new Date(Date.now() - 120_000).toISOString(),
    last_error: "",
    created_by: null,
    created_at: new Date(Date.now() - 86_400_000).toISOString(),
    updated_at: new Date().toISOString(),
    title: "Demo News 24 (simulated)",
  },
  {
    id: "demo-channel-2",
    media_asset_id: "demo-asset-2",
    slug: "demo-scraped-sports",
    resolver: "static",
    source_url: "https://demo.invalid/watch/sports",
    resolver_config: {},
    fallback_sources: [],
    protocol: "hls",
    is_dvr: false,
    dvr_window_seconds: 0,
    status: "error",
    upstream_expires_at: null,
    last_resolved_at: new Date(Date.now() - 600_000).toISOString(),
    last_error: "no .m3u8 or .mpd URL found in the page source",
    created_by: null,
    created_at: new Date(Date.now() - 172_800_000).toISOString(),
    updated_at: new Date().toISOString(),
    title: "Demo Sports (simulated)",
  },
];

const DEFAULT_RESOLVERS = ["api", "direct", "static"];

export function useLiveChannels(): UseLiveChannelsResult {
  const [channels, setChannels] = React.useState<LiveChannel[]>([]);
  const [resolvers, setResolvers] = React.useState<string[]>(DEFAULT_RESOLVERS);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isDemoMode, setIsDemoMode] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const isDemoModeRef = React.useRef(false);
  React.useEffect(() => {
    isDemoModeRef.current = isDemoMode;
  }, [isDemoMode]);

  const refresh = React.useCallback(async () => {
    if (isDemoModeRef.current) {
      setChannels(MOCK_CHANNELS);
      setIsLoading(false);
      return;
    }
    setIsLoading(true);
    try {
      const data = await apiJson<LiveChannelsResponse>("/admin/live/channels");
      // No channels yet is a real state. Filling it with fixtures would offer an
      // operator rows whose delete and preview actions then fail.
      setChannels(Array.isArray(data.channels) ? data.channels : []);
      if (Array.isArray(data.resolvers) && data.resolvers.length > 0) {
        setResolvers(data.resolvers);
      }
      setError(null);
    } catch (err) {
      console.warn("Live channels unreachable, switching to demo mode:", err);
      setChannels(MOCK_CHANNELS);
      setIsDemoMode(true);
      isDemoModeRef.current = true;
      setError("Backend unreachable. Showing a simulated channel list.");
    } finally {
      setIsLoading(false);
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  const createChannel = React.useCallback(
    async (req: CreateLiveChannelRequest): Promise<LiveChannel> => {
      if (isDemoModeRef.current) {
        throw new Error(
          "Demo mode: connect the backend to create a live channel.",
        );
      }
      const res = await apiFetch("/admin/live/channels", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(req),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        // Every failure here is operator input, so surface the backend's reason
        // rather than a generic message they cannot act on.
        throw new Error(body?.error || "Failed to create live channel");
      }
      await refresh();
      return body as LiveChannel;
    },
    [refresh],
  );

  const deleteChannel = React.useCallback(
    async (id: string) => {
      if (isDemoModeRef.current) {
        setChannels((prev) => prev.filter((c) => c.id !== id));
        return;
      }
      const res = await apiFetch(`/admin/live/channels/${id}`, {
        method: "DELETE",
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body?.error || "Failed to delete live channel");
      }
      await refresh();
    },
    [refresh],
  );

  const testResolve = React.useCallback(
    async (req: CreateLiveChannelRequest): Promise<LiveTestResult> => {
      if (isDemoModeRef.current) {
        throw new Error("Demo mode: connect the backend to test a source.");
      }
      const res = await apiFetch("/admin/live/test-resolve", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(req),
      });
      const body = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(body?.error || "Test resolve failed");
      }
      return body as LiveTestResult;
    },
    [],
  );

  return {
    channels,
    resolvers,
    isLoading,
    isDemoMode,
    error,
    refresh,
    createChannel,
    deleteChannel,
    testResolve,
  };
}
