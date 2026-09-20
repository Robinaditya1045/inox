import React, { useCallback, useEffect, useRef, useState } from "react";
import { useRTC } from "../../hooks/useRTC";
import { usePermissions } from "../../hooks/usePermissions";
import { Avatar } from "../common/Avatar";
import type { VoicePeer } from "../../types/rtc";
import {
  Headphones,
  HeadphoneOff,
  Maximize2,
  Mic,
  MicOff,
  Minimize2,
  Monitor,
  MonitorUp,
  PhoneOff,
  Pin,
  PinOff,
  Radio,
  ScreenShare,
  ScreenShareOff,
} from "lucide-react";
import styles from "./VoiceStage.module.css";

interface VoiceStageProps {
  roomId: string | undefined;
}

/** A live screen share. Muted for the person sharing it, who would otherwise
 *  hear their own machine back. */
const ScreenView: React.FC<{ stream: MediaStream; isLocal: boolean }> = ({
  stream,
  isLocal,
}) => {
  const ref = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    const element = ref.current;
    if (element && element.srcObject !== stream) {
      element.srcObject = stream;
      // A track can arrive well after the click that started the call, which is
      // late enough for autoplay to be refused; play() surfaces that instead of
      // leaving a silently frozen frame.
      element.play().catch(() => {});
    }
  }, [stream]);

  return (
    <video
      ref={ref}
      autoPlay
      playsInline
      muted={isLocal}
      className={styles.screenVideo}
    />
  );
};

interface TileProps {
  peer: VoicePeer;
  isSpeaking: boolean;
  isPinned?: boolean;
  onTogglePin?: () => void;
  compact?: boolean;
}

const ParticipantTile: React.FC<TileProps> = ({
  peer,
  isSpeaking,
  isPinned,
  onTogglePin,
  compact,
}) => {
  const classes = [
    styles.tile,
    compact ? styles.tileCompact : "",
    isSpeaking ? styles.tileSpeaking : "",
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={classes}>
      <span className={styles.avatarWrap}>
        <Avatar
          username={peer.username}
          size={compact ? "lg" : "xl"}
          className={peer.isMuted ? styles.avatarMuted : undefined}
        />
      </span>

      {peer.isScreenSharing && onTogglePin && (
        <button
          className={styles.tilePin}
          onClick={onTogglePin}
          aria-label={
            isPinned
              ? `Unpin ${peer.username}'s screen`
              : `Pin ${peer.username}'s screen`
          }
          aria-pressed={!!isPinned}
          title={isPinned ? "Unpin" : "Pin this screen"}
        >
          {isPinned ? <PinOff size={13} /> : <Pin size={13} />}
        </button>
      )}

      <span className={styles.nameplate}>
        {peer.isMuted ? (
          <MicOff
            size={12}
            style={{ color: "var(--color-danger)", flexShrink: 0 }}
            aria-label="Muted"
          />
        ) : (
          <Mic
            size={12}
            style={{
              color: isSpeaking
                ? "var(--color-speaking)"
                : "var(--color-text-muted)",
              flexShrink: 0,
            }}
            aria-hidden="true"
          />
        )}
        <span className={styles.nameplateText}>
          {peer.username}
          {peer.isLocal ? " (You)" : ""}
        </span>
        {peer.isScreenSharing && (
          <ScreenShare
            size={12}
            style={{ color: "var(--color-accent)", flexShrink: 0 }}
            aria-label="Sharing screen"
          />
        )}
      </span>
    </div>
  );
};

