export interface RoomTelemetry {
  room_id: string;
  participant_count: number;
  is_playing: boolean;
  media_url: string;
  media_time_seconds: number;
  sfu_peers_count: number;
  qoe_score: number;
}

export interface MetricsSnapshot {
  active_rooms: number;
  active_sfu_rooms: number;
  active_ws_connections: number;
  active_sfu_peers: number;
  ws_evictions_total: number;
  chat_messages_total: number;
  http_requests_total: number;
  http_errors_total: number;
  db_connections_in_use: number;
  redis_cache_hits: number;
  redis_cache_misses: number;
}

export interface SystemTelemetry {
  timestamp: number; // Epoch millis
  metrics: MetricsSnapshot;
  goroutines: number;
  heap_alloc_bytes: number;
  rooms: RoomTelemetry[];
}

export interface TelemetryHistoryPoint {
  timeLabel: string;
  timestamp: number;
  goroutines: number;
  heapMb: number;
  wsConnections: number;
  sfuPeers: number;
  activeRooms: number;
  httpRequests: number;
  httpErrors: number;
  dbPool: number;
  cacheHitRate: number;
}
