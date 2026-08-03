import React, { useState } from 'react';
import type { Room } from '../../types/room';
import { Hash, Film, Users, Copy, Check } from 'lucide-react';

interface RoomHeaderProps {
  activeRoom: Room | null;
  activePanel: 'chat' | 'members' | null;
  mediaUrl?: string;
}

export const RoomHeader: React.FC<RoomHeaderProps> = ({
  activeRoom,
  activePanel,
  mediaUrl,
}) => {
  const [copiedLink, setCopiedLink] = useState(false);

  const handleCopyInvite = () => {
    if (!activeRoom) return;
    const url = `${window.location.origin}/room/${activeRoom.id}`;
    navigator.clipboard.writeText(url).then(() => {
      setCopiedLink(true);
      setTimeout(() => setCopiedLink(false), 2500);
    }).catch(() => {});
  };

  return (
    <header
      style={{
        height: 'var(--header-height)',
        padding: '0 var(--space-4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: 'var(--color-canvas)',
        flexShrink: 0,
        gap: 'var(--space-3)',
      }}
    >
      {/* Current context label */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', overflow: 'hidden' }}>
        {activePanel === 'chat' && (
          <>
            <Hash size={14} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} aria-hidden="true" />
            <span style={{ fontWeight: 600, fontSize: 'var(--text-compact)', color: 'var(--color-text-primary)' }}>
              general
            </span>
            <span style={{ color: 'var(--color-border-default)', fontSize: 'var(--text-meta)' }} aria-hidden="true">│</span>
            <span style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {activeRoom?.name}
            </span>
          </>
        )}
        {activePanel === 'members' && (
          <>
            <Users size={14} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} aria-hidden="true" />
            <span style={{ fontWeight: 600, fontSize: 'var(--text-compact)', color: 'var(--color-text-primary)' }}>
              members
            </span>
          </>
        )}
        {activePanel === null && (
          <>
            <Film size={14} style={{ color: 'var(--color-text-muted)', flexShrink: 0 }} aria-hidden="true" />
            <span style={{ fontWeight: 600, fontSize: 'var(--text-compact)', color: 'var(--color-text-primary)' }}>
              watch-party
            </span>
            {mediaUrl && (
              <>
                <span style={{ color: 'var(--color-border-default)', fontSize: 'var(--text-meta)' }} aria-hidden="true">│</span>
                <span style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 200 }}>
                  {mediaUrl.split('/').pop() || mediaUrl}
                </span>
              </>
            )}
          </>
        )}
      </div>

      {/* Invite copy button */}
      <button
        onClick={handleCopyInvite}
        aria-label="Copy room invite link"
        title="Copy invite link"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 'var(--space-1)',
          padding: '4px var(--space-3)',
          borderRadius: 'var(--radius-md)',
          background: copiedLink ? 'var(--color-success-subtle)' : 'var(--color-surface-1)',
          border: `1px solid ${copiedLink ? 'var(--color-success-border)' : 'var(--color-border-default)'}`,
          color: copiedLink ? 'var(--color-success)' : 'var(--color-text-secondary)',
          fontSize: 'var(--text-meta)',
          fontWeight: 600,
          cursor: 'pointer',
          transition: 'all var(--transition-fast)',
          flexShrink: 0,
        }}
      >
        {copiedLink ? <Check size={13} /> : <Copy size={13} />}
        <span>{copiedLink ? 'Copied!' : 'Invite'}</span>
      </button>
    </header>
  );
};
