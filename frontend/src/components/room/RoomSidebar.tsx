import React from "react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import type { Room } from "../../types/room";
import { useRTC } from "../../hooks/useRTC";
import { useAuth } from "../../hooks/useAuth";
import { useInviteLink } from "../../hooks/useInviteLink";
import { Avatar } from "../common/Avatar";
import { VoiceStatusPanel } from "./VoiceStatusPanel";
import { UserTray } from "./UserTray";
import {
  Hash,
  Volume2,
  Film,
  Lock,
  Globe,
  ChevronDown,
  MicOff,
  ScreenShare,
  Link2,
  LogOut,
  type LucideIcon,
} from "lucide-react";
import menuStyles from "../common/Menu.module.css";
import styles from "./RoomSidebar.module.css";

export type RoomChannel = "general" | "voice" | "watch-party";

interface RoomSidebarProps {
  activeRoom: Room | null;
  activeChannel: RoomChannel;
  onSelectChannel: (channel: RoomChannel) => void;
  /** Shown in the room menu; omit to hide the entry */
  onLeave?: () => void;
  isMediaPlaying?: boolean;
}

export const RoomSidebar: React.FC<RoomSidebarProps> = ({
  activeRoom,
  activeChannel,
  onSelectChannel,
  onLeave,
  isMediaPlaying,
}) => {
  const { user } = useAuth();
  const { peers, speakingIds } = useRTC(activeRoom?.id);
  const { copy: copyInvite } = useInviteLink(activeRoom?.id);

  // The server's roster, so the channel lists everyone in the call -- including
  // people this client has no media for yet, and everyone at all when you have
  // not joined it yourself.
  const voiceCount = peers.length;

  const channelClass = (channel: RoomChannel) =>
    `${styles.channel} ${activeChannel === channel ? styles.channelActive : ""}`;

  const channelButton = (
    channel: RoomChannel,
    Icon: LucideIcon,
    trailing?: React.ReactNode,
  ) => (
    <button
      type="button"
      className={channelClass(channel)}
      onClick={() => onSelectChannel(channel)}
      aria-current={activeChannel === channel ? "page" : undefined}
    >
      <Icon size={20} className={styles.channelIcon} aria-hidden="true" />
      <span className={styles.channelName}>{channel}</span>
      {trailing}
    </button>
  );

  return (
    <aside className={styles.sidebar} aria-label="Room channels">
      {/* Room identity — opens the room menu, like a server header */}
      <DropdownMenu.Root>
        <DropdownMenu.Trigger asChild>
          <button
            type="button"
            className={styles.roomHeader}
            title={activeRoom?.name}
            aria-label={`${activeRoom?.name ?? "Room"} menu`}
          >
            <span className={styles.roomName}>
              {activeRoom?.name ?? "Room"}
            </span>
            <span className={styles.privacy}>
              {activeRoom?.is_private ? (
                <Lock size={13} aria-label="Private room" />
              ) : (
                <Globe size={13} aria-label="Public room" />
              )}
            </span>
            <span className={styles.chevron} aria-hidden="true">
              <ChevronDown size={16} />
            </span>
          </button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            className={`${menuStyles.content} ${styles.roomMenu}`}
            sideOffset={6}
            align="start"
          >
            <DropdownMenu.Item
              className={menuStyles.item}
              onSelect={copyInvite}
            >
              <span className={menuStyles.itemLabel}>Copy invite link</span>
              <span className={menuStyles.itemIcon}>
                <Link2 size={15} />
              </span>
            </DropdownMenu.Item>
            {onLeave && (
              <>
                <DropdownMenu.Separator className={menuStyles.separator} />
                <DropdownMenu.Item
                  className={`${menuStyles.item} ${menuStyles.danger}`}
                  onSelect={onLeave}
                >
                  <span className={menuStyles.itemLabel}>Leave room</span>
                  <span className={menuStyles.itemIcon}>
                    <LogOut size={15} />
                  </span>
                </DropdownMenu.Item>
              </>
            )}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>

      {/* Channels */}
      <nav className={styles.nav} aria-label="Channels">
        <div className={styles.group}>
          <div className={styles.groupLabel}>Text</div>
          {channelButton("general", Hash)}
        </div>

        <div className={styles.group}>
          <div className={styles.groupLabel}>Voice</div>
          {channelButton(
            "voice",
            Volume2,
            voiceCount > 0 && (
              <span
                className={styles.count}
                aria-label={`${voiceCount} in voice`}
              >
                {voiceCount}
              </span>
            ),
          )}

          {voiceCount > 0 && (
            <div className={styles.participants}>
              {peers.map((peer) => {
                const isSpeaking = speakingIds.has(peer.userId);
                return (
                  <div key={peer.userId} className={styles.participant}>
                    <span
                      className={`${styles.participantAvatar} ${
                        isSpeaking ? styles.speaking : ""
                      }`}
                    >
                      <Avatar
                        username={peer.username}
                        src={peer.isLocal ? user?.avatar_url : undefined}
                        size="xs"
                      />
                    </span>
                    <span
                      className={[
                        styles.participantName,
                        peer.isLocal ? styles.participantSelf : "",
                        isSpeaking ? styles.speakingName : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                    >
                      {peer.username}
                    </span>
                    {peer.isMuted && (
                      <span className={`${styles.stateIcon} ${styles.muted}`}>
                        <MicOff size={14} aria-label="Muted" />
                      </span>
                    )}
                    {peer.isScreenSharing && (
                      <span className={`${styles.stateIcon} ${styles.sharing}`}>
                        <ScreenShare size={14} aria-label="Sharing screen" />
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <div className={styles.group}>
          <div className={styles.groupLabel}>Watch Party</div>
          {channelButton(
            "watch-party",
            Film,
            isMediaPlaying && (
              <span className={styles.liveBadge}>
                <span className={styles.liveDot} aria-hidden="true" />
                LIVE
              </span>
            ),
          )}
        </div>
      </nav>

      {/* Voice state, then identity — always in this order, always at the foot */}
      <VoiceStatusPanel roomId={activeRoom?.id} roomName={activeRoom?.name} />
      <UserTray roomId={activeRoom?.id} />
    </aside>
  );
};
