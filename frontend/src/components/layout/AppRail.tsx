import React from "react";
import { NavLink, useLocation } from "react-router-dom";
import { useRoom } from "../../hooks/useRoom";
import { Compass, Plus } from "lucide-react";
import styles from "./AppRail.module.css";

/** Two-letter monogram for a room, e.g. "Friday Night Sci-Fi" -> "FN". */
function roomInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length === 0) return "??";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return (words[0][0] + words[1][0]).toUpperCase();
}

export const AppRail: React.FC = () => {
  const { rooms, invitations } = useRoom();
  const location = useLocation();

  const isHome = location.pathname === "/";
  const hasPendingInvites = invitations.length > 0;

  return (
    <nav className={styles.rail} aria-label="Main navigation">
      {/* Lobby */}
      <div className={styles.slot}>
        <span
          className={`${styles.pill} ${isHome ? styles.pillActive : ""}`}
          aria-hidden="true"
        />
        <NavLink
          to="/"
          end
          className={`${styles.item} ${styles.itemHome} ${isHome ? styles.itemHomeActive : ""}`}
          title="Lobby"
          aria-label="Lobby"
        >
          <Compass size={20} />
          {hasPendingInvites && (
            <span
              className={styles.badge}
              aria-label={`${invitations.length} pending invitations`}
            >
              {invitations.length}
            </span>
          )}
        </NavLink>
      </div>

      <div className={styles.divider} aria-hidden="true" />

      {/* One monogram per room */}
      {rooms.map((room) => {
        const isActive = location.pathname === `/room/${room.id}`;
        return (
          <div className={styles.slot} key={room.id}>
            <span
              className={`${styles.pill} ${isActive ? styles.pillActive : ""}`}
              aria-hidden="true"
            />
            <NavLink
              to={`/room/${room.id}`}
              className={`${styles.item} ${isActive ? styles.itemActive : ""}`}
              title={room.name}
              aria-label={room.name}
            >
              {roomInitials(room.name)}
            </NavLink>
          </div>
        );
      })}

      {/* Create a room — lives in the rail so it is reachable from inside a room too */}
      <div className={styles.slot}>
        <span className={styles.pill} aria-hidden="true" />
        <NavLink
          to="/?create=1"
          className={`${styles.item} ${styles.itemGhost}`}
          title="Create a room"
          aria-label="Create a room"
        >
          <Plus size={20} />
        </NavLink>
      </div>

      <div className={styles.spacer} />
    </nav>
  );
};
