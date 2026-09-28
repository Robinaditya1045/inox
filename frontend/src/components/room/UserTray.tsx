import React, { useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import { useRTC } from "../../hooks/useRTC";
import { Avatar } from "../common/Avatar";
import { IconButton } from "../common/IconButton";
import { SettingsShell } from "../profile/SettingsShell";
import { Mic, MicOff, Headphones, HeadphoneOff, Settings } from "lucide-react";
import styles from "./UserTray.module.css";

interface UserTrayProps {
  /** Omit outside a room — mic/deafen then render disabled rather than disappearing,
   *  so the tray never changes shape between the lobby and a room. */
  roomId?: string;
}

export const UserTray: React.FC<UserTrayProps> = ({ roomId }) => {
  const { user } = useAuth();
  const {
    connectionState,
    isAudioMuted,
    isDeafened,
    toggleMute,
    toggleDeafen,
  } = useRTC(roomId);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);

  const inVoice = connectionState === "connected";
  const muted = inVoice && isAudioMuted;
  const deafened = inVoice && isDeafened;

  let status = "Online";
  if (deafened) status = "Deafened";
  else if (muted) status = "Muted";
  else if (inVoice) status = "In voice";

  return (
    <>
      <div className={styles.tray}>
        <button
          type="button"
          className={styles.identity}
          onClick={() => setIsSettingsOpen(true)}
          aria-label="Profile and settings"
          title={user?.username}
        >
          <Avatar
            src={user?.avatar_url}
            username={user?.username || "U"}
            size="sm"
            status="online"
          />
          <span className={styles.names}>
            <span className={styles.username}>{user?.username || "You"}</span>
            <span
              className={`${styles.status} ${inVoice && !muted && !deafened ? styles.statusLive : ""}`}
            >
              <span key={status} className={styles.statusText}>
                {status}
              </span>
            </span>
          </span>
        </button>

        <IconButton
          label={muted ? "Unmute microphone" : "Mute microphone"}
          tooltip={
            inVoice
              ? muted
                ? "Unmute"
                : "Mute"
              : "Join voice to use your microphone"
          }
          isActive={muted}
          activeTone="danger"
          onClick={toggleMute}
          disabled={!inVoice}
          aria-pressed={muted}
        >
          {muted ? <MicOff size={18} /> : <Mic size={18} />}
        </IconButton>

        <IconButton
          label={deafened ? "Undeafen" : "Deafen"}
          tooltip={
            inVoice
              ? deafened
                ? "Undeafen"
                : "Deafen"
              : "Join voice to deafen"
          }
          isActive={deafened}
          activeTone="danger"
          onClick={toggleDeafen}
          disabled={!inVoice}
          aria-pressed={deafened}
        >
          {deafened ? <HeadphoneOff size={18} /> : <Headphones size={18} />}
        </IconButton>

        <IconButton
          label="User settings"
          onClick={() => setIsSettingsOpen(true)}
        >
          <Settings size={18} />
        </IconButton>
      </div>

      <SettingsShell
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </>
  );
};
