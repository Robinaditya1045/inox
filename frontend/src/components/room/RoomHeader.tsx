import React, { useState } from 'react';
import type { Room } from '../../types/room';
import {
  Hash,
  Film,
  Users,
  Copy,
  Check,
  MessageSquare,
  Tv,
} from 'lucide-react';

interface RoomHeaderProps {
  activeSection: 'chat' | 'members' | 'player';
  setActiveSection: (section: 'chat' | 'members' | 'player') => void;
  activeRoom: Room | null;
  mediaUrl: string | null;
}

export const RoomHeader: React.FC<RoomHeaderProps> = ({
  activeSection,
  setActiveSection,
  activeRoom,
  mediaUrl,
}) => {
  const [copiedLink, setCopiedLink] = useState(false);

  const handleCopyInvite = () => {
    if (!activeRoom) return;
    const inviteUrl = `${window.location.origin}/room/${activeRoom.id}`;
    navigator.clipboard.writeText(inviteUrl).then(() => {
      setCopiedLink(true);
      setTimeout(() => setCopiedLink(false), 2500);
    }).catch(() => {});
  };

  return (
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

      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
        <button
          onClick={handleCopyInvite}
          aria-label="Copy Room Invite Link"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            padding: '5px 12px',
            borderRadius: '6px',
            background: copiedLink ? 'rgba(16, 185, 129, 0.2)' : 'var(--color-bg-surface)',
            border: `1px solid ${copiedLink ? 'rgba(16, 185, 129, 0.5)' : 'var(--color-border-glass)'}`,
            color: copiedLink ? 'var(--color-accent-emerald)' : 'var(--color-text-secondary)',
            fontSize: '0.78rem',
            fontWeight: 600,
            cursor: 'pointer',
            transition: 'all 0.15s',
          }}
        >
          {copiedLink ? <Check size={14} /> : <Copy size={14} />}
          <span>{copiedLink ? 'Copied Link!' : 'Invite'}</span>
        </button>

        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', background: 'var(--color-bg-surface)', padding: '3px', borderRadius: '8px', border: '1px solid var(--color-border-glass)' }}>
          <button
            onClick={() => setActiveSection('chat')}
            title="Chat Panel"
            aria-label="Switch to Chat Panel"
            style={{
              padding: '5px 10px',
              borderRadius: '6px',
              background: activeSection === 'chat' ? 'var(--color-accent-purple)' : 'transparent',
              color: activeSection === 'chat' ? '#FFF' : 'var(--color-text-secondary)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: '0.75rem',
              fontWeight: 600,
              transition: 'all 0.15s',
            }}
          >
            <MessageSquare size={14} />
            <span>Chat</span>
          </button>
          <button
            onClick={() => setActiveSection('player')}
            title="Theater / Full Player"
            aria-label="Switch to Theater Player"
            style={{
              padding: '5px 10px',
              borderRadius: '6px',
              background: activeSection === 'player' ? 'var(--color-accent-purple)' : 'transparent',
              color: activeSection === 'player' ? '#FFF' : 'var(--color-text-secondary)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: '0.75rem',
              fontWeight: 600,
              transition: 'all 0.15s',
            }}
          >
            <Tv size={14} />
            <span>Theater</span>
          </button>
          <button
            onClick={() => setActiveSection('members')}
            title="Members Panel"
            aria-label="Switch to Members Panel"
            style={{
              padding: '5px 10px',
              borderRadius: '6px',
              background: activeSection === 'members' ? 'var(--color-accent-purple)' : 'transparent',
              color: activeSection === 'members' ? '#FFF' : 'var(--color-text-secondary)',
              border: 'none',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: '0.75rem',
              fontWeight: 600,
              transition: 'all 0.15s',
            }}
          >
            <Users size={14} />
            <span>Members</span>
          </button>
        </div>
      </div>
    </header>
  );
};
