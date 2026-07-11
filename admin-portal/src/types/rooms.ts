import type { RoomTelemetry } from "./telemetry"

export type ParticipantRole = "owner" | "moderator" | "member"
export type TrackStatus = "active" | "muted" | "reconnecting" | "failed"

export interface ParticipantState {
  user_id: string
  username: string
  role: ParticipantRole
  joined_at: string
  rtt_ms: number
  video_track: TrackStatus
  audio_track: TrackStatus
  codec: string
  qoe_score: number
  sync_drift_ms: number
}

export interface RoomInspectorDetail extends RoomTelemetry {
  room_name: string
  owner_id: string
  created_at: string
  max_participants: number
  participants: ParticipantState[]
  sync_status: "synchronized" | "minor_drift" | "out_of_sync"
}

export interface AdminInterventionLog {
  id: string
  timestamp: string
  room_id: string
  action: "FORCE_SYNC" | "KICK_PEER" | "TERMINATE_ROOM" | "PAUSE_ALL"
  target_user_id?: string
  details: string
}
