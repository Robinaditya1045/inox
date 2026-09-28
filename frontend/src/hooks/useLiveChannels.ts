import { useCallback, useEffect, useState } from "react";
import { apiClient } from "../api/client";
import type { LiveChannelSummary } from "../types/media";

interface UseLiveChannelsReturn {
  /** Keyed by the channel's media asset id, to join onto library entries */
  byAssetId: Map<string, LiveChannelSummary>;
  isLoading: boolean;
  refresh: () => void;
}

/**
 * How each live channel is sourced and whether it is up. Optional detail: if the
 * request fails the library still lists live channels, just without the labels.
 */
export const useLiveChannels = (): UseLiveChannelsReturn => {
  const [byAssetId, setByAssetId] = useState<Map<string, LiveChannelSummary>>(
    () => new Map(),
  );
  const [isLoading, setIsLoading] = useState(true);
  // Bumped by refresh(); each value is one fetch.
  const [version, setVersion] = useState(0);

  useEffect(() => {
    // A slow response for an older fetch must not overwrite a newer one.
    let cancelled = false;
    apiClient
      .get<LiveChannelSummary[]>("/live/channels")
      .then((data) => {
        if (cancelled) return;
        setByAssetId(new Map((data || []).map((c) => [c.media_asset_id, c])));
      })
      .catch(() => {
        if (!cancelled) setByAssetId(new Map());
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [version]);

  const refresh = useCallback(() => {
    setIsLoading(true);
    setVersion((v) => v + 1);
  }, []);

  return { byAssetId, isLoading, refresh };
};
