import React from "react";
import type { Room } from "../../types/room";
import type { RoomChannel } from "./RoomSidebar";
import { useInviteLink } from "../../hooks/useInviteLink";
import { IconButton } from "../common/IconButton";
import { Tooltip } from "../common/Tooltip";
import {
  Hash,
  Volume2,
  Film,
  Users,
  UserPlus,
  Check,
  LogOut,
  Menu,
  MessageSquare,
  type LucideIcon,
} from "lucide-react";
import styles from "./RoomHeader.module.css";

interface RoomHeaderProps {
  activeRoom: Room | null;
  activeChannel: RoomChannel;
  /** Which side-panel tab is on screen, or null when the panel is closed */
  openPanel: SidePanelTab | null;
  /** Opens the side panel on a tab, or closes it if that tab is already showing */
  onTogglePanel: (tab: SidePanelTab) => void;
  /** Hidden while the main view is already #general */
  showChatToggle?: boolean;
  /** Messages that arrived while the chat was out of sight */
  chatUnread?: number;
  onLeave: () => void;
  /** Opens the channel drawer on phones */
  onOpenChannels?: () => void;
  mediaUrl?: string;
  memberCount?: number;
}

export type SidePanelTab = "members" | "chat";

/** File name from a media URL. A malformed escape (e.g. "50%off.mp4") must not
 *  throw during render and blank the room for everyone playing it. */
function fileNameOf(url: string): string {
  const last = url.split(/[?#]/)[0].split("/").pop() || "";
  try {
    return decodeURIComponent(last);
  } catch {
    return last;
  }
}

const CHANNEL_ICON: Record<RoomChannel, LucideIcon> = {
  general: Hash,
  voice: Volume2,
  "watch-party": Film,
};

export const RoomHeader: React.FC<RoomHeaderProps> = ({
  activeRoom,
  activeChannel,
  openPanel,
  onTogglePanel,
  showChatToggle = true,
  chatUnread = 0,
  onLeave,
  onOpenChannels,
  mediaUrl,
  memberCount,
}) => {
  const { copied: copiedLink, copy: handleCopyInvite } = useInviteLink(
    activeRoom?.id,
  );

  const Icon = CHANNEL_ICON[activeChannel];

  let topic = "";
  if (activeChannel === "general") {
    topic = memberCount
      ? `${memberCount} member${memberCount === 1 ? "" : "s"}`
      : "";
  } else if (activeChannel === "voice") {
    topic = "Voice & screen share";
  } else if (mediaUrl) {
    topic = fileNameOf(mediaUrl);
  }

  return (
    <header className={styles.header}>
      {onOpenChannels && (
        <IconButton
          label="Open channels"
          className={styles.menuBtn}
          onClick={onOpenChannels}
          tooltip={false}
        >
          <Menu size={20} />
        </IconButton>
      )}

      <div className={styles.context}>
        <span className={styles.channelIcon} aria-hidden="true">
          <Icon size={22} />
        </span>
        <h1 key={activeChannel} className={styles.channelName}>
          {activeChannel}
        </h1>
        {topic && (
          <>
            <span className={styles.rule} aria-hidden="true" />
            <span key={topic} className={styles.topic} title={topic}>
              {topic}
            </span>
          </>
        )}
      </div>

      <div className={styles.actions}>
        {showChatToggle && (
          <IconButton
            label={
              openPanel === "chat"
                ? "Hide chat"
                : chatUnread > 0
                  ? `Show chat, ${chatUnread} unread`
                  : "Show chat"
            }
            tooltip={openPanel === "chat" ? "Hide chat" : "Chat"}
            isActive={openPanel === "chat"}
            onClick={() => onTogglePanel("chat")}
            aria-pressed={openPanel === "chat"}
            tooltipSide="bottom"
            className={styles.badgeHost}
          >
            <MessageSquare size={19} />
            {chatUnread > 0 && openPanel !== "chat" && (
              <span
                key={chatUnread}
                className={styles.unread}
                aria-hidden="true"
              >
                {chatUnread > 99 ? "99+" : chatUnread}
              </span>
            )}
          </IconButton>
        )}

        <IconButton
          label={
            openPanel === "members" ? "Hide member list" : "Show member list"
          }
          tooltip={openPanel === "members" ? "Hide members" : "Members"}
          isActive={openPanel === "members"}
          onClick={() => onTogglePanel("members")}
          aria-pressed={openPanel === "members"}
          tooltipSide="bottom"
        >
          <Users size={19} />
        </IconButton>

        <span className={styles.divider} aria-hidden="true" />

        <Tooltip content="Copy a link to this room" side="bottom">
          <button
            type="button"
            className={`${styles.action} ${styles.invite} ${copiedLink ? styles.inviteCopied : ""}`}
            onClick={handleCopyInvite}
            aria-label="Copy room invite link"
          >
            <span key={String(copiedLink)} className={styles.actionIcon}>
              {copiedLink ? <Check size={15} /> : <UserPlus size={15} />}
            </span>
            <span className={styles.actionLabel}>
              {copiedLink ? "Copied" : "Invite"}
            </span>
          </button>
        </Tooltip>

        <Tooltip content="Leave room" side="bottom">
          <button
            type="button"
            className={`${styles.action} ${styles.leave}`}
            onClick={onLeave}
            aria-label="Leave room"
          >
            <LogOut size={15} aria-hidden="true" />
            <span className={styles.actionLabel}>Leave</span>
          </button>
        </Tooltip>
      </div>
    </header>
  );
};
