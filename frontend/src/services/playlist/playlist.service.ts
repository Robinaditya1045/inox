import { apiClient } from "../../api/client";
import type { Playlist } from "../../types/media";
import { logger } from "../../utils/logger";

/** The signed-in user's own playlists. Every write returns the playlist as it now is. */
export const playlistService = {
  async list(): Promise<Playlist[]> {
    const response = await apiClient.get<Playlist[]>("/playlists");
    return Array.isArray(response) ? response : [];
  },

  async create(name: string, description = ""): Promise<Playlist> {
    logger.info("PlaylistService: Creating playlist", { name });
    return apiClient.post<Playlist>("/playlists", { name, description });
  },

  async update(
    id: string,
    name: string,
    description: string,
  ): Promise<Playlist> {
    return apiClient.patch<Playlist>(`/playlists/${id}`, {
      name,
      description,
    });
  },

  async remove(id: string): Promise<void> {
    logger.info("PlaylistService: Deleting playlist", { id });
    await apiClient.delete(`/playlists/${id}`);
  },

  async addItem(id: string, mediaAssetId: string): Promise<Playlist> {
    return apiClient.post<Playlist>(`/playlists/${id}/items`, {
      media_asset_id: mediaAssetId,
    });
  },

  async removeItem(id: string, itemId: string): Promise<Playlist> {
    return apiClient.delete<Playlist>(`/playlists/${id}/items/${itemId}`);
  },

  async reorder(id: string, itemIds: string[]): Promise<Playlist> {
    return apiClient.put<Playlist>(`/playlists/${id}/items/order`, {
      item_ids: itemIds,
    });
  },
};
