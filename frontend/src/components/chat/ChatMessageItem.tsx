import React, { useState } from "react";
import type { ChatMessage } from "../../types/chat";
import type { RoomRole } from "../../types/room";
import { Avatar } from "../common/Avatar";
import { Copy, Check, Crown, Shield } from "lucide-react";
import styles from "./ChatMessageItem.module.css";

interface MessageItemProps {
  message: ChatMessage;
  isOwn: boolean;
  /** If true, show avatar + username + timestamp. If false (grouped), body only. */
  isGroupLeader: boolean;
  /** Author's role in this room, resolved from the presence list. */
  authorRole?: RoomRole;
  /** Current user's name, used to highlight @mentions of them. */
  currentUsername?: string;
}

function formatTime(isoString: string): string {
  try {
    return new Date(isoString).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return "";
  }
}

/** Splits body text so @mentions can be highlighted without dangerouslySetInnerHTML. */
function renderBody(text: string, currentUsername?: string): React.ReactNode {
  const parts = text.split(/(@[A-Za-z0-9_.-]+)/g);
  return parts.map((part, i) => {
    if (!part.startsWith("@")) return part;
    const isMe =
      !!currentUsername &&
      part.slice(1).toLowerCase() === currentUsername.toLowerCase();
    return (
      <span key={i} className={styles.mention} data-self={isMe || undefined}>
        {part}
      </span>
    );
  });
}

export const MessageItem: React.FC<MessageItemProps> = React.memo(
  ({ message, isOwn, isGroupLeader, authorRole, currentUsername }) => {
    const [copied, setCopied] = useState(false);

    const mentionsMe =
      !!currentUsername &&
      new RegExp(`@${currentUsername}\\b`, "i").test(message.message);

    const handleCopy = () => {
      navigator.clipboard
        .writeText(message.message)
        .then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        })
        .catch(() => {});
    };

    return (
      <div
        className={[
          styles.row,
          isGroupLeader ? styles.rowLeader : "",
          mentionsMe ? styles.rowMention : "",
        ]
          .filter(Boolean)
          .join(" ")}
      >
        <div className={styles.gutter}>
          {isGroupLeader ? (
            <Avatar username={message.username || "U"} size="sm" />
          ) : (
            <span className={styles.hoverTime}>
              {formatTime(message.created_at)}
            </span>
          )}
        </div>

        <div className={styles.body}>
          {isGroupLeader && (
            <div className={styles.meta}>
              <span
                className={`${styles.username} ${isOwn ? styles.usernameOwn : ""}`}
              >
                {message.username}
              </span>

              {authorRole === "owner" && (
                <span className={`${styles.badge} ${styles.badgeOwner}`}>
                  <Crown size={9} aria-hidden="true" />
                  OWNER
                </span>
              )}
              {authorRole === "moderator" && (
                <span className={`${styles.badge} ${styles.badgeMod}`}>
                  <Shield size={9} aria-hidden="true" />
                  MOD
                </span>
              )}

              <span className={styles.time}>
                {formatTime(message.created_at)}
              </span>
            </div>
          )}

          <p className={styles.text}>
            {renderBody(message.message, currentUsername)}
          </p>
        </div>

        <div className={styles.actions}>
          <button
            className={styles.actionBtn}
            onClick={handleCopy}
            aria-label="Copy message text"
            title={copied ? "Copied" : "Copy text"}
          >
            {copied ? <Check size={15} /> : <Copy size={15} />}
          </button>
        </div>
      </div>
    );
  },
);

MessageItem.displayName = "MessageItem";

// Legacy export alias for backwards compatibility with ChatPanel
export { MessageItem as ChatMessageItem };
