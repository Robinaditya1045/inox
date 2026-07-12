import type { WSEventType, WSMessage } from '../../types/ws';
import { logger } from '../../utils/logger';

export type WSStatus = 'CONNECTING' | 'OPEN' | 'CLOSED';
type WSCallback = (msg: WSMessage<any>) => void;

class WSService {
  private socket: WebSocket | null = null;
  private status: WSStatus = 'CLOSED';
  private roomId: string | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private listeners: Map<WSEventType | '*', Set<WSCallback>> = new Map();
  private statusListeners: Set<(status: WSStatus) => void> = new Set();

  connect(roomId: string): void {
    if (this.socket && this.roomId === roomId && (this.status === 'OPEN' || this.status === 'CONNECTING')) {
      logger.debug('WSService: Already connected or connecting to room', { roomId });
      return;
    }

    this.disconnect();
    this.roomId = roomId;
    this.reconnectAttempts = 0;
    this.initiateConnection();
  }

  private initiateConnection(): void {
    if (!this.roomId) return;

    this.setStatus('CONNECTING');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Connect to backend WebSocket endpoint
    let wsBase = import.meta.env.VITE_WS_BASE_URL || `${protocol}//localhost:8080/api/v1`;
    if (window.location.hostname !== 'localhost' && window.location.hostname !== '127.0.0.1') {
      wsBase = wsBase.replace('localhost', window.location.hostname).replace('127.0.0.1', window.location.hostname);
    }
    const storedSessionId = localStorage.getItem('inox_session_id') || '';
    const url = `${wsBase}/rooms/${this.roomId}/ws${storedSessionId ? `?session_id=${storedSessionId}` : ''}`;

    logger.info('WSService: Initiating WebSocket handshake', { url, attempt: this.reconnectAttempts + 1 });

    try {
      this.socket = new WebSocket(url);

      this.socket.onopen = () => {
        logger.info('WSService: WebSocket connection established', { roomId: this.roomId });
        this.setStatus('OPEN');
        this.reconnectAttempts = 0;
      };

      this.socket.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data) as WSMessage;
          logger.debug('WSService: Received frame', { type: msg.type, sender: msg.sender_name });
          this.dispatchEvent(msg);
        } catch (err) {
          logger.warn('WSService: Failed to parse incoming WebSocket frame', { err, raw: event.data });
        }
      };

      this.socket.onerror = (error) => {
        logger.error('WSService: WebSocket error observed', { error });
      };

      this.socket.onclose = (event) => {
        logger.info('WSService: WebSocket connection closed', { code: event.code, reason: event.reason });
        this.setStatus('CLOSED');
        this.socket = null;

        // Auto-reconnect if not intentionally closed and under max attempts
        if (this.roomId && this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 10000);
          logger.info('WSService: Scheduling reconnect', { attempt: this.reconnectAttempts, delay });
          this.reconnectTimer = setTimeout(() => {
            if (this.roomId) {
              this.initiateConnection();
            }
          }, delay);
        }
      };
    } catch (err) {
      logger.error('WSService: Failed to create WebSocket instance', { err });
      this.setStatus('CLOSED');
    }
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.roomId = null;
    if (this.socket) {
      logger.info('WSService: Disconnecting active WebSocket');
      this.socket.onclose = null; // Prevent auto-reconnect trigger
      this.socket.close(1000, 'Client left room');
      this.socket = null;
    }
    this.setStatus('CLOSED');
  }

  send<T = unknown>(type: WSEventType, payload?: T, targetId?: string): void {
    if (!this.socket || this.status !== 'OPEN') {
      logger.warn('WSService: Cannot send message, socket not open', { type, status: this.status });
      return;
    }

    const msg: Partial<WSMessage<T>> = {
      type,
      payload,
      target_id: targetId,
      timestamp: Date.now(),
    };

    try {
      this.socket.send(JSON.stringify(msg));
      logger.debug('WSService: Frame sent', { type });
    } catch (err) {
      logger.error('WSService: Send frame failed', { err, type });
    }
  }

  on(type: WSEventType | '*', callback: WSCallback): () => void {
    if (!this.listeners.has(type)) {
      this.listeners.set(type, new Set());
    }
    this.listeners.get(type)!.add(callback);

    return () => {
      const set = this.listeners.get(type);
      if (set) {
        set.delete(callback);
        if (set.size === 0) {
          this.listeners.delete(type);
        }
      }
    };
  }

  onStatusChange(callback: (status: WSStatus) => void): () => void {
    this.statusListeners.add(callback);
    callback(this.status);
    return () => {
      this.statusListeners.delete(callback);
    };
  }

  getStatus(): WSStatus {
    return this.status;
  }

  private setStatus(status: WSStatus): void {
    if (this.status !== status) {
      this.status = status;
      this.statusListeners.forEach((cb) => cb(status));
    }
  }

  private dispatchEvent(msg: WSMessage): void {
    // Notify exact type listeners
    const exactListeners = this.listeners.get(msg.type);
    if (exactListeners) {
      exactListeners.forEach((cb) => cb(msg));
    }

    // Notify wildcard '*' listeners
    const wildcardListeners = this.listeners.get('*');
    if (wildcardListeners) {
      wildcardListeners.forEach((cb) => cb(msg));
    }
  }
}

export const wsService = new WSService();
