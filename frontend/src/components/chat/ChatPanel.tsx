import React, { useState, useRef, useEffect } from 'react';
import { useChat } from '../../hooks/useChat';
import { useAuth } from '../../hooks/useAuth';
import { usePermissions } from '../../hooks/usePermissions';
import { MessageItem } from './ChatMessageItem';
import { Spinner } from '../common/Spinner';
import { Send, Lock, MessageSquare, ArrowDown } from 'lucide-react';
import type { ChatMessage } from '../../types/chat';

interface ChatPanelProps {
  roomId: string | undefined;
}

/** Groups consecutive messages by the same user within a 5-minute window. */
function groupMessages(messages: ChatMessage[]): Array<{ message: ChatMessage; isLeader: boolean }> {
  const GROUPING_THRESHOLD_MS = 5 * 60 * 1000; // 5 minutes

  return messages.map((msg, i) => {
    if (i === 0) return { message: msg, isLeader: true };

    const prev = messages[i - 1];
    const sameUser = prev.user_id === msg.user_id;
    const withinWindow =
      Math.abs(
        new Date(msg.created_at).getTime() - new Date(prev.created_at).getTime()
      ) < GROUPING_THRESHOLD_MS;

    return { message: msg, isLeader: !(sameUser && withinWindow) };
  });
}

export const ChatPanel: React.FC<ChatPanelProps> = ({ roomId }) => {
  const [inputText, setInputText] = useState('');
  const [showScrollBottom, setShowScrollBottom] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const feedRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  const { messages, isLoadingHistory, sendMessage } = useChat(roomId);
  const { user } = useAuth();
  const permissions = usePermissions();

  const groupedMessages = groupMessages(messages);

  const scrollToBottom = (behavior: ScrollBehavior = 'smooth') => {
    messagesEndRef.current?.scrollIntoView({ behavior });
    setShowScrollBottom(false);
  };

  useEffect(() => {
    if (!showScrollBottom) {
      scrollToBottom('auto');
    }
  }, [messages.length]);

  const handleScroll = () => {
    if (!feedRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = feedRef.current;
    setShowScrollBottom(scrollHeight - scrollTop - clientHeight > 80);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || !permissions.can_send_messages) return;
    sendMessage(inputText);
    setInputText('');
    // Reset textarea height
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
    scrollToBottom('smooth');
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e as any);
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        width: '100%',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          height: 'var(--header-height)',
          padding: '0 var(--space-4)',
          display: 'flex',
          alignItems: 'center',
          borderBottom: '1px solid var(--color-border-subtle)',
          flexShrink: 0,
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 'var(--text-compact)', color: 'var(--color-text-primary)' }}>
          Chat
        </span>
      </div>

      {/* Message Feed */}
      <div
        ref={feedRef}
        onScroll={handleScroll}
        style={{
          flex: 1,
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column',
          paddingTop: 'var(--space-2)',
          paddingBottom: 'var(--space-2)',
        }}
      >
        {isLoadingHistory ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              gap: 'var(--space-3)',
            }}
          >
            <Spinner size={20} />
            <span style={{ fontSize: 'var(--text-meta)', color: 'var(--color-text-muted)' }}>
              Loading messages…
            </span>
          </div>
        ) : messages.length === 0 ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              height: '100%',
              gap: 'var(--space-3)',
              opacity: 0.6,
              textAlign: 'center',
              padding: 'var(--space-6)',
            }}
          >
            <MessageSquare size={22} style={{ color: 'var(--color-text-muted)' }} />
            <span style={{ fontSize: 'var(--text-compact)', color: 'var(--color-text-secondary)' }}>
              No messages yet — say hello!
            </span>
          </div>
        ) : (
          groupedMessages.map(({ message, isLeader }) => (
            <MessageItem
              key={message.id}
              message={message}
              isOwn={message.user_id === user?.id}
              isGroupLeader={isLeader}
            />
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Scroll to Bottom */}
      {showScrollBottom && (
        <button
          onClick={() => scrollToBottom('smooth')}
          aria-label="Scroll to new messages"
          style={{
            position: 'absolute',
            bottom: 80,
            left: '50%',
            transform: 'translateX(-50%)',
            padding: '5px var(--space-3)',
            borderRadius: 'var(--radius-full)',
            background: 'var(--color-accent)',
            color: 'var(--color-text-on-accent)',
            fontSize: 'var(--text-meta)',
            fontWeight: 600,
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-1)',
            border: 'none',
            boxShadow: 'var(--shadow-menu)',
            cursor: 'pointer',
            zIndex: 10,
            transition: 'opacity var(--transition-fast)',
          }}
        >
          <ArrowDown size={13} />
          New messages
        </button>
      )}

      {/* Composer */}
      <div
        style={{
          padding: 'var(--space-3) var(--space-4)',
          borderTop: '1px solid var(--color-border-subtle)',
          background: 'var(--color-canvas)',
        }}
      >
        {!permissions.can_send_messages ? (
          <div
            style={{
              padding: 'var(--space-3)',
              borderRadius: 'var(--radius-lg)',
              background: 'var(--color-surface-1)',
              border: '1px solid var(--color-border-default)',
              color: 'var(--color-text-muted)',
              fontSize: 'var(--text-meta)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 'var(--space-2)',
            }}
          >
            <Lock size={13} aria-hidden="true" />
            <span>Messaging restricted</span>
          </div>
        ) : (
          <form
            onSubmit={handleSubmit}
            style={{ display: 'flex', alignItems: 'flex-end', gap: 'var(--space-2)' }}
          >
            <textarea
              ref={textareaRef}
              value={inputText}
              onChange={(e) => {
                setInputText(e.target.value);
                e.target.style.height = 'auto';
                e.target.style.height = `${Math.min(e.target.scrollHeight, 120)}px`;
              }}
              onKeyDown={handleKeyDown}
              placeholder="Message #general"
              rows={1}
              aria-label="Message input"
              style={{
                flex: 1,
                padding: '8px var(--space-3)',
                borderRadius: 'var(--radius-lg)',
                background: 'var(--color-surface-1)',
                border: '1px solid var(--color-border-default)',
                color: 'var(--color-text-primary)',
                fontSize: 'var(--text-compact)',
                lineHeight: 1.4,
                resize: 'none',
                outline: 'none',
                maxHeight: '120px',
                overflowY: 'auto',
                fontFamily: 'inherit',
                transition: 'border-color var(--transition-fast)',
              }}
              onFocus={(e) => { e.currentTarget.style.borderColor = 'var(--color-accent-border)'; }}
              onBlur={(e) => { e.currentTarget.style.borderColor = 'var(--color-border-default)'; }}
            />
            <button
              type="submit"
              disabled={!inputText.trim()}
              aria-label="Send message"
              style={{
                width: 36,
                height: 36,
                borderRadius: 'var(--radius-lg)',
                background: inputText.trim() ? 'var(--color-accent)' : 'var(--color-surface-1)',
                color: inputText.trim() ? 'var(--color-text-on-accent)' : 'var(--color-text-muted)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid transparent',
                cursor: inputText.trim() ? 'pointer' : 'not-allowed',
                transition: 'all var(--transition-fast)',
                flexShrink: 0,
              }}
            >
              <Send size={15} />
            </button>
          </form>
        )}
      </div>
    </div>
  );
};
