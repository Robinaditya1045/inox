import React from "react";
import { useRTC } from "../../hooks/useRTC";
import { usePermissions } from "../../hooks/usePermissions";
import { IconButton } from "../common/IconButton";
import { Spinner } from "../common/Spinner";
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
        <div
          key="connecting"
          className={`${styles.stateView} ${styles.pending}`}
          role="status"
          aria-live="polite"
        >
          <Spinner size={12} color="currentColor" label={null} />
          <span>Connecting to voice…</span>
        </div>
      </div>
    );
  }

  if (connectionState === "disconnected") {
    return (
      <div className={styles.panel}>
        <button
          key="disconnected"
          type="button"
          className={`${styles.stateView} ${styles.wideBtn} ${styles.wideBtnAccent}`}
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
      <div key={connectionState} className={styles.stateView}>
        <div className={styles.statusRow}>
          <span
            className={`${styles.signal} ${hasFailed ? styles.signalDanger : ""}`}
            aria-hidden="true"
          >
            {hasFailed ? <AlertTriangle size={18} /> : <Signal size={18} />}
          </span>

          <div className={styles.statusText} role="status">
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
            <IconButton
              label={isScreenSharing ? "Stop screen sharing" : "Share screen"}
              isActive={isScreenSharing}
              onClick={toggleScreenShare}
              aria-pressed={isScreenSharing}
            >
              {isScreenSharing ? (
                <ScreenShareOff size={17} />
              ) : (
                <ScreenShare size={17} />
              )}
            </IconButton>
          )}

          <IconButton
            label="Disconnect from voice"
            tooltip="Disconnect"
            variant="danger"
            onClick={disconnectAudio}
          >
            <PhoneOff size={17} />
          </IconButton>
        </div>

        {hasFailed && (
          <button
            type="button"
            className={`${styles.wideBtn} ${styles.wideBtnAccent}`}
            onClick={connectAudio}
          >
            <Radio size={13} aria-hidden="true" />
            Reconnect
          </button>
        )}

        {isScreenSharing && !hasFailed && (
          <button
            type="button"
            className={`${styles.wideBtn} ${styles.wideBtnDanger}`}
            onClick={toggleScreenShare}
            aria-label="Stop sharing screen"
          >
            <Square size={13} aria-hidden="true" />
            Stop Sharing Screen
          </button>
        )}
      </div>
    </div>
  );
};
