import React from "react";
import { useRTC } from "../../hooks/useRTC";
import { usePermissions } from "../../hooks/usePermissions";
import {
  Signal,
  AlertTriangle,
  Radio,
  Lock,
  ScreenShare,
  ScreenShareOff,
  PhoneOff,
  Square,
} from "lucide-react";
import styles from "./VoiceStatusPanel.module.css";

interface VoiceStatusPanelProps {
  roomId: string | undefined;
  roomName?: string;
  channelName?: string;
}

/**
 * Voice connection state, pinned to the foot of the channel sidebar above the user tray.
 * Deliberately NOT in a bar under the player: leaving a call must always be the same pixel
 * whether the user is looking at chat, the member list or the watch party.
 */
export const VoiceStatusPanel: React.FC<VoiceStatusPanelProps> = ({
  roomId,
  roomName,
  channelName = "voice",
}) => {
  const {
    connectionState,
    isScreenSharing,
    connectAudio,
    disconnectAudio,
    toggleScreenShare,
  } = useRTC(roomId);

  const permissions = usePermissions();

  if (!permissions.can_stream_audio) {
    return (
      <div className={styles.panel}>
        <div className={styles.restricted}>
          <Lock size={14} aria-hidden="true" />
          <span>Voice restricted</span>
        </div>
      </div>
    );
  }

  if (connectionState === "connecting") {
    return (
      <div className={styles.panel}>
        <div className={styles.pending} role="status" aria-live="polite">
          <span className={styles.spinner} aria-hidden="true" />
          <span>Negotiating…</span>
        </div>
      </div>
    );
  }

  if (connectionState === "disconnected") {
    return (
      <div className={styles.panel}>
        <button
          className={`${styles.wideBtn} ${styles.wideBtnAccent}`}
          onClick={connectAudio}
          aria-label="Join voice channel"
        >
          <Radio size={15} aria-hidden="true" />
          Join Voice
        </button>
      </div>
    );
  }

  const hasFailed = connectionState === "failed";

  return (
    <div className={styles.panel}>
      <div className={styles.statusRow}>
        {hasFailed ? (
          <AlertTriangle
            size={18}
            aria-hidden="true"
            style={{ color: "var(--color-danger)", flexShrink: 0 }}
          />
        ) : (
          <Signal
            size={18}
            aria-hidden="true"
            style={{ color: "var(--color-live)", flexShrink: 0 }}
          />
        )}

        <div className={styles.statusText}>
          <span
            className={`${styles.statusTitle} ${hasFailed ? styles.statusTitleDanger : ""}`}
          >
            {hasFailed ? "Voice Disconnected" : "Voice Connected"}
          </span>
          <span className={styles.statusSub}>
            {hasFailed
              ? "Connection failed"
              : `${channelName}${roomName ? ` / ${roomName}` : ""}`}
          </span>
        </div>

        {permissions.can_share_screen && !hasFailed && (
          <button
            className={`${styles.iconBtn} ${isScreenSharing ? styles.iconBtnAccent : ""}`}
            onClick={toggleScreenShare}
            aria-label={
              isScreenSharing ? "Stop screen sharing" : "Share screen"
            }
            aria-pressed={isScreenSharing}
            title={isScreenSharing ? "Stop screen sharing" : "Share screen"}
          >
            {isScreenSharing ? (
              <ScreenShareOff size={16} />
            ) : (
              <ScreenShare size={16} />
            )}
          </button>
        )}

        <button
          className={`${styles.iconBtn} ${styles.iconBtnDanger}`}
          onClick={disconnectAudio}
          aria-label="Disconnect from voice"
          title="Disconnect"
        >
          <PhoneOff size={16} />
        </button>
      </div>

      {hasFailed && (
        <button
          className={`${styles.wideBtn} ${styles.wideBtnAccent}`}
          onClick={connectAudio}
        >
          <Radio size={13} aria-hidden="true" />
          Reconnect
        </button>
      )}

      {isScreenSharing && !hasFailed && (
        <button
          className={`${styles.wideBtn} ${styles.wideBtnDanger}`}
          onClick={toggleScreenShare}
          aria-label="Stop sharing screen"
        >
          <Square size={13} aria-hidden="true" />
          Stop Sharing Screen
        </button>
      )}
    </div>
  );
};
