import { apiClient } from "../../api/client";
import type {
  Friend,
  FriendRequestOutcome,
  FriendRequestsResponse,
  UserSearchResult,
} from "../../types";
import { logger } from "../../utils/logger";

export const friendService = {
  async listFriends(): Promise<Friend[]> {
    logger.debug("FriendService: Listing friends");
    const response = await apiClient.get<Friend[]>("/friends");
    return Array.isArray(response) ? response : [];
  },

  async listRequests(): Promise<FriendRequestsResponse> {
    logger.debug("FriendService: Listing friend requests");
    const response =
      await apiClient.get<FriendRequestsResponse>("/friends/requests");
    return {
      incoming: response?.incoming ?? [],
      outgoing: response?.outgoing ?? [],
    };
  },

  async sendRequest(username: string): Promise<FriendRequestOutcome> {
    logger.info("FriendService: Sending friend request", { username });
    return apiClient.post<FriendRequestOutcome>("/friends/requests", {
      username,
    });
  },

  async acceptRequest(requestId: string): Promise<Friend> {
    logger.info("FriendService: Accepting friend request", { requestId });
    return apiClient.post<Friend>(`/friends/requests/${requestId}/accept`);
  },

  async declineRequest(requestId: string): Promise<void> {
    logger.info("FriendService: Declining friend request", { requestId });
    await apiClient.post<void>(`/friends/requests/${requestId}/decline`);
  },

  async cancelRequest(requestId: string): Promise<void> {
    logger.info("FriendService: Cancelling friend request", { requestId });
    await apiClient.delete<void>(`/friends/requests/${requestId}`);
  },

  async removeFriend(userId: string): Promise<void> {
    logger.info("FriendService: Removing friend", { userId });
    await apiClient.delete<void>(`/friends/${userId}`);
  },

  async searchUsers(query: string): Promise<UserSearchResult[]> {
    const response = await apiClient.get<UserSearchResult[]>("/users/search", {
      params: { q: query },
    });
    return Array.isArray(response) ? response : [];
  },
};
