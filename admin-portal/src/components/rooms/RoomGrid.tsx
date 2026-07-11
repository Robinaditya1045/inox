import * as React from "react"
import type { RoomInspectorDetail } from "@/types/rooms"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Progress } from "@/components/ui/progress"
import { Search, Users, Play, Pause, ShieldAlert, CheckCircle2, AlertTriangle, Eye, Trash2, Radio } from "lucide-react"

interface RoomGridProps {
  rooms: RoomInspectorDetail[]
  isLoading: boolean
  onSelectRoom: (room: RoomInspectorDetail) => void
  onTerminateRoom: (roomId: string, roomName: string) => Promise<void>
}

export function RoomGrid({
  rooms,
  isLoading,
  onSelectRoom,
  onTerminateRoom,
}: RoomGridProps) {
  const [searchQuery, setSearchQuery] = React.useState("")
  const [statusFilter, setStatusFilter] = React.useState<string>("all")
  const [terminatingId, setTerminatingId] = React.useState<string | null>(null)

  const filteredRooms = React.useMemo(() => {
    return rooms.filter(room => {
      const matchesSearch =
        room.room_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        room.room_id.toLowerCase().includes(searchQuery.toLowerCase()) ||
        room.owner_id.toLowerCase().includes(searchQuery.toLowerCase())

      const matchesStatus = statusFilter === "all" || room.sync_status === statusFilter
      return matchesSearch && matchesStatus
    })
  }, [rooms, searchQuery, statusFilter])

  const handleTerminate = async (e: React.MouseEvent, id: string, name: string) => {
    e.stopPropagation()
    if (!window.confirm(`Are you sure you want to FORCE TERMINATE Watch Party "${name}"? This will drop all connected WebRTC peers and close the session.`)) return
    setTerminatingId(id)
    try {
      await onTerminateRoom(id, name)
    } finally {
      setTerminatingId(null)
    }
  }

  const getSyncBadge = (status: RoomInspectorDetail["sync_status"]) => {
    switch (status) {
      case "synchronized":
        return (
          <Badge variant="success" className="flex items-center gap-1 px-2 py-0.5 text-[10px]">
            <CheckCircle2 className="h-3 w-3" /> SYNCED
          </Badge>
        )
      case "minor_drift":
        return (
          <Badge variant="warning" className="flex items-center gap-1 px-2 py-0.5 text-[10px]">
            <AlertTriangle className="h-3 w-3" /> DRIFTING
          </Badge>
        )
      case "out_of_sync":
        return (
          <Badge variant="destructive" className="flex items-center gap-1 px-2 py-0.5 text-[10px] animate-pulse">
            <ShieldAlert className="h-3 w-3" /> OUT OF SYNC
          </Badge>
        )
    }
  }

  const formatTime = (sec: number) => {
    const mins = Math.floor(sec / 60)
    const secs = Math.floor(sec % 60)
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }

  return (
    <div className="space-y-4">
      {/* Search & Filter Controls */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-[#111111] p-4 rounded-xl border border-zinc-800">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-zinc-500" />
          <Input
            placeholder="Search by room name, ID, or owner username..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="pl-9 bg-black/60 border-zinc-800 text-xs font-mono text-white focus:border-zinc-700"
          />
        </div>

        <div className="flex items-center space-x-1.5 bg-black/40 p-1 rounded-lg border border-zinc-800/80">
          {(["all", "synchronized", "minor_drift", "out_of_sync"] as const).map(tab => (
            <button
              key={tab}
              onClick={() => setStatusFilter(tab)}
              className={`px-3 py-1.5 rounded-md text-xs font-mono font-medium transition-all duration-150 ${
                statusFilter === tab
                  ? "bg-zinc-800 text-white shadow-sm"
                  : "text-zinc-400 hover:text-white hover:bg-zinc-900"
              }`}
            >
              {tab.replace("_", " ").toUpperCase()}
              <span className="ml-1.5 text-[10px] text-zinc-500 font-normal">
                ({tab === "all" ? rooms.length : rooms.filter(r => r.sync_status === tab).length})
              </span>
            </button>
          ))}
        </div>
      </div>

      {/* Room Table */}
      <div className="rounded-xl border border-zinc-800 bg-[#111111] overflow-hidden shadow-xl">
        <Table>
          <TableHeader className="bg-zinc-900/60 border-b border-zinc-800">
            <TableRow className="hover:bg-transparent border-zinc-800">
              <TableHead className="w-[300px] text-xs font-mono text-zinc-400">ROOM / HOST</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">PLAYBACK / TIMELINE</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">PEERS / TRACKS</TableHead>
              <TableHead className="text-xs font-mono text-zinc-400">SYNC & QOE SCORE</TableHead>
              <TableHead className="text-right text-xs font-mono text-zinc-400">INTERVENE</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="h-48 text-center text-zinc-500 font-mono text-xs">
                  Loading active Watch Party sessions from telemetry hub...
                </TableCell>
              </TableRow>
            ) : filteredRooms.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="h-48 text-center text-zinc-500 font-mono text-xs">
                  No active Watch Party rooms found matching filter criteria.
                </TableCell>
              </TableRow>
            ) : (
              filteredRooms.map(room => (
                <TableRow
                  key={room.room_id}
                  onClick={() => onSelectRoom(room)}
                  className="cursor-pointer hover:bg-zinc-900/40 border-zinc-800 transition-colors"
                >
                  <TableCell className="py-3">
                    <div className="flex items-center space-x-3">
                      <div className="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-cyan-400">
                        <Users className="h-5 w-5" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <p className="font-semibold text-sm text-white truncate hover:text-cyan-400 transition-colors">
                          {room.room_name}
                        </p>
                        <p className="text-xs font-mono text-zinc-500 truncate mt-0.5">
                          ID: {room.room_id} • Host: {room.owner_id}
                        </p>
                      </div>
                    </div>
                  </TableCell>

                  <TableCell>
                    <div className="space-y-1">
                      <div className="flex items-center space-x-2">
                        {room.is_playing ? (
                          <Badge variant="success" className="px-1.5 py-0 text-[10px] flex items-center">
                            <Play className="h-2.5 w-2.5 mr-1 fill-current" /> PLAYING
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="px-1.5 py-0 text-[10px] text-zinc-400 flex items-center">
                            <Pause className="h-2.5 w-2.5 mr-1 fill-current" /> PAUSED
                          </Badge>
                        )}
                        <span className="text-xs font-mono text-zinc-300 font-bold">
                          {formatTime(room.media_time_seconds)}
                        </span>
                      </div>
                      <p className="text-[10px] font-mono text-zinc-500 truncate max-w-[200px]">
                        {room.media_url ? room.media_url.split("/").pop() : "No Media Loaded"}
                      </p>
                    </div>
                  </TableCell>

                  <TableCell>
                    <div className="space-y-1">
                      <div className="flex items-center text-xs font-mono text-zinc-300">
                        <Users className="h-3 w-3 mr-1.5 text-cyan-400" />
                        <span className="font-bold text-white mr-1">{room.participant_count}</span> / {room.max_participants} Peers
                      </div>
                      <div className="flex items-center text-[11px] font-mono text-zinc-500">
                        <Radio className="h-3 w-3 mr-1.5 text-purple-400" />
                        {room.sfu_peers_count} SFU RTP Tracks
                      </div>
                    </div>
                  </TableCell>

                  <TableCell>
                    <div className="space-y-1.5 w-36">
                      <div className="flex items-center justify-between">
                        {getSyncBadge(room.sync_status)}
                        <span className={`text-xs font-mono font-bold ${room.qoe_score > 90 ? "text-emerald-400" : room.qoe_score > 75 ? "text-amber-400" : "text-rose-400"}`}>
                          {room.qoe_score}%
                        </span>
                      </div>
                      <Progress value={room.qoe_score} className="h-1.5" />
                    </div>
                  </TableCell>

                  <TableCell className="text-right">
                    <div className="flex items-center justify-end space-x-2">
                      <Button
                        variant="outline-cyan"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectRoom(room)
                        }}
                        className="h-8 px-2.5 text-xs font-mono"
                      >
                        <Eye className="h-3.5 w-3.5 mr-1.5" /> Inspect
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={terminatingId === room.room_id}
                        onClick={(e) => handleTerminate(e, room.room_id, room.room_name)}
                        className="h-8 w-8 p-0 text-zinc-500 hover:text-rose-400 hover:bg-rose-500/10"
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
