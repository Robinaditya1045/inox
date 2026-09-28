import React, { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useAuth } from "../hooks/useAuth";
import { useRoom } from "../hooks/useRoom";
import { useInvitationActions } from "../hooks/useInvitationActions";
import { useMediaLibrary } from "../hooks/useMediaLibrary";
import { useRoomListPolling } from "../hooks/useRoomListPolling";
import { Button } from "../components/common/Button";
import { Badge } from "../components/common/Badge";
import { Avatar } from "../components/common/Avatar";
import { Skeleton } from "../components/common/Skeleton";
import { EmptyState } from "../components/common/EmptyState";
import { PageHeader } from "../components/layout/PageHeader";
import { CreateRoomModal } from "../components/room/CreateRoomModal";
import { StreamingRoomCard } from "../components/room/StreamingRoomCard";
import { Logo } from "../components/common/Logo";
import {
  Tv,
  Plus,
  Check,
  X,
  Bell,
  Globe,
  Lock,
  ArrowRight,
  Radio,
} from "lucide-react";
import { stagger } from "../utils/motion";
import {
  describeMedia,
  isStreaming,
  sortRoomsByActivity,
  viewersOf,
} from "../utils/roomActivity";
import styles from "./DashboardPage.module.css";

export const DashboardPage: React.FC = () => {
  const { user } = useAuth();
  const { rooms, invitations, hasLoadedRooms } = useRoom();
  const invites = useInvitationActions();
  // Titles and artwork for what rooms are playing come from the media library.
  const { assets: library } = useMediaLibrary();
  useRoomListPolling();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  // Only a first load shows skeletons; later refreshes (creating a room, joining)
  // keep the list on screen instead of blanking it.
  const showSkeleton = !hasLoadedRooms && rooms.length === 0;

  // Rooms playing something get their own section at the top; everything else
  // follows, with rooms people are sitting in ahead of empty ones.
  const { streaming, others } = useMemo(() => {
    const ordered = sortRoomsByActivity(rooms);
    return {
      streaming: ordered.filter(isStreaming),
      others: ordered.filter((room) => !isStreaming(room)),
    };
  }, [rooms]);

  return (
    <div className={styles.page}>
      <PageHeader icon={<Logo size={22} glyphOnly />} title="Lobby" />

      <div className={styles.scroll}>
        <div className={styles.content}>
          <section className={styles.hero} aria-label="Summary">
            <p className={styles.heroTitle}>
              Welcome back, {user?.username ?? "there"}
            </p>
            <p className={styles.heroMeta}>
              {rooms.length === 0 ? (
                hasLoadedRooms ? (
                  "Nothing's on yet. Start a room and bring people in."
                ) : (
                  "Checking what's on…"
                )
              ) : (
                <>
                  <strong>{rooms.length}</strong>{" "}
                  {rooms.length === 1 ? "room" : "rooms"} open
                  {streaming.length > 0 && (
                    <>
                      {" · "}
                      <strong>{streaming.length}</strong> streaming now
                    </>
                  )}
                  {invitations.length > 0 && (
                    <>
                      {" · "}
                      <strong>{invitations.length}</strong>{" "}
                      {invitations.length === 1 ? "invite" : "invites"} waiting
                    </>
                  )}
                </>
              )}
            </p>
          </section>

          {/* Pending Invitations */}
          {invitations.length > 0 && (
            <section className={styles.section} aria-labelledby="lobby-invites">
              <div className={styles.sectionHeader}>
                <Bell size={13} aria-hidden="true" />
                <h2 id="lobby-invites">Pending Invitations</h2>
                <Badge variant="danger" pop>
                  {invitations.length}
                </Badge>
              </div>

              <div className={styles.inviteList}>
                {invitations.map((inv, i) => (
                  <div
                    key={inv.id}
                    className={styles.invite}
                    style={stagger(i)}
                  >
                    <Avatar
                      username={inv.room_name || "Private Room"}
                      shape="square"
                      size="md"
                    />
                    <div className={styles.inviteText}>
                      <span className={styles.inviteName}>
                        {inv.room_name || "Private Room"}
                      </span>
                      <span className={styles.inviteFrom}>
                        Invited by @{inv.inviter_name || "someone"}
                      </span>
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
                        Accept
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </section>
          )}

          {/* Now streaming */}
          {streaming.length > 0 && (
            <section
              className={styles.section}
              aria-labelledby="lobby-streaming"
            >
              <div className={styles.sectionHeader}>
                <Radio size={13} aria-hidden="true" />
                <h2 id="lobby-streaming">Now Streaming</h2>
                <Badge variant="live">{streaming.length}</Badge>
              </div>
              <div className={styles.streamGrid}>
                {streaming.map((room, i) => (
                  <StreamingRoomCard
                    key={room.id}
                    room={room}
                    index={i}
                    preview={describeMedia(
                      room.activity?.media_url || room.current_media_url || "",
                      library,
                      room.activity?.kind,
                    )}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Every other room */}
          {(others.length > 0 || streaming.length === 0) && (
            <section className={styles.section} aria-labelledby="lobby-rooms">
              <div className={styles.sectionHeader}>
                <Tv size={13} aria-hidden="true" />
                <h2 id="lobby-rooms">
                  {streaming.length > 0 ? "Other Rooms" : "Active Rooms"}
                </h2>
              </div>

              {showSkeleton ? (
                <div
                  className={styles.grid}
                  aria-busy="true"
                  aria-label="Loading rooms"
                >
                  {[0, 1, 2].map((i) => (
                    <div key={i} className={styles.skeletonCard}>
                      <div className={styles.skeletonRow}>
                        <Skeleton
                          width={36}
                          height={36}
                          radius="var(--radius-lg)"
                        />
                        <div className={styles.skeletonLines}>
                          <Skeleton width={`${72 - i * 14}%`} height={12} />
                          <Skeleton width="30%" height={9} />
                        </div>
                      </div>
                      <Skeleton width="42%" height={10} />
                    </div>
                  ))}
                </div>
              ) : rooms.length === 0 ? (
                <EmptyState
                  card
                  as="h3"
                  icon={<Tv size={24} />}
                  title="No rooms are active right now"
                  description="Start a watch party, then share the link or invite friends from inside the room."
                  action={
                    <Button
                      variant="primary"
                      icon={<Plus size={14} />}
                      onClick={() => setIsCreateOpen(true)}
                    >
                      Create Room
                    </Button>
                  }
                />
              ) : (
                <div className={styles.grid}>
                  {others.map((room, i) => {
                    const memberCount = room.members?.length ?? 0;
                    const here = viewersOf(room);
                    return (
                      <Link
                        key={room.id}
                        to={`/room/${room.id}`}
                        className={styles.card}
                        style={stagger(i)}
                        aria-label={`Join ${room.name}`}
                      >
                        <div className={styles.cardTop}>
                          <Avatar
                            username={room.name}
                            shape="square"
                            size="md"
                          />
                          <div className={styles.cardTitle}>
                            <span className={styles.cardName}>{room.name}</span>
                            <span className={styles.cardPrivacy}>
                              {room.is_private ? (
                                <>
                                  <Lock size={11} aria-hidden="true" /> Private
                                </>
                              ) : (
                                <>
                                  <Globe size={11} aria-hidden="true" /> Public
                                </>
                              )}
                            </span>
                          </div>
                        </div>

                        <div className={styles.cardFooter}>
                          <span className={styles.presence}>
                            <span
                              className={`${styles.presenceDot} ${here > 0 ? styles.presenceDotLive : ""}`}
                              aria-hidden="true"
                            />
                            {here > 0
                              ? `${here} here now`
                              : memberCount > 0
                                ? `${memberCount} ${memberCount === 1 ? "member" : "members"}`
                                : "No one here yet"}
                          </span>
                          <span className={styles.joinChip} aria-hidden="true">
                            Join <ArrowRight size={12} />
                          </span>
                        </div>
                      </Link>
                    );
                  })}
                </div>
              )}
            </section>
          )}
        </div>
      </div>

      <CreateRoomModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
      />
    </div>
  );
};
