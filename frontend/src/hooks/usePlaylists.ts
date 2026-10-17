import { useCallback, useEffect, useState } from "react";
import { playlistService } from "../services/playlist/playlist.service";
import { APIError } from "../api/client";
import { useToast } from "./useToast";
import type { MediaAsset, Playlist } from "../types/media";

interface UsePlaylistsReturn {
  playlists: Playlist[];
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
  create: (name: string, description?: string) => Promise<Playlist | null>;
  update: (
    id: string,
    name: string,
    description: string,
  ) => Promise<Playlist | null>;
  remove: (id: string) => Promise<boolean>;
  addItem: (playlistId: string, asset: MediaAsset) => Promise<void>;
  /** Adds the video if the playlist lacks it, removes it if it has it. */
  toggleItem: (playlistId: string, asset: MediaAsset) => Promise<void>;
  removeItem: (playlistId: string, itemId: string) => Promise<void>;
  /** Moves one item up (-1) or down (+1) a place. */
  moveItem: (playlistId: string, itemId: string, by: -1 | 1) => Promise<void>;
}

const messageOf = (err: unknown, fallback: string) =>
  err instanceof APIError ? err.message : fallback;

/**
 * The signed-in user's playlists. Each write replaces the playlist with the
 * server's copy, so the list never drifts from what is stored; a failed write
 * is reported as a toast and leaves the list as it was.
 */
export const usePlaylists = (): UsePlaylistsReturn => {
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { toast } = useToast();

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setPlaylists(await playlistService.list());
    } catch (err) {
      setError(messageOf(err, "Failed to load your playlists"));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  // Most recently edited first, matching the server's order.
  const replace = useCallback((updated: Playlist) => {
    setPlaylists((prev) => [
      updated,
      ...prev.filter((p) => p.id !== updated.id),
    ]);
  }, []);

  const fail = useCallback(
    (err: unknown, title: string) =>
      toast({ title, description: messageOf(err, ""), tone: "danger" }),
    [toast],
  );

  const create = useCallback(
    async (name: string, description = "") => {
      try {
        const created = await playlistService.create(name, description);
        replace(created);
        return created;
      } catch (err) {
        fail(err, "Couldn't create the playlist");
        return null;
      }
    },
    [replace, fail],
  );

  const update = useCallback(
    async (id: string, name: string, description: string) => {
      try {
        const updated = await playlistService.update(id, name, description);
        replace(updated);
        return updated;
      } catch (err) {
        fail(err, "Couldn't save the playlist");
        return null;
      }
    },
    [replace, fail],
  );

  const remove = useCallback(
    async (id: string) => {
      try {
        await playlistService.remove(id);
        setPlaylists((prev) => prev.filter((p) => p.id !== id));
        return true;
      } catch (err) {
        fail(err, "Couldn't delete the playlist");
        return false;
      }
    },
    [fail],
  );

  // Needs no copy of the playlist, so it also works on one created a moment ago.
  const addItem = useCallback(
    async (playlistId: string, asset: MediaAsset) => {
      try {
        const updated = await playlistService.addItem(playlistId, asset.id);
        replace(updated);
        toast({
          title: `Saved to ${updated.name}`,
          tone: "success",
          duration: 2500,
        });
      } catch (err) {
        fail(err, "Couldn't add the video");
      }
    },
    [replace, fail, toast],
  );

  const toggleItem = useCallback(
    async (playlistId: string, asset: MediaAsset) => {
      const playlist = playlists.find((p) => p.id === playlistId);
      if (!playlist) return;
      const existing = playlist.items.find(
        (item) => item.media_asset_id === asset.id,
      );
      try {
        const updated = existing
          ? await playlistService.removeItem(playlistId, existing.id)
          : await playlistService.addItem(playlistId, asset.id);
        replace(updated);
        toast({
          title: existing
            ? `Removed from ${playlist.name}`
            : `Saved to ${playlist.name}`,
          tone: "success",
          duration: 2500,
        });
      } catch (err) {
        fail(err, "Couldn't update the playlist");
      }
    },
    [playlists, replace, fail, toast],
  );

  const removeItem = useCallback(
    async (playlistId: string, itemId: string) => {
      try {
        replace(await playlistService.removeItem(playlistId, itemId));
      } catch (err) {
        fail(err, "Couldn't remove the video");
      }
    },
    [replace, fail],
  );

  const moveItem = useCallback(
    async (playlistId: string, itemId: string, by: -1 | 1) => {
      const playlist = playlists.find((p) => p.id === playlistId);
      if (!playlist) return;
      const ids = playlist.items.map((item) => item.id);
      const from = ids.indexOf(itemId);
      const to = from + by;
      if (from < 0 || to < 0 || to >= ids.length) return;
      [ids[from], ids[to]] = [ids[to], ids[from]];

      // Shown moved straight away; the server's copy then replaces it.
      const byId = new Map(playlist.items.map((item) => [item.id, item]));
      replace({
        ...playlist,
        items: ids.map((id, position) => ({ ...byId.get(id)!, position })),
      });
      try {
        replace(await playlistService.reorder(playlistId, ids));
      } catch (err) {
        replace(playlist);
        fail(err, "Couldn't reorder the playlist");
      }
    },
    [playlists, replace, fail],
  );

  return {
    playlists,
    isLoading,
    error,
    refresh,
    create,
    update,
    remove,
    addItem,
    toggleItem,
    removeItem,
    moveItem,
  };
};
