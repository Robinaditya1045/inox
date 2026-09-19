import React, { useCallback, useEffect, useState, type ReactNode } from "react";
import { FriendsContext } from "../contexts/friends.context";
import { friendService } from "../services/friend/friend.service";
import type {
  Friend,
  FriendRequest,
  FriendRequestOutcome,
  UserSearchResult,
} from "../types";
import { useAuth } from "../hooks/useAuth";
import { APIError } from "../api/client";
import { logger } from "../utils/logger";

interface FriendsProviderProps {
  children: ReactNode;
}

export const FriendsProvider: React.FC<FriendsProviderProps> = ({
  children,
}) => {
  const [friends, setFriends] = useState<Friend[]>([]);
  const [incomingRequests, setIncomingRequests] = useState<FriendRequest[]>([]);
  const [outgoingRequests, setOutgoingRequests] = useState<FriendRequest[]>([]);
  const [isLoadingFriends, setIsLoadingFriends] = useState(false);
  const [friendsError, setFriendsError] = useState<string | null>(null);

  const { user } = useAuth();

  const clearFriendsError = useCallback(() => setFriendsError(null), []);

  // One refresh for both lists: accepting a request moves a row from one to the
  // other, so refetching only half of it would leave the UI disagreeing with itself.
  const refreshFriends = useCallback(async () => {
    if (!user) {
      setFriends([]);
      setIncomingRequests([]);
      setOutgoingRequests([]);
      return;
    }
    setIsLoadingFriends(true);
    try {
      const [friendList, requests] = await Promise.all([
        friendService.listFriends(),
        friendService.listRequests(),
      ]);
      setFriends(friendList);
      setIncomingRequests(requests.incoming);
      setOutgoingRequests(requests.outgoing);
    } catch (err) {
      logger.warn("FriendsProvider: Failed to refresh friends", { err });
    } finally {
      setIsLoadingFriends(false);
    }
  }, [user]);

  useEffect(() => {
    refreshFriends();
  }, [refreshFriends]);

  /** Surfaces the server's message — "already friends", "no user found with that
   *  username" — rather than a generic failure, since every one of them is
   *  something the person typing can act on. */
  const failWith = useCallback((err: unknown, fallback: string): Error => {
    const message = err instanceof APIError ? err.message : fallback;
    setFriendsError(message);
    return err instanceof Error ? err : new Error(message);
  }, []);

  const sendRequest = useCallback(
    async (username: string): Promise<FriendRequestOutcome> => {
      setFriendsError(null);
      try {
        const outcome = await friendService.sendRequest(username);
        await refreshFriends();
        return outcome;
      } catch (err) {
        throw failWith(err, "Failed to send friend request.");
      }
    },
    [refreshFriends, failWith],
  );

  const acceptRequest = useCallback(
    async (requestId: string) => {
      setFriendsError(null);
      try {
        await friendService.acceptRequest(requestId);
        await refreshFriends();
      } catch (err) {
        throw failWith(err, "Failed to accept friend request.");
      }
    },
    [refreshFriends, failWith],
  );

  const declineRequest = useCallback(
    async (requestId: string) => {
      setFriendsError(null);
      try {
        await friendService.declineRequest(requestId);
        await refreshFriends();
      } catch (err) {
        throw failWith(err, "Failed to decline friend request.");
      }
    },
    [refreshFriends, failWith],
  );

  const cancelRequest = useCallback(
    async (requestId: string) => {
      setFriendsError(null);
      try {
        await friendService.cancelRequest(requestId);
        await refreshFriends();
      } catch (err) {
        throw failWith(err, "Failed to cancel friend request.");
      }
    },
    [refreshFriends, failWith],
  );

  const removeFriend = useCallback(
    async (userId: string) => {
      setFriendsError(null);
      try {
        await friendService.removeFriend(userId);
        await refreshFriends();
      } catch (err) {
        throw failWith(err, "Failed to remove friend.");
      }
    },
    [refreshFriends, failWith],
  );

  // Search is deliberately not cached in provider state: results carry a
  // relationship that goes stale the moment any of the actions above run.
  const searchUsers = useCallback(
    async (query: string): Promise<UserSearchResult[]> => {
      try {
        return await friendService.searchUsers(query);
      } catch (err) {
        logger.warn("FriendsProvider: User search failed", { err });
        return [];
      }
    },
    [],
  );

  const value = {
    friends,
    incomingRequests,
    outgoingRequests,
    isLoadingFriends,
    friendsError,
    sendRequest,
    acceptRequest,
    declineRequest,
    cancelRequest,
    removeFriend,
    searchUsers,
    refreshFriends,
    clearFriendsError,
  };

  return (
    <FriendsContext.Provider value={value}>{children}</FriendsContext.Provider>
  );
};
