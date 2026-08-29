import React, { useEffect, useRef } from "react";
import { useRTC } from "../../hooks/useRTC";
import { useAuth } from "../../hooks/useAuth";
import { usePresence } from "../../hooks/usePresence";
import { Avatar } from "../common/Avatar";
import { MicOff, Radio, ScreenShare } from "lucide-react";
import styles from "./VoiceStage.module.css";

interface VoiceStageProps {
  roomId: string | undefined;
}

/** Local screen-share preview. Muted so the sharer never hears themselves. */
const ScreenPreview: React.FC<{ stream: MediaStream }> = ({ stream }) => {
  const ref = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    if (ref.current) {
      ref.current.srcObject = stream;
    }
  }, [stream]);

  return (
    <video
      ref={ref}
      autoPlay
      playsInline
      muted
      className={styles.screenVideo}
    />
  );
};

export const VoiceStage: React.FC<VoiceStageProps> = ({ roomId }) => {
  const {
    connectionState,
    remoteStreams,
    isAudioMuted,
    isScreenSharing,
    localScreenStream,
  } = useRTC(roomId);
  const { user } = useAuth();
  const { members } = usePresence();

  const inVoice = connectionState === "connected";
  const peers = Array.from(remoteStreams.keys()).map((userId) => ({
    userId,
    username: members.find((m) => m.user_id === userId)?.username || "Peer",
  }));

  if (!inVoice && peers.length === 0) {
    return (
      <div className={styles.stage}>
        <div className={styles.empty}>
          <Radio size={30} aria-hidden="true" />
          <span className={styles.emptyTitle}>No one is in voice</span>
          <span className={styles.emptyBody}>
            Join from the panel at the bottom of the sidebar to talk and share
            your screen.
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className={styles.stage}>
      {isScreenSharing && localScreenStream && (
        <div className={`${styles.tile} ${styles.screenTile}`}>
          <ScreenPreview stream={localScreenStream} />
          <span className={styles.nameplate}>
            <ScreenShare
              size={13}
              style={{ color: "var(--color-accent)" }}
              aria-hidden="true"
            />
            <span className={styles.nameplateText}>You are sharing</span>
          </span>
        </div>
      )}

      {inVoice && (
        <div
          className={`${styles.tile} ${styles.tileLive} ${isAudioMuted ? styles.tileMuted : ""}`}
        >
          <span className={styles.avatarWrap}>
            <Avatar
              username={user?.username || "You"}
              src={user?.avatar_url}
              size="xl"
            />
          </span>
          <span className={styles.nameplate}>
            {isAudioMuted && (
              <MicOff
                size={13}
                style={{ color: "var(--color-danger)" }}
                aria-label="Muted"
              />
            )}
            <span className={styles.nameplateText}>
              {user?.username || "You"}
            </span>
          </span>
        </div>
      )}

      {peers.map((peer) => (
        <div key={peer.userId} className={`${styles.tile} ${styles.tileLive}`}>
          <span className={styles.avatarWrap}>
            <Avatar username={peer.username} size="xl" />
          </span>
          <span className={styles.nameplate}>
            <span className={styles.nameplateText}>{peer.username}</span>
          </span>
        </div>
      ))}
    </div>
  );
};
