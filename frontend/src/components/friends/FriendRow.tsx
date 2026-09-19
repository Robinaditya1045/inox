import React, { type ReactNode } from "react";
import { Avatar } from "../common/Avatar";
import styles from "../../pages/FriendsPage.module.css";

interface FriendRowProps {
  username: string;
  avatarUrl?: string;
  /** Secondary line: "Friends since …", "Sent 2 days ago", etc. */
  meta?: string;
  /** Trailing controls — the only thing that differs between the four lists. */
  children?: ReactNode;
}

export const FriendRow: React.FC<FriendRowProps> = ({
  username,
  avatarUrl,
  meta,
  children,
}) => {
  return (
    <div className={styles.row}>
      <Avatar src={avatarUrl} username={username} size="sm" />
      <div className={styles.identity}>
        <span className={styles.username}>{username}</span>
        {meta && <span className={styles.meta}>{meta}</span>}
      </div>
      {children && <div className={styles.actions}>{children}</div>}
    </div>
  );
};
