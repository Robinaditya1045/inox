import React, { useMemo, useState } from "react";
import { useFriends } from "../hooks/useFriends";
import { useRoom } from "../hooks/useRoom";
import { useToast } from "../hooks/useToast";
import { Button } from "../components/common/Button";
import { Badge } from "../components/common/Badge";
import { Alert } from "../components/common/Alert";
import { Skeleton } from "../components/common/Skeleton";
import { EmptyState } from "../components/common/EmptyState";
import { Tabs } from "../components/common/Tabs";
import { PageHeader } from "../components/layout/PageHeader";
import { FriendRow } from "../components/friends/FriendRow";
import { AddFriendPanel } from "../components/friends/AddFriendPanel";
import { Users, UserPlus, Inbox, Check, X, UserMinus, Tv } from "lucide-react";
import { stagger } from "../utils/motion";
import { tabIds } from "../utils/tabs";
import styles from "./FriendsPage.module.css";

type Tab = "friends" | "pending" | "add";

const TAB_PREFIX = "friends";

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

const RowSkeletons: React.FC = () => (
  <div className={styles.loading} aria-busy="true" aria-label="Loading friends">
    {[0, 1, 2, 3].map((i) => (
      <div key={i} className={styles.skeletonRow}>
        <Skeleton width={36} height={36} circle />
        <div className={styles.skeletonText}>
          <Skeleton width={`${34 - i * 4}%`} height={12} />
          <Skeleton width="22%" height={9} />
        </div>
      </div>
    ))}
  </div>
);

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
  const { toast } = useToast();

  const [tab, setTab] = useState<Tab>("friends");
  const [busyId, setBusyId] = useState<string | null>(null);
  const [invited, setInvited] = useState<Record<string, boolean>>({});
  // Removing a friend takes two clicks: the first arms it, the second confirms.
  const [confirmRemoveId, setConfirmRemoveId] = useState<string | null>(null);

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

  const panel = tabIds(TAB_PREFIX, tab);
  const showSkeleton = isLoadingFriends && friends.length === 0;

  return (
    <div className={styles.page}>
      <PageHeader icon={<Users size={20} />} title="Friends">
        <Tabs<Tab>
          variant="pill"
          ariaLabel="Friends views"
          idPrefix={TAB_PREFIX}
          value={tab}
          onChange={(next) => {
            setTab(next);
            setConfirmRemoveId(null);
          }}
          items={[
            {
              id: "friends",
              label: "All",
              badge:
                friends.length > 0 ? <Badge>{friends.length}</Badge> : null,
            },
            {
              id: "pending",
              label: "Pending",
              badge:
                totalPending > 0 ? (
                  <Badge variant={pendingCount > 0 ? "danger" : "default"} pop>
                    {totalPending}
                  </Badge>
                ) : null,
            },
            {
              id: "add",
              label: "Add Friend",
              icon: <UserPlus size={14} />,
              accent: true,
            },
          ]}
        />
      </PageHeader>

      <div className={styles.body}>
        <div
          key={tab}
          className={styles.panel}
          role="tabpanel"
          id={panel.panel}
          aria-labelledby={panel.tab}
        >
          {/* The add panel owns its own errors so it can pair them with its form. */}
          {friendsError && tab !== "add" && <Alert>{friendsError}</Alert>}

          {tab === "friends" &&
            (showSkeleton ? (
              <RowSkeletons />
            ) : friends.length === 0 && !isLoadingFriends ? (
              <EmptyState
                card
                as="h2"
                icon={<Users size={24} />}
                title="No friends yet"
                description="Add someone by username and they'll show up here once they accept."
                action={
                  <Button
                    variant="primary"
                    icon={<UserPlus size={14} />}
                    onClick={() => setTab("add")}
                  >
                    Add Friend
                  </Button>
                }
              />
            ) : (
              <section aria-label="All friends">
                <div className={styles.sectionLabel}>
                  All friends — {friends.length}
                </div>
                <div className={styles.list}>
                  {friends.map((friend, i) => {
                    const armed = confirmRemoveId === friend.friendship_id;
                    return (
                      <FriendRow
                        key={friend.friendship_id}
                        username={friend.username}
                        avatarUrl={friend.avatar_url}
                        meta={`Friends since ${relativeTime(friend.friends_since)}`}
                        style={stagger(i)}
                      >
                        {/* Inviting is only meaningful from inside a room, so it appears
                            only when there is one to invite them to. */}
                        {activeRoom && !armed && (
                          <Button
                            variant="secondary"
                            size="sm"
                            icon={
                              invited[friend.user_id] ? (
                                <Check size={12} />
                              ) : (
                                <Tv size={12} />
                              )
                            }
                            disabled={invited[friend.user_id]}
                            onClick={async () => {
                              try {
                                await inviteUser(friend.username);
                                setInvited((prev) => ({
                                  ...prev,
                                  [friend.user_id]: true,
                                }));
                                toast({
                                  tone: "success",
                                  title: `Invited ${friend.username}`,
                                  description: `They'll see an invite to ${activeRoom.name}.`,
                                });
                              } catch (err) {
                                toast({
                                  tone: "danger",
                                  title: `Couldn't invite ${friend.username}`,
                                  description:
                                    err instanceof Error
                                      ? err.message
                                      : undefined,
                                });
                              }
                            }}
                          >
                            {invited[friend.user_id]
                              ? "Invited"
                              : `Invite to ${activeRoom.name}`}
                          </Button>
                        )}
                        {armed ? (
                          <span className={styles.confirm}>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => setConfirmRemoveId(null)}
                            >
                              Keep
                            </Button>
                            <Button
                              variant="danger"
                              size="sm"
                              icon={<UserMinus size={12} />}
                              isLoading={busyId === friend.friendship_id}
                              onClick={() =>
                                run(friend.friendship_id, () =>
                                  removeFriend(friend.user_id),
                                ).then(() => setConfirmRemoveId(null))
                              }
                              aria-label={`Confirm removing ${friend.username}`}
                            >
                              Remove friend
                            </Button>
                          </span>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            icon={<UserMinus size={12} />}
                            onClick={() =>
                              setConfirmRemoveId(friend.friendship_id)
                            }
                            aria-label={`Remove ${friend.username}`}
                          >
                            Remove
                          </Button>
                        )}
                      </FriendRow>
                    );
                  })}
                </div>
              </section>
            ))}

          {tab === "pending" && (
            <>
              {showSkeleton && totalPending === 0 && <RowSkeletons />}

              {totalPending === 0 && !isLoadingFriends && (
                <EmptyState
                  card
                  as="h2"
                  icon={<Inbox size={24} />}
                  title="Nothing pending"
                  description="Friend requests you send or receive will land here."
                />
              )}

              {incomingRequests.length > 0 && (
                <section aria-label="Incoming requests">
                  <div className={styles.sectionLabel}>
                    <Inbox size={12} aria-hidden="true" /> Incoming —{" "}
                    {incomingRequests.length}
                  </div>
                  <div className={styles.list}>
                    {incomingRequests.map((req, i) => (
                      <FriendRow
                        key={req.id}
                        username={req.username}
                        avatarUrl={req.avatar_url}
                        meta={`Asked ${relativeTime(req.created_at)}`}
                        style={stagger(i)}
                      >
                        <Button
                          variant="secondary"
                          size="sm"
                          icon={<X size={12} />}
                          disabled={busyId === req.id}
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
                          onClick={() =>
                            run(req.id, () => acceptRequest(req.id))
                          }
                        >
                          Accept
                        </Button>
                      </FriendRow>
                    ))}
                  </div>
                </section>
              )}

              {outgoingRequests.length > 0 && (
                <section aria-label="Sent requests">
                  <div className={styles.sectionLabel}>
                    <UserPlus size={12} aria-hidden="true" /> Sent —{" "}
                    {outgoingRequests.length}
                  </div>
                  <div className={styles.list}>
                    {outgoingRequests.map((req, i) => (
                      <FriendRow
                        key={req.id}
                        username={req.username}
                        avatarUrl={req.avatar_url}
                        meta={`Sent ${relativeTime(req.created_at)} · awaiting reply`}
                        style={stagger(i)}
                      >
                        <Button
                          variant="ghost"
                          size="sm"
                          icon={<X size={12} />}
                          isLoading={busyId === req.id}
                          onClick={() =>
                            run(req.id, () => cancelRequest(req.id))
                          }
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
    </div>
  );
};
