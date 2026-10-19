-- Migration 016: Optional per-room join password.
-- The empty string means "no password", so the column is NOT NULL and every
-- existing room keeps its current behaviour until an owner sets one.
ALTER TABLE rooms
ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';
