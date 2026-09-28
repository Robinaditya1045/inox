import React, { useMemo, useState } from "react";
import { NavLink } from "react-router-dom";
import { useRoom } from "../../hooks/useRoom";
import { useFriends } from "../../hooks/useFriends";
import { useInvitationActions } from "../../hooks/useInvitationActions";
import { CreateRoomModal } from "../room/CreateRoomModal";
import { Badge } from "../common/Badge";
import { Button } from "../common/Button";
import { IconButton } from "../common/IconButton";
import { Avatar } from "../common/Avatar";
import { Skeleton } from "../common/Skeleton";
import { EmptyState } from "../common/EmptyState";
import { UserTray } from "../room/UserTray";
import { Plus, Lock, Bell, Check, X, Users, Tv, Radio } from "lucide-react";
import { stagger } from "../../utils/motion";
import {
  isStreaming,
  sortRoomsByActivity,
  viewersOf,
} from "../../utils/roomActivity";
import styles from "./HomeSidebar.module.css";

export const HomeSidebar: React.FC = () => {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const { rooms, activeRoom, invitations, hasLoadedRooms } = useRoom();
  const { friends, incomingRequests } = useFriends();
  const invites = useInvitationActions();

  const showSkeleton = !hasLoadedRooms && rooms.length === 0;
  // Same order as the lobby: streaming rooms first, then occupied, then empty.
  const orderedRooms = useMemo(() => sortRoomsByActivity(rooms), [rooms]);

  return (
    <>
      <aside className={styles.sidebar} aria-label="Rooms and invitations">
        <div className={styles.header}>
          <span className={styles.headerTitle}>Lobby</span>
          <IconButton
            label="Create a room"
            size="sm"
            onClick={() => setIsCreateOpen(true)}
            tooltipSide="bottom"
          >
            <Plus size={16} />
          </IconButton>
        </div>

        <div className={styles.scroll}>
          {/* Friends — the lobby's other destination, kept above the room list so a
              waiting friend request is visible without opening the page. */}
          <nav className={styles.section} aria-label="Lobby">
            <NavLink
              to="/friends"
              className={({ isActive }) =>
                `${styles.navItem} ${isActive ? styles.navItemActive : ""}`
              }
            >
              <span className={styles.navIcon} aria-hidden="true">
                <Users size={20} />
              </span>
              <span className={styles.navLabel}>
                <span className={styles.navName}>Friends</span>
              </span>
              {incomingRequests.length > 0 ? (
                <Badge variant="danger" pop>
                  {incomingRequests.length}
                </Badge>
              ) : (
                friends.length > 0 && (
                  <span className={styles.count}>{friends.length}</span>
                )
              )}
            </NavLink>
          </nav>

          {/* Pending Invitations */}
          {invitations.length > 0 && (
            <section
              className={styles.section}
              aria-labelledby="sidebar-invites"
            >
              <div className={styles.sectionLabel}>
                <span className={styles.sectionLabelText} id="sidebar-invites">
                  <Bell size={12} aria-hidden="true" />
                  Invites
                </span>
                <Badge variant="danger" pop>
                  {invitations.length}
                </Badge>
              </div>

              {invitations.map((inv, i) => (
                <div key={inv.id} className={styles.invite} style={stagger(i)}>
                  <div>
                    <div className={styles.inviteName}>
                      {inv.room_name || "Private Room"}
                    </div>
                    <div className={styles.inviteFrom}>
                      from @{inv.inviter_name || "someone"}
                    </div>
                  </div>
                  <div className={styles.inviteActions}>
                    <Button
                      variant="secondary"
                      size="sm"
                      icon={<X size={12} />}
                      isLoading={invites.isBusy(inv, "decline")}
                      disabled={invites.isBusy(inv, "accept")}
                      onClick={() => invites.decline(inv)}
                    >
                      Decline
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      icon={<Check size={12} />}
                      isLoading={invites.isBusy(inv, "accept")}
                      disabled={invites.isBusy(inv, "decline")}
                      onClick={() => invites.accept(inv)}
                    >
                      Join
                    </Button>
                  </div>
                </div>
              ))}
            </section>
          )}

          {/* Room List */}
          <section className={styles.section} aria-labelledby="sidebar-rooms">
            <div className={styles.sectionLabel}>
              <span id="sidebar-rooms">Active rooms — {rooms.length}</span>
              <IconButton
                label="Create a room"
                size="sm"
                variant="ghost"
                onClick={() => setIsCreateOpen(true)}
              >
                <Plus size={14} />
              </IconButton>
            </div>

            {showSkeleton && (
              <div aria-busy="true" aria-label="Loading rooms">
                {[0, 1, 2].map((i) => (
                  <div key={i} className={styles.skeletonRow}>
                    <Skeleton
                      width={32}
                      height={32}
                      radius="var(--radius-lg)"
                    />
                    <div className={styles.skeletonText}>
                      <Skeleton width={`${70 - i * 12}%`} height={10} />
                      <Skeleton width="36%" height={8} />
                    </div>
                  </div>
                ))}
              </div>
            )}

            {rooms.length === 0 && hasLoadedRooms && (
              <EmptyState
                compact
                icon={<Tv size={20} />}
                title="No rooms are live"
                description="Start one and invite your friends."
                action={
                  <Button
                    size="sm"
                    icon={<Plus size={14} />}
                    onClick={() => setIsCreateOpen(true)}
                  >
                    Create room
                  </Button>
                }
              />
            )}

            {orderedRooms.map((room, i) => {
              const isCurrent = activeRoom?.id === room.id;
              const memberCount = room.members?.length ?? 0;
              const streaming = isStreaming(room);
              const here = viewersOf(room);

              return (
                <NavLink
                  key={room.id}
                  to={`/room/${room.id}`}
                  title={room.name}
                  style={stagger(i + 1)}
                  className={({ isActive }) =>
                    `${styles.navItem} ${isActive || isCurrent ? styles.navItemActive : ""}`
                  }
                >
                  <Avatar username={room.name} size="sm" shape="square" />
                  <span className={styles.navLabel}>
                    <span className={styles.navName}>{room.name}</span>
                    <span className={styles.navMeta}>
                      {streaming ? (
                        <span className={styles.streamingMeta}>
                          <Radio size={11} aria-hidden="true" />
                          Streaming · {here} watching
                        </span>
                      ) : here > 0 ? (
                        <>
                          <span className={styles.liveDot} aria-hidden="true" />
                          {here} here now
                        </>
                      ) : memberCount > 0 ? (
                        <>
                          {memberCount}{" "}
                          {memberCount === 1 ? "member" : "members"}
                        </>
                      ) : (
                        "No one here yet"
                      )}
                      {room.is_private && (
                        <>
                          {" · "}
                          <Lock size={10} aria-label="Private" />
                        </>
                      )}
                    </span>
                  </span>
                </NavLink>
              );
            })}
          </section>
        </div>

        <UserTray />
      </aside>

      <CreateRoomModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
      />
    </>
  );
};
