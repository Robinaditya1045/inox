import * as React from "react"
import type { RoomInspectorDetail } from "@/types/rooms"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Users, Play, Pause, RefreshCw, Radio, Wifi, Video, Mic, MicOff, VideoOff, UserX, Trash2, CheckCircle2, AlertTriangle, ShieldAlert } from "lucide-react"

interface RoomSessionInspectorProps {
  room: RoomInspectorDetail | null
  isOpen: boolean
  onClose: () => void
  onForceSync: (roomId: string, targetTimeSec: number, isPlaying: boolean) => Promise<void>
  onKickParticipant: (roomId: string, userId: string, username: string) => Promise<void>
  onTerminateRoom: (roomId: string, roomName: string) => Promise<void>
}

export function RoomSessionInspector({
  room,
  isOpen,
  onClose,
  onForceSync,
  onKickParticipant,
  onTerminateRoom,
}: RoomSessionInspectorProps) {
  const [syncTime, setSyncTime] = React.useState<number>(0)
  const [syncPlaying, setSyncPlaying] = React.useState<boolean>(true)
  const [isSyncing, setIsSyncing] = React.useState<boolean>(false)
  const [kickingId, setKickingId] = React.useState<string | null>(null)

  React.useEffect(() => {
    if (room) {
      setSyncTime(room.media_time_seconds || 0)
      setSyncPlaying(room.is_playing)
    }
  }, [room])

  if (!room) return null

  const handleSyncSubmit = async () => {
    setIsSyncing(true)
    try {
      await onForceSync(room.room_id, syncTime, syncPlaying)
    } finally {
      setIsSyncing(false)
    }
  }

  const handleKick = async (userId: string, username: string) => {
    if (!window.confirm(`Are you sure you want to kick "${username}" from this Watch Party?`)) return
    setKickingId(userId)
    try {
      await onKickParticipant(room.room_id, userId, username)
    } finally {
      setKickingId(null)
    }
  }

  const handleTerminate = async () => {
    if (!window.confirm(`FORCE TERMINATE Watch Party "${room.room_name}"? All peers will be disconnected.`)) return
    await onTerminateRoom(room.room_id, room.room_name)
    onClose()
  }

  const formatTime = (sec: number) => {
    const mins = Math.floor(sec / 60)
    const secs = Math.floor(sec % 60)
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[850px] border-zinc-800 bg-[#0a0a0a] text-white p-6 shadow-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader className="pb-3 border-b border-zinc-800">
          <div className="flex items-center justify-between">
            <DialogTitle className="text-lg font-semibold flex items-center">
              <Users className="h-5 w-5 mr-2 text-cyan-400" /> Watch Party Session Inspector
            </DialogTitle>
            <div className="flex items-center space-x-2">
              <Badge variant={room.qoe_score > 90 ? "success" : room.qoe_score > 75 ? "warning" : "destructive"} className="font-mono text-xs">
                QoE SCORE: {room.qoe_score}%
              </Badge>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleTerminate}
                className="h-7 text-xs font-mono"
              >
                <Trash2 className="h-3 w-3 mr-1" /> Terminate Room
              </Button>
            </div>
          </div>
          <DialogDescription className="text-xs font-mono text-zinc-400">
            {room.room_name} • ID: {room.room_id} • Host: {room.owner_id}
          </DialogDescription>
        </DialogHeader>

        {/* SECTION 1: Authoritative Playback Sync Control */}
        <div className="p-4 rounded-xl bg-[#111111] border border-zinc-800 space-y-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              <Radio className="h-4 w-4 text-emerald-400 animate-pulse" />
              <span className="text-sm font-bold text-white font-mono">Authoritative Playback Sync (SYNC_PLAYBACK)</span>
            </div>
            <span className="text-xs font-mono text-zinc-400">
              Media: {room.media_url ? room.media_url.split("/").pop() : "No asset loaded"}
            </span>
          </div>

          <p className="text-xs text-zinc-400">
            Broadcast an authoritative server timestamp and state to force all connected Watch Party players to reconcile sync drift.
          </p>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3 pt-1">
            <div className="flex items-center space-x-2 bg-black/60 p-2.5 rounded-lg border border-zinc-800">
              <button
                type="button"
                onClick={() => setSyncPlaying(!syncPlaying)}
                className={`px-3 py-1.5 rounded text-xs font-mono font-bold flex items-center transition-colors ${syncPlaying ? "bg-emerald-500 text-black" : "bg-zinc-800 text-zinc-300"}`}
              >
                {syncPlaying ? <Play className="h-3.5 w-3.5 mr-1 fill-current" /> : <Pause className="h-3.5 w-3.5 mr-1 fill-current" />}
                {syncPlaying ? "PLAYING" : "PAUSED"}
              </button>
              <span className="text-xs font-mono text-zinc-400">State Override</span>
            </div>

            <div className="flex flex-col justify-center bg-black/60 p-2.5 rounded-lg border border-zinc-800">
              <div className="flex justify-between text-xs font-mono mb-1">
                <span className="text-zinc-500">Timestamp:</span>
                <span className="text-cyan-400 font-bold">{formatTime(syncTime)} ({syncTime.toFixed(1)}s)</span>
              </div>
              <input
                type="range"
                min={0}
                max={600}
                step={1}
                value={syncTime}
                onChange={e => setSyncTime(Number(e.target.value))}
                className="w-full h-1 bg-zinc-700 rounded-lg appearance-none cursor-pointer accent-cyan-500"
              />
            </div>

            <div className="flex items-center">
              <Button
                variant="cyan"
                onClick={handleSyncSubmit}
                disabled={isSyncing}
                className="w-full h-full font-mono text-xs"
              >
                <RefreshCw className={`h-3.5 w-3.5 mr-2 ${isSyncing ? "animate-spin" : ""}`} />
                {isSyncing ? "Broadcasting..." : "Force Sync All Peers"}
              </Button>
            </div>
          </div>
        </div>

        {/* SECTION 2: Participant & WebRTC Track Inspector */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-mono font-bold text-zinc-400 uppercase tracking-wider">
              Connected Participants & SFU RTP Tracks ({room.participants.length})
            </span>
            <span className="text-xs font-mono text-zinc-500">
              Pion WebRTC SFU • RTP Fan-out
            </span>
          </div>

          <div className="rounded-xl border border-zinc-800 bg-[#111111] overflow-hidden">
            <Table>
              <TableHeader className="bg-zinc-900/60 border-b border-zinc-800">
                <TableRow className="hover:bg-transparent border-zinc-800">
                  <TableHead className="text-xs font-mono text-zinc-400">USER / ROLE</TableHead>
                  <TableHead className="text-xs font-mono text-zinc-400">RTT / LATENCY</TableHead>
                  <TableHead className="text-xs font-mono text-zinc-400">WEBRTC TRACKS</TableHead>
                  <TableHead className="text-xs font-mono text-zinc-400">SYNC DRIFT</TableHead>
                  <TableHead className="text-right text-xs font-mono text-zinc-400">ACTION</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {room.participants.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="h-24 text-center text-zinc-500 font-mono text-xs">
                      No active participants in this session workspace.
                    </TableCell>
                  </TableRow>
                ) : (
                  room.participants.map(p => (
                    <TableRow key={p.user_id} className="hover:bg-zinc-900/40 border-zinc-800">
                      <TableCell className="py-2.5">
                        <div>
                          <p className="font-semibold text-xs text-white flex items-center">
                            {p.username}
                            {p.role === "owner" && (
                              <Badge variant="cyan" className="ml-2 px-1.5 py-0 text-[9px]">HOST</Badge>
                            )}
                          </p>
                          <p className="text-[10px] font-mono text-zinc-500">ID: {p.user_id}</p>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className="flex items-center text-xs font-mono">
                          <Wifi className={`h-3 w-3 mr-1.5 ${p.rtt_ms < 50 ? "text-emerald-400" : p.rtt_ms < 150 ? "text-amber-400" : "text-rose-400"}`} />
                          <span className={p.rtt_ms < 50 ? "text-emerald-400 font-bold" : p.rtt_ms < 150 ? "text-amber-400" : "text-rose-400 font-bold"}>
                            {p.rtt_ms} ms
                          </span>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <span className="flex items-center text-[11px] font-mono bg-black/60 px-2 py-0.5 rounded border border-zinc-800">
                            {p.video_track === "active" ? <Video className="h-3 w-3 text-emerald-400 mr-1" /> : <VideoOff className="h-3 w-3 text-rose-400 mr-1" />}
                            {p.audio_track === "active" ? <Mic className="h-3 w-3 text-emerald-400 mr-1" /> : <MicOff className="h-3 w-3 text-amber-400 mr-1" />}
                            <span className="text-zinc-300 ml-0.5">{p.codec}</span>
                          </span>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className="flex items-center text-xs font-mono">
                          {Math.abs(p.sync_drift_ms) <= 50 ? (
                            <span className="text-emerald-400 flex items-center">
                              <CheckCircle2 className="h-3 w-3 mr-1" /> ±{Math.abs(p.sync_drift_ms)}ms
                            </span>
                          ) : Math.abs(p.sync_drift_ms) <= 200 ? (
                            <span className="text-amber-400 flex items-center">
                              <AlertTriangle className="h-3 w-3 mr-1" /> {p.sync_drift_ms > 0 ? `+${p.sync_drift_ms}` : p.sync_drift_ms}ms
                            </span>
                          ) : (
                            <span className="text-rose-400 font-bold flex items-center">
                              <ShieldAlert className="h-3 w-3 mr-1" /> {p.sync_drift_ms > 0 ? `+${p.sync_drift_ms}` : p.sync_drift_ms}ms
                            </span>
                          )}
                        </div>
                      </TableCell>

                      <TableCell className="text-right">
                        {p.role !== "owner" && (
                          <Button
                            variant="destructive-outline"
                            size="sm"
                            disabled={kickingId === p.user_id}
                            onClick={() => handleKick(p.user_id, p.username)}
                            className="h-7 px-2 text-[11px] font-mono"
                          >
                            <UserX className="h-3 w-3 mr-1" /> Kick
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
