import { createContext } from 'react';
import type { WSEventType, WSMessage } from '../types/ws';
import type { WSStatus } from '../services/ws/ws.service';

export interface WSContextValue {
  status: WSStatus;
  isConnected: boolean;
  send: <T = unknown>(type: WSEventType, payload?: T, targetId?: string) => void;
  subscribe: (type: WSEventType | '*', callback: (msg: WSMessage<any>) => void) => () => void;
}

export const WSContext = createContext<WSContextValue | null>(null);
