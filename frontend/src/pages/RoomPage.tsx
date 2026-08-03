import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useRoom } from '../hooks/useRoom';
import { usePermissions } from '../hooks/usePermissions';
import { useRTC } from '../hooks/useRTC';
import { usePlayerSync } from '../hooks/usePlayerSync';
import { useRoomSocket } from '../hooks/useRoomSocket';
import { Button } from '../components/common/Button';
import { Spinner } from '../components/common/Spinner';
import { WatchPartyPlayer } from '../components/player/WatchPartyPlayer';
import { MediaLibraryPicker } from '../components/player/MediaLibraryPicker';
import { ChatPanel } from '../components/chat/ChatPanel';
import { MemberList } from '../components/room/MemberList';
import { AudioRenderer } from '../components/rtc/AudioRenderer';
import { RoomSidebar } from '../components/room/RoomSidebar';
import { RoomHeader } from '../components/room/RoomHeader';
import { CallControlBar } from '../components/room/CallControlBar';
import { Shield } from 'lucide-react';
import shellStyles from '../components/room/VideoRoomShell.module.css';

type ActivePanel = 'chat' | 'members' | null;

export const RoomPage: React.FC = () => {
  const { roomId } = useParams<{ roomId: string }>();
  const { activeRoom, joinRoom, leaveRoom, isLoadingRoom, roomError } = useRoom();
  const permissions = usePermissions();
  const navigate = useNavigate();

  const [activePanel, setActivePanel] = useState<ActivePanel>(null);
  const [isLibraryOpen, setIsLibraryOpen] = useState(false);

  // Initialize room WebSocket connection
  useRoomSocket(activeRoom?.id);

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

  const handleTogglePanel = (panel: 'chat' | 'members') => {
    setActivePanel((prev) => (prev === panel ? null : panel));
  };

  // ── Loading / error states ────────────────────────────────────────────
  if (isLoadingRoom && !activeRoom) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <Spinner size={28} />
        <span style={{ color: 'var(--color-text-secondary)', fontSize: 'var(--text-compact)' }}>
          Joining room…
        </span>
      </div>
    );
  }

  if (roomError || (!activeRoom && !isLoadingRoom)) {
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100%',
          flexDirection: 'column',
          gap: 'var(--space-4)',
          padding: 'var(--space-8)',
          textAlign: 'center',
        }}
      >
        <div
          style={{
            width: 44,
            height: 44,
            borderRadius: '50%',
            background: 'var(--color-danger-subtle)',
            color: 'var(--color-danger)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <Shield size={22} />
        </div>
        <div>
          <h2
            style={{
              fontSize: 'var(--text-heading)',
              fontWeight: 700,
              color: 'var(--color-text-primary)',
              marginBottom: 'var(--space-2)',
            }}
          >
            Room not accessible
          </h2>
          <p style={{ color: 'var(--color-text-secondary)', maxWidth: 360, fontSize: 'var(--text-compact)' }}>
            {roomError || 'This room does not exist or you lack permission to join.'}
          </p>
        </div>
        <Button variant="secondary" onClick={() => navigate('/')}>
          Return to Lobby
        </Button>
      </div>
    );
  }

  // ── Main room view ─────────────────────────────────────────────────────
  return (
    <>
      {/* aria-live region for voice events */}
      <div id="room-announcer" aria-live="polite" aria-atomic="false" className="sr-only" />

      <div
        className={`${shellStyles.shell} ${activePanel ? shellStyles.shellPanelOpen : ''}`}
      >
        {/* Left Room Sidebar */}
        <div className={shellStyles.roomSidebarArea}>
          <RoomSidebar
            activeRoom={activeRoom}
            activePanel={activePanel}
          />
        </div>

        {/* Main Workspace (player always visible) */}
        <div className={shellStyles.workspaceArea}>
          <RoomHeader
            activeRoom={activeRoom}
            activePanel={activePanel}
            mediaUrl={mediaUrl}
          />

          {/* Player — takes remaining height */}
          <div style={{ flex: 1, overflow: 'hidden', padding: 'var(--space-3)', minHeight: 0 }}>
            <WatchPartyPlayer
              onOpenLibrary={
                permissions.can_control_playback
                  ? () => setIsLibraryOpen(true)
                  : undefined
              }
            />
          </div>
        </div>

        {/* Right Context Panel — chat or members */}
        {activePanel && (
          <div className={shellStyles.panelArea}>
            {activePanel === 'chat' ? (
              <ChatPanel roomId={activeRoom?.id} />
            ) : (
              <MemberList />
            )}
          </div>
        )}

        {/* Call Controls — persistent bottom bar */}
        <div className={shellStyles.callBarArea}>
          <CallControlBar
            roomId={activeRoom?.id}
            onLeave={handleLeave}
            activePanel={activePanel}
            onTogglePanel={handleTogglePanel}
          />
        </div>
      </div>

      {/* Invisible Audio Renderer */}
      <AudioRenderer remoteStreams={rtc.remoteStreams} isDeafened={rtc.isDeafened} />

      {/* Media Library Modal */}
      <MediaLibraryPicker
        isOpen={isLibraryOpen}
        onClose={() => setIsLibraryOpen(false)}
        currentUrl={mediaUrl}
        onSelectUrl={setMediaUrl}
      />
    </>
  );
};
