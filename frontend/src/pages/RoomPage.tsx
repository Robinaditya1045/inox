import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useRoom } from '../hooks/useRoom';
import { usePermissions } from '../hooks/usePermissions';
import { Button } from '../components/common/Button';
import { Spinner } from '../components/common/Spinner';
import { WatchPartyPlayer } from '../components/player/WatchPartyPlayer';
import { ChatPanel } from '../components/chat/ChatPanel';
import { MemberList } from '../components/room/MemberList';
import { VoiceChannelBar } from '../components/rtc/VoiceChannelBar';
import { AudioRenderer } from '../components/rtc/AudioRenderer';
import { useRTC } from '../hooks/useRTC';
import { Tv, Users, MessageSquare, Shield, LogOut, Lock, Globe } from 'lucide-react';

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, isLoadingRoom, roomError } = useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [rightTab, setRightTab] = useState<'chat' | 'members'>('chat');
  const rtc = useRTC(activeRoom?.id);

  useEffect(() => {
    if (roomId && activeRoom?.id !== roomId) {
      joinRoom(roomId).catch(() => {
        // Error is set in context
      });
    }
  }, [roomId, activeRoom?.id, joinRoom]);

  const handleLeave = async () => {
    await leaveRoom();
    navigate('/');
  };

  if (isLoadingRoom && !activeRoom) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', flexDirection: 'column', gap: '16px' }}>
        <Spinner size={36} color="var(--color-accent-purple)" />
        <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.95rem' }}>Joining watch party...</span>
      </div>
    );
  }

  if (roomError || (!activeRoom && !isLoadingRoom)) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', flexDirection: 'column', gap: '20px', padding: '32px', textAlign: 'center' }}>
        <div style={{ width: '56px', height: '56px', borderRadius: '50%', background: 'rgba(244, 63, 94, 0.15)', color: 'var(--color-accent-rose)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Shield size={28} />
        </div>
        <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.75rem', fontWeight: 700 }}>Room Not Accessible</h2>
        <p style={{ color: 'var(--color-text-secondary)', maxWidth: '400px' }}>
          {roomError || 'The watch party room you requested does not exist or you do not have permission to join.'}
        </p>
        <Button variant="secondary" onClick={() => navigate('/')}>
          Return to Lobby
        </Button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      {/* Room Top Bar */}
      <header
        className="glass-panel"
        style={{
          height: 'var(--header-height)',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          borderBottom: '1px solid var(--color-border-glass)',
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <Tv size={22} color="var(--color-accent-purple)" />
            <h1 style={{ fontFamily: 'var(--font-display)', fontSize: '1.35rem', fontWeight: 700 }}>
              {activeRoom?.name}
            </h1>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 10px', borderRadius: '14px', background: 'var(--color-bg-surface)', border: '1px solid var(--color-border-glass)', fontSize: '0.8rem', color: 'var(--color-text-secondary)' }}>
            {activeRoom?.is_private ? <Lock size={14} color="var(--color-accent-rose)" /> : <Globe size={14} color="var(--color-accent-cyan)" />}
            <span>{activeRoom?.is_private ? 'Private' : 'Public'}</span>
          </div>

          {/* RBAC Role Badge */}
          <div
            style={{
              padding: '4px 10px',
              borderRadius: '14px',
              background: permissions.isOwner ? 'rgba(170, 59, 255, 0.2)' : 'rgba(0, 240, 255, 0.15)',
              border: `1px solid ${permissions.isOwner ? 'var(--color-accent-purple)' : 'var(--color-accent-cyan)'}`,
              color: permissions.isOwner ? 'var(--color-accent-purple)' : 'var(--color-accent-cyan)',
              fontSize: '0.75rem',
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
            }}
          >
            {permissions.role}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '0.9rem', color: 'var(--color-text-secondary)' }}>
            <Users size={16} />
            <span>{activeRoom?.members?.length || 1} Members</span>
          </div>

          <Button variant="danger" size="sm" icon={<LogOut size={16} />} onClick={handleLeave}>
            Leave Room
          </Button>
        </div>
      </header>

      {/* 3-Column Workspace Stage (Player, Chat, Voice SFU placeholders for M3/M4/M5) */}
      <main style={{ flex: 1, display: 'grid', gridTemplateColumns: '1fr 340px', gridTemplateRows: '1fr 180px', gap: '16px', padding: '16px', overflow: 'hidden' }}>
        {/* Center Stage: Watch Party Player Stage (Milestone 3) */}
        <div
          style={{
            gridColumn: '1 / 2',
            gridRow: '1 / 2',
            borderRadius: '16px',
            overflow: 'hidden',
          }}
        >
          <WatchPartyPlayer />
        </div>

        {/* Right Stage: Real-Time Chat & Member List (Milestone 4) */}
        <div
          className="glass-panel"
          style={{
            gridColumn: '2 / 3',
            gridRow: '1 / 3',
            borderRadius: '16px',
            display: 'flex',
            flexDirection: 'column',
            overflow: 'hidden',
            border: '1px solid var(--color-border-hover)',
          }}
        >
          {/* Tab Header */}
          <div style={{ display: 'flex', borderBottom: '1px solid var(--color-border-glass)', background: 'rgba(5, 7, 10, 0.4)' }}>
            <button
              onClick={() => setRightTab('chat')}
              style={{
                flex: 1,
                padding: '14px',
                background: rightTab === 'chat' ? 'rgba(170, 59, 255, 0.15)' : 'transparent',
                borderBottom: `2px solid ${rightTab === 'chat' ? 'var(--color-accent-purple)' : 'transparent'}`,
                color: rightTab === 'chat' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                fontSize: '0.85rem',
                fontWeight: 600,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '8px',
                cursor: 'pointer',
                transition: 'all var(--transition-fast)',
              }}
            >
              <MessageSquare size={16} />
              <span>Room Chat</span>
            </button>
            <button
              onClick={() => setRightTab('members')}
              style={{
                flex: 1,
                padding: '14px',
                background: rightTab === 'members' ? 'rgba(170, 59, 255, 0.15)' : 'transparent',
                borderBottom: `2px solid ${rightTab === 'members' ? 'var(--color-accent-purple)' : 'transparent'}`,
                color: rightTab === 'members' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                fontSize: '0.85rem',
                fontWeight: 600,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '8px',
                cursor: 'pointer',
                transition: 'all var(--transition-fast)',
              }}
            >
              <Users size={16} />
              <span>Members ({activeRoom?.members?.length || 0})</span>
            </button>
          </div>

          {/* Tab Body */}
          <div style={{ flex: 1, overflow: 'hidden' }}>
            {rightTab === 'chat' ? (
              <ChatPanel roomId={activeRoom?.id} />
            ) : (
              <MemberList />
            )}
          </div>
        </div>

        {/* Bottom Stage: WebRTC Pion SFU Voice Channel (Milestone 5) */}
        <VoiceChannelBar roomId={activeRoom?.id} />

        {/* Invisible Audio Renderer for Remote Peer Voice Streams */}
        <AudioRenderer remoteStreams={rtc.remoteStreams} isDeafened={rtc.isDeafened} />
      </main>
    </div>
  );
};