export const VoiceStage: React.FC<VoiceStageProps> = ({ roomId }) => {
  const {
    connectionState,
    peers,
    speakingIds,
    isAudioMuted,
    isDeafened,
    isScreenSharing,
    connectAudio,
    disconnectAudio,
    toggleMute,
    toggleDeafen,
    toggleScreenShare,
  } = useRTC(roomId);
  const permissions = usePermissions();

  const [pinnedId, setPinnedId] = useState<string | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const stageRef = useRef<HTMLDivElement | null>(null);

  const inVoice = connectionState === "connected";
  const isJoining = connectionState === "connecting";

  // Anyone sharing, whether or not their first frame has arrived yet.
  const sharing = peers.filter((peer) => peer.isScreenSharing);
  // A pin on someone who has since stopped sharing is simply not in effect,
  // rather than something to clear and re-render for.
  const activePin = sharing.some((peer) => peer.userId === pinnedId)
    ? pinnedId
    : null;
  const presenter =
    sharing.find((peer) => peer.userId === activePin) ??
    // Someone else's screen is what you came to look at; your own is a preview.
    sharing.find((peer) => !peer.isLocal) ??
    sharing[0] ??
    null;

  useEffect(() => {
    const onChange = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", onChange);
    return () => document.removeEventListener("fullscreenchange", onChange);
  }, []);

  const toggleFullscreen = useCallback(() => {
    const element = stageRef.current;
    if (!element) return;
    if (document.fullscreenElement) {
      void document.exitFullscreen().catch(() => {});
    } else {
      void element.requestFullscreen().catch(() => {});
    }
  }, []);

  const controls = (
    <div className={styles.controls}>
      {inVoice || isJoining ? (
        <>
          <button
            className={`${styles.ctrl} ${isAudioMuted ? styles.ctrlDanger : ""}`}
            onClick={toggleMute}
            aria-label={isAudioMuted ? "Unmute microphone" : "Mute microphone"}
            aria-pressed={isAudioMuted}
            title={isAudioMuted ? "Unmute" : "Mute"}
          >
            {isAudioMuted ? <MicOff size={18} /> : <Mic size={18} />}
          </button>

          <button
            className={`${styles.ctrl} ${isDeafened ? styles.ctrlDanger : ""}`}
            onClick={toggleDeafen}
            aria-label={isDeafened ? "Undeafen" : "Deafen"}
            aria-pressed={isDeafened}
            title={isDeafened ? "Undeafen" : "Deafen"}
          >
            {isDeafened ? <HeadphoneOff size={18} /> : <Headphones size={18} />}
          </button>

          {permissions.can_share_screen && (
            <button
              className={`${styles.ctrl} ${isScreenSharing ? styles.ctrlAccent : ""}`}
              onClick={toggleScreenShare}
              aria-label={
                isScreenSharing ? "Stop presenting" : "Present your screen"
              }
              aria-pressed={isScreenSharing}
              title={isScreenSharing ? "Stop presenting" : "Present now"}
            >
              {isScreenSharing ? (
                <ScreenShareOff size={18} />
              ) : (
                <MonitorUp size={18} />
              )}
            </button>
          )}

          {presenter && (
            <button
              className={styles.ctrl}
              onClick={toggleFullscreen}
              aria-label={isFullscreen ? "Exit full screen" : "Full screen"}
              title={isFullscreen ? "Exit full screen" : "Full screen"}
            >
              {isFullscreen ? <Minimize2 size={18} /> : <Maximize2 size={18} />}
            </button>
          )}

          <button
            className={`${styles.ctrl} ${styles.ctrlLeave}`}
            onClick={disconnectAudio}
            aria-label="Leave voice"
            title="Leave voice"
          >
            <PhoneOff size={18} />
          </button>
        </>
      ) : (
        permissions.can_stream_audio && (
          <button
            className={styles.joinBtn}
            onClick={() => void connectAudio()}
            aria-label="Join voice channel"
          >
            <Radio size={15} aria-hidden="true" />
            Join voice
          </button>
        )
      )}
    </div>
  );

  if (peers.length === 0) {
    return (
      <div className={styles.shell}>
        <div className={styles.empty}>
          <Radio size={30} aria-hidden="true" />
          <span className={styles.emptyTitle}>
            {isJoining ? "Connecting…" : "No one is in voice"}
          </span>
          <span className={styles.emptyBody}>
            Join to talk, or present your screen for everyone in the room.
          </span>
        </div>
        {controls}
      </div>
    );
  }

  // ── Presentation mode ─────────────────────────────────────────────────
  // One screen fills the stage and everyone else shrinks to a filmstrip beside
  // it, the way a call behaves the moment someone starts presenting.
  if (presenter) {
    const others = peers.filter((peer) => peer.userId !== presenter.userId);

    return (
      <div className={styles.shell}>
        <div className={styles.body}>
          <div className={styles.stageMain} ref={stageRef}>
            {presenter.screenStream ? (
              <ScreenView
                stream={presenter.screenStream}
                isLocal={presenter.isLocal}
              />
            ) : (
              <div className={styles.awaitingScreen}>
                <Monitor size={26} aria-hidden="true" />
                <span>Waiting for {presenter.username}&apos;s screen…</span>
              </div>
            )}

            <span className={styles.presenterChip}>
              <ScreenShare
                size={13}
                style={{ color: "var(--color-accent)" }}
                aria-hidden="true"
              />
              <span className={styles.nameplateText}>
                {presenter.isLocal
                  ? "You are presenting"
                  : `${presenter.username} is presenting`}
              </span>
            </span>

            <div className={styles.stageActions}>
              {sharing.length > 1 && (
                <button
                  className={styles.stageBtn}
                  onClick={() =>
                    setPinnedId((prev) =>
                      prev === presenter.userId ? null : presenter.userId,
                    )
                  }
                  aria-label={
                    activePin === presenter.userId
                      ? "Unpin this screen"
                      : "Pin this screen"
                  }
                  aria-pressed={activePin === presenter.userId}
                  title={
                    activePin === presenter.userId
                      ? "Unpin"
                      : "Keep this screen"
                  }
                >
                  {activePin === presenter.userId ? (
                    <PinOff size={15} />
                  ) : (
                    <Pin size={15} />
                  )}
                </button>
              )}
              <button
                className={styles.stageBtn}
                onClick={toggleFullscreen}
                aria-label={isFullscreen ? "Exit full screen" : "Full screen"}
                title={isFullscreen ? "Exit full screen" : "Full screen"}
              >
                {isFullscreen ? (
                  <Minimize2 size={15} />
                ) : (
                  <Maximize2 size={15} />
                )}
              </button>
            </div>
          </div>

          <div className={styles.filmstrip}>
            <ParticipantTile
              peer={presenter}
              isSpeaking={speakingIds.has(presenter.userId)}
              isPinned={activePin === presenter.userId}
              onTogglePin={() =>
                setPinnedId((prev) =>
                  prev === presenter.userId ? null : presenter.userId,
                )
              }
              compact
            />
            {others.map((peer) => (
              <ParticipantTile
                key={peer.userId}
                peer={peer}
                isSpeaking={speakingIds.has(peer.userId)}
                isPinned={activePin === peer.userId}
                onTogglePin={() =>
                  setPinnedId((prev) =>
                    prev === peer.userId ? null : peer.userId,
                  )
                }
                compact
              />
            ))}
          </div>
        </div>
        {controls}
      </div>
    );
  }

  // ── Gallery ───────────────────────────────────────────────────────────
  return (
    <div className={styles.shell}>
      <div className={styles.body}>
        <div
          className={styles.grid}
          data-count={peers.length > 4 ? "many" : peers.length}
        >
          {peers.map((peer) => (
            <ParticipantTile
              key={peer.userId}
              peer={peer}
              isSpeaking={speakingIds.has(peer.userId)}
            />
          ))}
        </div>
      </div>
      {controls}
    </div>
  );
};
