import * as React from "react"
import { getSessionId, telemetryWsUrl } from "@/lib/api"
import type { SystemTelemetry, TelemetryHistoryPoint } from "@/types/telemetry"

export interface UseTelemetryStreamResult {
  current: SystemTelemetry | null
  history: TelemetryHistoryPoint[]
  isConnected: boolean
  isDemoMode: boolean
  toggleDemoMode: () => void
  error: string | null
}

const MAX_HISTORY_POINTS = 40

export function useTelemetryStream(url?: string): UseTelemetryStreamResult {
  const [current, setCurrent] = React.useState<SystemTelemetry | null>(null)
  const [history, setHistory] = React.useState<TelemetryHistoryPoint[]>([])
  const [isConnected, setIsConnected] = React.useState<boolean>(false)
  const [isDemoMode, setIsDemoMode] = React.useState<boolean>(false)
  const [error, setError] = React.useState<string | null>(null)

  const latestPayloadRef = React.useRef<SystemTelemetry | null>(null)
  const hasNewPayloadRef = React.useRef<boolean>(false)
  const wsRef = React.useRef<WebSocket | null>(null)
  const retryCountRef = React.useRef<number>(0)
  const watchdogTimerRef = React.useRef<ReturnType<typeof setInterval> | null>(null)
  const reconnectTimeoutRef = React.useRef<ReturnType<typeof setTimeout> | null>(null)
  // Last time *this* socket saw traffic. Seeded at open so a healthy but
  // briefly quiet connection is not mistaken for a dead one.
  const lastActivityRef = React.useRef<number>(0)
  // Whether the current live connection ever delivered a payload. Kept separate
  // from latestPayloadRef because demo mode writes mock snapshots into that ref,
  // which would otherwise make a genuinely dead backend look alive.
  const receivedLiveDataRef = React.useRef<boolean>(false)

  const generateMockSnapshot = React.useCallback((): SystemTelemetry => {
    const now = Date.now()
    const prev = latestPayloadRef.current
    const baseGoroutines = prev ? prev.goroutines : 142
    const baseHeap = prev ? prev.heap_alloc_bytes : 48.5 * 1024 * 1024
    const baseWS = prev ? prev.metrics.active_ws_connections : 28
    const baseReqs = prev ? prev.metrics.http_requests_total : 15420

    return {
      timestamp: now,
      goroutines: Math.max(110, Math.min(300, baseGoroutines + Math.floor(Math.random() * 9) - 4)),
      heap_alloc_bytes: Math.max(30 * 1024 * 1024, baseHeap + Math.floor((Math.random() * 0.8 - 0.4) * 1024 * 1024)),
      metrics: {
        active_rooms: 8,
        active_sfu_rooms: 6,
        active_ws_connections: Math.max(12, baseWS + Math.floor(Math.random() * 3) - 1),
        active_sfu_peers: 24,
        ws_evictions_total: prev ? prev.metrics.ws_evictions_total : 0,
        chat_messages_total: prev ? prev.metrics.chat_messages_total + Math.floor(Math.random() * 3) : 842,
        http_requests_total: baseReqs + Math.floor(Math.random() * 15) + 5,
        http_errors_total: prev ? prev.metrics.http_errors_total + (Math.random() > 0.92 ? 1 : 0) : 12,
        db_connections_in_use: Math.floor(Math.random() * 4) + 6,
        redis_cache_hits: prev ? prev.metrics.redis_cache_hits + Math.floor(Math.random() * 40) + 10 : 9420,
        redis_cache_misses: prev ? prev.metrics.redis_cache_misses + Math.floor(Math.random() * 3) : 180,
      },
      rooms: [
        {
          room_id: "room-alpha-99",
          participant_count: 12,
          is_playing: true,
          media_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/master.m3u8",
          media_time_seconds: ((now / 1000) % 600),
          sfu_peers_count: 12,
          qoe_score: 98,
        },
        {
          room_id: "room-beta-42",
          participant_count: 8,
          is_playing: true,
          media_url: "https://cdn.inox.stream/hls/matrix_remaster/master.m3u8",
          media_time_seconds: ((now / 1000) % 3600),
          sfu_peers_count: 8,
          qoe_score: 100,
        },
        {
          room_id: "room-gamma-07",
          participant_count: 4,
          is_playing: false,
          media_url: "https://cdn.inox.stream/hls/interstellar/master.m3u8",
          media_time_seconds: 1420.5,
          sfu_peers_count: 4,
          qoe_score: 95,
        }
      ]
    }
  }, [])

  React.useEffect(() => {
    const syncInterval = setInterval(() => {
      if (isDemoMode) {
        const mockData = generateMockSnapshot()
        latestPayloadRef.current = mockData
        hasNewPayloadRef.current = true
      }

      if (!hasNewPayloadRef.current || !latestPayloadRef.current) return

      const payload = latestPayloadRef.current
      hasNewPayloadRef.current = false

      setCurrent(payload)
      setHistory(prevHistory => {
        const date = new Date(payload.timestamp)
        const timeLabel = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`
        const heapMb = Number((payload.heap_alloc_bytes / (1024 * 1024)).toFixed(1))
        const totalQueries = payload.metrics.redis_cache_hits + payload.metrics.redis_cache_misses
        const cacheHitRate = totalQueries > 0 ? Number(((payload.metrics.redis_cache_hits / totalQueries) * 100).toFixed(1)) : 100

        const newPoint: TelemetryHistoryPoint = {
          timeLabel,
          timestamp: payload.timestamp,
          goroutines: payload.goroutines,
          heapMb,
          wsConnections: payload.metrics.active_ws_connections,
          sfuPeers: payload.metrics.active_sfu_peers,
          activeRooms: payload.metrics.active_rooms,
          httpRequests: payload.metrics.http_requests_total,
          httpErrors: payload.metrics.http_errors_total,
          dbPool: payload.metrics.db_connections_in_use,
          cacheHitRate,
        }

        const nextHistory = [...prevHistory, newPoint]
        if (nextHistory.length > MAX_HISTORY_POINTS) {
          return nextHistory.slice(nextHistory.length - MAX_HISTORY_POINTS)
        }
        return nextHistory
      })
    }, 200)

    return () => clearInterval(syncInterval)
  }, [isDemoMode, generateMockSnapshot])

  React.useEffect(() => {
    if (isDemoMode) {
      // Simulated data is not a live stream. Reporting isConnected here made the
      // dashboard render "LIVE WEBSOCKET STREAM / 5Hz LIVE" over mock numbers.
      setIsConnected(false)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      if (watchdogTimerRef.current) clearInterval(watchdogTimerRef.current)
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
      // Drop live samples so the mock series starts clean.
      latestPayloadRef.current = null
      hasNewPayloadRef.current = false
      setHistory([])
      return
    }

    let isSubscribed = true

    if (!getSessionId()) {
      // Without a session the backend replies 401 to the upgrade request, so
      // don't even attempt it — just surface how to authenticate.
      console.warn("No inox_session_id found. Falling back to Demo Mode.")
      setIsDemoMode(true)
      setError("No admin session found. Open this portal with ?session_id=<id> to stream live telemetry.")
      return
    }

    const targetUrl = telemetryWsUrl(url)

    // Entering live mode: drop any simulated samples so the charts never mix
    // mock and real series into one line.
    latestPayloadRef.current = null
    hasNewPayloadRef.current = false
    receivedLiveDataRef.current = false
    setCurrent(null)
    setHistory([])

    const connect = () => {
      if (!isSubscribed) return
      try {
        const ws = new WebSocket(targetUrl)
        wsRef.current = ws

        ws.onopen = () => {
          if (!isSubscribed) return
          setIsConnected(true)
          setError(null)
          retryCountRef.current = 0
          lastActivityRef.current = Date.now()

          if (watchdogTimerRef.current) clearInterval(watchdogTimerRef.current)
          watchdogTimerRef.current = setInterval(() => {
            // The backend pushes a snapshot every 2s; 10s of silence means the
            // socket is wedged. Comparing against the connection open time (not
            // a zeroed payload timestamp) keeps a fresh connection from being
            // torn down before its first snapshot arrives.
            if (Date.now() - lastActivityRef.current > 10000 && ws.readyState === WebSocket.OPEN) {
              ws.close()
            }
          }, 5000)
        }

        ws.onmessage = (event) => {
          if (!isSubscribed) return
          try {
            const data: SystemTelemetry = JSON.parse(event.data)
            latestPayloadRef.current = data
            hasNewPayloadRef.current = true
            lastActivityRef.current = Date.now()
            receivedLiveDataRef.current = true
          } catch (err) {
            console.error("Failed to parse telemetry JSON:", err)
          }
        }

        ws.onerror = () => {
          // Suppress noise, onclose will trigger cleanly
        }

        ws.onclose = () => {
          if (!isSubscribed) return
          setIsConnected(false)
          if (watchdogTimerRef.current) clearInterval(watchdogTimerRef.current)
          wsRef.current = null

          if (retryCountRef.current >= 2 && !receivedLiveDataRef.current) {
            setIsDemoMode(true)
            setError("Backend unreachable after 3 attempts. Running interactive Demo Mode.")
            return
          }

          const baseDelay = Math.min(15000, 1000 * Math.pow(2, retryCountRef.current))
          const jitter = baseDelay * (0.8 + 0.4 * Math.random())
          retryCountRef.current += 1

          if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
          reconnectTimeoutRef.current = setTimeout(connect, jitter)
        }
      } catch (err) {
        if (isSubscribed) {
          console.error("Failed to initialize telemetry WebSocket:", err)
          setIsConnected(false)
          setError("Failed to initialize WebSocket. Running interactive Demo Mode.")
          setIsDemoMode(true)
        }
      }
    }

    connect()

    return () => {
      isSubscribed = false
      if (watchdogTimerRef.current) clearInterval(watchdogTimerRef.current)
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [url, isDemoMode])

  const toggleDemoMode = React.useCallback(() => {
    // State updaters must stay pure — React may invoke them twice in StrictMode,
    // so the reset side effects live here rather than inside setIsDemoMode.
    setIsDemoMode(prev => !prev)
    retryCountRef.current = 0
    receivedLiveDataRef.current = false
    setError(null)
  }, [])

  return {
    current,
    history,
    isConnected,
    isDemoMode,
    toggleDemoMode,
    error,
  }
}
