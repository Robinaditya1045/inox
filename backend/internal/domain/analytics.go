package domain

import "time"

// AnalyticsEvent represents an historical milestone or diagnostic trace in the system.
type AnalyticsEvent struct {
	ID        string         `json:"id"`
	EventType string         `json:"event_type"`
	RoomID    *string        `json:"room_id"`
	UserID    *string        `json:"user_id"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
}

// RoomSession represents a tracked lifecycle period of an active watch party room.
type RoomSession struct {
	ID                   string     `json:"id"`
	RoomID               string     `json:"room_id"`
	StartedAt            time.Time  `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at"`
	PeakParticipants     int        `json:"peak_participants"`
	TotalDurationSeconds int        `json:"total_duration_seconds"`
}
