-- Migration 006: Create persistent analytics events and room session tracking tables
CREATE TABLE IF NOT EXISTS analytics_events (
    id VARCHAR(64) PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    room_id UUID NULL REFERENCES rooms(id) ON DELETE SET NULL,
    user_id UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS room_sessions (
    id VARCHAR(64) PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP WITH TIME ZONE NULL,
    peak_participants INT DEFAULT 1,
    total_duration_seconds INT DEFAULT 0
);

-- Indexes for fast chronological query and filtering by room or event type
CREATE INDEX IF NOT EXISTS idx_analytics_events_room_created ON analytics_events(room_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_type_created ON analytics_events(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_room_sessions_room_started ON room_sessions(room_id, started_at DESC);
