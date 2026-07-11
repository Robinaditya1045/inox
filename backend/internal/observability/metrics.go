package observability

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Registry maintains thread-safe atomic counters and gauges for real-time observability.
// By using sync/atomic, metric updates achieve near-zero latency without mutex lock contention.
type Registry struct {
	activeRooms         int64
	activeSFURooms      int64
	activeWSConnections int64
	activeSFUPeers      int64
	wsEvictionsTotal    uint64
	chatMessagesTotal   uint64
	httpRequestsTotal   uint64
	httpErrorsTotal     uint64
	dbConnectionsInUse  int64
	redisCacheHits      uint64
	redisCacheMisses    uint64
}

var globalRegistry = &Registry{}

// Global returns the singleton instance of the metrics registry.
func Global() *Registry {
	return globalRegistry
}

// IncActiveRooms increments the active room gauge.
func (r *Registry) IncActiveRooms() {
	atomic.AddInt64(&r.activeRooms, 1)
}

// DecActiveRooms decrements the active room gauge.
func (r *Registry) DecActiveRooms() {
	atomic.AddInt64(&r.activeRooms, -1)
}

// IncActiveSFURooms increments the active SFU media room gauge.
func (r *Registry) IncActiveSFURooms() {
	atomic.AddInt64(&r.activeSFURooms, 1)
}

// DecActiveSFURooms decrements the active SFU media room gauge.
func (r *Registry) DecActiveSFURooms() {
	atomic.AddInt64(&r.activeSFURooms, -1)
}

// IncActiveWSConnections increments the active WebSocket connection gauge.
func (r *Registry) IncActiveWSConnections() {
	atomic.AddInt64(&r.activeWSConnections, 1)
}

// DecActiveWSConnections decrements the active WebSocket connection gauge.
func (r *Registry) DecActiveWSConnections() {
	atomic.AddInt64(&r.activeWSConnections, -1)
}

// IncActiveSFUPeers increments the active WebRTC peer connection gauge.
func (r *Registry) IncActiveSFUPeers() {
	atomic.AddInt64(&r.activeSFUPeers, 1)
}

// DecActiveSFUPeers decrements the active WebRTC peer connection gauge.
func (r *Registry) DecActiveSFUPeers() {
	atomic.AddInt64(&r.activeSFUPeers, -1)
}

// IncWSEvictions records a WebSocket send buffer overflow eviction.
func (r *Registry) IncWSEvictions() {
	atomic.AddUint64(&r.wsEvictionsTotal, 1)
}

// IncChatMessages records a real-time chat message broadcast.
func (r *Registry) IncChatMessages() {
	atomic.AddUint64(&r.chatMessagesTotal, 1)
}

// IncHTTPRequest records an HTTP request and increments errors if status >= 400.
func (r *Registry) IncHTTPRequest(status int) {
	atomic.AddUint64(&r.httpRequestsTotal, 1)
	if status >= 400 {
		atomic.AddUint64(&r.httpErrorsTotal, 1)
	}
}

// SetDBConnectionsInUse sets the current gauge of PostgreSQL connection pool usage.
func (r *Registry) SetDBConnectionsInUse(count int64) {
	atomic.StoreInt64(&r.dbConnectionsInUse, count)
}

// IncRedisCacheHit records a successful cache lookup.
func (r *Registry) IncRedisCacheHit() {
	atomic.AddUint64(&r.redisCacheHits, 1)
}

// IncRedisCacheMiss records a cache miss.
func (r *Registry) IncRedisCacheMiss() {
	atomic.AddUint64(&r.redisCacheMisses, 1)
}

// Snapshot returns a structured snapshot of current metric values.
type Snapshot struct {
	ActiveRooms         int64  `json:"active_rooms"`
	ActiveSFURooms      int64  `json:"active_sfu_rooms"`
	ActiveWSConnections int64  `json:"active_ws_connections"`
	ActiveSFUPeers      int64  `json:"active_sfu_peers"`
	WSEvictionsTotal    uint64 `json:"ws_evictions_total"`
	ChatMessagesTotal   uint64 `json:"chat_messages_total"`
	HTTPRequestsTotal   uint64 `json:"http_requests_total"`
	HTTPErrorsTotal     uint64 `json:"http_errors_total"`
	DBConnectionsInUse  int64  `json:"db_connections_in_use"`
	RedisCacheHits      uint64 `json:"redis_cache_hits"`
	RedisCacheMisses    uint64 `json:"redis_cache_misses"`
}

