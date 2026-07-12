import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useRoom } from '../hooks/useRoom';
import { usePermissions } from '../hooks/usePermissions';
import { Button } from '../components/common/Button';
import { Spinner } from '../components/common/Spinner';
import { WatchPartyPlayer } from '../components/player/WatchPartyPlayer';
import { MediaLibraryPicker } from '../components/player/MediaLibraryPicker';
import { ChatPanel } from '../components/chat/ChatPanel';
import { MemberList } from '../components/room/MemberList';
import { VoiceChannelBar } from '../components/rtc/VoiceChannelBar';
import { AudioRenderer } from '../components/rtc/AudioRenderer';
import { useRTC } from '../hooks/useRTC';
import { usePlayerSync } from '../hooks/usePlayerSync';
import {
  Tv,
  Users,
  MessageSquare,
  Shield,
  LogOut,
  Lock,
  Globe,
  ChevronDown,
  Hash,
  Volume2,
  Film,
} from 'lucide-react';

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, isLoadingRoom, roomError } = useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [activeSection, setActiveSection] = useState<'chat' | 'members' | 'player'>('chat');
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);

  const rtc = useRTC(activeRoom?.id);
  const { setMediaUrl, mediaUrl } = usePlayerSync();

  useEffect(() => {
    if (roomId && activeRoom?.id !== roomId) {
      joinRoom(roomId).catch(() => {});
    }
  }, [roomId, activeRoom?.id, joinRoom]);

  const handleLeave = async () => {
    await leaveRoom();
    navigate('/');
  };

  if (isLoadingRoom && !activeRoom) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', flexDirection: 'column', gap: '12px' }}>
        <Spinner size={30} color="var(--color-accent-purple)" />
        <span style={{ color: 'var(--color-text-secondary)', fontSize: '0.88rem' }}>Joining room...</span>
      </div>
    );
  }

  if (roomError || (!activeRoom && !isLoadingRoom)) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%', flexDirection: 'column', gap: '16px', padding: '32px', textAlign: 'center' }}>
        <div style={{ width: '48px', height: '48px', borderRadius: '50%', background: 'rgba(244,63,94,0.15)', color: 'var(--color-accent-rose)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Shield size={24} />
        </div>
        <h2 style={{ fontFamily: 'var(--font-display)', fontSize: '1.4rem', fontWeight: 700 }}>Room Not Accessible</h2>
        <p style={{ color: 'var(--color-text-secondary)', maxWidth: '360px', fontSize: '0.9rem' }}>
          {roomError || 'This room does not exist or you lack permission to join.'}
        </p>
        <Button variant="secondary" onClick={() => navigate('/')}>Return to Lobby</Button>
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', width: '100%', height: '100%', flex: 1, overflow: 'hidden', background: 'var(--color-bg-obsidian)' }}>
      {/* ── Left Sidebar (Discord-style channel list) ──────────────────── */}
      <aside
        style={{
          width: '220px',
          flexShrink: 0,
          background: 'rgba(5,7,10,0.7)',
          borderRight: '1px solid var(--color-border-glass)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Room Header */}
        <div
          style={{
            padding: '12px 14px',
            borderBottom: '1px solid var(--color-border-glass)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            flexShrink: 0,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
            <Tv size={16} color="var(--color-accent-purple)" style={{ flexShrink: 0 }} />
            <span
              style={{
                fontWeight: 700,
                fontSize: '0.92rem',
                color: 'var(--color-text-primary)',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {activeRoom?.name}
            </span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0 }}>
            {activeRoom?.is_private ? (
              <Lock size={12} color="var(--color-accent-rose)" />
            ) : (
              <Globe size={12} color="var(--color-accent-cyan)" />
            )}
          </div>
        </div>

        {/* Channels / Sections */}
        <nav style={{ flex: 1, overflowY: 'auto', padding: '8px 6px' }}>
          {/* Text Channels */}
          <div style={{ marginBottom: '4px' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                padding: '4px 6px',
                color: 'var(--color-text-muted)',
                fontSize: '0.7rem',
                fontWeight: 700,
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                cursor: 'default',
              }}
            >
              <ChevronDown size={12} />
              Text Channels
            </div>

            <button
              onClick={() => setActiveSection('chat')}
              style={{
                width: '100%',
                padding: '5px 8px',
                borderRadius: '6px',
                background: activeSection === 'chat' ? 'rgba(170,59,255,0.2)' : 'transparent',
                color: activeSection === 'chat' ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                fontSize: '0.875rem',
                fontWeight: activeSection === 'chat' ? 600 : 400,
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                cursor: 'pointer',
                border: 'none',
                transition: 'all 0.12s',
                textAlign: 'left',
              }}
              onMouseEnter={(e) => {
                if (activeSection !== 'chat') e.currentTarget.style.background = 'rgba(255,255,255,0.05)';
              }}
              onMouseLeave={(e) => {
                if (activeSection !== 'chat') e.currentTarget.style.background = 'transparent';
              }}
            >
              <Hash size={16} color={activeSection === 'chat' ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} />
              general
            </button>
          </div>

          {/* Voice Channels */}
          <div style={{ marginBottom: '4px', marginTop: '12px' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                padding: '4px 6px',
                color: 'var(--color-text-muted)',
                fontSize: '0.7rem',
                fontWeight: 700,
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                cursor: 'default',
              }}
            >
              <ChevronDown size={12} />
              Voice Channels
            </div>

            <div
              style={{
                padding: '5px 8px',
                borderRadius: '6px',
                color: 'var(--color-text-secondary)',
                fontSize: '0.875rem',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Volume2 size={16} color="var(--color-text-muted)" />
              voice
            </div>
          </div>

          {/* Watch Party */}
          <div style={{ marginTop: '12px' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                padding: '4px 6px',
                color: 'var(--color-text-muted)',
                fontSize: '0.7rem',
                fontWeight: 700,
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                cursor: 'default',
              }}
            >
              <ChevronDown size={12} />
              Watch Party
            </div>

            <button
              onClick={() => setActiveSection('player')}
              style={{
                width: '100%',
                padding: '5px 8px',
                borderRadius: '6px',
                background: activeSection === 'player' ? 'rgba(170,59,255,0.2)' : 'transparent',
                color: activeSection === 'player' ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                fontSize: '0.875rem',
                fontWeight: activeSection === 'player' ? 600 : 400,
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                cursor: 'pointer',
                border: 'none',
                transition: 'all 0.12s',
                textAlign: 'left',
              }}
              onMouseEnter={(e) => {
                if (activeSection !== 'player') e.currentTarget.style.background = 'rgba(255,255,255,0.05)';
              }}
              onMouseLeave={(e) => {
                if (activeSection !== 'player') e.currentTarget.style.background = 'transparent';
              }}
            >
              <Film size={16} color={activeSection === 'player' ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} />
              watch-party
            </button>

            <button
              onClick={() => setActiveSection('members')}
              style={{
                width: '100%',
                padding: '5px 8px',
                borderRadius: '6px',
                background: activeSection === 'members' ? 'rgba(170,59,255,0.2)' : 'transparent',
                color: activeSection === 'members' ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                fontSize: '0.875rem',
                fontWeight: activeSection === 'members' ? 600 : 400,
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                cursor: 'pointer',
                border: 'none',
                transition: 'all 0.12s',
                textAlign: 'left',
                marginTop: '2px',
              }}
              onMouseEnter={(e) => {
                if (activeSection !== 'members') e.currentTarget.style.background = 'rgba(255,255,255,0.05)';
              }}
              onMouseLeave={(e) => {
                if (activeSection !== 'members') e.currentTarget.style.background = 'transparent';
              }}
            >
              <Users size={16} color={activeSection === 'members' ? 'var(--color-accent-purple)' : 'var(--color-text-muted)'} />
              members ({activeRoom?.members?.length || 1})
            </button>
          </div>
        </nav>

        {/* Current User / Leave */}
        <div
          style={{
            padding: '8px 10px',
            borderTop: '1px solid var(--color-border-glass)',
            background: 'rgba(5,7,10,0.5)',
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
            <div
              style={{
                width: '26px',
                height: '26px',
                borderRadius: '50%',
                background: permissions.isOwner ? 'rgba(170,59,255,0.3)' : 'rgba(0,240,255,0.2)',
                color: permissions.isOwner ? 'var(--color-accent-purple)' : 'var(--color-accent-cyan)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '0.7rem',
                fontWeight: 700,
              }}
            >
              {permissions.role?.charAt(0).toUpperCase()}
            </div>
            <span style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--color-text-secondary)' }}>
              {permissions.role}
            </span>
          </div>
          <button
            onClick={handleLeave}
            title="Leave Room"
            style={{
              width: '26px',
              height: '26px',
              borderRadius: '6px',
              background: 'rgba(244,63,94,0.15)',
              border: '1px solid rgba(244,63,94,0.3)',
              color: 'var(--color-accent-rose)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
              transition: 'all 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.background = 'rgba(244,63,94,0.3)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.background = 'rgba(244,63,94,0.15)';
            }}
          >
            <LogOut size={13} />
          </button>
        </div>
      </aside>

      {/* ── Main Content Area ──────────────────────────────────────────── */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
        {/* Top Channel Header */}
        <header
          style={{
            height: '40px',
            padding: '0 16px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderBottom: '1px solid var(--color-border-glass)',
            background: 'rgba(8,10,16,0.7)',
            flexShrink: 0,
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            {activeSection === 'chat' && (
              <>
                <Hash size={15} color="var(--color-text-muted)" />
                <span style={{ fontWeight: 600, fontSize: '0.88rem', color: 'var(--color-text-primary)' }}>general</span>
                <span style={{ color: 'var(--color-border-hover)', fontSize: '0.75rem' }}>│</span>
                <span style={{ fontSize: '0.78rem', color: 'var(--color-text-muted)' }}>Room chat for {activeRoom?.name}</span>
              </>
            )}
            {activeSection === 'player' && (
              <>
                <Film size={15} color="var(--color-text-muted)" />
                <span style={{ fontWeight: 600, fontSize: '0.88rem', color: 'var(--color-text-primary)' }}>watch-party</span>
                <span style={{ color: 'var(--color-border-hover)', fontSize: '0.75rem' }}>│</span>
                <span style={{ fontSize: '0.78rem', color: 'var(--color-text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '240px' }}>
                  {mediaUrl ? mediaUrl.split('/').pop() : 'No media selected'}
                </span>
              </>
            )}
            {activeSection === 'members' && (
              <>
                <Users size={15} color="var(--color-text-muted)" />
                <span style={{ fontWeight: 600, fontSize: '0.88rem', color: 'var(--color-text-primary)' }}>members</span>
              </>
            )}
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            {/* Quick nav pills */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
              <button
                onClick={() => setActiveSection('chat')}
                title="Chat"
                style={{
                  width: '28px',
                  height: '28px',
                  borderRadius: '6px',
                  background: activeSection === 'chat' ? 'rgba(170,59,255,0.2)' : 'transparent',
                  color: activeSection === 'chat' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                  border: 'none',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transition: 'all 0.12s',
                }}
              >
                <MessageSquare size={15} />
              </button>
              <button
                onClick={() => setActiveSection('player')}
                title="Watch Party Player"
                style={{
                  width: '28px',
                  height: '28px',
                  borderRadius: '6px',
                  background: activeSection === 'player' ? 'rgba(170,59,255,0.2)' : 'transparent',
                  color: activeSection === 'player' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                  border: 'none',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transition: 'all 0.12s',
                }}
              >
                <Tv size={15} />
              </button>
              <button
                onClick={() => setActiveSection('members')}
                title="Members"
                style={{
                  width: '28px',
                  height: '28px',
                  borderRadius: '6px',
                  background: activeSection === 'members' ? 'rgba(170,59,255,0.2)' : 'transparent',
                  color: activeSection === 'members' ? 'var(--color-accent-purple)' : 'var(--color-text-secondary)',
                  border: 'none',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  transition: 'all 0.12s',
                }}
              >
                <Users size={15} />
              </button>
            </div>
          </div>
        </header>

        {/* ── Content Body ──────────────────────────────────── */}
        <div style={{ flex: 1, display: 'flex', width: '100%', overflow: 'hidden', minHeight: 0, minWidth: 0 }}>
          {/* Player panel — takes center space when activeSection === 'player' */}
          <div
            style={{
              display: activeSection === 'player' ? 'flex' : 'none',
              flex: 1,
              width: '100%',
              flexDirection: 'column',
              overflow: 'hidden',
              padding: '12px',
              gap: '10px',
              minWidth: 0,
            }}
          >
            <div style={{ flex: 1, width: '100%', overflow: 'hidden', borderRadius: '10px', display: 'flex', minHeight: 0 }}>
              <WatchPartyPlayer onOpenLibrary={permissions.can_control_playback ? () => setIsLibraryOpen(true) : undefined} />
            </div>
            {/* Voice bar inside player section */}
            <div style={{ flexShrink: 0, width: '100%' }}>
              <VoiceChannelBar roomId={activeRoom?.id} />
            </div>
          </div>

          {/* Chat panel — either takes full screen in 'chat' mode OR right 380px sidebar when in 'player' mode */}
          <div
            style={{
              display: activeSection === 'chat' || activeSection === 'player' ? 'flex' : 'none',
              width: activeSection === 'player' ? '380px' : '100%',
              flex: activeSection === 'chat' ? 1 : 'none',
              flexShrink: 0,
              flexDirection: 'column',
              overflow: 'hidden',
              borderLeft: activeSection === 'player' ? '1px solid var(--color-border-glass)' : 'none',
            }}
          >
            <ChatPanel roomId={activeRoom?.id} />
          </div>

          {/* Members panel */}
          <div
            style={{
              display: activeSection === 'members' ? 'flex' : 'none',
              flex: 1,
              width: '100%',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
          >
            <MemberList />
          </div>
        </div>

        {/* Voice Bar — shown as persistent bottom bar when in chat/members view */}
        {activeSection !== 'player' && (
          <div style={{ flexShrink: 0, width: '100%', padding: '6px 12px 8px', borderTop: '1px solid var(--color-border-glass)', background: 'rgba(5,7,10,0.6)' }}>
            <VoiceChannelBar roomId={activeRoom?.id} />
          </div>
        )}
      </div>

      {/* Invisible Audio Renderer */}
      <AudioRenderer remoteStreams={rtc.remoteStreams} isDeafened={rtc.isDeafened} />

      {/* Media Library Picker Modal */}
      <MediaLibraryPicker
        isOpen={isLibraryOpen}
        onClose={() => setIsLibraryOpen(false)}
        currentUrl={mediaUrl}
        onSelectUrl={setMediaUrl}
      />
    </div>
  );
};
