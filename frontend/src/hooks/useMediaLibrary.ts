import { useState, useEffect, useCallback } from 'react';
import { apiClient } from '../api/client';
import type { MediaAsset } from '../types/media';

interface UseMediaLibraryReturn {
  assets: MediaAsset[];
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
}

export const useMediaLibrary = (): UseMediaLibraryReturn => {
  const [assets, setAssets] = useState<MediaAsset[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchAssets = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await apiClient.get<MediaAsset[]>('/media', {
        params: { limit: 100, offset: 0 },
      });
      // Only surface ready assets to room members
      const ready = (data || []).filter((a) => a.status === 'ready');
      setAssets(ready);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load media library');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAssets();
  }, [fetchAssets]);

  return { assets, isLoading, error, refresh: fetchAssets };
};
