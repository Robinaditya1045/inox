import { useState, useEffect, useCallback } from 'react';
import { wsService, type WSStatus } from '../services/ws/ws.service';
import type { WSEventType, WSMessage } from '../types/ws';

export const useRoomSocket = (roomId?: string) => {
  const [status, setStatus] = useState<WSStatus>(wsService.getStatus());

  useEffect(() => {
    const unsubscribe = wsService.onStatusChange((newStatus) => {
      setStatus(newStatus);
    });
    return () => unsubscribe();
  }, []);

  // Encapsulate connection lifecycle if a roomId is provided
  useEffect(() => {
    if (roomId) {
      wsService.connect(roomId);
    }
    return () => {
      if (roomId) {
        // wsService.disconnect() is intentionally not called here to avoid 
        // disconnecting when the component unmounts but the user is still in the room.
        // Disconnection should happen on explicitly leaving the room.
      }
    };
  }, [roomId]);

  const send = useCallback(<T = unknown>(type: WSEventType, payload?: T, targetId?: string) => {
    wsService.send<T>(type, payload, targetId);
  }, []);

  const subscribe = useCallback((type: WSEventType | '*', callback: (msg: WSMessage<any>) => void) => {
    return wsService.on(type, callback);
  }, []);

  return {
    status,
    isConnected: status === 'OPEN',
    send,
    subscribe,
  };
};
