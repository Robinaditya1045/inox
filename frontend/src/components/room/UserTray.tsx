import React, { useState } from "react";
import { useAuth } from "../../hooks/useAuth";
import { useRTC } from "../../hooks/useRTC";
import { Avatar } from "../common/Avatar";
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
            <span className={styles.status}>{status}</span>
          </span>
        </button>

        <button
          className={`${styles.btn} ${muted ? styles.btnActive : ""}`}
          onClick={toggleMute}
          disabled={!inVoice}
          aria-label={muted ? "Unmute microphone" : "Mute microphone"}
          aria-pressed={muted}
          title={
            inVoice
              ? muted
                ? "Unmute"
                : "Mute"
              : "Join voice to use your microphone"
          }
        >
          {muted ? <MicOff size={17} /> : <Mic size={17} />}
        </button>

        <button
          className={`${styles.btn} ${deafened ? styles.btnActive : ""}`}
          onClick={toggleDeafen}
          disabled={!inVoice}
          aria-label={deafened ? "Undeafen" : "Deafen"}
          aria-pressed={deafened}
          title={
            inVoice
              ? deafened
                ? "Undeafen"
                : "Deafen"
              : "Join voice to deafen"
          }
        >
          {deafened ? <HeadphoneOff size={17} /> : <Headphones size={17} />}
        </button>

        <button
          className={styles.btn}
          onClick={() => setIsSettingsOpen(true)}
          aria-label="User settings"
          title="User settings"
        >
          <Settings size={17} />
        </button>
      </div>

      <SettingsShell
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </>
  );
};
