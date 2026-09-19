export interface Friend {
  friendship_id: string;
  user_id: string;
  username: string;
  avatar_url?: string;
  friends_since: string;
}

export type FriendRequestDirection = "incoming" | "outgoing";

export interface FriendRequest {
  id: string;
  direction: FriendRequestDirection;
  user_id: string;
  username: string;
  avatar_url?: string;
  created_at: string;
}

export interface FriendRequestsResponse {
  incoming: FriendRequest[];
  outgoing: FriendRequest[];
}

/**
 * What sending a request actually did. Adding someone who already asked you
 * accepts their request instead of opening a second one, so the caller has to
 * branch on this rather than assume a request is now pending.
 */
export interface FriendRequestOutcome {
  status: "pending" | "accepted";
  request?: FriendRequest;
  friend?: Friend;
}

export type UserRelationship =
  "none" | "friends" | "request_sent" | "request_received";

export interface UserSearchResult {
  user_id: string;
  username: string;
  avatar_url?: string;
  relationship: UserRelationship;
}
