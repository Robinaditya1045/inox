import * as React from "react"
import { apiFetch } from "@/lib/api"
import type { RoomInspectorDetail, AdminInterventionLog } from "@/types/rooms"
import type { RoomTelemetry } from "@/types/telemetry"

export interface UseRoomInspectorResult {
  rooms: RoomInspectorDetail[]
  isLoading: boolean
  isDemoMode: boolean
  error: string | null
  logs: AdminInterventionLog[]
  forceSyncPlayback: (roomId: string, targetTimeSec: number, isPlaying: boolean) => Promise<void>
  kickParticipant: (roomId: string, userId: string, username: string) => Promise<void>
  terminateRoom: (roomId: string, roomName: string) => Promise<void>
  refresh: () => void
  toggleDemoMode: () => void
}

/** Derives a sync classification from a room's QoE score. */
function syncStatusForQoE(qoeScore: number): RoomInspectorDetail["sync_status"] {
  if (qoeScore > 90) return "synchronized"
  if (qoeScore > 75) return "minor_drift"
  return "out_of_sync"
}

const MOCK_ROOM_DETAILS: RoomInspectorDetail[] = [
  {
    room_id: "room-cyber-101",
    room_name: "Cyberpunk 4K Premiere Watch Party",
    owner_id: "usr_robin_admin",
    created_at: new Date(Date.now() - 3600000 * 2).toISOString(),
    participant_count: 14,
    max_participants: 50,
    is_playing: true,
    media_url: "https://cdn.inox.stream/hls/cyberpunk_trailer/master.m3u8",
    media_time_seconds: 142.5,
    sfu_peers_count: 28,
    qoe_score: 96,
    sync_status: "synchronized",
    participants: [
      { user_id: "usr_robin_admin", username: "robin (Host)", role: "owner", joined_at: "2h ago", rtt_ms: 14, video_track: "active", audio_track: "active", codec: "VP8 + Opus", qoe_score: 99, sync_drift_ms: 0 },
      { user_id: "usr_alice_88", username: "alice_streamer", role: "moderator", joined_at: "1h 45m ago", rtt_ms: 28, video_track: "active", audio_track: "active", codec: "VP8 + Opus", qoe_score: 98, sync_drift_ms: -12 },
      { user_id: "usr_bob_dev", username: "bob_viewer", role: "member", joined_at: "45m ago", rtt_ms: 45, video_track: "active", audio_track: "muted", codec: "VP8 + Opus", qoe_score: 95, sync_drift_ms: +24 },
      { user_id: "usr_charlie_lag", username: "charlie_mobile", role: "member", joined_at: "10m ago", rtt_ms: 185, video_track: "reconnecting", audio_track: "active", codec: "H264 + Opus", qoe_score: 78, sync_drift_ms: -140 },
    ]
  },
  {
    room_id: "room-matrix-202",
    room_name: "The Matrix Remastered - Friday Night",
    owner_id: "usr_neo_matrix",
    created_at: new Date(Date.now() - 3600000 * 4).toISOString(),
    participant_count: 8,
    max_participants: 25,
    is_playing: true,
    media_url: "https://cdn.inox.stream/hls/matrix_lobby/master.m3u8",
    media_time_seconds: 210.8,
    sfu_peers_count: 16,
    qoe_score: 88,
    sync_status: "minor_drift",
    participants: [
      { user_id: "usr_neo_matrix", username: "neo (Host)", role: "owner", joined_at: "4h ago", rtt_ms: 22, video_track: "active", audio_track: "active", codec: "VP8 + Opus", qoe_score: 96, sync_drift_ms: 0 },
      { user_id: "usr_morpheus_01", username: "morpheus", role: "moderator", joined_at: "3h ago", rtt_ms: 35, video_track: "active", audio_track: "active", codec: "VP8 + Opus", qoe_score: 94, sync_drift_ms: +45 },
      { user_id: "usr_cypher_bad", username: "cypher_glitched", role: "member", joined_at: "20m ago", rtt_ms: 420, video_track: "failed", audio_track: "reconnecting", codec: "VP8 + Opus", qoe_score: 62, sync_drift_ms: +650 },
    ]
  },
  {
    room_id: "room-interstellar-303",
    room_name: "Interstellar IMAX 60FPS Test Lab",
    owner_id: "usr_cooper_pilot",
    created_at: new Date(Date.now() - 3600000 * 6).toISOString(),
    participant_count: 22,
    max_participants: 100,
    is_playing: false,
    media_url: "https://cdn.inox.stream/hls/interstellar/master.m3u8",
    media_time_seconds: 315.0,
    sfu_peers_count: 44,
    qoe_score: 92,
    sync_status: "synchronized",
    participants: [
      { user_id: "usr_cooper_pilot", username: "cooper (Host)", role: "owner", joined_at: "6h ago", rtt_ms: 18, video_track: "active", audio_track: "active", codec: "AV1 + Opus", qoe_score: 98, sync_drift_ms: 0 },
      { user_id: "usr_brand_doc", username: "brand_science", role: "moderator", joined_at: "5h ago", rtt_ms: 25, video_track: "active", audio_track: "active", codec: "AV1 + Opus", qoe_score: 97, sync_drift_ms: +8 },
    ]
  },
  {
    room_id: "room-stale-999",
    room_name: "Abandoned Test Session #999",
    owner_id: "usr_test_bot",
    created_at: new Date(Date.now() - 3600000 * 24).toISOString(),
    participant_count: 1,
    max_participants: 10,
    is_playing: false,
    media_url: "",
    media_time_seconds: 0,
    sfu_peers_count: 0,
    qoe_score: 55,
    sync_status: "out_of_sync",
    participants: [
      { user_id: "usr_test_bot", username: "bot_ghost", role: "owner", joined_at: "24h ago", rtt_ms: 850, video_track: "failed", audio_track: "failed", codec: "None", qoe_score: 55, sync_drift_ms: 0 },
    ]
  }
]

