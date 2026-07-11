import { useTelemetryStream } from "@/hooks/useTelemetryStream"
import { MetricCard } from "@/components/dashboard/MetricCard"
import { LiveTelemetryChart } from "@/components/dashboard/LiveTelemetryChart"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Progress } from "@/components/ui/progress"
import { Cpu, Server, Users, Database, Activity, Wifi, RefreshCw, ShieldAlert, Zap, Layers } from "lucide-react"

export function SystemPulseDashboard() {
  const { current, history, isConnected, isDemoMode, toggleDemoMode, error } = useTelemetryStream()

  const goroutines = current?.goroutines || 0
  const heapMb = current ? Number((current.heap_alloc_bytes / (1024 * 1024)).toFixed(1)) : 0
  const activeRooms = current?.metrics?.active_rooms || 0
  const sfuPeers = current?.metrics?.active_sfu_peers || 0
  const wsConnections = current?.metrics?.active_ws_connections || 0
  const httpRequests = current?.metrics?.http_requests_total || 0
  const httpErrors = current?.metrics?.http_errors_total || 0
  const dbPool = current?.metrics?.db_connections_in_use || 0
  const redisHits = current?.metrics?.redis_cache_hits || 0
  const redisMisses = current?.metrics?.redis_cache_misses || 0

  const totalRedisQueries = redisHits + redisMisses
  const cacheHitRate = totalRedisQueries > 0 ? Number(((redisHits / totalRedisQueries) * 100).toFixed(1)) : 100
  const httpErrorRate = httpRequests > 0 ? Number(((httpErrors / httpRequests) * 100).toFixed(2)) : 0.0

  return (
    <div className="space-y-6 animate-pulse-subtle" style={{ animationDuration: "0.3s", animationIterationCount: 1 }}>
      {/* Top Status & Mode Control Bar */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4 p-4 rounded-xl border border-zinc-800 bg-gradient-to-r from-zinc-900/80 to-zinc-950/80 backdrop-blur-md">
        <div className="flex items-center space-x-3">
          <div className={`p-2 rounded-lg ${isConnected ? "bg-emerald-500/10 text-emerald-400" : isDemoMode ? "bg-amber-500/10 text-amber-400" : "bg-rose-500/10 text-rose-400"}`}>
            {isConnected ? <Wifi className="h-5 w-5 animate-pulse" /> : isDemoMode ? <Zap className="h-5 w-5" /> : <ShieldAlert className="h-5 w-5" />}
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-semibold text-white text-sm">
                {isConnected ? "LIVE WEBSOCKET STREAM" : isDemoMode ? "DEMO SIMULATION MODE" : "DISCONNECTED"}
              </span>
              <Badge variant={isConnected ? "success" : isDemoMode ? "warning" : "destructive"}>
                {isConnected ? "5Hz LIVE" : isDemoMode ? "2Hz FALLBACK" : "OFFLINE"}
              </Badge>
            </div>
            <p className="text-xs font-mono text-zinc-400 mt-0.5">
              {isDemoMode
                ? "Backend offline or unreachable. Streaming simulated high-frequency RED/USE ticks."
                : "Connected to ws://localhost:8080/api/v1/admin/telemetry/ws via Vite Proxy."}
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {error && (
            <span className="text-xs text-rose-400 font-mono flex items-center bg-rose-500/10 px-2.5 py-1 rounded-md border border-rose-500/20">
              <ShieldAlert className="h-3.5 w-3.5 mr-1" /> {error}
            </span>
          )}
          
          <Button
            variant={isDemoMode ? "emerald" : "outline"}
            size="sm"
            onClick={toggleDemoMode}
            className="text-xs font-mono"
          >
            <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
            {isDemoMode ? "Try Reconnect Live" : "Switch to Demo Mode"}
          </Button>
        </div>
      </div>

      {/* RED & USE Metric Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Goroutines (USE)"
          value={goroutines}
          icon={Cpu}
          iconColor="text-emerald-400"
          category="USE"
          categoryLabel="SATURATION"
          trend="↓ 0.0% leak variance"
          trendPositive={goroutines < 800}
          progressValue={Math.min(100, (goroutines / 1000) * 100)}
          footerText="Threshold: 1,000"
        />

        <MetricCard
          title="Heap Memory (USE)"
          value={heapMb}
          unit="MB"
          icon={Server}
          iconColor="text-cyan-400"
          category="USE"
          categoryLabel="UTILIZATION"
          trend="4.2 MB/s GC recycle"
          trendPositive={heapMb < 200}
          progressValue={Math.min(100, (heapMb / 512) * 100)}
          footerText="Max Heap: 512 MB"
        />

        <MetricCard
          title="Active WS / SFU Peers"
          value={`${wsConnections} / ${sfuPeers}`}
          icon={Users}
          iconColor="text-amber-400"
          category="USE"
          categoryLabel="CAPACITY"
          trend={`${activeRooms} active rooms`}
          trendPositive={true}
          progressValue={Math.min(100, ((wsConnections + sfuPeers) / 200) * 100)}
          footerText="100% RTP fan-out"
        />

        <MetricCard
          title="HTTP Requests (RED)"
          value={httpRequests.toLocaleString()}
          icon={Activity}
          iconColor="text-rose-400"
          category="RED"
          categoryLabel="RATE / ERROR"
          trend={`${httpErrorRate}% error rate`}
          trendPositive={httpErrorRate < 1.0}
          progressValue={Math.min(100, httpErrorRate * 10)}
          footerText={`${httpErrors} total 4xx/5xx`}
        />
      </div>

      {/* Live Time-Series Charts Grid — Recharts / SVG Zero-Dependency */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <LiveTelemetryChart
          title="Goroutine Concurrency & Leak Monitoring"
          description="Real-time USE metric tracking active Goroutines across WebSocket Hub and Pion SFU."
          data={history}
          dataKey="goroutines"
          color="#10b981"
          minValue={100}
          maxValue={350}
        />

        <LiveTelemetryChart
          title="Heap Memory Allocation & GC Cycles"
          description="Real-time USE metric monitoring Go runtime heap footprint in megabytes."
          data={history}
          dataKey="heapMb"
          color="#06b6d4"
          unit="MB"
          minValue={20}
          maxValue={120}
        />

        <LiveTelemetryChart
          title="WebSocket & SFU Track Saturation"
          description="Real-time count of connected client WebSockets and Pion WebRTC peer tracks."
          data={history}
          dataKey="wsConnections"
          color="#f59e0b"
          minValue={0}
          maxValue={80}
        />

        <LiveTelemetryChart
          title="HTTP API Traffic & Request Volume"
          description="Real-time RED rate metric monitoring cumulative HTTP requests processed."
          data={history}
          dataKey="httpRequests"
          color="#a855f7"
        />
      </div>

      {/* Infrastructure Saturation & Cache Efficiency Table */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card className="border-zinc-800 bg-[#111111]">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base text-white flex items-center">
                <Database className="h-4 w-4 mr-2 text-emerald-400" /> PostgreSQL Pool & Redis Cache
              </CardTitle>
              <Badge variant="success">HEALTHY</Badge>
            </div>
            <CardDescription className="text-xs">
              USE Saturation metrics for pgx connection pool and Redis lookup efficiency.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4 pt-1">
            <div>
              <div className="flex justify-between text-xs font-mono mb-1">
                <span className="text-zinc-400">Redis Cache Hit Rate</span>
                <span className="text-emerald-400 font-bold">{cacheHitRate}% ({redisHits.toLocaleString()} hits)</span>
              </div>
              <Progress value={cacheHitRate} className="h-1.5" />
            </div>

            <div>
              <div className="flex justify-between text-xs font-mono mb-1">
                <span className="text-zinc-400">PostgreSQL pgx Pool Saturation</span>
                <span className="text-cyan-400 font-bold">{dbPool} / 25 connections in use</span>
              </div>
              <Progress value={Math.min(100, (dbPool / 25) * 100)} className="h-1.5" />
            </div>
          </CardContent>
        </Card>

        <Card className="border-zinc-800 bg-[#111111]">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base text-white flex items-center">
                <Layers className="h-4 w-4 mr-2 text-amber-400" /> Active Watch Party Rooms Summary
              </CardTitle>
              <Badge variant="warning">{current?.rooms?.length || 0} ROOMS LIVE</Badge>
            </div>
            <CardDescription className="text-xs">
              Authoritative playback state and participant distribution across active rooms.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-1">
            <div className="space-y-2 max-h-[140px] overflow-y-auto pr-1">
              {current?.rooms && current.rooms.length > 0 ? (
                current.rooms.map((room, idx) => (
                  <div key={idx} className="flex items-center justify-between p-2 rounded-lg bg-zinc-900/60 border border-zinc-800/80 text-xs font-mono">
                    <div className="flex items-center space-x-2 truncate">
                      <span className={`h-2 w-2 rounded-full ${room.is_playing ? "bg-emerald-500 animate-pulse" : "bg-zinc-600"}`} />
                      <span className="text-white font-medium truncate max-w-[140px]">{room.room_id}</span>
                    </div>
                    <div className="flex items-center space-x-3 text-zinc-400">
                      <span>{room.participant_count} peers</span>
                      <Badge variant={room.qoe_score > 90 ? "success" : "warning"} className="text-[10px] px-1 py-0">
                        {room.qoe_score}% QoE
                      </Badge>
                    </div>
                  </div>
                ))
              ) : (
                <div className="text-center py-6 text-zinc-500 text-xs font-mono">
                  No active rooms currently broadcasting.
                </div>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
