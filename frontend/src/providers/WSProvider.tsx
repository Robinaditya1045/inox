import React, { useState, useEffect, useCallback, type ReactNode } from 'react';
import { WSContext } from '../contexts/ws.context';
import { wsService, type WSStatus } from '../services/ws/ws.service';
import type { WSEventType, WSMessage } from '../types/ws';
import { useRoom } from '../hooks/useRoom';

interface WSProviderProps {
  children: ReactNode;
}

export const WSProvider: React.FC<WSProviderProps> = ({ children }) => {
  const [status, setStatus] = useState<WSStatus>(wsService.getStatus());
  const { activeRoom } = useRoom();

  useEffect(() => {
    const unsubscribe = wsService.onStatusChange((newStatus) => {
      setStatus(newStatus);
    });
    return () => unsubscribe();
  }, []);

  useEffect(() => {
    if (activeRoom?.id) {
      wsService.connect(activeRoom.id);
    } else {
      wsService.disconnect();
    }
    return () => {
      // Don't disconnect on unmount if still in room, but clean up on room leave
    };
  }, [activeRoom?.id]);

  const send = useCallback(<T = unknown>(type: WSEventType, payload?: T, targetId?: string) => {
    wsService.send<T>(type, payload, targetId);
  }, []);

  const subscribe = useCallback((type: WSEventType | '*', callback: (msg: WSMessage<any>) => void) => {
    return wsService.on(type, callback);
  }, []);

  const value = {
    status,
    isConnected: status === 'OPEN',
    send,
    subscribe,
  };

  return <WSContext.Provider value={value}>{children}</WSContext.Provider>;
};
