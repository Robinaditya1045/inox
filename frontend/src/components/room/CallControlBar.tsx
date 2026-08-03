import React from 'react';
import { useRTC } from '../../hooks/useRTC';
import { usePermissions } from '../../hooks/usePermissions';
import { useRoomSocket } from '../../hooks/useRoomSocket';
import {
  Mic,
  MicOff,
  Volume2,
  VolumeX,
  Monitor,
  MonitorOff,
  PhoneOff,
  Radio,
  MessageSquare,
  Users,
  Lock,
} from 'lucide-react';
import styles from './CallControlBar.module.css';

type ActivePanel = 'chat' | 'members' | null;

interface CallControlBarProps {
  roomId: string | undefined;
  onLeave: () => void;
  activePanel: ActivePanel;
  onTogglePanel: (panel: 'chat' | 'members') => void;
}

export const CallControlBar: React.FC<CallControlBarProps> = ({
  roomId,
  onLeave,
  activePanel,
  onTogglePanel,
}) => {
  const {
    connectionState,
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
  const { status: wsStatus } = useRoomSocket();

  // ── WebSocket status indicator ────────────────────────────────
  const wsStatusClass =
    wsStatus === 'OPEN'
      ? styles.statusConnected
      : wsStatus === 'CONNECTING'
      ? styles.statusConnecting
      : styles.statusDisconnected;

  const wsStatusLabel =
    wsStatus === 'OPEN' ? 'Connected' : wsStatus === 'CONNECTING' ? 'Connecting…' : 'Offline';

  return (
    <div className={styles.bar} role="toolbar" aria-label="Call controls">
      {/* ── WS Status ─────────────────────────────────────────── */}
      <div className={styles.connectionStatus} aria-label={`WebSocket: ${wsStatusLabel}`}>
        <div className={`${styles.statusDot} ${wsStatusClass}`} aria-hidden="true" />
        <span>{wsStatusLabel}</span>
      </div>

      <div className={styles.divider} aria-hidden="true" />

      {/* ── Voice Controls ───────────────────────────────────── */}
      {connectionState === 'disconnected' || connectionState === 'failed' ? (
        /* Not connected — show permission state and connect prompt */
        <div className={styles.connectPrompt}>
          {!permissions.can_stream_audio ? (
            <>
              <Lock size={13} aria-hidden="true" />
              <span>Voice restricted</span>
            </>
          ) : (
            <>
              <Radio size={13} aria-hidden="true" style={{ color: 'var(--color-text-muted)' }} />
              <button
                className={`${styles.controlBtn} ${styles.controlBtnAccent}`}
                style={{ width: 'auto', padding: '0 var(--space-3)', gap: 'var(--space-2)', fontSize: 'var(--text-meta)', fontWeight: 600 }}
                onClick={connectAudio}
                aria-label="Connect to voice"
                title="Connect to voice"
              >
                <Radio size={13} />
                Connect Audio
              </button>
            </>
          )}
        </div>
      ) : connectionState === 'connecting' ? (
        /* Connecting spinner */
        <div className={styles.connectPrompt}>
          <div
            aria-hidden="true"
            style={{
              width: 14,
              height: 14,
              borderRadius: '50%',
              border: '2px solid var(--color-border-default)',
              borderTopColor: 'var(--color-accent)',
              animation: 'spin 0.8s linear infinite',
            }}
          />
          <span>Negotiating…</span>
        </div>
      ) : (
        /* Connected — full controls */
        <>
          {/* Mute microphone */}
          <button
            className={`${styles.controlBtn} ${isAudioMuted ? styles.controlBtnActive : ''}`}
            onClick={toggleMute}
            aria-label={isAudioMuted ? 'Unmute microphone' : 'Mute microphone'}
            aria-pressed={isAudioMuted}
            title={isAudioMuted ? 'Unmute microphone' : 'Mute microphone'}
          >
            {isAudioMuted ? <MicOff size={16} /> : <Mic size={16} />}
          </button>

          {/* Deafen */}
          <button
            className={`${styles.controlBtn} ${isDeafened ? styles.controlBtnActive : ''}`}
            onClick={toggleDeafen}
            aria-label={isDeafened ? 'Undeafen' : 'Deafen audio'}
            aria-pressed={isDeafened}
            title={isDeafened ? 'Undeafen' : 'Deafen audio'}
          >
            {isDeafened ? <VolumeX size={16} /> : <Volume2 size={16} />}
          </button>

          {/* Screen share */}
          {permissions.can_share_screen ? (
            <button
              className={`${styles.controlBtn} ${isScreenSharing ? styles.controlBtnAccent : ''}`}
              onClick={toggleScreenShare}
              aria-label={isScreenSharing ? 'Stop screen sharing' : 'Share screen'}
              aria-pressed={isScreenSharing}
              title={isScreenSharing ? 'Stop screen sharing' : 'Share screen'}
            >
              {isScreenSharing ? <MonitorOff size={16} /> : <Monitor size={16} />}
            </button>
          ) : (
            <button
              className={styles.controlBtn}
              disabled
              aria-label="Screen sharing not permitted"
              title="Screen sharing not permitted"
            >
              <Monitor size={16} />
            </button>
          )}

          {isScreenSharing && (
            <span className={styles.sharingLabel} aria-live="polite">
              Sharing
            </span>
          )}

          {/* Disconnect voice */}
          <button
            className={`${styles.controlBtn}`}
            style={{
              background: 'var(--color-danger-subtle)',
              color: 'var(--color-danger)',
              borderColor: 'var(--color-danger-border)',
            }}
            onClick={disconnectAudio}
            aria-label="Disconnect from voice"
            title="Disconnect from voice"
          >
            <PhoneOff size={15} />
          </button>
        </>
      )}

      <div className={styles.divider} aria-hidden="true" />

      {/* ── Panel toggles ─────────────────────────────────────── */}
      <button
        className={`${styles.controlBtn} ${activePanel === 'chat' ? styles.controlBtnAccent : ''}`}
        onClick={() => onTogglePanel('chat')}
        aria-label={activePanel === 'chat' ? 'Close chat' : 'Open chat'}
        aria-pressed={activePanel === 'chat'}
        title="Chat"
      >
        <MessageSquare size={16} />
      </button>

      <button
        className={`${styles.controlBtn} ${activePanel === 'members' ? styles.controlBtnAccent : ''}`}
        onClick={() => onTogglePanel('members')}
        aria-label={activePanel === 'members' ? 'Close members list' : 'Open members list'}
        aria-pressed={activePanel === 'members'}
        title="Members"
      >
        <Users size={16} />
      </button>

      {/* ── Leave ─────────────────────────────────────────────── */}
      <button
        className={styles.leaveBtn}
        onClick={onLeave}
        aria-label="Leave room"
        title="Leave room"
      >
        <PhoneOff size={13} />
        Leave
      </button>
    </div>
  );
};
