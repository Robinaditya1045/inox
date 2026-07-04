import React from 'react';
import { useRTC } from '../../hooks/useRTC';
import { usePermissions } from '../../hooks/usePermissions';
import { Button } from '../common/Button';
import { Radio, Mic, MicOff, Volume2, VolumeX, Monitor, MonitorOff, PhoneOff, Lock } from 'lucide-react';

interface VoiceChannelBarProps {
  roomId: string | undefined;
}

export const VoiceChannelBar: React.FC<VoiceChannelBarProps> = ({ roomId }) => {
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

  if (connectionState === 'disconnected' || connectionState === 'failed') {
    return (
      <div
        className="glass-panel"
        style={{
          gridColumn: '1 / 2',
          gridRow: '2 / 3',
          borderRadius: '16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '20px 24px',
          background: 'radial-gradient(circle at 10% 50%, rgba(0, 240, 255, 0.1) 0%, rgba(17, 22, 34, 0.8) 100%)',
          border: '1px solid var(--color-border-hover)',
          boxShadow: '0 8px 30px rgba(0, 0, 0, 0.5)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div
            style={{
              width: '48px',
              height: '48px',
              borderRadius: '14px',
              background: 'rgba(0, 240, 255, 0.15)',
              color: 'var(--color-accent-cyan)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              boxShadow: '0 0 20px var(--color-accent-cyan-glow)',
            }}
          >
            <Radio size={24} />
          </div>
          <div>
            <h4 style={{ fontSize: '1.1rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>
              Pion SFU Voice Channel
            </h4>
            <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.85rem', marginTop: '2px' }}>
              Real-time WebRTC audio routing & high-definition screen sharing.
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          {!permissions.can_stream_audio ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 14px', borderRadius: '10px', background: 'rgba(244, 63, 94, 0.1)', color: 'var(--color-accent-rose)', fontSize: '0.8rem', fontWeight: 600 }}>
              <Lock size={14} />
              <span>Voice Restricted</span>
            </div>
          ) : (
            <Button
              variant="primary"
              size="md"
              icon={<Radio size={18} />}
              onClick={connectAudio}
              style={{
                background: 'linear-gradient(135deg, #00F0FF 0%, #0072FF 100%)',
                color: '#000',
                fontWeight: 700,
                boxShadow: '0 0 20px rgba(0, 240, 255, 0.4)',
              }}
            >
              Connect Audio
            </Button>
          )}
        </div>
      </div>
    );
  }

  if (connectionState === 'connecting') {
    return (
      <div
        className="glass-panel"
        style={{
          gridColumn: '1 / 2',
          gridRow: '2 / 3',
          borderRadius: '16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '20px',
          gap: '12px',
          background: 'rgba(11, 14, 20, 0.8)',
          border: '1px solid var(--color-border-hover)',
        }}
      >
        <div className="spinner" style={{ width: '20px', height: '20px', border: '2px solid var(--color-accent-cyan)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
        <span style={{ fontSize: '0.95rem', fontWeight: 600, color: 'var(--color-accent-cyan)' }}>
          Negotiating WebRTC SDP Handshake with Pion SFU...
        </span>
      </div>
    );
  }

  return (
    <div
      className="glass-panel"
      style={{
        gridColumn: '1 / 2',
        gridRow: '2 / 3',
        borderRadius: '16px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '16px 24px',
        background: 'linear-gradient(90deg, rgba(16, 185, 129, 0.15) 0%, rgba(11, 14, 20, 0.9) 50%, rgba(11, 14, 20, 0.9) 100%)',
        border: '1px solid rgba(16, 185, 129, 0.4)',
        boxShadow: '0 0 25px rgba(16, 185, 129, 0.2)',
      }}
    >
      {/* Connected Status Indicator */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ width: '42px', height: '42px', borderRadius: '12px', background: 'rgba(16, 185, 129, 0.2)', color: 'var(--color-accent-emerald)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Radio size={22} />
          </div>
          <div style={{ position: 'absolute', top: '-3px', right: '-3px', width: '12px', height: '12px', borderRadius: '50%', background: 'var(--color-accent-emerald)', border: '2px solid var(--color-bg-surface)', boxShadow: '0 0 10px var(--color-accent-emerald)' }} />
        </div>

        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '1rem', fontWeight: 700, color: '#FFF' }}>Voice Connected</span>
            <span style={{ fontSize: '0.7rem', fontWeight: 700, padding: '2px 8px', borderRadius: '10px', background: 'rgba(16, 185, 129, 0.2)', color: 'var(--color-accent-emerald)', border: '1px solid rgba(16, 185, 129, 0.4)', textTransform: 'uppercase' }}>
              Pion SFU Live
            </span>
          </div>
          <span style={{ fontSize: '0.8rem', color: 'var(--color-text-secondary)' }}>
            STUN ICE negotiated • Low latency audio channel
          </span>
        </div>
      </div>

      {/* Media Action Controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
        {/* Mute Mic Button */}
        <button
          onClick={toggleMute}
          title={isAudioMuted ? 'Unmute Microphone' : 'Mute Microphone'}
          style={{
            width: '44px',
            height: '44px',
            borderRadius: '12px',
            background: isAudioMuted ? 'rgba(244, 63, 94, 0.2)' : 'var(--color-bg-surface)',
            border: `1px solid ${isAudioMuted ? 'var(--color-accent-rose)' : 'var(--color-border-glass)'}`,
            color: isAudioMuted ? 'var(--color-accent-rose)' : '#FFF',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
            boxShadow: isAudioMuted ? '0 0 15px rgba(244, 63, 94, 0.4)' : 'none',
          }}
        >
          {isAudioMuted ? <MicOff size={20} /> : <Mic size={20} />}
        </button>

        {/* Deafen Button */}
        <button
          onClick={toggleDeafen}
          title={isDeafened ? 'Undeafen Audio' : 'Deafen Audio'}
          style={{
            width: '44px',
            height: '44px',
            borderRadius: '12px',
            background: isDeafened ? 'rgba(244, 63, 94, 0.2)' : 'var(--color-bg-surface)',
            border: `1px solid ${isDeafened ? 'var(--color-accent-rose)' : 'var(--color-border-glass)'}`,
            color: isDeafened ? 'var(--color-accent-rose)' : '#FFF',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
            boxShadow: isDeafened ? '0 0 15px rgba(244, 63, 94, 0.4)' : 'none',
          }}
        >
          {isDeafened ? <VolumeX size={20} /> : <Volume2 size={20} />}
        </button>

        {/* Screen Share Toggle */}
        {permissions.can_share_screen && (
          <button
            onClick={toggleScreenShare}
            title={isScreenSharing ? 'Stop Screen Sharing' : 'Share Screen'}
            style={{
              padding: '0 16px',
              height: '44px',
              borderRadius: '12px',
              background: isScreenSharing ? 'rgba(170, 59, 255, 0.25)' : 'var(--color-bg-surface)',
              border: `1px solid ${isScreenSharing ? 'var(--color-accent-purple)' : 'var(--color-border-glass)'}`,
              color: isScreenSharing ? 'var(--color-accent-purple)' : '#FFF',
              fontSize: '0.85rem',
              fontWeight: 600,
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
              boxShadow: isScreenSharing ? '0 0 20px rgba(170, 59, 255, 0.5)' : 'none',
            }}
          >
            {isScreenSharing ? <MonitorOff size={18} /> : <Monitor size={18} />}
            <span>{isScreenSharing ? 'Stop Sharing' : 'Share Screen'}</span>
          </button>
        )}

        {/* Disconnect Voice Button */}
        <button
          onClick={disconnectAudio}
          title="Disconnect from Voice"
          style={{
            width: '44px',
            height: '44px',
            borderRadius: '12px',
            background: 'rgba(244, 63, 94, 0.15)',
            border: '1px solid var(--color-accent-rose)',
            color: 'var(--color-accent-rose)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
            marginLeft: '8px',
          }}
        >
          <PhoneOff size={20} />
        </button>
      </div>
    </div>
  );
};
