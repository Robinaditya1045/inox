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
          width: '100%',
          borderRadius: '10px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 14px',
          background: 'radial-gradient(circle at 10% 50%, rgba(0, 240, 255, 0.08) 0%, rgba(17, 22, 34, 0.8) 100%)',
          border: '1px solid var(--color-border-hover)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div
            style={{
              width: '32px',
              height: '32px',
              borderRadius: '8px',
              background: 'rgba(0, 240, 255, 0.15)',
              color: 'var(--color-accent-cyan)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Radio size={16} />
          </div>
          <div>
            <h4 style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--color-text-primary)' }}>
              Pion SFU Voice
            </h4>
            <p style={{ color: 'var(--color-text-secondary)', fontSize: '0.75rem', marginTop: '1px' }}>
              WebRTC audio & screen share
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
          width: '100%',
          borderRadius: '10px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '10px 14px',
          gap: '10px',
          background: 'rgba(11, 14, 20, 0.8)',
          border: '1px solid var(--color-border-hover)',
        }}
      >
        <div className="spinner" style={{ width: '16px', height: '16px', border: '2px solid var(--color-accent-cyan)', borderTopColor: 'transparent', borderRadius: '50%', animation: 'spin 1s linear infinite' }} />
        <span style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--color-accent-cyan)' }}>
          Negotiating WebRTC SDP...
        </span>
      </div>
    );
  }

  return (
    <div
      className="glass-panel"
      style={{
        width: '100%',
        borderRadius: '10px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '8px 14px',
        background: 'linear-gradient(90deg, rgba(16, 185, 129, 0.12) 0%, rgba(11, 14, 20, 0.9) 50%, rgba(11, 14, 20, 0.9) 100%)',
        border: '1px solid rgba(16, 185, 129, 0.35)',
      }}
    >
      {/* Connected Status Indicator */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <div style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <div style={{ width: '30px', height: '30px', borderRadius: '8px', background: 'rgba(16, 185, 129, 0.2)', color: 'var(--color-accent-emerald)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Radio size={15} />
          </div>
          <div style={{ position: 'absolute', top: '-2px', right: '-2px', width: '8px', height: '8px', borderRadius: '50%', background: 'var(--color-accent-emerald)', border: '2px solid var(--color-bg-surface)' }} />
        </div>

        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <span style={{ fontSize: '0.82rem', fontWeight: 700, color: '#FFF' }}>Voice Connected</span>
            <span style={{ fontSize: '0.65rem', fontWeight: 700, padding: '1px 6px', borderRadius: '8px', background: 'rgba(16, 185, 129, 0.2)', color: 'var(--color-accent-emerald)', border: '1px solid rgba(16, 185, 129, 0.3)', textTransform: 'uppercase' }}>
              Live
            </span>
          </div>
          <span style={{ fontSize: '0.72rem', color: 'var(--color-text-secondary)' }}>
            ICE negotiated · low latency
          </span>
        </div>
      </div>

      {/* Media Action Controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
        {/* Mute Mic Button */}
        <button
          onClick={toggleMute}
          title={isAudioMuted ? 'Unmute Microphone' : 'Mute Microphone'}
          style={{
            width: '32px',
            height: '32px',
            borderRadius: '8px',
            background: isAudioMuted ? 'rgba(244, 63, 94, 0.2)' : 'var(--color-bg-surface)',
            border: `1px solid ${isAudioMuted ? 'var(--color-accent-rose)' : 'var(--color-border-glass)'}`,
            color: isAudioMuted ? 'var(--color-accent-rose)' : '#FFF',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
          }}
        >
          {isAudioMuted ? <MicOff size={15} /> : <Mic size={15} />}
        </button>

        {/* Deafen Button */}
        <button
          onClick={toggleDeafen}
          title={isDeafened ? 'Undeafen Audio' : 'Deafen Audio'}
          style={{
            width: '32px',
            height: '32px',
            borderRadius: '8px',
            background: isDeafened ? 'rgba(244, 63, 94, 0.2)' : 'var(--color-bg-surface)',
            border: `1px solid ${isDeafened ? 'var(--color-accent-rose)' : 'var(--color-border-glass)'}`,
            color: isDeafened ? 'var(--color-accent-rose)' : '#FFF',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
          }}
        >
          {isDeafened ? <VolumeX size={15} /> : <Volume2 size={15} />}
        </button>

        {/* Screen Share Toggle */}
        {permissions.can_share_screen && (
          <button
            onClick={toggleScreenShare}
            title={isScreenSharing ? 'Stop Screen Sharing' : 'Share Screen'}
            style={{
              padding: '0 10px',
              height: '32px',
              borderRadius: '8px',
              background: isScreenSharing ? 'rgba(170, 59, 255, 0.25)' : 'var(--color-bg-surface)',
              border: `1px solid ${isScreenSharing ? 'var(--color-accent-purple)' : 'var(--color-border-glass)'}`,
              color: isScreenSharing ? 'var(--color-accent-purple)' : '#FFF',
              fontSize: '0.75rem',
              fontWeight: 600,
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
            }}
          >
            {isScreenSharing ? <MonitorOff size={14} /> : <Monitor size={14} />}
            <span>{isScreenSharing ? 'Stop' : 'Share'}</span>
          </button>
        )}

        {/* Disconnect Voice Button */}
        <button
          onClick={disconnectAudio}
          title="Disconnect from Voice"
          style={{
            width: '32px',
            height: '32px',
            borderRadius: '8px',
            background: 'rgba(244, 63, 94, 0.15)',
            border: '1px solid var(--color-accent-rose)',
            color: 'var(--color-accent-rose)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
            marginLeft: '4px',
          }}
        >
          <PhoneOff size={15} />
        </button>
      </div>
    </div>
  );
};
