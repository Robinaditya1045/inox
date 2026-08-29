import React from "react";
import type { Room } from "../../types/room";
import { usePresence } from "../../hooks/usePresence";
import { useRTC } from "../../hooks/useRTC";
import { useAuth } from "../../hooks/useAuth";
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
} from "lucide-react";
import styles from "./RoomSidebar.module.css";

export type RoomChannel = "general" | "voice" | "watch-party";

interface RoomSidebarProps {
  activeRoom: Room | null;
  activeChannel: RoomChannel;
  onSelectChannel: (channel: RoomChannel) => void;
  onOpenRoomMenu?: () => void;
  isMediaPlaying?: boolean;
}

export const RoomSidebar: React.FC<RoomSidebarProps> = ({
  activeRoom,
  activeChannel,
  onSelectChannel,
  onOpenRoomMenu,
  isMediaPlaying,
}) => {
  const { members } = usePresence();
  const { user } = useAuth();
  const { connectionState, remoteStreams, isAudioMuted, isScreenSharing } =
    useRTC(activeRoom?.id);

  const inVoice = connectionState === "connected";

  // Who the SFU is actually forwarding, resolved to names via the presence list
  const remotePeers = Array.from(remoteStreams.keys()).map((userId) => ({
    userId,
    username: members.find((m) => m.user_id === userId)?.username || "Peer",
  }));

  const voiceCount = (inVoice ? 1 : 0) + remotePeers.length;

  const channelClass = (channel: RoomChannel) =>
    `${styles.channel} ${activeChannel === channel ? styles.channelActive : ""}`;

  return (
    <aside className={styles.sidebar} aria-label="Room channels">
      {/* Room identity */}
      <button
        className={styles.roomHeader}
        onClick={onOpenRoomMenu}
        title={activeRoom?.name}
      >
        <span className={styles.roomName}>{activeRoom?.name ?? "Room"}</span>
        {activeRoom?.is_private ? (
          <Lock
            size={13}
            style={{ color: "var(--color-text-muted)", flexShrink: 0 }}
            aria-label="Private room"
          />
        ) : (
          <Globe
            size={13}
            style={{ color: "var(--color-text-muted)", flexShrink: 0 }}
            aria-label="Public room"
          />
        )}
        <ChevronDown
          size={15}
          style={{ color: "var(--color-text-secondary)", flexShrink: 0 }}
          aria-hidden="true"
        />
      </button>

      {/* Channels */}
      <nav className={styles.nav}>
        <div className={styles.group}>
          <div className={styles.groupLabel}>
            <span className={styles.groupLabelText}>Text</span>
          </div>
          <button
            className={channelClass("general")}
            onClick={() => onSelectChannel("general")}
            aria-current={activeChannel === "general"}
          >
            <Hash size={18} className={styles.channelIcon} aria-hidden="true" />
            <span className={styles.channelName}>general</span>
          </button>
        </div>

        <div className={styles.group}>
          <div className={styles.groupLabel}>
            <span className={styles.groupLabelText}>Voice</span>
          </div>
          <button
            className={channelClass("voice")}
            onClick={() => onSelectChannel("voice")}
            aria-current={activeChannel === "voice"}
          >
            <Volume2
              size={18}
              className={styles.channelIcon}
              aria-hidden="true"
            />
            <span className={styles.channelName}>voice</span>
            {voiceCount > 0 && (
              <span className={styles.count}>{voiceCount}</span>
            )}
          </button>

          {voiceCount > 0 && (
            <div className={styles.participants}>
              {inVoice && (
                <div className={styles.participant}>
                  <span className={styles.participantAvatar}>
                    <Avatar
                      username={user?.username || "You"}
                      src={user?.avatar_url}
                      size="xs"
                    />
                  </span>
                  <span
                    className={`${styles.participantName} ${styles.participantSelf}`}
                  >
                    {user?.username || "You"}
                  </span>
                  {isAudioMuted && (
                    <MicOff
                      size={13}
                      style={{ color: "var(--color-danger)" }}
                      aria-label="Muted"
                    />
                  )}
                  {isScreenSharing && (
                    <ScreenShare
                      size={13}
                      style={{ color: "var(--color-accent)" }}
                      aria-label="Sharing screen"
                    />
                  )}
                </div>
              )}
              {remotePeers.map((peer) => (
                <div key={peer.userId} className={styles.participant}>
                  <span className={styles.participantAvatar}>
                    <Avatar username={peer.username} size="xs" />
                  </span>
                  <span className={styles.participantName}>
                    {peer.username}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className={styles.group}>
          <div className={styles.groupLabel}>
            <span className={styles.groupLabelText}>Watch Party</span>
          </div>
          <button
            className={channelClass("watch-party")}
            onClick={() => onSelectChannel("watch-party")}
            aria-current={activeChannel === "watch-party"}
          >
            <Film size={18} className={styles.channelIcon} aria-hidden="true" />
            <span className={styles.channelName}>watch-party</span>
            {isMediaPlaying && (
              <span className={styles.liveBadge}>
                <span className={styles.liveDot} aria-hidden="true" />
                LIVE
              </span>
            )}
          </button>
        </div>
      </nav>

      {/* Voice state, then identity — always in this order, always at the foot */}
      <VoiceStatusPanel roomId={activeRoom?.id} roomName={activeRoom?.name} />
      <UserTray roomId={activeRoom?.id} />
    </aside>
  );
};