// GetSnapshot captures the current state of all atomic counters and gauges.
func (r *Registry) GetSnapshot() Snapshot {
	return Snapshot{
		ActiveRooms:         atomic.LoadInt64(&r.activeRooms),
		ActiveSFURooms:      atomic.LoadInt64(&r.activeSFURooms),
		ActiveWSConnections: atomic.LoadInt64(&r.activeWSConnections),
		ActiveSFUPeers:      atomic.LoadInt64(&r.activeSFUPeers),
		WSEvictionsTotal:    atomic.LoadUint64(&r.wsEvictionsTotal),
		ChatMessagesTotal:   atomic.LoadUint64(&r.chatMessagesTotal),
		HTTPRequestsTotal:   atomic.LoadUint64(&r.httpRequestsTotal),
		HTTPErrorsTotal:     atomic.LoadUint64(&r.httpErrorsTotal),
		DBConnectionsInUse:  atomic.LoadInt64(&r.dbConnectionsInUse),
		RedisCacheHits:      atomic.LoadUint64(&r.redisCacheHits),
		RedisCacheMisses:    atomic.LoadUint64(&r.redisCacheMisses),
	}
}

// FormatPrometheus exports all registered metrics in standard Prometheus text exposition format.
// This allows Grafana and Prometheus scrapers to monitor Inox without third-party module dependencies.
func (r *Registry) FormatPrometheus() string {
	s := r.GetSnapshot()
	var b strings.Builder

	b.WriteString("# HELP inox_active_rooms_total Number of currently active watch party rooms\n")
	b.WriteString("# TYPE inox_active_rooms_total gauge\n")
	b.WriteString(fmt.Sprintf("inox_active_rooms_total %d\n\n", s.ActiveRooms))

	b.WriteString("# HELP inox_active_sfu_rooms_total Number of active WebRTC SFU media rooms\n")
	b.WriteString("# TYPE inox_active_sfu_rooms_total gauge\n")
	b.WriteString(fmt.Sprintf("inox_active_sfu_rooms_total %d\n\n", s.ActiveSFURooms))

	b.WriteString("# HELP inox_active_ws_connections_current Number of active WebSocket client connections\n")
	b.WriteString("# TYPE inox_active_ws_connections_current gauge\n")
	b.WriteString(fmt.Sprintf("inox_active_ws_connections_current %d\n\n", s.ActiveWSConnections))

	b.WriteString("# HELP inox_active_sfu_peers_current Number of active WebRTC SFU peer connections\n")
	b.WriteString("# TYPE inox_active_sfu_peers_current gauge\n")
	b.WriteString(fmt.Sprintf("inox_active_sfu_peers_current %d\n\n", s.ActiveSFUPeers))

	b.WriteString("# HELP inox_ws_client_evictions_total Total WebSocket connections evicted due to full send buffers\n")
	b.WriteString("# TYPE inox_ws_client_evictions_total counter\n")
	b.WriteString(fmt.Sprintf("inox_ws_client_evictions_total %d\n\n", s.WSEvictionsTotal))

	b.WriteString("# HELP inox_chat_messages_total Total real-time chat messages broadcasted across rooms\n")
	b.WriteString("# TYPE inox_chat_messages_total counter\n")
	b.WriteString(fmt.Sprintf("inox_chat_messages_total %d\n\n", s.ChatMessagesTotal))

	b.WriteString("# HELP inox_http_requests_total Total HTTP requests handled\n")
	b.WriteString("# TYPE inox_http_requests_total counter\n")
	b.WriteString(fmt.Sprintf("inox_http_requests_total %d\n\n", s.HTTPRequestsTotal))

	b.WriteString("# HELP inox_http_errors_total Total HTTP error responses (status >= 400)\n")
	b.WriteString("# TYPE inox_http_errors_total counter\n")
	b.WriteString(fmt.Sprintf("inox_http_errors_total %d\n\n", s.HTTPErrorsTotal))

	b.WriteString("# HELP inox_db_connections_in_use Active database connection pool usage\n")
	b.WriteString("# TYPE inox_db_connections_in_use gauge\n")
	b.WriteString(fmt.Sprintf("inox_db_connections_in_use %d\n\n", s.DBConnectionsInUse))

	b.WriteString("# HELP inox_redis_cache_hits_total Total Redis cache hit events\n")
	b.WriteString("# TYPE inox_redis_cache_hits_total counter\n")
	b.WriteString(fmt.Sprintf("inox_redis_cache_hits_total %d\n\n", s.RedisCacheHits))

	b.WriteString("# HELP inox_redis_cache_misses_total Total Redis cache miss events\n")
	b.WriteString("# TYPE inox_redis_cache_misses_total counter\n")
	b.WriteString(fmt.Sprintf("inox_redis_cache_misses_total %d\n", s.RedisCacheMisses))

	return b.String()
}
