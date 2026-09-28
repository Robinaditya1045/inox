import React, { type CSSProperties, type ReactNode } from "react";
import { Avatar } from "../common/Avatar";
import styles from "../../pages/FriendsPage.module.css";

interface FriendRowProps {
  username: string;
  avatarUrl?: string;
  /** Secondary line: "Friends since …", "Sent 2 days ago", etc. */
  meta?: string;
  /** Trailing controls — the only thing that differs between the four lists. */
  children?: ReactNode;
  style?: CSSProperties;
}

export const FriendRow: React.FC<FriendRowProps> = ({
  username,
  avatarUrl,
  meta,
  children,
  style,
}) => {
  return (
    <div className={styles.row} style={style}>
      <Avatar src={avatarUrl} username={username} size="md" />
      <div className={styles.identity}>
        <span className={styles.username}>{username}</span>
        {meta && <span className={styles.meta}>{meta}</span>}
      </div>
      {children && <div className={styles.actions}>{children}</div>}
    </div>
  );
};
