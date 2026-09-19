import React, { useMemo, useState } from "react";
import { useFriends } from "../hooks/useFriends";
import { useRoom } from "../hooks/useRoom";
import { Button } from "../components/common/Button";
import { Badge } from "../components/common/Badge";
import { Spinner } from "../components/common/Spinner";
import { FriendRow } from "../components/friends/FriendRow";
import { AddFriendPanel } from "../components/friends/AddFriendPanel";
import { Users, UserPlus, Inbox, Check, X, UserMinus, Tv } from "lucide-react";
import styles from "./FriendsPage.module.css";

type Tab = "friends" | "pending" | "add";

/** "3 days ago" without pulling in a date library for four call sites. */
function relativeTime(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const days = Math.floor((Date.now() - then) / 86_400_000);
  if (days <= 0) return "today";
  if (days === 1) return "yesterday";
  if (days < 30) return `${days} days ago`;
  const months = Math.floor(days / 30);
  return months === 1 ? "a month ago" : `${months} months ago`;
}

export const FriendsPage: React.FC = () => {
  const {
    friends,
    incomingRequests,
    outgoingRequests,
    isLoadingFriends,
    friendsError,
    acceptRequest,
    declineRequest,
    cancelRequest,
    removeFriend,
  } = useFriends();
  const { activeRoom, inviteUser } = useRoom();

  const [tab, setTab] = useState<Tab>("friends");
  const [busyId, setBusyId] = useState<string | null>(null);
  const [invited, setInvited] = useState<Record<string, boolean>>({});

  const pendingCount = incomingRequests.length;
  const totalPending = useMemo(
    () => incomingRequests.length + outgoingRequests.length,
    [incomingRequests.length, outgoingRequests.length],
  );

  /** Runs one friend action at a time and keeps that row's buttons busy. */
  const run = async (id: string, action: () => Promise<void>) => {
    setBusyId(id);
    try {
      await action();
    } catch {
      /* friendsError renders the message */
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>Friends</h1>
          <p className={styles.subtitle}>
            {friends.length} {friends.length === 1 ? "friend" : "friends"}
            {pendingCount > 0 && ` · ${pendingCount} waiting on you`}
          </p>
        </div>
        {tab !== "add" && (
          <Button
            variant="primary"
            size="sm"
            icon={<UserPlus size={14} />}
            onClick={() => setTab("add")}
          >
            Add Friend
          </Button>
        )}
      </div>

      <div className={styles.tabs} role="tablist" aria-label="Friends views">
        <button
          role="tab"
          aria-selected={tab === "friends"}
          className={`${styles.tab} ${tab === "friends" ? styles.tabActive : ""}`}
          onClick={() => setTab("friends")}
        >
          <Users size={14} /> All
          {friends.length > 0 && <Badge>{friends.length}</Badge>}
        </button>
        <button
          role="tab"
          aria-selected={tab === "pending"}
          className={`${styles.tab} ${tab === "pending" ? styles.tabActive : ""}`}
          onClick={() => setTab("pending")}
        >
          <Inbox size={14} /> Pending
          {totalPending > 0 && (
            <Badge variant={pendingCount > 0 ? "danger" : "default"}>
              {totalPending}
            </Badge>
          )}
        </button>
        <button
          role="tab"
          aria-selected={tab === "add"}
          className={`${styles.tab} ${tab === "add" ? styles.tabActive : ""}`}
          onClick={() => setTab("add")}
        >
          <UserPlus size={14} /> Add Friend
        </button>
      </div>

      <div className={styles.body}>
        {/* The add panel owns its own errors so it can pair them with its form. */}
        {friendsError && tab !== "add" && (
          <div className={styles.error}>{friendsError}</div>
        )}

        {isLoadingFriends && friends.length === 0 && tab !== "add" && (
          <div className={styles.loading}>
            <Spinner size={16} />
            Loading friends…
          </div>
        )}

        {tab === "friends" &&
          (friends.length === 0 && !isLoadingFriends ? (
            <div className={styles.empty}>
              <Users
                size={28}
                style={{ color: "var(--color-text-muted)", opacity: 0.5 }}
              />
              <div>
                <p className={styles.emptyTitle}>No friends yet</p>
                <p className={styles.emptyHint}>
                  Add someone by username and they'll show up here once they
                  accept.
                </p>
              </div>
              <Button
                variant="primary"
                size="sm"
                icon={<UserPlus size={14} />}
                onClick={() => setTab("add")}
              >
                Add Friend
              </Button>
            </div>
          ) : (
            <div className={styles.list}>
              {friends.map((friend) => (
                <FriendRow
                  key={friend.friendship_id}
                  username={friend.username}
                  avatarUrl={friend.avatar_url}
                  meta={`Friends since ${relativeTime(friend.friends_since)}`}
                >
                  {/* Inviting is only meaningful from inside a room, so it appears
                      only when there is one to invite them to. */}
                  {activeRoom && (
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<Tv size={12} />}
                      disabled={invited[friend.user_id]}
                      onClick={async () => {
                        try {
                          await inviteUser(friend.username);
                          setInvited((prev) => ({
                            ...prev,
                            [friend.user_id]: true,
                          }));
                        } catch {
                          /* roomError renders in the room view */
                        }
                      }}
                    >
                      {invited[friend.user_id]
                        ? "Invited"
                        : `Invite to ${activeRoom.name}`}
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="sm"
                    icon={<UserMinus size={12} />}
                    isLoading={busyId === friend.friendship_id}
                    onClick={() =>
                      run(friend.friendship_id, () =>
                        removeFriend(friend.user_id),
                      )
                    }
                    aria-label={`Remove ${friend.username}`}
                  >
                    Remove
                  </Button>
                </FriendRow>
              ))}
            </div>
          ))}

        {tab === "pending" && (
          <>
            {totalPending === 0 && !isLoadingFriends && (
              <div className={styles.empty}>
                <Inbox
                  size={28}
                  style={{ color: "var(--color-text-muted)", opacity: 0.5 }}
                />
                <div>
                  <p className={styles.emptyTitle}>Nothing pending</p>
                  <p className={styles.emptyHint}>
                    Friend requests you send or receive will land here.
                  </p>
                </div>
              </div>
            )}

            {incomingRequests.length > 0 && (
              <section>
                <div className={styles.sectionLabel}>
                  <Inbox size={12} /> Incoming
                  <Badge variant="danger">{incomingRequests.length}</Badge>
                </div>
                <div className={styles.list}>
                  {incomingRequests.map((req) => (
                    <FriendRow
                      key={req.id}
                      username={req.username}
                      avatarUrl={req.avatar_url}
                      meta={`Asked ${relativeTime(req.created_at)}`}
                    >
                      <Button
                        variant="secondary"
                        size="sm"
                        icon={<X size={12} />}
                        isLoading={busyId === req.id}
                        onClick={() =>
                          run(req.id, () => declineRequest(req.id))
                        }
                      >
                        Decline
                      </Button>
                      <Button
                        variant="primary"
                        size="sm"
                        icon={<Check size={12} />}
                        isLoading={busyId === req.id}
                        onClick={() => run(req.id, () => acceptRequest(req.id))}
                      >
                        Accept
                      </Button>
                    </FriendRow>
                  ))}
                </div>
              </section>
            )}

            {outgoingRequests.length > 0 && (
              <section>
                <div className={styles.sectionLabel}>
                  <UserPlus size={12} /> Sent
                  <Badge>{outgoingRequests.length}</Badge>
                </div>
                <div className={styles.list}>
                  {outgoingRequests.map((req) => (
                    <FriendRow
                      key={req.id}
                      username={req.username}
                      avatarUrl={req.avatar_url}
                      meta={`Sent ${relativeTime(req.created_at)} · awaiting reply`}
                    >
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={<X size={12} />}
                        isLoading={busyId === req.id}
                        onClick={() => run(req.id, () => cancelRequest(req.id))}
                      >
                        Cancel
                      </Button>
                    </FriendRow>
                  ))}
                </div>
              </section>
            )}
          </>
        )}

        {tab === "add" && <AddFriendPanel />}
      </div>
    </div>
  );
};
