import React, { useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { useRoom } from "../../hooks/useRoom";
import { useFriends } from "../../hooks/useFriends";
import { useAuth } from "../../hooks/useAuth";
import { Plus, Users } from "lucide-react";
import { Tooltip } from "../common/Tooltip";
import { Avatar } from "../common/Avatar";
import { Logo } from "../common/Logo";
import { CreateRoomModal } from "../room/CreateRoomModal";
import { SettingsShell } from "../profile/SettingsShell";
import { stagger } from "../../utils/motion";
import styles from "./AppRail.module.css";

/** Two-letter monogram for a room, e.g. "Friday Night Sci-Fi" -> "FN". */
function roomInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "??";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return (words[0][0] + words[1][0]).toUpperCase();
}

interface AppRailProps {
  /** In a room on tablet, the room sidebar already carries the user tray */
  variant?: "home" | "room";
}

export const AppRail: React.FC<AppRailProps> = ({ variant = "home" }) => {
  const { rooms, invitations } = useRoom();
  const { incomingRequests } = useFriends();
  const { user } = useAuth();
  const location = useLocation();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);

  const isHome = location.pathname === "/";
  const isFriends = location.pathname === "/friends";
  const hasPendingInvites = invitations.length > 0;
  const hasFriendRequests = incomingRequests.length > 0;

  // Labels open to the right, like a server list. (Touch devices never show
  // tooltips, so the phone's bottom bar is unaffected.)
  const side = "right" as const;
  let slotIndex = 0;

  return (
    <>
      <nav
        className={styles.rail}
        data-variant={variant}
        aria-label="Main navigation"
      >
        {/* Lobby */}
        <div
          className={styles.slot}
          data-active={isHome || undefined}
          style={stagger(slotIndex++)}
        >
          <span className={styles.pill} aria-hidden="true" />
          <Tooltip content="Lobby" side={side}>
            <NavLink to="/" end className={styles.item} aria-label="Lobby">
              <Logo size={26} glyphOnly />
              {hasPendingInvites && (
                <span
                  className={styles.badge}
                  aria-label={`${invitations.length} pending invitations`}
                >
                  {invitations.length}
                </span>
              )}
            </NavLink>
          </Tooltip>
        </div>

        {/* Friends — a lobby-level destination, so it sits above the room list */}
        <div
          className={styles.slot}
          data-active={isFriends || undefined}
          style={stagger(slotIndex++)}
        >
          <span className={styles.pill} aria-hidden="true" />
          <Tooltip content="Friends" side={side}>
            <NavLink to="/friends" className={styles.item} aria-label="Friends">
              <Users size={20} />
              {hasFriendRequests && (
                <span
                  className={styles.badge}
                  aria-label={`${incomingRequests.length} pending friend requests`}
                >
                  {incomingRequests.length}
                </span>
              )}
            </NavLink>
          </Tooltip>
        </div>

        <div className={styles.divider} aria-hidden="true" />

        {/* One monogram per room */}
        {rooms.map((room) => {
          const isActive = location.pathname === `/room/${room.id}`;
          return (
            <div
              className={styles.slot}
              key={room.id}
              data-active={isActive || undefined}
              style={stagger(slotIndex++)}
            >
              <span className={styles.pill} aria-hidden="true" />
              <Tooltip content={room.name} side={side}>
                <NavLink
                  to={`/room/${room.id}`}
                  className={styles.item}
                  aria-label={room.name}
                >
                  {roomInitials(room.name)}
                </NavLink>
              </Tooltip>
            </div>
          );
        })}

        {/* Create a room — lives in the rail so it is reachable from inside a room too */}
        <div className={styles.slot} style={stagger(slotIndex++)}>
          <span className={styles.pill} aria-hidden="true" />
          <Tooltip content="Create a room" side={side}>
            <button
              type="button"
              className={`${styles.item} ${styles.itemCreate}`}
              aria-label="Create a room"
              onClick={() => setIsCreateOpen(true)}
            >
              <Plus size={22} />
            </button>
          </Tooltip>
        </div>

        <div className={styles.spacer} />

        {/* Profile & settings — only where no sidebar shows the user tray */}
        <div className={`${styles.slot} ${styles.compactOnly}`}>
          <Tooltip content="Profile & settings" side={side}>
            <button
              type="button"
              className={`${styles.item} ${styles.itemYou}`}
              aria-label="Profile and settings"
              onClick={() => setIsSettingsOpen(true)}
            >
              <Avatar
                src={user?.avatar_url}
                username={user?.username || "U"}
                size="md"
                status="online"
              />
            </button>
          </Tooltip>
        </div>
      </nav>

      <CreateRoomModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
      />
      <SettingsShell
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </>
  );
};
