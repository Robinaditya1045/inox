import * as React from "react"
import { useRoomInspector } from "@/hooks/useRoomInspector"
import { RoomGrid } from "@/components/rooms/RoomGrid"
import { RoomSessionInspector } from "@/components/rooms/RoomSessionInspector"
import { MetricCard } from "@/components/dashboard/MetricCard"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { RoomInspectorDetail } from "@/types/rooms"
import { Users, Radio, CheckCircle2, RefreshCw, ShieldAlert, Activity, Clock, ShieldCheck } from "lucide-react"

export function RoomCommandCenter() {
  const { rooms, isLoading, isDemoMode, error, logs, forceSyncPlayback, kickParticipant, terminateRoom, refresh } = useRoomInspector()
  const [selectedRoom, setSelectedRoom] = React.useState<RoomInspectorDetail | null>(null)

  const totalPeers = React.useMemo(() => rooms.reduce((acc, r) => acc + r.participant_count, 0), [rooms])
  const totalSfuTracks = React.useMemo(() => rooms.reduce((acc, r) => acc + r.sfu_peers_count, 0), [rooms])
  const avgQoe = React.useMemo(() => {
    if (rooms.length === 0) return 100
    const sum = rooms.reduce((acc, r) => acc + r.qoe_score, 0)
    return Math.round(sum / rooms.length)
  }, [rooms])
  const syncedRooms = React.useMemo(() => rooms.filter(r => r.sync_status === "synchronized").length, [rooms])

  return (
    <div className="space-y-6 animate-pulse-subtle" style={{ animationDuration: "0.3s", animationIterationCount: 1 }}>
      {/* Top Banner & Control Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-4 rounded-xl border border-zinc-800 bg-gradient-to-r from-zinc-900/80 to-zinc-950/80 backdrop-blur-md">
        <div className="flex items-center space-x-3">
          <div className="p-2.5 rounded-lg bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
            <Users className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h2 className="font-bold text-white text-base tracking-tight">
                Room & Session Inspector
              </h2>
              <Badge variant={isDemoMode ? "warning" : "success"} className="text-[10px]">
                {isDemoMode ? "DEMO TELEMETRY" : "WEBSOCKET HUB"}
              </Badge>
            </div>
            <p className="text-xs font-mono text-zinc-400 mt-0.5">
              Monitor real-time Watch Party sessions, inspect WebRTC SFU audio/video tracks, and enforce authoritative playback sync.
            </p>
          </div>
        </div>

        <div className="flex items-center space-x-3">
          {error && (
            <span className="text-xs text-amber-400 font-mono flex items-center bg-amber-500/10 px-2.5 py-1 rounded-md border border-amber-500/20">
              <ShieldAlert className="h-3.5 w-3.5 mr-1" /> {error}
            </span>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={() => refresh()}
            className="text-xs font-mono"
          >
            <RefreshCw className="h-3.5 w-3.5 mr-1.5" /> Refresh Telemetry
          </Button>
        </div>
      </div>

      {/* Room Metric Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Active Watch Parties"
          value={rooms.length}
          icon={Users}
          iconColor="text-cyan-400"
          category="USE"
          categoryLabel="SESSIONS"
          trend={`${syncedRooms} fully synchronized`}
          trendPositive={true}
          progressValue={Math.min(100, (rooms.length / 25) * 100)}
          footerText="Pion WebRTC Rooms"
        />

        <MetricCard
          title="Connected SFU Peers"
          value={totalPeers}
          icon={Activity}
          iconColor="text-emerald-400"
          category="USE"
          categoryLabel="PARTICIPANTS"
          trend={`${totalSfuTracks} RTP media tracks`}
          trendPositive={true}
          progressValue={Math.min(100, (totalPeers / 200) * 100)}
          footerText="Selective Forwarding Unit"
        />

        <MetricCard
          title="Average Platform QoE"
          value={avgQoe}
          icon={CheckCircle2}
          iconColor="text-purple-400"
          category="RED"
          categoryLabel="EXPERIENCE"
          trend={avgQoe >= 90 ? "Excellent stream quality" : "Network buffering detected"}
          trendPositive={avgQoe >= 90}
          progressValue={avgQoe}
          footerText="Target: >90% composite"
        />

        <MetricCard
          title="Playback Sync Drift"
          value={rooms.length > 0 ? Math.round(((rooms.length - syncedRooms) / rooms.length) * 100) : 0}
          icon={Radio}
          iconColor="text-amber-400"
          category="RED"
          categoryLabel="DRIFT RATE"
          trend={`${rooms.length - syncedRooms} rooms reconciling`}
          trendPositive={syncedRooms === rooms.length}
          progressValue={rooms.length > 0 ? ((rooms.length - syncedRooms) / rooms.length) * 100 : 0}
          footerText="Server Authoritative Sync"
        />
      </div>

      {/* Room Grid Table */}
      <RoomGrid
        rooms={rooms}
        isLoading={isLoading}
        onSelectRoom={(r) => setSelectedRoom(r)}
        onTerminateRoom={terminateRoom}
      />

      {/* Admin Intervention Audit Log */}
      <div className="space-y-3 pt-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-bold text-white font-mono flex items-center">
            <ShieldCheck className="h-4 w-4 mr-2 text-emerald-400" /> Administrative Intervention Audit Log
          </h3>
          <span className="text-xs font-mono text-zinc-500">
            Real-time governance & RBAC enforcement
          </span>
        </div>

        <div className="rounded-xl border border-zinc-800 bg-[#111111] overflow-hidden">
          <Table>
            <TableHeader className="bg-zinc-900/60 border-b border-zinc-800">
              <TableRow className="hover:bg-transparent border-zinc-800">
                <TableHead className="w-[120px] text-xs font-mono text-zinc-400">TIMESTAMP</TableHead>
                <TableHead className="w-[160px] text-xs font-mono text-zinc-400">ROOM ID</TableHead>
                <TableHead className="w-[140px] text-xs font-mono text-zinc-400">ACTION</TableHead>
                <TableHead className="text-xs font-mono text-zinc-400">INTERVENTION DETAILS</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={4} className="h-20 text-center text-zinc-500 font-mono text-xs">
                    No administrative interventions recorded in current session.
                  </TableCell>
                </TableRow>
              ) : (
                logs.map(log => (
                  <TableRow key={log.id} className="hover:bg-zinc-900/40 border-zinc-800">
                    <TableCell className="text-xs font-mono text-zinc-400 py-2.5">
                      <div className="flex items-center">
                        <Clock className="h-3 w-3 mr-1.5 text-zinc-500" />
                        {log.timestamp}
                      </div>
                    </TableCell>
                    <TableCell className="text-xs font-mono text-cyan-400 font-medium">
                      {log.room_id}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={log.action === "FORCE_SYNC" ? "cyan" : log.action === "KICK_PEER" ? "warning" : "destructive"}
                        className="font-mono text-[10px] px-2 py-0.5"
                      >
                        {log.action}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-xs font-mono text-zinc-300">
                      {log.details}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      {/* Room Session Inspector Dialog */}
      <RoomSessionInspector
        room={selectedRoom ? (rooms.find(r => r.room_id === selectedRoom.room_id) || selectedRoom) : null}
        isOpen={!!selectedRoom}
        onClose={() => setSelectedRoom(null)}
        onForceSync={forceSyncPlayback}
        onKickParticipant={kickParticipant}
        onTerminateRoom={terminateRoom}
      />
    </div>
  )
}