export function useRoomInspector(liveRooms?: RoomTelemetry[]): UseRoomInspectorResult {
  const [rooms, setRooms] = React.useState<RoomInspectorDetail[]>(MOCK_ROOM_DETAILS)
  const [isLoading] = React.useState<boolean>(false)
  const [isDemoMode, setIsDemoMode] = React.useState<boolean>(true)
  const [error, setError] = React.useState<string | null>(null)
  const [logs, setLogs] = React.useState<AdminInterventionLog[]>([
    {
      id: "log-init-1",
      timestamp: new Date(Date.now() - 120000).toLocaleTimeString(),
      room_id: "room-cyber-101",
      action: "FORCE_SYNC",
      details: "Broadcasted server-authoritative timestamp 140.0s across 14 peers (sync drift reconciled)."
    },
    {
      id: "log-init-2",
      timestamp: new Date(Date.now() - 300000).toLocaleTimeString(),
      room_id: "room-matrix-202",
      action: "KICK_PEER",
      target_user_id: "usr_agent_smith",
      details: "Evicted participant due to excessive SFU packet drop rate (>15%) and buffer starvation."
    }
  ])

  React.useEffect(() => {
    if (isDemoMode) return
    if (!liveRooms) return

    // Live telemetry carries no participant roster, room name, or owner, so
    // those are represented as unknown rather than borrowed from a mock room —
    // previously an unrecognized live room was padded with an arbitrary mock
    // entry, showing fabricated participants under a real room ID.
    const merged: RoomInspectorDetail[] = liveRooms.map(lr => ({
      ...lr,
      room_name: lr.room_id,
      owner_id: "—",
      created_at: new Date().toISOString(),
      max_participants: Math.max(lr.participant_count, 50),
      participants: [],
      sync_status: syncStatusForQoE(lr.qoe_score),
    }))

    // Assigning unconditionally (including the empty case) means "no rooms
    // live" clears the grid instead of leaving stale rows behind.
    setRooms(merged)
  }, [liveRooms, isDemoMode])

  const forceSyncPlayback = React.useCallback(async (roomId: string, targetTimeSec: number, isPlaying: boolean) => {
    const actionLog: AdminInterventionLog = {
      id: `log-${Date.now()}`,
      timestamp: new Date().toLocaleTimeString(),
      room_id: roomId,
      action: "FORCE_SYNC",
      details: `Broadcasted SYNC_PLAYBACK event: timestamp=${targetTimeSec.toFixed(1)}s, state=${isPlaying ? "PLAYING" : "PAUSED"}.`
    }

    if (isDemoMode) {
      setRooms(prev => prev.map(r => {
        if (r.room_id !== roomId) return r
        return {
          ...r,
          media_time_seconds: targetTimeSec,
          is_playing: isPlaying,
          qoe_score: Math.min(100, r.qoe_score + 5),
          sync_status: "synchronized",
          participants: r.participants.map(p => ({ ...p, sync_drift_ms: 0, video_track: "active" as const }))
        }
      }))
      setLogs(prev => [actionLog, ...prev])
      return
    }

    try {
      const res = await apiFetch(`/admin/rooms/${roomId}/sync`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ media_time_seconds: targetTimeSec, is_playing: isPlaying })
      })
      if (!res.ok) throw new Error(`Server returned ${res.status}`)
      setError(null)
      setLogs(prev => [actionLog, ...prev])
    } catch (err) {
      // Do not append a success entry to the audit log for an action the server
      // rejected — the log is meant to record what actually happened.
      console.error("Force sync failed:", err)
      setError(
        `Force sync failed for ${roomId}: ${err instanceof Error ? err.message : String(err)}. ` +
        `The backend exposes no admin playback-sync endpoint yet.`
      )
    }
  }, [isDemoMode])

  const kickParticipant = React.useCallback(async (roomId: string, userId: string, username: string) => {
    const actionLog: AdminInterventionLog = {
      id: `log-${Date.now()}`,
      timestamp: new Date().toLocaleTimeString(),
      room_id: roomId,
      action: "KICK_PEER",
      target_user_id: userId,
      details: `Evicted participant "${username}" (${userId}) from room workspace.`
    }

    if (isDemoMode) {
      setRooms(prev => prev.map(r => {
        if (r.room_id !== roomId) return r
        const updatedParticipants = r.participants.filter(p => p.user_id !== userId)
        return {
          ...r,
          participant_count: updatedParticipants.length,
          sfu_peers_count: Math.max(0, r.sfu_peers_count - 2),
          participants: updatedParticipants
        }
      }))
      setLogs(prev => [actionLog, ...prev])
      return
    }

    try {
      const res = await apiFetch(`/rooms/${roomId}/members/${userId}`, {
        method: "DELETE"
      })
      if (!res.ok) throw new Error(`Server returned ${res.status}`)

      setError(null)
      // Only drop the participant locally once the server has confirmed it;
      // removing them on failure made a rejected kick look like it worked.
      setRooms(prev => prev.map(r => {
        if (r.room_id !== roomId) return r
        const updatedParticipants = r.participants.filter(p => p.user_id !== userId)
        return {
          ...r,
          participant_count: Math.max(0, r.participant_count - 1),
          participants: updatedParticipants
        }
      }))
      setLogs(prev => [actionLog, ...prev])
    } catch (err) {
      console.error("Kick participant failed:", err)
      setError(`Failed to evict ${username}: ${err instanceof Error ? err.message : String(err)}`)
    }
  }, [isDemoMode])

  const terminateRoom = React.useCallback(async (roomId: string, roomName: string) => {
    const actionLog: AdminInterventionLog = {
      id: `log-${Date.now()}`,
      timestamp: new Date().toLocaleTimeString(),
      room_id: roomId,
      action: "TERMINATE_ROOM",
      details: `Forcefully terminated Watch Party session "${roomName}" and purged SFU media tracks.`
    }

    if (isDemoMode) {
      setRooms(prev => prev.filter(r => r.room_id !== roomId))
      setLogs(prev => [actionLog, ...prev])
      return
    }

    try {
      // The backend has no /admin/rooms route; room deletion is the owner-scoped
      // DELETE /rooms/{id} endpoint registered in backend/internal/api/router.go.
      const res = await apiFetch(`/rooms/${roomId}`, { method: "DELETE" })
      if (!res.ok) throw new Error(`Server returned ${res.status}`)
      setError(null)
      setRooms(prev => prev.filter(r => r.room_id !== roomId))
      setLogs(prev => [actionLog, ...prev])
    } catch (err) {
      console.error("Terminate room failed:", err)
      setError(`Failed to terminate "${roomName}": ${err instanceof Error ? err.message : String(err)}`)
    }
  }, [isDemoMode])

  const toggleDemoMode = React.useCallback(() => {
    const next = !isDemoMode
    setIsDemoMode(next)
    // Returning to demo mode restores the sample rooms; leaving it hands control
    // back to the live telemetry effect.
    if (next) setRooms([...MOCK_ROOM_DETAILS])
    setError(null)
  }, [isDemoMode])

  const refresh = React.useCallback(() => {
    // Only demo mode is refreshed from the mock fixtures; in live mode the rooms
    // come from the telemetry stream, and overwriting them with mock data made
    // "Refresh Telemetry" replace real rooms with fake ones.
    if (isDemoMode) {
      setRooms([...MOCK_ROOM_DETAILS])
    }
    setError(null)
  }, [isDemoMode])

  return {
    rooms,
    isLoading,
    isDemoMode,
    error,
    logs,
    forceSyncPlayback,
    kickParticipant,
    terminateRoom,
    refresh,
    toggleDemoMode,
  }
}
