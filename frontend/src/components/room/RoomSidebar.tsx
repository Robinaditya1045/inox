import React from 'react';
import type { Room } from '../../types/room';
import { usePresence } from '../../hooks/usePresence';
import { Tv, Lock, Globe, Hash, Volume2, Users, Film } from 'lucide-react';

interface RoomSidebarProps {
  activeRoom: Room | null;
  activePanel: 'chat' | 'members' | null;
  isPlayerFullscreen?: boolean;
}

const navItemStyle = (isActive: boolean): React.CSSProperties => ({
  width: '100%',
  padding: '5px var(--space-2)',
  borderRadius: 'var(--radius-md)',
  background: isActive ? 'var(--color-surface-2)' : 'transparent',
  color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
  fontSize: 'var(--text-compact)',
  fontWeight: isActive ? 600 : 400,
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--space-2)',
  cursor: 'default',
  border: 'none',
  transition: 'background-color var(--transition-fast), color var(--transition-fast)',
  textAlign: 'left' as const,
});

const sectionLabelStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 'var(--space-1)',
  padding: '4px var(--space-2)',
  color: 'var(--color-text-muted)',
  fontSize: 'var(--text-label)',
  fontWeight: 700,
  textTransform: 'uppercase',
  letterSpacing: '0.07em',
  userSelect: 'none',
};

export const RoomSidebar: React.FC<RoomSidebarProps> = ({ activeRoom, activePanel }) => {
  const { members } = usePresence();

  return (
    <aside
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        overflow: 'hidden',
      }}
    >
      {/* Room Identity Header */}
      <div
        style={{
          height: 'var(--header-height)',
          padding: '0 var(--space-4)',
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-2)',
          borderBottom: '1px solid var(--color-border-subtle)',
          flexShrink: 0,
        }}
      >
        <Tv size={14} style={{ color: 'var(--color-accent)', flexShrink: 0 }} aria-hidden="true" />
        <span
          style={{
            flex: 1,
            fontWeight: 700,
            fontSize: 'var(--text-compact)',
            color: 'var(--color-text-primary)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {activeRoom?.name ?? 'Room'}
        </span>
        {activeRoom?.is_private ? (
          <Lock size={12} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} aria-label="Private room" />
        ) : (
          <Globe size={12} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} aria-label="Public room" />
        )}
      </div>

      {/* Channel Nav */}
      <nav
        style={{ flex: 1, overflowY: 'auto', padding: 'var(--space-3) var(--space-2)' }}
        aria-label="Room channels"
      >
        {/* Text Channels */}
        <div style={{ marginBottom: 'var(--space-3)' }}>
          <div style={sectionLabelStyle}>Text</div>
          <div
            style={{
              ...navItemStyle(activePanel === 'chat'),
              cursor: 'default',
            }}
          >
            <Hash size={14} style={{ color: activePanel === 'chat' ? 'var(--color-accent)' : 'var(--color-text-muted)' }} aria-hidden="true" />
            <span>general</span>
          </div>
        </div>

        {/* Voice */}
        <div style={{ marginBottom: 'var(--space-3)' }}>
          <div style={sectionLabelStyle}>Voice</div>
          <div style={navItemStyle(false)}>
            <Volume2 size={14} style={{ color: 'var(--color-text-muted)' }} aria-hidden="true" />
            <span>voice</span>
          </div>
        </div>

        {/* Watch Party */}
        <div>
          <div style={sectionLabelStyle}>Watch Party</div>
          <div style={navItemStyle(activePanel === null)}>
            <Film size={14} style={{ color: activePanel === null ? 'var(--color-accent)' : 'var(--color-text-muted)' }} aria-hidden="true" />
            <span>watch-party</span>
          </div>
          <div style={{ ...navItemStyle(activePanel === 'members'), marginTop: 2 }}>
            <Users size={14} style={{ color: activePanel === 'members' ? 'var(--color-accent)' : 'var(--color-text-muted)' }} aria-hidden="true" />
            <span>
              members
              {members.length > 0 && (
                <span style={{ color: 'var(--color-text-muted)', fontWeight: 400 }}>
                  {' '}({members.length})
                </span>
              )}
            </span>
          </div>
        </div>
      </nav>
    </aside>
  );
};
