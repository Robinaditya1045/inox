import React from 'react';
import type { ChatMessage } from '../../types/chat';
import { Avatar } from '../common/Avatar';

interface MessageItemProps {
  message: ChatMessage;
  isOwn: boolean;
  /** If true, show avatar + username + timestamp. If false (grouped), show body only with left gutter. */
  isGroupLeader: boolean;
}

function formatTime(isoString: string): string {
  try {
    return new Date(isoString).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  } catch {
    return '';
  }
}

export const MessageItem: React.FC<MessageItemProps> = React.memo(
  ({ message, isOwn, isGroupLeader }) => {
    return (
      <div
        style={{
          display: 'flex',
          flexDirection: 'row',
          alignItems: 'flex-start',
          gap: 'var(--space-3)',
          paddingLeft: 'var(--space-4)',
          paddingRight: 'var(--space-4)',
          paddingTop: isGroupLeader ? 'var(--space-3)' : 2,
          paddingBottom: 0,
          width: '100%',
        }}
      >
        {/* Avatar gutter — always 36px wide for alignment, visible only on leader */}
        <div style={{ width: 36, flexShrink: 0, paddingTop: isGroupLeader ? 2 : 0 }}>
          {isGroupLeader && (
            <Avatar
              username={message.username || 'U'}
              size="sm"
              shape={isOwn ? 'square' : 'circle'}
            />
          )}
        </div>

        {/* Message content */}
        <div style={{ flex: 1, minWidth: 0 }}>
          {isGroupLeader && (
            <div
              style={{
                display: 'flex',
                alignItems: 'baseline',
                gap: 'var(--space-2)',
                marginBottom: 2,
              }}
            >
              <span
                style={{
                  fontSize: 'var(--text-compact)',
                  fontWeight: 600,
                  color: isOwn ? 'var(--color-accent)' : 'var(--color-text-primary)',
                  lineHeight: 1,
                }}
              >
                {isOwn ? 'You' : message.username}
              </span>
              <span style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)', lineHeight: 1 }}>
                {formatTime(message.created_at)}
              </span>
            </div>
          )}

          <p
            style={{
              fontSize: 'var(--text-compact)',
              lineHeight: 1.45,
              color: 'var(--color-text-primary)',
              wordBreak: 'break-word',
              margin: 0,
              whiteSpace: 'pre-wrap',
            }}
          >
            {message.message}
          </p>
        </div>
      </div>
    );
  }
);

MessageItem.displayName = 'MessageItem';

// Legacy export alias for backwards compatibility with ChatPanel
export { MessageItem as ChatMessageItem };
