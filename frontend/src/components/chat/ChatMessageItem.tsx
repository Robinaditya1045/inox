import React from 'react';
import type { ChatMessage } from '../../types/chat';

interface ChatMessageItemProps {
  message: ChatMessage;
  isOwn: boolean;
}

export const ChatMessageItem: React.FC<ChatMessageItemProps> = ({ message, isOwn }) => {
  const formatTime = (isoString: string): string => {
    try {
      const date = new Date(isoString);
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  };

  const getInitials = (name: string): string => {
    return name
      .split(' ')
      .map((part) => part[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: isOwn ? 'row-reverse' : 'row',
        alignItems: 'flex-start',
        gap: '10px',
        width: '100%',
        animation: 'fadeIn 0.25s ease-out',
      }}
    >
      {/* Avatar Icon */}
      <div
        style={{
          width: '32px',
          height: '32px',
          borderRadius: '10px',
          background: isOwn ? 'var(--color-accent-purple)' : 'var(--color-bg-surface-hover)',
          border: `1px solid ${isOwn ? 'rgba(255, 255, 255, 0.2)' : 'var(--color-border-glass)'}`,
          color: '#FFF',
          fontSize: '0.75rem',
          fontWeight: 700,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          boxShadow: isOwn ? '0 0 10px rgba(170, 59, 255, 0.3)' : 'none',
        }}
      >
        {getInitials(message.username || 'U')}
      </div>

      {/* Bubble Box */}
      <div
        style={{
          maxWidth: '78%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: isOwn ? 'flex-end' : 'flex-start',
          gap: '4px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '0 4px' }}>
          <span
            style={{
              fontSize: '0.75rem',
              fontWeight: 600,
              color: isOwn ? 'var(--color-accent-purple)' : 'var(--color-text-primary)',
            }}
          >
            {isOwn ? 'You' : message.username}
          </span>
          <span style={{ fontSize: '0.68rem', color: 'var(--color-text-muted)' }}>
            {formatTime(message.created_at)}
          </span>
        </div>

        <div
          style={{
            padding: '10px 14px',
            borderRadius: isOwn ? '14px 2px 14px 14px' : '2px 14px 14px 14px',
            background: isOwn
              ? 'linear-gradient(135deg, rgba(170, 59, 255, 0.25) 0%, rgba(139, 92, 246, 0.2) 100%)'
              : 'var(--color-bg-surface)',
            border: `1px solid ${isOwn ? 'rgba(170, 59, 255, 0.4)' : 'var(--color-border-glass)'}`,
            color: 'var(--color-text-primary)',
            fontSize: '0.9rem',
            lineHeight: 1.45,
            wordBreak: 'break-word',
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.3)',
          }}
        >
          {message.message}
        </div>
      </div>
    </div>
  );
};
