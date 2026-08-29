import React, { useState } from "react";
import type { Room } from "../../types/room";
import type { RoomChannel } from "./RoomSidebar";
import { Hash, Volume2, Film, Users, Copy, Check, LogOut } from "lucide-react";
import styles from "./RoomHeader.module.css";

interface RoomHeaderProps {
  activeRoom: Room | null;
  activeChannel: RoomChannel;
  showMembers: boolean;
  onToggleMembers: () => void;
  onLeave: () => void;
  mediaUrl?: string;
  memberCount?: number;
}

const CHANNEL_ICON: Record<
  RoomChannel,
  React.ComponentType<{ size?: number; style?: React.CSSProperties }>
> = {
  general: Hash,
  voice: Volume2,
  "watch-party": Film,
};

export const RoomHeader: React.FC<RoomHeaderProps> = ({
  activeRoom,
  activeChannel,
  showMembers,
  onToggleMembers,
  onLeave,
  mediaUrl,
  memberCount,
}) => {
  const [copiedLink, setCopiedLink] = useState(false);

  const handleCopyInvite = () => {
    if (!activeRoom) return;
    const url = `${window.location.origin}/room/${activeRoom.id}`;
    navigator.clipboard
      .writeText(url)
      .then(() => {
        setCopiedLink(true);
        setTimeout(() => setCopiedLink(false), 2500);
      })
      .catch(() => {});
  };

  const Icon = CHANNEL_ICON[activeChannel];

  let topic = "";
  if (activeChannel === "general") {
    topic = memberCount
      ? `${memberCount} member${memberCount === 1 ? "" : "s"}`
      : "";
  } else if (activeChannel === "voice") {
    topic = "Voice & screen share";
  } else if (mediaUrl) {
    topic = decodeURIComponent(mediaUrl.split("/").pop() || "");
  }

  return (
    <header className={styles.header}>
      <div className={styles.context}>
        <Icon
          size={20}
          style={{ color: "var(--color-text-muted)", flexShrink: 0 }}
        />
        <span className={styles.channelName}>{activeChannel}</span>
        {topic && (
          <>
            <span className={styles.rule} aria-hidden="true" />
            <span className={styles.topic}>{topic}</span>
          </>
        )}
      </div>

      <div className={styles.spacer} />

      <button
        className={`${styles.iconBtn} ${showMembers ? styles.iconBtnActive : ""}`}
        onClick={onToggleMembers}
        aria-label={showMembers ? "Hide member list" : "Show member list"}
        aria-pressed={showMembers}
        title="Members"
      >
        <Users size={18} />
      </button>

      <button
        className={`${styles.action} ${copiedLink ? styles.actionCopied : ""}`}
        onClick={handleCopyInvite}
        aria-label="Copy room invite link"
        title="Copy invite link"
      >
        {copiedLink ? <Check size={14} /> : <Copy size={14} />}
        <span>{copiedLink ? "Copied" : "Invite"}</span>
      </button>

      <button
        className={styles.leave}
        onClick={onLeave}
        aria-label="Leave room"
        title="Leave room"
      >
        <LogOut size={14} />
        <span>Leave</span>
      </button>
    </header>
  );
};
