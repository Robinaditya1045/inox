-- Migration: Create index on room_members.room_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_room_members_room_id ON room_members(room_id);
