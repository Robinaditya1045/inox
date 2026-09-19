import { createContext } from "react";
import type {
  Friend,
  FriendRequest,
  FriendRequestOutcome,
  UserSearchResult,
} from "../types";

export interface FriendsContextValue {
  friends: Friend[];
  incomingRequests: FriendRequest[];
  outgoingRequests: FriendRequest[];
  isLoadingFriends: boolean;
  friendsError: string | null;
  /** Sends a request by username. Resolves with what actually happened — a
   *  pending request, or an immediate friendship if they had already asked. */
  sendRequest: (username: string) => Promise<FriendRequestOutcome>;
  acceptRequest: (requestId: string) => Promise<void>;
  declineRequest: (requestId: string) => Promise<void>;
  cancelRequest: (requestId: string) => Promise<void>;
  removeFriend: (userId: string) => Promise<void>;
  searchUsers: (query: string) => Promise<UserSearchResult[]>;
  refreshFriends: () => Promise<void>;
  clearFriendsError: () => void;
}

export const FriendsContext = createContext<FriendsContextValue | null>(null);
