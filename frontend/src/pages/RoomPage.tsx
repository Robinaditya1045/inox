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
import { RoomSidebar } from '../components/room/RoomSidebar';
import { RoomHeader } from '../components/room/RoomHeader';
import { useRTC } from '../hooks/useRTC';
import { usePlayerSync } from '../hooks/usePlayerSync';
import { useRoomSocket } from '../hooks/useRoomSocket';
import { Shield } from 'lucide-react';

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, isLoadingRoom, roomError } = useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [activeSection, setActiveSection] = useState<'chat' | 'members' | 'player'>('player');
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);

  const [isMobileViewport, setIsMobileViewport] = useState(window.innerWidth < 1024);

  // Initialize room WebSocket connection
  useRoomSocket(activeRoom?.id);

  const rtc = useRTC(activeRoom?.id);
  const { setMediaUrl, mediaUrl } = usePlayerSync();

  useEffect(() => {
    const handleResize = () => setIsMobileViewport(window.innerWidth < 1024);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);


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
      <RoomSidebar
        activeRoom={activeRoom}
        permissions={permissions}
        activeSection={activeSection}
        setActiveSection={setActiveSection}
        onLeave={handleLeave}
      />

      {/* ── Main Content Area ──────────────────────────────────────────── */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
        {/* Top Channel Header */}
        <RoomHeader
          activeSection={activeSection}
          setActiveSection={setActiveSection}
          activeRoom={activeRoom}
          mediaUrl={mediaUrl}
        />

        {/* ── Content Body (Responsive Multi-Panel Layout) ──────── */}
        <div style={{ flex: 1, display: 'flex', flexDirection: isMobileViewport ? 'column' : 'row', width: '100%', overflow: 'hidden', minHeight: 0, minWidth: 0 }}>
          {/* Main Video Player Panel */}
          <div
            style={{
              display: !isMobileViewport || activeSection === 'player' || activeSection === 'chat' || activeSection === 'members' ? 'flex' : 'none',
              flex: !isMobileViewport && activeSection !== 'player' ? 1 : isMobileViewport && activeSection !== 'player' ? 'none' : 1,
              height: isMobileViewport && activeSection !== 'player' ? '240px' : '100%',
              width: '100%',
              flexDirection: 'column',
              overflow: 'hidden',
              padding: '12px',
              gap: '10px',
              minWidth: 0,
              flexShrink: isMobileViewport ? 0 : 1,
            }}
          >
            <div style={{ flex: 1, width: '100%', overflow: 'hidden', borderRadius: '10px', display: 'flex', minHeight: 0 }}>
              <WatchPartyPlayer onOpenLibrary={permissions.can_control_playback ? () => setIsLibraryOpen(true) : undefined} />
            </div>
            {/* Voice bar below video player */}
            <div style={{ flexShrink: 0, width: '100%' }}>
              <VoiceChannelBar roomId={activeRoom?.id} />
            </div>
          </div>

          {/* Side / Stacked Panel (Chat or Members) */}
          {activeSection !== 'player' && (
            <div
              style={{
                width: isMobileViewport ? '100%' : '380px',
                flex: isMobileViewport ? 1 : 'none',
                flexShrink: 0,
                display: 'flex',
                flexDirection: 'column',
                overflow: 'hidden',
                borderLeft: !isMobileViewport ? '1px solid var(--color-border-glass)' : 'none',
                borderTop: isMobileViewport ? '1px solid var(--color-border-glass)' : 'none',
                background: 'var(--color-bg-obsidian)',
              }}
            >
              {activeSection === 'chat' ? (
                <ChatPanel roomId={activeRoom?.id} />
              ) : (
                <MemberList />
              )}
            </div>
          )}
        </div>
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
