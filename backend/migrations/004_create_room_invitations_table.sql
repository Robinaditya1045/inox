-- Migration: Create room_invitations table for inviting users into private rooms
CREATE TABLE IF NOT EXISTS room_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(room_id, invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_room_invitations_invitee_id ON room_invitations(invitee_id, status);
CREATE INDEX IF NOT EXISTS idx_room_invitations_room_id ON room_invitations(room_id);
