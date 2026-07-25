import React from 'react';
import type { Room } from '../../types/room';
import {
  Tv,
  Lock,
  Globe,
  ChevronDown,
  Hash,
  Volume2,
  Film,
  Users,
  LogOut,
} from 'lucide-react';

interface RoomSidebarProps {
  activeRoom: Room | null;
  permissions: { role?: string | null; isOwner: boolean };
  activeSection: 'chat' | 'members' | 'player';
  setActiveSection: (section: 'chat' | 'members' | 'player') => void;
  onLeave: () => void;
}

export const RoomSidebar: React.FC<RoomSidebarProps> = ({
  activeRoom,
  permissions,
  activeSection,
  setActiveSection,
  onLeave,
}) => {
  return (
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

      <nav style={{ flex: 1, overflowY: 'auto', padding: '8px 6px' }}>
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
          onClick={onLeave}
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
  );
};
