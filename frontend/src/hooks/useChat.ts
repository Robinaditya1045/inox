import { useState, useEffect, useCallback } from 'react';
import { useWS } from './useWS';
import { usePermissions } from './usePermissions';
import { chatService } from '../services/chat/chat.service';
import type { ChatMessage } from '../types/chat';
import type { WSChatPayload } from '../types/ws';
import { logger } from '../utils/logger';

export interface UseChatReturn {
  messages: ChatMessage[];
  isLoadingHistory: boolean;
  error: string | null;
  sendMessage: (text: string) => void;
}

export const useChat = (roomId: string | undefined): UseChatReturn => {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isLoadingHistory, setIsLoadingHistory] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const { send, subscribe } = useWS();
  const permissions = usePermissions();

  // Load historical messages via REST API when room changes
  useEffect(() => {
    if (!roomId) {
      setMessages([]);
      return;
    }

    let isMounted = true;
    setIsLoadingHistory(true);
    setError(null);

    chatService
      .getRecentMessages(roomId, 50)
      .then((history) => {
        if (isMounted) {
          setMessages(history);
          setIsLoadingHistory(false);
          logger.info('useChat: Historical messages loaded', { count: history.length });
        }
      })
      .catch((err) => {
        if (isMounted) {
          setError('Failed to load chat history');
          setIsLoadingHistory(false);
          logger.error('useChat: History fetch failed', { err });
        }
      });

    return () => {
      isMounted = false;
    };
  }, [roomId]);

  // Subscribe to real-time incoming CHAT_MESSAGE WebSocket frames
  useEffect(() => {
    if (!roomId) return;

    const unsubscribe = subscribe('CHAT_MESSAGE', (msg) => {
      const payload = msg.payload as WSChatPayload;
      if (payload && payload.message) {
        const newBubble: ChatMessage = {
          id: payload.message_id || `ws-${Date.now()}-${Math.random().toString(36).substring(2, 9)}`,
          room_id: msg.room_id || roomId,
          user_id: msg.sender_id || 'unknown',
          username: msg.sender_name || 'Participant',
          message: payload.message,
          created_at: new Date(msg.timestamp || Date.now()).toISOString(),
        };

        setMessages((prev) => {
          // Avoid duplicate insertion if ID matches existing message
          if (prev.some((m) => m.id === newBubble.id)) {
            return prev;
          }
          return [...prev, newBubble];
        });
        logger.debug('useChat: Appended live chat frame', { sender: newBubble.username });
      }
    });

    return () => {
      unsubscribe();
    };
  }, [roomId, subscribe]);

  const sendMessage = useCallback((text: string) => {
    if (!text.trim()) return;

    if (!permissions.can_send_messages) {
      logger.warn('useChat: Permission denied to send chat message');
      return;
    }

    const payload: WSChatPayload = {
      message: text.trim(),
    };

    send('CHAT_MESSAGE', payload);
    logger.debug('useChat: Emitted CHAT_MESSAGE frame', { text: text.trim() });
  }, [permissions.can_send_messages, send]);

  return {
    messages,
    isLoadingHistory,
    error,
    sendMessage,
  };
};
