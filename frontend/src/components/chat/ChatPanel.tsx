import React, { useState, useRef, useEffect } from 'react';
import { useChat } from '../../hooks/useChat';
import { useAuth } from '../../hooks/useAuth';
import { usePermissions } from '../../hooks/usePermissions';
import { ChatMessageItem } from './ChatMessageItem';
import { Spinner } from '../common/Spinner';
import { Send, Lock, MessageSquare, ArrowDown } from 'lucide-react';

interface ChatPanelProps {
  roomId: string | undefined;
}

export const ChatPanel: React.FC<ChatPanelProps> = ({ roomId }) => {
  const [inputText, setInputText] = useState('');
  const [showScrollBottom, setShowScrollBottom] = useState(false);
  
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const feedRef = useRef<HTMLDivElement | null>(null);

  const { messages, isLoadingHistory, sendMessage } = useChat(roomId);
  const { user } = useAuth();
  const permissions = usePermissions();

  const scrollToBottom = (behavior: ScrollBehavior = 'smooth') => {
    messagesEndRef.current?.scrollIntoView({ behavior });
    setShowScrollBottom(false);
  };

  // Auto-scroll on new message unless scrolled up
  useEffect(() => {
    if (!showScrollBottom) {
      scrollToBottom('auto');
    }
  }, [messages.length]);

  const handleScroll = () => {
    if (!feedRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = feedRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 60;
    setShowScrollBottom(!isAtBottom);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || !permissions.can_send_messages) return;
    sendMessage(inputText);
    setInputText('');
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
      {/* Message Feed Container */}
      <div
        ref={feedRef}
        onScroll={handleScroll}
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '16px',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
      >
        {isLoadingHistory ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '12px' }}>
            <Spinner size={24} color="var(--color-accent-purple)" />
            <span style={{ fontSize: '0.85rem', color: 'var(--color-text-secondary)' }}>Loading message history...</span>
          </div>
        ) : messages.length === 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '100%', gap: '12px', opacity: 0.6, textAlign: 'center', padding: '20px' }}>
            <div style={{ width: '48px', height: '48px', borderRadius: '16px', background: 'var(--color-bg-surface)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <MessageSquare size={24} color="var(--color-text-muted)" />
            </div>
            <span style={{ fontSize: '0.9rem', color: 'var(--color-text-secondary)' }}>No messages yet. Be the first to start the conversation!</span>
          </div>
        ) : (
          messages.map((msg) => (
            <ChatMessageItem
              key={msg.id}
              message={msg}
              isOwn={msg.user_id === user?.id}
            />
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Floating Scroll-to-Bottom Pill */}
      {showScrollBottom && (
        <button
          onClick={() => scrollToBottom('smooth')}
          style={{
            position: 'absolute',
            bottom: '80px',
            left: '50%',
            transform: 'translateX(-50%)',
            padding: '6px 14px',
            borderRadius: '20px',
            background: 'var(--color-accent-purple)',
            color: '#FFF',
            fontSize: '0.75rem',
            fontWeight: 600,
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            border: 'none',
            boxShadow: '0 4px 15px rgba(170, 59, 255, 0.5)',
            cursor: 'pointer',
            zIndex: 10,
            transition: 'all var(--transition-fast)',
          }}
        >
          <span>New messages</span>
          <ArrowDown size={14} />
        </button>
      )}

      {/* Bottom Message Input Area */}
      <div
        style={{
          padding: '16px',
          borderTop: '1px solid var(--color-border-glass)',
          background: 'rgba(5, 7, 10, 0.4)',
        }}
      >
        {!permissions.can_send_messages ? (
          <div
            style={{
              padding: '12px',
              borderRadius: '12px',
              background: 'rgba(244, 63, 94, 0.1)',
              border: '1px solid rgba(244, 63, 94, 0.3)',
              color: 'var(--color-accent-rose)',
              fontSize: '0.8rem',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '8px',
            }}
          >
            <Lock size={16} />
            <span>Text chat is restricted or muted by moderators</span>
          </div>
        ) : (
          <form onSubmit={handleSubmit} style={{ display: 'flex', alignItems: 'flex-end', gap: '8px' }}>
            <textarea
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a message..."
              rows={1}
              style={{
                flex: 1,
                padding: '10px 14px',
                borderRadius: '12px',
                background: 'var(--color-bg-surface)',
                border: '1px solid var(--color-border-glass)',
                color: 'var(--color-text-primary)',
                fontSize: '0.9rem',
                lineHeight: 1.4,
                resize: 'none',
                outline: 'none',
                maxHeight: '100px',
                fontFamily: 'inherit',
                transition: 'border-color var(--transition-fast)',
              }}
            />
            <button
              type="submit"
              disabled={!inputText.trim()}
              style={{
                width: '40px',
                height: '40px',
                borderRadius: '12px',
                background: inputText.trim() ? 'var(--color-accent-purple)' : 'var(--color-bg-surface-hover)',
                color: inputText.trim() ? '#FFF' : 'var(--color-text-muted)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                border: 'none',
                cursor: inputText.trim() ? 'pointer' : 'not-allowed',
                transition: 'all var(--transition-fast)',
                flexShrink: 0,
                boxShadow: inputText.trim() ? '0 0 12px rgba(170, 59, 255, 0.4)' : 'none',
              }}
            >
              <Send size={18} />
            </button>
          </form>
        )}
      </div>
    </div>
  );
};
